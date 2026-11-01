package main

import (
	"flag"
	"fmt"
	"log"
	"path/filepath"
	"sort"

	"github.com/siiitschiii/parlinfo/pkg/config"
	"github.com/siiitschiii/parlinfo/pkg/contacts"
	"github.com/siiitschiii/parlinfo/pkg/igapi"
	"github.com/siiitschiii/parlinfo/pkg/voteposting"
	"github.com/siiitschiii/parlinfo/pkg/voteposting/platforms"
	"github.com/siiitschiii/parlinfo/pkg/voteposting/platforms/bluesky"
	"github.com/siiitschiii/parlinfo/pkg/voteposting/platforms/facebook"
	"github.com/siiitschiii/parlinfo/pkg/voteposting/platforms/instagram"
	"github.com/siiitschiii/parlinfo/pkg/voteposting/platforms/x"
	"github.com/siiitschiii/parlinfo/pkg/voteposting/testfixtures"
	"github.com/siiitschiii/parlinfo/pkg/votes"
)

type namedPlatform struct {
	name string
	plat platforms.Platform
}

// platformCredentials holds all platform credentials loaded from env.
type platformCredentials struct {
	xAPIKey       string
	xAPISecret    string
	xAccessToken  string
	xAccessSecret string
	xEnabled      bool

	bskyHandle   string
	bskyPassword string
	bskyEnabled  bool

	igUserID      string
	igAccessToken string
	// The GitHub repo whose gh-pages branch serves the carousel images, and a
	// write token for it. Not Instagram credentials — see main.go.
	imageHostOwner string
	imageHostName  string
	imageHostToken string
	igEnabled      bool

	fbPageID      string
	fbAccessToken string
	fbEnabled     bool
}

func main() {
	fixture := flag.String("fixture", "all", "fixture name from AllFixtures() keys, or 'all'")
	platform := flag.String("platform", "all", "platform to post to: x, bluesky, instagram, facebook, or all")
	contactsFile := flag.String("contacts", filepath.Join("data", "contacts_test.yaml"), "contacts YAML file (default: test contacts with fake handles)")
	maxChars := flag.Int("x-max-chars", x.DefaultMaxChars, "per-post character limit for X (280 for free accounts, 2000 for Premium)")
	channelKey := flag.String("channel", config.DefaultChannelKey, fmt.Sprintf("channel whose credentials to use %v", config.ChannelKeys()))
	jurisdictionKey := flag.String("jurisdiction", "", fmt.Sprintf("post live votes for this jurisdiction instead of fixtures %v", config.JurisdictionKeys()))
	fetchLimit := flag.Int("fetch", 50, "with -jurisdiction: number of individual votes to fetch from the source")
	numGroups := flag.Int("n", 1, "with -jurisdiction: number of vote groups to post, newest first")
	minVotes := flag.Int("min-votes", 1, "with -jurisdiction: only post groups with at least this many votes")
	flag.Parse()

	channel, err := config.LookupChannel(*channelKey)
	if err != nil {
		log.Fatal(err)
	}

	creds := loadCredentials(channel)
	validatePlatform(*platform, creds, channel)

	// Load contacts for tagging
	contactMapper, err := contacts.LoadContacts(*contactsFile)
	if err != nil {
		log.Printf("Warning: Could not load contacts for tagging: %v", err)
		contactMapper = nil
	}

	// Live votes go through the same channel's credentials as the fixtures, so
	// any parliament can be tried on the test accounts — the Nationalrat, say,
	// without a CHPARL test account of its own.
	var fixtures map[string][]votes.Vote
	if *jurisdictionKey != "" {
		fixtures = liveFixtures(*jurisdictionKey, *fetchLimit, *numGroups, *minVotes)
	} else {
		fixtures = resolveFixtures(*fixture)
	}
	plats := buildPlatforms(*platform, creds, contactMapper, *maxChars)

	// Post each fixture to each platform
	fixtureNames := make([]string, 0, len(fixtures))
	for k := range fixtures {
		fixtureNames = append(fixtureNames, k)
	}
	sort.Strings(fixtureNames)

	for _, name := range fixtureNames {
		group := fixtures[name]
		for _, p := range plats {
			fmt.Printf("\n━━━ %s / %s ━━━\n", p.name, name)

			content, err := p.plat.Format(group)
			if err != nil {
				log.Printf("Format error (%s / %s): %v", p.name, name, err)
				continue
			}

			fmt.Printf("Preview:\n%s\n\n", content.String())

			_, _, err = p.plat.Post(content)
			if err != nil {
				log.Printf("Post error (%s / %s): %v", p.name, name, err)
				continue
			}

			fmt.Printf("Posted %s / %s successfully\n", p.name, name)
		}
	}
}

