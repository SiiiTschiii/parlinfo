package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	contactsPkg "github.com/siiitschiii/parlinfo/pkg/contacts"
)

type PlatformStat struct {
	Name  string
	Count int
}

func main() {
	// Every jurisdiction's file, because the table counts politicians the bot
	// can tag and it can tag them wherever they sit.
	files, err := filepath.Glob(filepath.Join("data", "*", "contacts.yaml"))
	if err != nil || len(files) == 0 {
		log.Fatalf("Error finding contacts files: %v", err)
	}
	sort.Strings(files)

	mapper, err := contactsPkg.LoadContactFiles(files...)
	if err != nil {
		log.Fatalf("Error loading contacts: %v", err)
	}
	all := mapper.GetAllContacts()

	// Count platforms
	stats := map[string]int{
		"X (Twitter)": 0,
		"Instagram":   0,
		"Facebook":    0,
		"LinkedIn":    0,
		"TikTok":      0,
		"Bluesky":     0,
	}

	// Verified only: an unverified candidate is a lead, not an account we can
	// tag, and the table's own column calls them verified.
	label := map[string]string{
		"x": "X (Twitter)", "instagram": "Instagram", "facebook": "Facebook",
		"linkedin": "LinkedIn", "tiktok": "TikTok", "bluesky": "Bluesky",
	}
	curated := 0
	for _, contact := range all {
		counted := false
		for _, platform := range contactsPkg.Platforms {
			if len(contact.Verified(platform)) > 0 {
				stats[label[platform]]++
				counted = true
			}
		}
		if counted {
			curated++
		}
	}

	// Convert to slice and sort by count (descending)
	var platformStats []PlatformStat
	for name, count := range stats {
		platformStats = append(platformStats, PlatformStat{Name: name, Count: count})
	}
	sort.Slice(platformStats, func(i, j int) bool {
		return platformStats[i].Count > platformStats[j].Count
	})

	// Find max count for scaling
	maxCount := 0
	for _, stat := range platformStats {
		if stat.Count > maxCount {
			maxCount = stat.Count
		}
	}

	// Update README.md
	updateREADME(stats, curated)
}

// statsLine is the sentence in README.md that carries the per-platform counts.
// The generator must find it: a README edit that stops matching would otherwise
// leave the CI freshness check passing against numbers nobody updates.
var statsLine = regexp.MustCompile(`(from the curated mapping in \[data/\]\(data/\): )[^.]*\.`)

func updateREADME(stats map[string]int, _ int) {
	readmeData, err := os.ReadFile("README.md")
	if err != nil {
		log.Fatalf("Could not read README.md: %v", err)
	}
	readme := string(readmeData)
	if !statsLine.MatchString(readme) {
		log.Fatalf("README.md has no platform statistics sentence matching %s", statsLine)
	}

	// Most taggable first, ties by name, so the sentence is stable between runs.
	names := make([]string, 0, len(stats))
	for name := range stats {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		if stats[names[i]] != stats[names[j]] {
			return stats[names[i]] > stats[names[j]]
		}
		return names[i] < names[j]
	})
	parts := make([]string, len(names))
	for i, name := range names {
		parts[i] = fmt.Sprintf("%s %d", strings.TrimSuffix(name, " (Twitter)"), stats[name])
	}

	updated := statsLine.ReplaceAllString(readme, "${1}"+strings.Join(parts, ", ")+".")
	if updated == readme {
		fmt.Fprintf(os.Stderr, "\n✓ README.md is already up to date (no changes needed)\n\n")
		return
	}
	if err := os.WriteFile("README.md", []byte(updated), 0644); err != nil {
		log.Fatalf("Could not write README.md: %v", err)
	}
	fmt.Fprintf(os.Stderr, "\n✓ Updated README.md platform statistics: %s\n\n", strings.Join(parts, ", "))
}
