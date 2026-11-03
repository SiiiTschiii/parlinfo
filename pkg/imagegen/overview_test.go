package imagegen

import (
	"strings"
	"testing"

	"github.com/siiitschiii/parlinfo/pkg/voteposting/testfixtures"
	"github.com/siiitschiii/parlinfo/pkg/votes"
)

// overviewLines is every wrapped label line of an overview, in order.
func overviewLines(ol overviewLayout) []string {
	var lines []string
	for _, r := range ol.rows {
		lines = append(lines, r.lines...)
	}
	return lines
}

func intPtr(i int) *int { return &i }

func TestOverviewTally(t *testing.T) {
	cases := []struct {
		name string
		v    votes.Vote
		want string
		long string
	}{
		{"ja/nein", votes.Vote{Yes: intPtr(62), No: intPtr(51), Abstention: intPtr(3)}, "62 : 51", "62 Ja, 51 Nein"},
		// Nein is folded into "ohne", as QuorumTally does everywhere else.
		{"quorum", votes.Vote{Type: "Quorum", Yes: intPtr(129), No: intPtr(1), Absent: intPtr(50)}, "129 Zust. · 51 ohne", "129 Zustimmungen, 51 ohne Zustimmung"},
		{"auswahl", votes.Vote{ChoiceA: intPtr(50), ChoiceB: intPtr(24), ChoiceC: intPtr(40), ChoiceD: intPtr(0)}, "A 50 · B 24 · C 40", "A: 50, B: 24, C: 40"},
	}
	for _, c := range cases {
		if got := overviewTally(c.v); got != c.want {
			t.Errorf("%s: overviewTally = %q, want %q", c.name, got, c.want)
		}
		if got := overviewTallyLong(c.v); got != c.long {
			t.Errorf("%s: overviewTallyLong = %q, want %q", c.name, got, c.long)
		}
	}
}

func TestVoteMark(t *testing.T) {
	cases := []struct {
		name   string
		v      votes.Vote
		kind   markKind
		letter string
	}{
		{"accepted", votes.Vote{Decision: "angenommen"}, markAccepted, ""},
		{"rejected", votes.Vote{Decision: "abgelehnt"}, markRejected, ""},
		// No decision from the source: the row claims nothing.
		{"no verdict", votes.Vote{Yes: intPtr(130), No: intPtr(44)}, markNone, ""},
		{"auswahl", votes.Vote{Decision: "Auswahl B", ChoiceA: intPtr(3), ChoiceB: intPtr(9)}, markAuswahl, "B"},
		{"auswahl unknown", votes.Vote{Decision: "Auswahl AB", ChoiceA: intPtr(3)}, markNone, ""},
	}
	for _, c := range cases {
		kind, letter := voteMark(c.v)
		if kind != c.kind || letter != c.letter {
			t.Errorf("%s: voteMark = (%v, %q), want (%v, %q)", c.name, kind, letter, c.kind, c.letter)
		}
	}
}

// A group with no verdict at all — the Kantonsrat reports none for most votes —
// must not print "0 angenommen, 0 abgelehnt".
func TestVerdictTotals_NoVerdictMeansNoTotals(t *testing.T) {
	if _, _, ok := verdictTotals(testfixtures.KantonsratMultiVote()); ok {
		t.Fatal("expected no totals for a group without decisions")
	}
	a, r, ok := verdictTotals(testfixtures.MultiVoteGroup())
	if !ok || a != 1 || r != 1 {
		t.Fatalf("verdictTotals = (%d, %d, %v), want (1, 1, true)", a, r, ok)
	}
	if alt := titleAlt(testfixtures.KantonsratMultiVote()); strings.Contains(alt, "angenommen, ") {
		t.Fatalf("alt text states totals the card does not:\n%s", alt)
	}
}

// Every multi-vote fixture's overview fits inside the frame: the rows end above
// the bottom margin and the title is whole.
func TestOverview_FitsEveryFixture(t *testing.T) {
	fonts, err := loadFontSet()
	if err != nil {
		t.Fatal(err)
	}
	for name, group := range testfixtures.AllFixtures() {
		if len(group) < 2 || votes.IsWahlgeschaeftGroup(group) {
			continue
		}
		c := carouselCards(group, SelectColor(group[0].Jurisdiction, group[0].Affair.Number), fonts)[0]
		inset := c.inset(fonts)
		dry := newCursor(inset, imgHeight)
		if err := c.layout(nil, dry, inset); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if bottom := centredStart(inset, dry.contentHeight()) + dry.contentHeight(); bottom > imgHeight-padding {
			t.Errorf("%s: overview ends at %d, below the margin at %d", name, bottom, imgHeight-padding)
		}
		_, lines, ol, err := fitOverview(group, inset, imgWidth-2*padding, fonts)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(strings.Join(lines, " "), "…") {
			t.Errorf("%s: title was cut: %q", name, lines)
		}
		if len(ol.rows) != len(group) {
			t.Errorf("%s: %d rows for %d votes", name, len(ol.rows), len(group))
		}
	}
}

// Rows shrink before labels are cut: ten votes whose labels wrap at the
// largest row size are set smaller, with every label still whole.
func TestFitOverview_ShrinksRowsBeforeCuttingLabels(t *testing.T) {
	fonts, err := loadFontSet()
	if err != nil {
		t.Fatal(err)
	}
	group := testfixtures.TenVoteStressTest()
	for i := range group {
		group[i].Subtitle = "Änderungsantrag zu Art. 12 Abs. 2 der Bau- und Zonenordnung, Gewerbezonen"
	}
	_, _, ol, err := fitOverview(group, bandInset(group[0]), imgWidth-2*padding, fonts)
	if err != nil {
		t.Fatal(err)
	}
	if lineHeight(ol.faces.text) >= lineHeight(fonts.overviewRows[0].text) {
		t.Errorf("rows stayed at the largest size (%d)", lineHeight(ol.faces.text))
	}
	if strings.Contains(strings.Join(overviewLines(ol), " "), "…") {
		t.Errorf("labels were cut: %q", overviewLines(ol))
	}
}
