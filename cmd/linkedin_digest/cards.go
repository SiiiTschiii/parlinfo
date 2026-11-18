package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/siiitschiii/parlinfo/pkg/imagegen"
	"github.com/siiitschiii/parlinfo/pkg/votes"
)

// postEntries lists, for each post render writes and in the same order, the
// sitting and the businesses its entries are about, in the order they are
// numbered.
func postEntries(sections []section, o options) []sittingEntries {
	var out []sittingEntries
	for _, s := range posts(sections, o) {
		se := sittingEntries{Sitting: s}
		for _, pt := range parts(s, o.All) {
			for _, e := range pt.entries {
				se.Groups = append(se.Groups, e.group)
			}
		}
		out = append(out, se)
	}
	return out
}

// sittingEntries is one post's sitting and its entries' businesses.
type sittingEntries struct {
	Sitting sitting
	Groups  [][]votes.Vote
}

// writeCards writes the cards to attach to each post into dir: the first card
// of every entry's business, the one the bot leads with on the other
// platforms, as <date>-<body>-<n>.jpg with n the entry's number, and the alt
// text LinkedIn asks for each image in <date>-<body>-alt.txt. Only the first,
// because a post of five businesses with whole carousels would be a pile of
// images nobody pages through; the rest is a click away on parlinfo.ch.
//
// It returns the files it wrote. Files left by an earlier run for the same
// sitting are removed first, so a post that lost an entry loses its card.
func writeCards(dir string, sections []section, o options) ([]string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	var written []string
	for _, se := range postEntries(sections, o) {
		prefix := filepath.Join(dir, cardPrefix(se.Sitting))
		stale, _ := filepath.Glob(prefix + "-*")
		for _, f := range stale {
			if err := os.Remove(f); err != nil {
				return nil, err
			}
		}

		var alt strings.Builder
		for i, g := range se.Groups {
			cards, err := imagegen.GenerateCards(g)
			if err != nil {
				return nil, fmt.Errorf("%s, entry %d: %w", se.Sitting.heading(), i+1, err)
			}
			path := fmt.Sprintf("%s-%d.jpg", prefix, i+1)
			if err := os.WriteFile(path, cards[0].JPEG, 0o644); err != nil {
				return nil, err
			}
			written = append(written, path)
			// An alt text runs over several lines, so each gets a heading
			// rather than a number.
			if i > 0 {
				alt.WriteString("\n")
			}
			fmt.Fprintf(&alt, "━━━ Bild %d ━━━\n%s\n", i+1, cards[0].Alt)
		}
		if alt.Len() > 0 {
			path := prefix + "-alt.txt"
			if err := os.WriteFile(path, []byte(alt.String()), 0o644); err != nil {
				return nil, err
			}
			written = append(written, path)
		}
	}
	return written, nil
}

// cardPrefix names a sitting's files by its day, or its first and last, and
// the body's first word: "2026-11-09-kantonsrat",
// "2026-09-28-2026-10-02-nationalrat".
func cardPrefix(s sitting) string {
	body := "sitzung"
	if f := strings.Fields(s.Body); len(f) > 0 {
		body = strings.ToLower(f[0])
	}
	if s.Date.IsZero() {
		return body
	}
	days := s.Date.Format("2006-01-02")
	if !s.Last.IsZero() && !sameDay(s.Date, s.Last) {
		days += "-" + s.Last.Format("2006-01-02")
	}
	return days + "-" + body
}
