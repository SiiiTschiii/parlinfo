package imagegen

import (
	"fmt"
	"github.com/siiitschiii/parlinfo/pkg/voteposting/voteformat"
	"golang.org/x/image/font"
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

// longGroup is n votes of the given type, each with a label that wraps.
func longGroup(n int, voteType string) []votes.Vote {
	base := testfixtures.TenVoteStressTest()
	group := make([]votes.Vote, n)
	for i := range group {
		v := base[0]
		v.Subtitle = strings.Repeat("Änderungsantrag zu Art. 12 Abs. 2 der Bau- und Zonenordnung ", 3)
		v.Type = voteType
		if voteType == "Quorum" {
			v.Decision = ""
		}
		group[i] = v
	}
	group[0].Title = strings.Repeat("Totalrevision der Bau- und Zonenordnung der Stadt Zürich ", 6)
	return group
}

// A group too long for the card at any row size keeps the rows that leave the
// title whole and says how many it left out, rather than shrinking or cutting
// the title or drawing the rest below the frame.
func TestFitOverview_TooManyVotesEndsInSummaryRow(t *testing.T) {
	fonts, err := loadFontSet()
	if err != nil {
		t.Fatal(err)
	}
	group := longGroup(24, "")
	inset := bandInset(group[0]) + dateCorner(group[0]).height(fonts)
	face, lines, ol, err := fitOverview(group, inset, imgWidth-2*padding, fonts)
	if err != nil {
		t.Fatal(err)
	}
	if !overviewFits(face, lines, ol, inset, fonts) {
		t.Fatal("overview overruns the card")
	}
	// The title wins over the list: it stays whole and larger than the rows.
	if strings.Contains(strings.Join(lines, " "), "…") || !titleOutranksRows(face, ol, fonts) {
		t.Errorf("title gave way to the rows: %q", lines)
	}
	last := ol.rows[len(ol.rows)-1]
	shown := len(ol.rows) - 1
	if last.mark != markOmitted || !strings.Contains(last.lines[0], fmt.Sprintf("%d weitere", len(group)-shown)) {
		t.Fatalf("expected a closing row naming the %d omitted votes, got %+v", len(group)-shown, last)
	}
}

// However long the heading and the labels, every vote's tally survives the
// alt text's cap.
func TestTitleAlt_KeepsEveryTally(t *testing.T) {
	group := longGroup(12, "Quorum")
	for i := range group {
		group[i].Yes, group[i].No, group[i].Absent = intPtr(129), intPtr(0), intPtr(51)
	}
	alt := titleAlt(group)
	if n := len([]rune(alt)); n > maxAltRunes {
		t.Fatalf("alt text is %d runes, over the cap", n)
	}
	if !strings.HasSuffix(alt, "12. – 129 Zustimmungen, 51 ohne Zustimmung") &&
		!strings.HasSuffix(alt, "12. – 129 Zust. · 51 ohne") {
		t.Fatalf("last vote's tally was cut:\n%s", alt)
	}
}

// A quorum row's tally already says it was a quorum vote; an Ausgabenbremse
// is tallied the same way but keeps its label, which is the only place that
// says which procedure it was.
func TestOverviewLabel_DropsOnlyTheQuorumSuffix(t *testing.T) {
	quorum := votes.Vote{Type: "Quorum", Subtitle: "Antrag"}
	if got := overviewLabel(quorum, 0, 2); strings.Contains(got, "Quorum") {
		t.Errorf("quorum label = %q, want no type suffix", got)
	}
	brake := votes.Vote{Type: "Ausgabenbremse", Subtitle: "Antrag"}
	if got, want := overviewLabel(brake, 0, 2), voteformat.SubVoteLabel(brake, 0, 2); got != want {
		t.Errorf("Ausgabenbremse label = %q, want %q", got, want)
	}
}

// A two-vote group is set larger than a group past overviewShortGroup: the
// rows a long group needs would leave a short one half empty.
func TestFitOverview_ShortGroupIsSetLarger(t *testing.T) {
	fonts, err := loadFontSet()
	if err != nil {
		t.Fatal(err)
	}
	short := testfixtures.MultiVoteGroup()
	long := testfixtures.KantonsratMultiVote()
	fit := func(g []votes.Vote) (font.Face, overviewLayout) {
		face, _, ol, err := fitOverview(g, bandInset(g[0]), imgWidth-2*padding, fonts)
		if err != nil {
			t.Fatal(err)
		}
		return face, ol
	}
	sFace, sRows := fit(short)
	_, lRows := fit(long)
	if lineHeight(sRows.faces.text) <= lineHeight(fonts.overviewRows[0].text) {
		t.Errorf("short group's rows (%d) not above the long-group sizes (%d)", lineHeight(sRows.faces.text), lineHeight(fonts.overviewRows[0].text))
	}
	if lineHeight(lRows.faces.text) > lineHeight(fonts.overviewRows[0].text) {
		t.Errorf("five-vote group took a short-group row size (%d)", lineHeight(lRows.faces.text))
	}
	if lineHeight(sFace) <= lineHeight(fonts.overviewTitleFloor) {
		t.Errorf("short group's title (%d) not set above the floor", lineHeight(sFace))
	}
}

// The overview says how many votes it lists, and no pass/fail totals: the
// card does not print them, so its description must not either.
func TestTitleAlt_CountsVotesWithoutTotals(t *testing.T) {
	alt := titleAlt(testfixtures.MultiVoteGroup())
	if !strings.Contains(alt, "2 Abstimmungen\n1. ") {
		t.Errorf("alt text lacks the vote count before the list:\n%s", alt)
	}
	if strings.Contains(alt, "angenommen, ") {
		t.Errorf("alt text states pass/fail totals the card does not:\n%s", alt)
	}
}

// A row without a verdict draws no mark, and a group where no row has one
// gives the mark column to the labels; a group with any verdict keeps the
// column, so its labels still line up.
func TestBuildOverview_MarkColumnOnlyWhenSomeRowHasAVerdict(t *testing.T) {
	fonts, err := loadFontSet()
	if err != nil {
		t.Fatal(err)
	}
	faces := fonts.overviewRows[0]
	none := buildOverview(testfixtures.KantonsratMultiVote(), faces, 0)
	some := buildOverview(testfixtures.MultiVoteGroup(), faces, 0)
	if none.markW != 0 {
		t.Errorf("group without verdicts reserves a %dpx mark column", none.markW)
	}
	if some.markW == 0 || some.labelX <= none.labelX {
		t.Errorf("group with verdicts lost its mark column (markW=%d, labelX=%d vs %d)", some.markW, some.labelX, none.labelX)
	}
}
