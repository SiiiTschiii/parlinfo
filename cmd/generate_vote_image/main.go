package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/siiitschiii/parlinfo/pkg/config"
	"github.com/siiitschiii/parlinfo/pkg/imagegen"
	"github.com/siiitschiii/parlinfo/pkg/voteposting"
	"github.com/siiitschiii/parlinfo/pkg/voteposting/platforms/instagram"
	"github.com/siiitschiii/parlinfo/pkg/voteposting/testfixtures"
	"github.com/siiitschiii/parlinfo/pkg/votes"
)

func main() {
	fixture := flag.String("fixture", "all", "fixture name from AllFixtures() keys, or 'all'")
	outDir := flag.String("out", "out/images", "output directory for generated JPEGs")
	platform := flag.String("platform", "", "optional: 'instagram' to preview formatted caption alongside images")
	jurisdictionKey := flag.String("jurisdiction", "", "render live votes for this jurisdiction instead of fixtures")
	fetchLimit := flag.Int("fetch", 35, "with -jurisdiction: number of individual votes to fetch from the source")
	numGroups := flag.Int("n", 3, "with -jurisdiction: number of vote groups to render")
	flag.Parse()

	var fixtures map[string][]votes.Vote
	var fixtureNames []string

	if *jurisdictionKey != "" {
		fixtures, fixtureNames = liveGroups(*jurisdictionKey, *fetchLimit, *numGroups)
	} else {
		allFixtures := testfixtures.AllFixtures()
		if *fixture == "all" {
			fixtures = allFixtures
			fixtureNames = testfixtures.FixtureNames
		} else {
			group, ok := allFixtures[*fixture]
			if !ok {
				log.Fatalf("Unknown fixture %q. Available: %v", *fixture, testfixtures.FixtureNames)
			}
			fixtures = map[string][]votes.Vote{*fixture: group}
			fixtureNames = []string{*fixture}
		}
	}

	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		log.Fatalf("Creating output directory: %v", err)
	}

	for idx, name := range fixtureNames {
		group := fixtures[name]
		images, err := imagegen.GenerateCarousel(group)
		if err != nil {
			log.Printf("Error generating %s: %v", name, err)
			continue
		}

		for i, imgData := range images {
			filename := fmt.Sprintf("%02d_%s_%d.jpg", idx, name, i)
			path := filepath.Join(*outDir, filename)
			if err := os.WriteFile(path, imgData, 0o644); err != nil {
				log.Printf("Error writing %s: %v", path, err)
				continue
			}
			fmt.Printf("Wrote %s (%d bytes)\n", path, len(imgData))
		}

		// If Instagram platform is requested, also show formatted caption
		if *platform == "instagram" {
			content, err := instagram.FormatCarousel(group)
			if err != nil {
				log.Printf("Error formatting Instagram content for %s: %v", name, err)
				continue
			}
			fmt.Printf("\n━━━ Instagram preview: %s ━━━\n", name)
			fmt.Printf("📸 %d image(s), caption (%d chars):\n", len(content.Images), len([]rune(content.Caption)))
			fmt.Printf("%s\n\n", content.Caption)
		}
	}
}

// liveGroups fetches what the bot would actually post for a jurisdiction right
// now, so the carousel can be checked against real data rather than fixtures.
// See voteposting.LatestGroups.
func liveGroups(jurisdictionKey string, fetchLimit, numGroups int) (map[string][]votes.Vote, []string) {
	jurisdiction, err := config.LookupJurisdiction(jurisdictionKey)
	if err != nil {
		log.Fatalf("%v. Available: %v", err, config.JurisdictionKeys())
	}

	groups, err := voteposting.LatestGroups(jurisdiction.NewSource(), jurisdiction.Key, fetchLimit, numGroups)
	if err != nil {
		log.Fatalf("Error preparing votes: %v", err)
	}
	if len(groups) == 0 {
		log.Fatalf("No votes found for %s", jurisdiction.Key)
	}

	out := make(map[string][]votes.Vote, len(groups))
	names := make([]string, 0, len(groups))
	for _, g := range groups {
		// Named for what the group is, so the filenames say which sitting and
		// which business matter a JPEG belongs to.
		name := fmt.Sprintf("%s_%s", g[0].DateString(), sanitise(g[0].Affair.Number))
		// Two groups can share a business number across sitting days only, so a
		// collision here means the same key twice; suffix rather than lose one.
		for _, taken := out[name]; taken; _, taken = out[name] {
			name += "_"
		}
		out[name] = g
		names = append(names, name)
	}
	return out, names
}

// sanitise strips what a business number can carry that a filename should not.
func sanitise(s string) string {
	if s == "" {
		return "ohne-geschaeft"
	}
	return strings.NewReplacer("/", "-", " ", "-", "#", "").Replace(s)
}
