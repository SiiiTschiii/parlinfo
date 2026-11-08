package imagegen

import (
	"strings"
	"testing"

	"github.com/siiitschiii/parlinfo/pkg/voteposting/testfixtures"
	"github.com/siiitschiii/parlinfo/pkg/votes"
)

// crowdedProcess is the most a status card has to hold: votes.MaxProcessSteps
// steps, each detail long enough to wrap, and a next step of the longest length
// a process may carry, which is more than the footer's two lines hold.
func crowdedProcess() votes.Process {
	detail := "Die vorberatende Kommission prüft das Geschäft und stellt dem Rat einen Antrag"
	next := []rune(strings.Repeat("Die Kommission des Ständerats entscheidet, ob sie zustimmt. ", 3))
	p := votes.Process{Next: strings.TrimSpace(string(next[:votes.MaxNextRunes]))}
	for i := 0; i < votes.MaxProcessSteps; i++ {
		state := votes.StepDone
		switch i {
		case votes.MaxProcessSteps - 2:
			state = votes.StepCurrent
		case votes.MaxProcessSteps - 1:
			state = votes.StepNext
		}
		p.Steps = append(p.Steps, votes.ProcessStep{Label: "Kommission des Ständerats", Detail: detail, State: state})
	}
	return p
}

// A crowded status card under the longest title any fixture has still fits its
// frame: the steps keep every row, the title gives way, and the content clears
// both the band and the footer.
func TestStatusCard_FitsTheCrowdedCase(t *testing.T) {
	fonts, err := loadFontSet()
	if err != nil {
		t.Fatal(err)
	}
	group := testfixtures.ExtremeTitleFullRoster()
	p := crowdedProcess()
	if err := p.Validate(); err != nil {
		t.Fatalf("the test's own process is invalid: %v", err)
	}
	group[0].Affair.Process = &p

	bg := SelectColor(group[0].Jurisdiction, group[0].Affair.Number)
	cards := carouselCards(group, bg, fonts)
	if len(cards) != 2 || cards[1].name != "status card" {
		t.Fatalf("want the combined card then the status card, got %d cards", len(cards))
	}
	if n := len(cards[0].footer.lines(fonts)); n != footerMaxLines {
		t.Errorf("footer wraps to %d lines, want it cut at %d", n, footerMaxLines)
	}
	// The description is cut for length here; the next step, which the
	// footer shows, has to survive the cut.
	if !strings.Contains(cards[1].alt, p.Next) {
		t.Errorf("status card alt text lost the next step:\n%s", cards[1].alt)
	}

	for _, c := range cards {
		inset := c.inset(fonts)
		dry := newCursor(inset, imgHeight)
		mustLayout(t, c, nil, dry, inset)
		startY := centredStart(inset, dry.contentHeight())
		if bottom := startY + dry.contentHeight(); bottom > imgHeight-padding {
			t.Errorf("%s: content ends %dpx into the footer", c.name, bottom-(imgHeight-padding))
		}
		if gap := startY - inset; gap < padding {
			t.Errorf("%s: content starts %dpx below the band, under the %dpx margin", c.name, gap, padding)
		}
	}

	steps := statusSteps(&p, fonts, 1)
	if got := stepsHeight(steps, fonts); got <= 0 {
		t.Fatalf("steps measure %dpx", got)
	}
	if titleBudget(cards[1].inset(fonts), statusReserve(group[0], statusSteps(&p, fonts, 2), fonts)) >= statusMinTitle {
		t.Error("the crowded case no longer needs the one-line fallback; make it more crowded so the fallback stays tested")
	}
}

// The footer is charged to the card's content, not drawn over it: a card with
// one is laid out shorter by exactly its height.
func TestFooter_IsReservedNotOverlaid(t *testing.T) {
	fonts, err := loadFontSet()
	if err != nil {
		t.Fatal(err)
	}
	group := testfixtures.ProcessGemeinderatPostulat()
	bg := SelectColor(group[0].Jurisdiction, group[0].Affair.Number)
	with := carouselCards(group, bg, fonts)[0]
	without := with
	without.footer = cardFooter{}

	foot := with.footer.height(fonts)
	if foot <= footerGap(fonts) {
		t.Fatalf("footer reserves %dpx, no more than its gap", foot)
	}
	if got := with.inset(fonts) - without.inset(fonts); got != foot {
		t.Errorf("footer adds %dpx to the inset, want its height %d", got, foot)
	}
}

// Every card of a group with a process carries the next step where a reader
// meets it: the first card's footer, and the alt text of both that card and
// the status card.
func TestProcessCards_SayWhatComesNext(t *testing.T) {
	for name, group := range map[string][]votes.Vote{
		"open":  testfixtures.ProcessNationalratMotion(),
		"ended": testfixtures.ProcessGemeinderatAbgelehnt(),
		"multi": testfixtures.ProcessMultiVote(),
	} {
		t.Run(name, func(t *testing.T) {
			fonts, err := loadFontSet()
			if err != nil {
				t.Fatal(err)
			}
			bg := SelectColor(group[0].Jurisdiction, group[0].Affair.Number)
			cards := carouselCards(group, bg, fonts)
			if !HasStatusCard(group) || cards[1].name != "status card" {
				t.Fatalf("status card is not second: %q", cards[1].name)
			}
			want := votes.GroupProcess(group).Next
			for _, c := range cards[:2] {
				if c.footer.text == "" || !strings.Contains(c.footer.text, want) {
					t.Errorf("%s: footer %q does not say %q", c.name, c.footer.text, want)
				}
				if !strings.Contains(c.alt, want) {
					t.Errorf("%s: alt text does not say %q:\n%s", c.name, want, c.alt)
				}
			}
			for i := 0; len(group) > 1 && i < len(group); i++ {
				c := cards[ResultCardIndex(group, i)]
				if c.footer.text != "" {
					t.Errorf("%s carries the footer; only the first card and the status card do", c.name)
				}
			}
		})
	}
}

// Steps left out of a long history show on the card and in its description,
// so neither reads as though the steps on either side followed one another.
func TestStatusCard_ShowsSkippedSteps(t *testing.T) {
	fonts, err := loadFontSet()
	if err != nil {
		t.Fatal(err)
	}
	group := testfixtures.ProcessNationalratMotion()
	p := *votes.GroupProcess(group)
	p.Steps = append([]votes.ProcessStep{p.Steps[0], {State: votes.StepSkipped, Skipped: 3}}, p.Steps[1:]...)
	if err := p.Validate(); err != nil {
		t.Fatalf("invalid: %v", err)
	}
	for i := range group {
		group[i].Affair.Process = &p
	}
	bg := SelectColor(group[0].Jurisdiction, group[0].Affair.Number)
	status := carouselCards(group, bg, fonts)[1]
	if !strings.Contains(status.alt, "… 3 weitere Schritte") {
		t.Errorf("alt text does not say steps were left out:\n%s", status.alt)
	}
	if got := statusSteps(&p, fonts, 2)[1].label; got != "3 weitere Schritte" {
		t.Errorf("skip label = %q", got)
	}
	if _, err := renderCard(status, bg, fonts); err != nil {
		t.Fatal(err)
	}
}
