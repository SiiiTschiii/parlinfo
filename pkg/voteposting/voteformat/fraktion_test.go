package voteformat

import (
	"strings"
	"testing"

	"github.com/siiitschiii/parlinfo/pkg/votes"
)

func stimmabgabe(fraktion, verhalten string) votes.MemberVote {
	return votes.MemberVote{
		Fraktion: fraktion,
		Choice:   verhalten,
	}
}

// repeat creates n copies of a member vote.
func repeat(s votes.MemberVote, n int) []votes.MemberVote {
	out := make([]votes.MemberVote, n)
	for i := range out {
		out[i] = s
	}
	return out
}

func TestAggregateFraktionCounts(t *testing.T) {
	stimmabgaben := []votes.MemberVote{
		stimmabgabe("SP", "Ja"),
		stimmabgabe("SP", "Ja"),
		stimmabgabe("SP", "Abwesend"),
		stimmabgabe("FDP", "Nein"),
		stimmabgabe("", "Ja"), // empty Fraktion — should be omitted
	}

	counts := AggregateFraktionCounts(votes.Vote{MemberVotes: stimmabgaben})

	if len(counts) != 2 {
		t.Fatalf("expected 2 factions, got %d", len(counts))
	}
	if counts["SP"].Counts["Ja"] != 2 {
		t.Errorf("SP Ja: got %d, want 2", counts["SP"].Counts["Ja"])
	}
	if counts["SP"].Counts["Abwesend"] != 1 {
		t.Errorf("SP Abwesend: got %d, want 1", counts["SP"].Counts["Abwesend"])
	}
	if counts["FDP"].Counts["Nein"] != 1 {
		t.Errorf("FDP Nein: got %d, want 1", counts["FDP"].Counts["Nein"])
	}
	if _, ok := counts[""]; ok {
		t.Error("empty Fraktion should be omitted")
	}
}

func TestFormatFraktionBreakdown_JaNein(t *testing.T) {
	// Realistic 7-faction Ja/Nein vote (79 Ja / 29 Nein / 0 Enth / 17 Abw = 125 total)
	var stimmabgaben []votes.MemberVote
	stimmabgaben = append(stimmabgaben, repeat(stimmabgabe("SP", "Ja"), 32)...)
	stimmabgaben = append(stimmabgaben, repeat(stimmabgabe("SP", "Abwesend"), 5)...)
	stimmabgaben = append(stimmabgaben, repeat(stimmabgabe("Grüne", "Ja"), 17)...)
	stimmabgaben = append(stimmabgaben, repeat(stimmabgabe("Grüne", "Abwesend"), 1)...)
	stimmabgaben = append(stimmabgaben, repeat(stimmabgabe("FDP", "Nein"), 16)...)
	stimmabgaben = append(stimmabgaben, repeat(stimmabgabe("FDP", "Abwesend"), 7)...)
	stimmabgaben = append(stimmabgaben, repeat(stimmabgabe("GLP", "Ja"), 13)...)
	stimmabgaben = append(stimmabgaben, repeat(stimmabgabe("GLP", "Abwesend"), 2)...)
	stimmabgaben = append(stimmabgaben, repeat(stimmabgabe("SVP", "Nein"), 13)...)
	stimmabgaben = append(stimmabgaben, repeat(stimmabgabe("Die Mitte/EVP", "Ja"), 8)...)
	stimmabgaben = append(stimmabgaben, repeat(stimmabgabe("Die Mitte/EVP", "Abwesend"), 2)...)
	stimmabgaben = append(stimmabgaben, repeat(stimmabgabe("AL", "Ja"), 8)...)
	stimmabgaben = append(stimmabgaben, repeat(stimmabgabe("", "Ja"), 1)...) // empty — omitted

	counts := AggregateFraktionCounts(votes.Vote{MemberVotes: stimmabgaben})
	result := FormatFraktionBreakdown(counts, votes.German)

	t.Logf("Output:\n%s", result)

	// Header
	if !strings.HasPrefix(result, "🏛️ Fraktionen (Ja/Nein/Enth/Abw):") {
		t.Errorf("unexpected header, got first line: %s", strings.SplitN(result, "\n", 2)[0])
	}

	// Sorted by total members descending:
	// SP=37, FDP=23, Grüne=18, GLP=15, SVP=13, Die Mitte/EVP=10, AL=8
	lines := strings.Split(result, "\n")
	if len(lines) != 8 { // header + 7 factions
		t.Fatalf("expected 8 lines, got %d", len(lines))
	}

	expectedOrder := []string{"SP", "FDP", "Grüne", "GLP", "SVP", "Die Mitte/EVP", "AL"}
	for i, name := range expectedOrder {
		line := lines[i+1]
		if !strings.HasPrefix(line, name+" ") {
			t.Errorf("line %d: expected faction %q, got %q", i+1, name, line)
		}
	}

	// Verify specific counts
	if !strings.Contains(result, "SP 32/0/0/5") {
		t.Errorf("SP counts wrong, got:\n%s", result)
	}
	if !strings.Contains(result, "FDP 0/16/0/7") {
		t.Errorf("FDP counts wrong, got:\n%s", result)
	}
	if !strings.Contains(result, "AL 8/0/0/0") {
		t.Errorf("AL counts wrong, got:\n%s", result)
	}
}

