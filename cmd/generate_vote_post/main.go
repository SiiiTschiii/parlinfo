// generate_vote_post renders what the bot would post for the most recent votes,
// ignoring the vote log so already-posted votes still show up.
//
// Rendering two jurisdictions side by side is the design loop for post copy:
//
//	go run ./cmd/generate_vote_post -jurisdiction zurich-city  -n 1
//	go run ./cmd/generate_vote_post -jurisdiction zurich-canton -n 1
package main

import (
	"flag"
	"fmt"
	"log"
	"strings"

	"github.com/siiitschiii/parlinfo/pkg/config"
	"github.com/siiitschiii/parlinfo/pkg/contacts"
	"github.com/siiitschiii/parlinfo/pkg/votelog"
	"github.com/siiitschiii/parlinfo/pkg/voteposting"
	"github.com/siiitschiii/parlinfo/pkg/voteposting/platforms"
	"github.com/siiitschiii/parlinfo/pkg/voteposting/platforms/bluesky"
	"github.com/siiitschiii/parlinfo/pkg/voteposting/platforms/facebook"
	"github.com/siiitschiii/parlinfo/pkg/voteposting/platforms/instagram"
	"github.com/siiitschiii/parlinfo/pkg/voteposting/platforms/x"
)

func main() {
	numVotes := flag.Int("n", 1, "number of vote groups to preview")
	fetchLimit := flag.Int("fetch", 35, "number of individual votes to fetch from the source")
	platform := flag.String("platform", "", "platform to preview: x, bluesky, instagram, facebook (default: all)")
	jurisdictionKey := flag.String("jurisdiction", "zurich-city", "jurisdiction to preview")
	// The limit decides where the title is cut and where the thread breaks, so
	// previewing at a different one than the account posts at answers a
	// question nobody asked. Defaults to the free-tier 280; the live accounts
	// run at ZURICH_X_MAX_CHARS, so pass that value to see what they would say.
	maxChars := flag.Int("x-max-chars", x.DefaultMaxChars, "per-post character limit for X (280 free, 2000 Premium)")
	flag.Parse()

	showX := *platform == "" || strings.EqualFold(*platform, "x")
	showBluesky := *platform == "" || strings.EqualFold(*platform, "bluesky") || strings.EqualFold(*platform, "bsky")
	showInstagram := *platform == "" || strings.EqualFold(*platform, "instagram") || strings.EqualFold(*platform, "ig")
	showFacebook := *platform == "" || strings.EqualFold(*platform, "facebook") || strings.EqualFold(*platform, "fb")

	if !showX && !showBluesky && !showInstagram && !showFacebook {
		log.Fatalf("Unknown platform %q. Use: x, bluesky, instagram, facebook", *platform)
	}

	jurisdiction, err := config.LookupJurisdiction(*jurisdictionKey)
	if err != nil {
		log.Fatalf("%v. Available: %v", err, config.JurisdictionKeys())
	}

	contactMapper, err := contacts.LoadContactFiles(contacts.PathsFor(jurisdiction.Key)...)
	if err != nil {
		log.Printf("Warning: Could not load contacts for tagging: %v", err)
		contactMapper = nil
	}

	// An empty log shows every recent vote, not just unposted ones, and no age
	// guard so the preview still works during a recess.
	emptyLog := votelog.NewEmpty(jurisdiction.Key, votelog.PlatformX)
	source, err := jurisdiction.Source()
	if err != nil {
		log.Fatal(err)
	}
	groups, err := voteposting.PrepareVoteGroups(source, emptyLog, *fetchLimit, 0)
	if err != nil {
		log.Fatalf("Error preparing votes: %v", err)
	}
	if len(groups) == 0 {
		log.Fatalf("No votes found for %s", jurisdiction.Key)
	}

	fmt.Printf("Jurisdiction: %s (%s)\n", jurisdiction.Name, jurisdiction.Key)

	first := true
	preview := func(name string, logPlatform votelog.Platform, poster platforms.Platform) {
		if !first {
			fmt.Println()
		}
		first = false
		fmt.Printf("━━━ %s ━━━\n", name)
		logs := voteposting.SingleLog(jurisdiction.Key, votelog.NewEmpty(jurisdiction.Key, logPlatform))
		if _, err := voteposting.PostToPlatform(groups, poster, logs, true); err != nil {
			log.Fatalf("Error: %v", err)
		}
	}

	if showX {
		xPlatform := x.NewXPlatform("", "", "", "", contactMapper, *numVotes)
		xPlatform.SetMaxChars(*maxChars)
		preview("X/Twitter", votelog.PlatformX, xPlatform)
	}
	if showBluesky {
		preview("Bluesky", votelog.PlatformBluesky, bluesky.NewBlueskyPlatform("", "", *numVotes, contactMapper))
	}
	if showInstagram {
		igPlatform := instagram.NewInstagramPlatform(*numVotes)
		igPlatform.SetContactMapper(contactMapper)
		preview("Instagram", votelog.PlatformInstagram, igPlatform)
	}
	if showFacebook {
		preview("Facebook", votelog.PlatformFacebook, facebook.NewFacebookPlatform(*numVotes))
	}
}
