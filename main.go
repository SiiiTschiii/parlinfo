package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/siiitschiii/parlinfo/pkg/config"
	"github.com/siiitschiii/parlinfo/pkg/contacts"
	"github.com/siiitschiii/parlinfo/pkg/igapi"
	"github.com/siiitschiii/parlinfo/pkg/votelog"
	"github.com/siiitschiii/parlinfo/pkg/voteposting"
	"github.com/siiitschiii/parlinfo/pkg/voteposting/platforms"
	"github.com/siiitschiii/parlinfo/pkg/voteposting/platforms/bluesky"
	"github.com/siiitschiii/parlinfo/pkg/voteposting/platforms/facebook"
	"github.com/siiitschiii/parlinfo/pkg/voteposting/platforms/instagram"
	"github.com/siiitschiii/parlinfo/pkg/voteposting/platforms/x"
	"github.com/siiitschiii/parlinfo/pkg/votes"
)

// channelPlatform pairs a constructed platform with the vote-log key it uses.
//
// The instance is built once per channel and shared across that channel's
// jurisdictions. The per-run post budget is a counter on the instance, so
// rebuilding it per jurisdiction would reset the counter and silently multiply
// what the account posts in an hour.
type channelPlatform struct {
	displayName string
	logPlatform votelog.Platform
	poster      platforms.Platform
	// postFrom, when set, is the first sitting date this platform posts. A
	// platform that goes live after the others has no history of its own, and
	// without this it would work through whatever the source still returns.
	postFrom time.Time
}

