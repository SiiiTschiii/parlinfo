// check_unposted is a dry-run mirror of main.go for local debugging.
// It reads the actual vote logs, respects MAX_VOTES_TO_CHECK and MAX_POSTS_PER_RUN,
// and prints what would be posted without making any API calls.
//
// Usage:
//
//	go run ./cmd/check_unposted [-platform x|bluesky|facebook] [-jurisdiction KEY] [-n N] [-max-posts M]
//
// Flags override the corresponding environment variables.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/siiitschiii/parlinfo/pkg/config"
	"github.com/siiitschiii/parlinfo/pkg/contacts"
	"github.com/siiitschiii/parlinfo/pkg/votelog"
	"github.com/siiitschiii/parlinfo/pkg/voteposting"
	"github.com/siiitschiii/parlinfo/pkg/voteposting/platforms"
	"github.com/siiitschiii/parlinfo/pkg/voteposting/platforms/bluesky"
	"github.com/siiitschiii/parlinfo/pkg/voteposting/platforms/facebook"
	"github.com/siiitschiii/parlinfo/pkg/voteposting/platforms/x"
	"github.com/siiitschiii/parlinfo/pkg/votes"
)

func main() {
	platformFlag := flag.String("platform", "", "platform to check: x, bluesky, facebook (default: all)")
	jurisdictionFlag := flag.String("jurisdiction", "", "limit to one jurisdiction (default: every jurisdiction on the channel)")
	nFlag := flag.Int("n", 0, "override MAX_VOTES_TO_CHECK")
	maxPostsFlag := flag.Int("max-posts", 0, "override MAX_POSTS_PER_RUN for the chosen platform")
	flag.Parse()

	showX := *platformFlag == "" || strings.EqualFold(*platformFlag, "x")
	showBluesky := *platformFlag == "" || strings.EqualFold(*platformFlag, "bluesky") || strings.EqualFold(*platformFlag, "bsky")
	showFacebook := *platformFlag == "" || strings.EqualFold(*platformFlag, "facebook") || strings.EqualFold(*platformFlag, "fb")
	if !showX && !showBluesky && !showFacebook {
		log.Fatalf("Unknown platform %q. Use: x, bluesky, facebook", *platformFlag)
	}

	if err := config.Validate(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	maxVotesToCheck := getEnvInt("MAX_VOTES_TO_CHECK", 50)
	if *nFlag > 0 {
		maxVotesToCheck = *nFlag
	}

	fmt.Printf("Configuration: check last %d votes", maxVotesToCheck)
	if *platformFlag != "" {
		fmt.Printf(", platform: %s", *platformFlag)
	}
	fmt.Println()

	for _, channel := range config.Channels() {
		jurisdictions, err := channel.ResolveJurisdictions()
		if err != nil {
			log.Fatalf("Error resolving channel %q: %v", channel.Key, err)
		}
		if *jurisdictionFlag != "" {
			jurisdictions = filterJurisdictions(jurisdictions, *jurisdictionFlag)
			if len(jurisdictions) == 0 {
				continue
			}
		}

		contactMapper := loadContacts(jurisdictions)

		if showX {
			maxPosts := pick(*maxPostsFlag, channel.EnvInt("X_MAX_POSTS_PER_RUN", 10))
			check(channel.Key, "X/Twitter", votelog.PlatformX, jurisdictions, maxVotesToCheck, maxPosts,
				time.Time{}, x.NewXPlatform("", "", "", "", contactMapper, maxPosts))
		}

		if showBluesky {
			maxPosts := pick(*maxPostsFlag, channel.EnvInt("BLUESKY_MAX_POSTS_PER_RUN", 10))
			check(channel.Key, "Bluesky", votelog.PlatformBluesky, jurisdictions, maxVotesToCheck, maxPosts,
				time.Time{}, bluesky.NewBlueskyPlatform("", "", maxPosts, contactMapper))
		}

		if showFacebook {
			maxPosts := pick(*maxPostsFlag, channel.EnvInt("FB_MAX_POSTS_PER_RUN", 5))
			// Optional here, unlike in main.go: a dry run without it shows what
			// Facebook would post with no start date, which is the question
			// someone setting it up is asking.
			var postFrom time.Time
			if raw := channel.Env("FB_POST_FROM"); raw != "" {
				// A mistyped date is not "no date": it would show the unlimited
				// backlog while the bot itself refuses to start.
				var err error
				if postFrom, err = config.ParseDate(raw); err != nil {
					log.Fatalf("Channel %q: %sFB_POST_FROM: %v", channel.Key, channel.EnvPrefix(), err)
				}
			}
			check(channel.Key, "Facebook", votelog.PlatformFacebook, jurisdictions, maxVotesToCheck, maxPosts,
				postFrom, facebook.NewFacebookPlatform(maxPosts))
		}
	}
}

// check mirrors main.go's per-channel-platform run: one platform instance, one
// shared budget, jurisdictions merged oldest-first. Keeping the shapes identical
// is the point — a dry run that batched differently would not predict anything.
func check(
	channelKey, displayName string,
	logPlatform votelog.Platform,
	jurisdictions []config.Jurisdiction,
	maxVotesToCheck, maxPosts int,
	postFrom time.Time,
	poster platforms.Platform,
) {
	fmt.Printf("\n━━━ %s / %s ━━━\n", channelKey, displayName)

	logs := make(voteposting.VoteLogs, len(jurisdictions))
	var perJurisdiction [][][]votes.Vote

	for _, j := range jurisdictions {
		voteLog, err := votelog.Load(j.Key, logPlatform)
		if err != nil {
			log.Fatalf("Error loading %s/%s vote log: %v", j.Key, displayName, err)
		}
		fmt.Printf("Loaded %s/%s vote log: %d votes already posted\n", j.Key, displayName, voteLog.Count())
		logs[j.Key] = voteLog

		source, err := j.Source()
		if err != nil {
			log.Fatal(err)
		}
		groups, err := voteposting.PrepareVoteGroups(source, voteLog, maxVotesToCheck, j.MaxAgeDays)
		if err != nil {
			log.Fatalf("Error preparing %s votes for %s: %v", j.Key, displayName, err)
		}
		perJurisdiction = append(perJurisdiction, voteposting.DropBefore(groups, postFrom))
	}

	merged := voteposting.MergeOldestFirst(perJurisdiction...)
	if len(merged) == 0 {
		fmt.Printf("✨ No new votes to post on %s!\n", displayName)
		return
	}

	fmt.Printf("Found %d group(s) — would post up to %d per run\n\n", len(merged), maxPosts)
	if _, err := voteposting.PostToPlatform(merged, poster, logs, true); err != nil {
		log.Printf("Error: %v", err)
	}
}

func loadContacts(jurisdictions []config.Jurisdiction) *contacts.Mapper {
	keys := make([]string, 0, len(jurisdictions))
	for _, j := range jurisdictions {
		keys = append(keys, j.Key)
	}
	mapper, err := contacts.LoadContactFiles(contacts.PathsFor(keys...)...)
	if err != nil {
		log.Printf("Warning: Could not load contacts for tagging: %v", err)
		return nil
	}
	return mapper
}

func filterJurisdictions(js []config.Jurisdiction, key string) []config.Jurisdiction {
	var out []config.Jurisdiction
	for _, j := range js {
		if j.Key == key {
			out = append(out, j)
		}
	}
	return out
}

func pick(override, fallback int) int {
	if override > 0 {
		return override
	}
	return fallback
}

func getEnvInt(key string, defaultValue int) int {
	if val := os.Getenv(key); val != "" {
		if intVal, err := strconv.Atoi(val); err == nil {
			return intVal
		}
	}
	return defaultValue
}