// loadCredentials reads platform credentials for a channel, using the same
// channel-scoped names the scheduled run reads: ZURICH_X_API_KEY and so on.
func loadCredentials(channel config.Channel) platformCredentials {
	c := platformCredentials{
		xAPIKey:        channel.Env("X_API_KEY"),
		xAPISecret:     channel.Env("X_API_SECRET"),
		xAccessToken:   channel.Env("X_ACCESS_TOKEN"),
		xAccessSecret:  channel.Env("X_ACCESS_SECRET"),
		bskyHandle:     channel.Env("BLUESKY_HANDLE"),
		bskyPassword:   channel.Env("BLUESKY_PASSWORD"),
		igUserID:       channel.Env("IG_USER_ID"),
		igAccessToken:  channel.Env("IG_ACCESS_TOKEN"),
		imageHostToken: channel.Env("IMAGE_HOST_TOKEN"),
		fbPageID:       channel.Env("FB_PAGE_ID"),
		fbAccessToken:  channel.Env("FB_ACCESS_TOKEN"),
	}
	c.xEnabled = c.xAPIKey != "" && c.xAPISecret != "" && c.xAccessToken != "" && c.xAccessSecret != ""
	c.bskyEnabled = c.bskyHandle != "" && c.bskyPassword != ""
	c.fbEnabled = c.fbPageID != "" && c.fbAccessToken != ""

	if repo := channel.Env("IMAGE_HOST_REPO"); repo != "" {
		owner, name, err := igapi.ParseRepo(repo)
		if err != nil {
			log.Fatalf("%sIMAGE_HOST_REPO: %v", channel.EnvPrefix(), err)
		}
		c.imageHostOwner, c.imageHostName = owner, name
	}
	c.igEnabled = c.igUserID != "" && c.igAccessToken != "" && c.imageHostToken != "" && c.imageHostOwner != ""
	return c
}

// validatePlatform checks that the selected platform has valid credentials configured.
func validatePlatform(platform string, creds platformCredentials, channel config.Channel) {
	prefix := channel.EnvPrefix()
	if !creds.xEnabled && !creds.bskyEnabled && !creds.fbEnabled && platform != "instagram" && platform != "facebook" {
		log.Fatalf("No platform credentials configured for channel %q. Set %[2]sX_API_KEY/%[2]sX_API_SECRET/%[2]sX_ACCESS_TOKEN/%[2]sX_ACCESS_SECRET for X, or %[2]sBLUESKY_HANDLE/%[2]sBLUESKY_PASSWORD for Bluesky. Instagram (stub mode without %[2]sIG_USER_ID/%[2]sIG_ACCESS_TOKEN/%[2]sIMAGE_HOST_REPO/%[2]sIMAGE_HOST_TOKEN, or real mode with them) is available via -platform instagram, and Facebook (stub mode without %[2]sFB_PAGE_ID/%[2]sFB_ACCESS_TOKEN, real mode with them) via -platform facebook.", channel.Key, prefix)
	}

	if platform == "x" && !creds.xEnabled {
		log.Fatal("X credentials required but not set")
	}
	if platform == "bluesky" && !creds.bskyEnabled {
		log.Fatal("Bluesky credentials required but not set")
	}
	if platform != "all" && platform != "x" && platform != "bluesky" && platform != "instagram" && platform != "facebook" {
		log.Fatalf("Unknown platform %q — use x, bluesky, instagram, facebook, or all", platform)
	}
}