func main() {
	if err := config.Validate(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	maxVotesToCheck := getEnvInt("MAX_VOTES_TO_CHECK", 50)
	skipVoteLog := os.Getenv("SKIP_VOTE_LOG") == "true"

	fmt.Printf("Configuration: Check last %d votes\n", maxVotesToCheck)

	hasErrors := false
	anyPlatformConfigured := false
	anyJurisdictionEnabled := false

	channels, err := selectedChannels(os.Getenv("CHANNEL"))
	if err != nil {
		log.Fatalf("Invalid CHANNEL: %v", err)
	}

	for _, channel := range channels {
		// Only jurisdictions explicitly cleared to post. A newly registered
		// body ships disabled: its vote log has to be seeded and its first
		// posts reviewed before it runs unattended, and merging code should not
		// substitute for either.
		jurisdictions, err := channel.EnabledJurisdictions()
		if err != nil {
			log.Fatalf("Error resolving channel %q: %v", channel.Key, err)
		}
		if len(jurisdictions) == 0 {
			log.Printf("⚠️  Channel %q: no enabled jurisdictions, skipping", channel.Key)
			continue
		}
		anyJurisdictionEnabled = true

		plats := buildPlatforms(channel, jurisdictions)
		if len(plats) == 0 {
			log.Printf("⚠️  Channel %q: no platform credentials configured, skipping", channel.Key)
			continue
		}
		anyPlatformConfigured = true

		for _, p := range plats {
			if runChannelPlatform(channel, jurisdictions, p, skipVoteLog, maxVotesToCheck) {
				hasErrors = true
			}
		}
	}

	// Distinguish the two ways a run can end up with nothing to do. They have
	// different causes and different fixes, and reporting the wrong one sends
	// whoever is debugging into the credentials when the switch is the problem.
	if !anyJurisdictionEnabled {
		log.Fatalf("No jurisdiction is enabled. Nothing posts unless its switch is set: %s. Known jurisdictions: %v.",
			"JURISDICTION_<JURISDICTION>_ENABLED=true", config.JurisdictionKeys())
	}
	if !anyPlatformConfigured {
		log.Fatal("No platform credentials configured for any channel. Set <CHANNEL>_X_API_KEY/<CHANNEL>_X_API_SECRET/<CHANNEL>_X_ACCESS_TOKEN/<CHANNEL>_X_ACCESS_SECRET for X, <CHANNEL>_BLUESKY_HANDLE/<CHANNEL>_BLUESKY_PASSWORD for Bluesky, <CHANNEL>_IG_USER_ID/<CHANNEL>_IG_ACCESS_TOKEN/<CHANNEL>_IMAGE_HOST_REPO/<CHANNEL>_IMAGE_HOST_TOKEN for Instagram, or <CHANNEL>_FB_PAGE_ID/<CHANNEL>_FB_ACCESS_TOKEN/<CHANNEL>_FB_POST_FROM for Facebook.")
	}

	if hasErrors {
		log.Println("❌ Action failed: one or more platforms encountered errors. Check logs above.")
		os.Exit(1)
	}
}

// selectedChannels returns the channel named by CHANNEL, or every channel when
// it is unset.
//
// Each channel's accounts are posted by a workflow of their own, so that one
// account's schedule, failures and pauses never touch another's. Without this
// every workflow would also walk the other channels and depend on their
// switches being off in its environment to leave them alone.
func selectedChannels(key string) ([]config.Channel, error) {
	if key == "" {
		return config.Channels(), nil
	}
	c, err := config.LookupChannel(key)
	if err != nil {
		return nil, fmt.Errorf("%w; known channels: %v", err, config.ChannelKeys())
	}
	return []config.Channel{c}, nil
}

// buildPlatforms constructs one instance per platform for a channel, using that
// channel's credentials. Platforms without complete credentials are skipped.
func buildPlatforms(channel config.Channel, jurisdictions []config.Jurisdiction) []channelPlatform {
	// Tagging is configured per jurisdiction but a platform instance is per
	// channel, so its mapper has to cover every jurisdiction the channel serves.
	contactMapper := loadContacts(jurisdictions)

	var plats []channelPlatform

	apiKey := channel.Env("X_API_KEY")
	apiSecret := channel.Env("X_API_SECRET")
	accessToken := channel.Env("X_ACCESS_TOKEN")
	accessSecret := channel.Env("X_ACCESS_SECRET")
	if apiKey != "" && apiSecret != "" && accessToken != "" && accessSecret != "" {
		xPlatform := x.NewXPlatform(
			apiKey, apiSecret, accessToken, accessSecret,
			contactMapper,
			channel.EnvInt("X_MAX_POSTS_PER_RUN", 10),
		)
		xPlatform.SetMaxChars(channel.EnvInt("X_MAX_CHARS", x.DefaultMaxChars))
		plats = append(plats, channelPlatform{displayName: "X/Twitter", logPlatform: votelog.PlatformX, poster: xPlatform})
	} else {
		log.Printf("⚠️  Channel %q: X/Twitter not configured (missing X_API_KEY/X_API_SECRET/X_ACCESS_TOKEN/X_ACCESS_SECRET)", channel.Key)
	}

	bskyHandle := channel.Env("BLUESKY_HANDLE")
	bskyPassword := channel.Env("BLUESKY_PASSWORD")
	if bskyHandle != "" && bskyPassword != "" {
		plats = append(plats, channelPlatform{
			displayName: "Bluesky",
			logPlatform: votelog.PlatformBluesky,
			poster: bluesky.NewBlueskyPlatform(
				bskyHandle, bskyPassword,
				channel.EnvInt("BLUESKY_MAX_POSTS_PER_RUN", 10),
				contactMapper,
			),
		})
	} else {
		log.Printf("⚠️  Channel %q: Bluesky not configured (missing BLUESKY_HANDLE/BLUESKY_PASSWORD)", channel.Key)
	}

	igUserID := channel.Env("IG_USER_ID")
	igAccessToken := channel.Env("IG_ACCESS_TOKEN")
	// Instagram fetches carousel images by URL, so they are published to a
	// GitHub Pages branch first. The repo and its write token are named for
	// that job, not for Instagram: IMAGE_HOST_REPO is "owner/name".
	imageHostRepo := channel.Env("IMAGE_HOST_REPO")
	imageHostToken := channel.Env("IMAGE_HOST_TOKEN")
	if igUserID != "" && igAccessToken != "" && imageHostToken != "" && imageHostRepo != "" {
		// Set but unusable is a configuration error, not a reason to fall back
		// to no Instagram: that would post the other platforms and leave a
		// green run behind.
		hostOwner, hostName, err := igapi.ParseRepo(imageHostRepo)
		if err != nil {
			log.Fatalf("Channel %q: %sIMAGE_HOST_REPO: %v", channel.Key, channel.EnvPrefix(), err)
		}
		igPlatform := instagram.NewInstagramPlatformWithCredentials(
			igUserID, igAccessToken, imageHostToken, hostOwner, hostName,
			channel.EnvInt("IG_MAX_POSTS_PER_RUN", 5),
		)
		igPlatform.SetContactMapper(contactMapper)
		plats = append(plats, channelPlatform{displayName: "Instagram", logPlatform: votelog.PlatformInstagram, poster: igPlatform})
	} else {
		log.Printf("⚠️  Channel %q: Instagram not configured (missing IG_USER_ID/IG_ACCESS_TOKEN/IMAGE_HOST_REPO/IMAGE_HOST_TOKEN)", channel.Key)
	}

	// The Page token is the one Instagram already uses, once it carries
	// pages_manage_posts — see pkg/fbapi/README.md. It is read under its own
	// name all the same, so the Page ID alone decides whether Facebook posts.
	fbPageID := channel.Env("FB_PAGE_ID")
	fbAccessToken := channel.Env("FB_ACCESS_TOKEN")
	if fbPageID != "" && fbAccessToken != "" {
		// Facebook goes live without a vote log, and a vote log must never be
		// seeded to make up for that: it would record posts that were never
		// made. The start date does the same job honestly. It is required, so
		// that dropping the variable cannot silently re-open the backlog.
		postFrom, err := config.ParseDate(channel.Env("FB_POST_FROM"))
		if err != nil || postFrom.IsZero() {
			log.Fatalf("Channel %q: %sFB_POST_FROM must be the first sitting date Facebook posts, as YYYY-MM-DD: %v",
				channel.Key, channel.EnvPrefix(), err)
		}
		plats = append(plats, channelPlatform{
			displayName: "Facebook",
			logPlatform: votelog.PlatformFacebook,
			poster: facebook.NewFacebookPlatformWithCredentials(
				fbPageID, fbAccessToken,
				channel.EnvInt("FB_MAX_POSTS_PER_RUN", 5),
			),
			postFrom: postFrom,
		})
	} else {
		log.Printf("⚠️  Channel %q: Facebook not configured (missing FB_PAGE_ID/FB_ACCESS_TOKEN)", channel.Key)
	}

	return plats
}

// loadContacts merges every configured jurisdiction's contacts file. A missing
// or empty file is normal — a jurisdiction ships before its contacts are
// curated — so this degrades to no tagging rather than failing the run.
func loadContacts(jurisdictions []config.Jurisdiction) *contacts.Mapper {
	mapper, err := contacts.LoadContactFiles(contacts.PathsFor(config.ContactsKeys(jurisdictions)...)...)
	if err != nil {
		log.Printf("Warning: Could not load contacts for tagging: %v", err)
		return nil
	}
	return mapper
}

// runChannelPlatform posts one channel's jurisdictions to one platform,
// sharing a single per-run budget across them. It returns true if anything
// went wrong, including a jurisdiction whose votes could not be fetched.
func runChannelPlatform(
	channel config.Channel,
	jurisdictions []config.Jurisdiction,
	p channelPlatform,
	skipVoteLog bool,
	maxVotesToCheck int,
) bool {
	fmt.Printf("\n━━━ %s / %s ━━━\n", channel.Key, p.displayName)

	logs := make(voteposting.VoteLogs, len(jurisdictions))
	var perJurisdiction [][][]votes.Vote
	failed := false

	for _, j := range jurisdictions {
		var vl *votelog.VoteLog
		if skipVoteLog {
			vl = votelog.NewNoOp(j.Key, p.logPlatform)
			fmt.Printf("⚠️  SKIP_VOTE_LOG=true — treating all %s votes as unposted, not saving vote log\n", j.Key)
		} else {
			var err error
			vl, err = votelog.Load(j.Key, p.logPlatform)
			if err != nil {
				log.Fatalf("Error loading %s/%s vote log: %v", j.Key, p.displayName, err)
			}
			fmt.Printf("Loaded %s/%s vote log: %d votes already posted\n", j.Key, p.displayName, vl.Count())

		}
		logs[j.Key] = vl

		groups, err := voteposting.PrepareVoteGroups(j.NewSource(), vl, maxVotesToCheck, j.MaxAgeDays)
		if err != nil {
			// One body's source being down must not silence the others: they
			// share a channel, not a data source. Skip it for this run and let
			// the exit code carry the failure, so the outage is still visible.
			log.Printf("❌ Error preparing %s votes for %s, skipping %s this run: %v", j.Key, p.displayName, j.Key, err)
			failed = true
			continue
		}
		if before := len(groups); !p.postFrom.IsZero() && before > 0 {
			groups = voteposting.DropBefore(groups, p.postFrom)
			if dropped := before - len(groups); dropped > 0 {
				fmt.Printf("Skipping %d group(s) from %s sitting before %s\n", dropped, j.Key, p.postFrom.Format("2006-01-02"))
			}
		}
		if len(groups) > 0 {
			fmt.Printf("Found %d group(s) from %s\n", len(groups), j.Key)
		}
		perJurisdiction = append(perJurisdiction, groups)
	}

	merged := voteposting.MergeOldestFirst(perJurisdiction...)
	if len(merged) == 0 {
		fmt.Printf("No new votes to post on %s!\n", p.displayName)
		return failed
	}

	posted, err := voteposting.PostToPlatform(merged, p.poster, logs, false)
	if err != nil {
		// A vote the pipeline refused to publish does not undo the ones it
		// did: say what went out before reporting the failure, or the log
		// reads as though the run posted nothing. A genuine posting failure
		// gets no such line — there "some skipped" would be a lie.
		if voteposting.IsRejectedVoteError(err) && posted > 0 {
			fmt.Printf("Posted %d group(s) to %s (some skipped — see warnings above)\n", posted, p.displayName)
		}
		log.Printf("❌ Error posting to %s: %v", p.displayName, err)
		return true
	}

	fmt.Printf("🎉 Posted %d new group(s) to %s!\n", posted, p.displayName)
	return failed
}

// getEnvInt gets an integer from environment variable with a default value
func getEnvInt(key string, defaultValue int) int {
	if val := os.Getenv(key); val != "" {
		if intVal, err := strconv.Atoi(val); err == nil {
			return intVal
		}
	}
	return defaultValue
}
