package voteformat

import (
	"fmt"
	"sort"
	"strings"

	"github.com/siiitschiii/parlinfo/pkg/votes"
)

// FraktionCounts holds vote counts per faction, keyed by Abstimmungsverhalten value.
type FraktionCounts struct {
	Counts map[string]int // e.g. {"Ja": 32, "Nein": 0, "Enthaltung": 0, "Abwesend": 5}
}

// AggregateFraktionCounts groups member votes by Fraktion, counting each Choice.
// Members with no Fraktion are omitted — every source has a small tail of
// unmapped members, and inventing a bucket for them would be worse than leaving
// them out of the table; they still count towards the vote totals, which are
// reported independently.
// The returned map contains exactly the factions present in the data.
func AggregateFraktionCounts(v votes.Vote) map[string]*FraktionCounts {
	result := make(map[string]*FraktionCounts)
	for _, m := range v.MemberVotes {
		if m.Fraktion == "" {
			continue
		}
		fc, ok := result[m.Fraktion]
		if !ok {
			fc = &FraktionCounts{Counts: make(map[string]int)}
			result[m.Fraktion] = fc
		}
		fc.Counts[quorumChoice(v.Type, m.Choice)]++
	}
	return result
}

// Column names for a quorum vote, which does not have the usual four.
const (
	choiceZustimmung = "Zustimmung"
	choiceOhne       = "ohne Zustimmung"
)

// quorumChoice renames a member's recorded choice for a quorum vote.
//
// A quorum vote counts supporters against a threshold and offers no Nein at
// all, so everyone who does not support it is filed by the source under
// "Abwesend" — including the members who were sitting in the chamber and chose
// not to press the button. For the 15.06.2026 Ausgabenbremse the official record
// puts that at 6 genuinely absent against 46 present and abstaining, where the
// API reports a flat 52. "ohne Zustimmung" is true of both groups, so the table
// stops asserting an attendance it cannot support — and stays correct whether or
// not the source ever separates the two.
func quorumChoice(voteType, choice string) string {
	if !isThresholdVoteType(voteType) {
		return choice
	}
	switch choice {
	case "Ja":
		return choiceZustimmung
	// "Nein" does not occur on a quorum vote today, but upstream #179 is
	// considering mapping the Kantonsrat's "Nicht abgestimmt" onto it. It would
	// mean the same thing as the current bucket — did not support — so it joins
	// it rather than opening a third column that splits one group in two.
	case "Nein", "Abwesend":
		return choiceOhne
	}
	return choice
}

// headerAbbrev maps Abstimmungsverhalten values to short header labels.
var headerAbbrev = map[string]string{
	"Enthaltung":     "Enth",
	"Abwesend":       "Abw",
	choiceZustimmung: "Zust.",
	choiceOhne:       "ohne",
}

// metaValues are Abstimmungsverhalten values that always sort last in the header.
var metaValues = map[string]bool{
	"Enthaltung": true,
	"Abwesend":   true,
	choiceOhne:   true,
}

// FormatFraktionBreakdown formats the aggregated counts into the display string.
// Returns "" if the input is empty.
// Header legend is built dynamically from distinct Abstimmungsverhalten values.
// Fraktionen sorted by total members descending (sum of all counts); ties broken alphabetically.
// The header is written in l; the counts are keyed in the source vocabulary
// whatever the language.
func FormatFraktionBreakdown(counts map[string]*FraktionCounts, l votes.Language) string {
	if len(counts) == 0 {
		return ""
	}

	// Collect all distinct Abstimmungsverhalten keys.
	keySet := make(map[string]bool)
	for _, fc := range counts {
		for k := range fc.Counts {
			keySet[k] = true
		}
	}

	// Sort keys: non-meta first (natural order), then Enthaltung, then Abwesend.
	var primary []string
	hasEnth := false
	hasAbw := false
	hasOhne := false
	hasStandardVote := false
	for k := range keySet {
		if k == "Enthaltung" {
			hasEnth = true
		} else if k == "Abwesend" {
			hasAbw = true
		} else if k == choiceOhne {
			hasOhne = true
		} else if !metaValues[k] {
			primary = append(primary, k)
			if k == "Ja" || k == "Nein" {
				hasStandardVote = true
			}
		}
	}

	// For standard Ja/Nein votes, always include all 4 columns
	// (Ja, Nein, Enthaltung, Abwesend) for consistency, even when
	// no one voted that way.
	if hasStandardVote {
		if !keySet["Ja"] {
			primary = append(primary, "Ja")
		}
		if !keySet["Nein"] {
			primary = append(primary, "Nein")
		}
		hasEnth = true
		hasAbw = true
	}

	sort.Strings(primary)
	var columns []string
	columns = append(columns, primary...)
	if hasEnth {
		columns = append(columns, "Enthaltung")
	}
	if hasAbw {
		columns = append(columns, "Abwesend")
	}
	// A quorum vote's non-supporters sort last for the same reason Abwesend
	// does: it is the residual bucket, not a position anyone voted for.
	if hasOhne {
		columns = append(columns, choiceOhne)
	}

	// Build header legend with abbreviations.
	headerParts := make([]string, len(columns))
	for i, col := range columns {
		if label, ok := localHeader(l, col); ok {
			headerParts[i] = label
		} else if abbr, ok := headerAbbrev[col]; ok {
			headerParts[i] = abbr
		} else {
			headerParts[i] = col
		}
	}
	header := fmt.Sprintf("🏛️ %s (%s):", W(l).Fraktionen, strings.Join(headerParts, "/"))

	// Sort factions by total members descending, ties alphabetically.
	type fraktionEntry struct {
		name  string
		total int
	}
	var factions []fraktionEntry
	for name, fc := range counts {
		total := 0
		for _, v := range fc.Counts {
			total += v
		}
		factions = append(factions, fraktionEntry{name: name, total: total})
	}
	sort.Slice(factions, func(i, j int) bool {
		if factions[i].total != factions[j].total {
			return factions[i].total > factions[j].total
		}
		return factions[i].name < factions[j].name
	})

	// Build output lines.
	var lines []string
	lines = append(lines, header)
	for _, f := range factions {
		fc := counts[f.name]
		vals := make([]string, len(columns))
		for i, col := range columns {
			vals[i] = fmt.Sprintf("%d", fc.Counts[col])
		}
		lines = append(lines, fmt.Sprintf("%s %s", f.name, strings.Join(vals, "/")))
	}

	return strings.Join(lines, "\n")
}

// localHeader names the standard columns in the post's language. The quorum
// columns are not among them: only the Zürich bodies take quorum votes, and
// headerAbbrev names those.
func localHeader(l votes.Language, col string) (string, bool) {
	switch col {
	case "Ja":
		return W(l).Yes, true
	case "Nein":
		return W(l).No, true
	case "Enthaltung":
		return W(l).AbstainHeader, true
	case "Abwesend":
		return W(l).AbsentHeader, true
	}
	return "", false
}