// liveFixtures fetches the newest live vote groups a jurisdiction would post,
// keyed by sitting date and business number. Groups with fewer than minVotes
// votes are skipped, so a multi-vote thread can be found among the singles.
func liveFixtures(jurisdictionKey string, fetchLimit, numGroups, minVotes int) map[string][]votes.Vote {
	jurisdiction, err := config.LookupJurisdiction(jurisdictionKey)
	if err != nil {
		log.Fatalf("%v. Available: %v", err, config.JurisdictionKeys())
	}
	source, err := jurisdiction.Source()
	if err != nil {
		log.Fatal(err)
	}
	groups, err := voteposting.LatestGroups(source, jurisdiction.Key, fetchLimit, 0)
	if err != nil {
		log.Fatalf("Error preparing %s votes: %v", jurisdiction.Key, err)
	}

	out := map[string][]votes.Vote{}
	for _, g := range groups {
		if len(g) < minVotes {
			continue
		}
		out[fmt.Sprintf("%s %s %s", jurisdiction.Key, g[0].DateString(), g[0].Affair.Number)] = g
		if len(out) == numGroups {
			break
		}
	}
	if len(out) == 0 {
		log.Fatalf("No %s vote group with at least %d vote(s) among the latest %d votes; raise -fetch", jurisdiction.Key, minVotes, fetchLimit)
	}
	return out
}

// resolveFixtures loads the requested fixture(s) from the test fixture map.
func resolveFixtures(fixture string) map[string][]votes.Vote {
	if fixture == "instagram-long-multi-vote-truncation" {
		return map[string][]votes.Vote{
			fixture: testfixtures.InstagramLongMultiVoteTruncation(),
		}
	}

	allFixtures := testfixtures.AllFixtures()

	if fixture == "all" {
		return allFixtures
	}

	group, ok := allFixtures[fixture]
	if !ok {
		var names []string
		for k := range allFixtures {
			names = append(names, k)
		}
		names = append(names, "instagram-long-multi-vote-truncation")
		sort.Strings(names)
		log.Fatalf("Unknown fixture %q. Available: %v", fixture, names)
	}
	return map[string][]votes.Vote{fixture: group}
}

// buildPlatforms constructs the list of platforms to post to based on flags and credentials.
func buildPlatforms(platform string, creds platformCredentials, contactMapper *contacts.Mapper, maxChars int) []namedPlatform {
	var plats []namedPlatform

	if (platform == "all" || platform == "x") && creds.xEnabled {
		xPlat := x.NewXPlatform(creds.xAPIKey, creds.xAPISecret, creds.xAccessToken, creds.xAccessSecret, contactMapper, 100)
		xPlat.SetMaxChars(maxChars)
		plats = append(plats, namedPlatform{name: "X", plat: xPlat})
	}
	if (platform == "all" || platform == "bluesky") && creds.bskyEnabled {
		plats = append(plats, namedPlatform{
			name: "Bluesky",
			plat: bluesky.NewBlueskyPlatform(creds.bskyHandle, creds.bskyPassword, 100, contactMapper),
		})
	}
	if platform == "all" || platform == "instagram" {
		var igPlat *instagram.InstagramPlatform
		if creds.igEnabled {
			igPlat = instagram.NewInstagramPlatformWithCredentials(
				creds.igUserID, creds.igAccessToken, creds.imageHostToken,
				creds.imageHostOwner, creds.imageHostName, 100,
			)
			fmt.Println("📷 Instagram: real mode (credentials configured)")
		} else {
			igPlat = instagram.NewInstagramPlatform(100)
			fmt.Println("📷 Instagram: stub mode (no credentials)")
		}
		igPlat.SetContactMapper(contactMapper)
		plats = append(plats, namedPlatform{name: "Instagram", plat: igPlat})
	}
	if platform == "all" || platform == "facebook" {
		var fbPlat *facebook.FacebookPlatform
		if creds.fbEnabled {
			fbPlat = facebook.NewFacebookPlatformWithCredentials(creds.fbPageID, creds.fbAccessToken, 100)
			fmt.Println("📘 Facebook: real mode (credentials configured)")
		} else {
			fbPlat = facebook.NewFacebookPlatform(100)
			fmt.Println("📘 Facebook: stub mode (no credentials)")
		}
		plats = append(plats, namedPlatform{name: "Facebook", plat: fbPlat})
	}

	return plats
}