func TestFormatFraktionBreakdown_Auswahl(t *testing.T) {
	var stimmabgaben []votes.MemberVote
	stimmabgaben = append(stimmabgaben, repeat(stimmabgabe("SP", "A"), 20)...)
	stimmabgaben = append(stimmabgaben, repeat(stimmabgabe("SP", "B"), 10)...)
	stimmabgaben = append(stimmabgaben, repeat(stimmabgabe("SP", "Abwesend"), 5)...)
	stimmabgaben = append(stimmabgaben, repeat(stimmabgabe("FDP", "B"), 12)...)
	stimmabgaben = append(stimmabgaben, repeat(stimmabgabe("FDP", "C"), 4)...)
	stimmabgaben = append(stimmabgaben, repeat(stimmabgabe("FDP", "Abwesend"), 7)...)

	counts := AggregateFraktionCounts(votes.Vote{MemberVotes: stimmabgaben})
	result := FormatFraktionBreakdown(counts, votes.German)

	t.Logf("Output:\n%s", result)

	// Header should be dynamic: A/B/C/Abw (no Enth, no Ja/Nein)
	if !strings.HasPrefix(result, "🏛️ Fraktionen (A/B/C/Abw):") {
		t.Errorf("unexpected header, got first line: %s", strings.SplitN(result, "\n", 2)[0])
	}

	// SP=35 total, FDP=23 total → SP first
	lines := strings.Split(result, "\n")
	if !strings.HasPrefix(lines[1], "SP ") {
		t.Errorf("expected SP first, got: %s", lines[1])
	}
	if !strings.HasPrefix(lines[2], "FDP ") {
		t.Errorf("expected FDP second, got: %s", lines[2])
	}

	// SP: A=20, B=10, C=0, Abw=5
	if !strings.Contains(result, "SP 20/10/0/5") {
		t.Errorf("SP counts wrong, got:\n%s", result)
	}
	// FDP: A=0, B=12, C=4, Abw=7
	if !strings.Contains(result, "FDP 0/12/4/7") {
		t.Errorf("FDP counts wrong, got:\n%s", result)
	}
}

func TestFormatFraktionBreakdown_Empty(t *testing.T) {
	result := FormatFraktionBreakdown(nil, votes.German)
	if result != "" {
		t.Errorf("expected empty string for nil, got %q", result)
	}

	result = FormatFraktionBreakdown(map[string]*FraktionCounts{}, votes.German)
	if result != "" {
		t.Errorf("expected empty string for empty map, got %q", result)
	}
}

func TestFormatFraktionBreakdown_TieBreaking(t *testing.T) {
	// Two factions with same total — should be sorted alphabetically
	var stimmabgaben []votes.MemberVote
	stimmabgaben = append(stimmabgaben, repeat(stimmabgabe("Zebra", "Ja"), 10)...)
	stimmabgaben = append(stimmabgaben, repeat(stimmabgabe("Alpha", "Ja"), 10)...)

	counts := AggregateFraktionCounts(votes.Vote{MemberVotes: stimmabgaben})
	result := FormatFraktionBreakdown(counts, votes.German)

	t.Logf("Output:\n%s", result)

	lines := strings.Split(result, "\n")
	if !strings.HasPrefix(lines[1], "Alpha ") {
		t.Errorf("expected Alpha first (alphabetical tie-break), got: %s", lines[1])
	}
	if !strings.HasPrefix(lines[2], "Zebra ") {
		t.Errorf("expected Zebra second, got: %s", lines[2])
	}
}

func TestFormatFraktionBreakdown_SingleFaction(t *testing.T) {
	stimmabgaben := repeat(stimmabgabe("SP", "Ja"), 5)

	counts := AggregateFraktionCounts(votes.Vote{MemberVotes: stimmabgaben})
	result := FormatFraktionBreakdown(counts, votes.German)

	t.Logf("Output:\n%s", result)

	lines := strings.Split(result, "\n")
	if len(lines) != 2 { // header + 1 faction
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	if !strings.Contains(result, "SP 5") {
		t.Errorf("SP count wrong, got:\n%s", result)
	}
}

// Roughly 1% of members are not mapped to a faction in either source. They must
// be left out of the table rather than bucketed under an empty name — and,
// critically, must not take the process down. Their vote still counts, because
// the totals come from the source and are never summed from this list.
func TestAggregateFraktionCounts_UnmappedFraktion(t *testing.T) {
	members := []votes.MemberVote{
		stimmabgabe("SVP", "Ja"),
		{Name: "Unmapped Member", Choice: "Ja"},
		{Name: "Also Unmapped", Fraktion: "   ", Choice: "Nein"},
	}

	counts := AggregateFraktionCounts(votes.Vote{MemberVotes: members})

	if _, ok := counts[""]; ok {
		t.Error("members without a faction must not create an empty-named row")
	}
	if got := len(counts); got != 2 {
		// "   " is a distinct (if ugly) faction name; only "" is dropped.
		t.Errorf("got %d factions, want 2", got)
	}
	if counts["SVP"].Counts["Ja"] != 1 {
		t.Errorf("mapped members should still be counted, got %v", counts["SVP"].Counts)
	}

	if out := FormatFraktionBreakdown(counts, votes.German); out == "" {
		t.Error("a breakdown with unmapped members should still render")
	}
}
