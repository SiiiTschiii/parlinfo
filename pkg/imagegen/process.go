package imagegen

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"strings"

	"golang.org/x/image/font"

	"github.com/siiitschiii/parlinfo/pkg/voteposting/voteformat"
	"github.com/siiitschiii/parlinfo/pkg/votes"
)

// Where a business stands in its process (votes.Process) reaches the carousel
// twice: as a one-line footer on the first card, saying what comes next, and as
// a status card of its own, second in the carousel, showing the whole path.
//
// The footer is what a reader scrolling past sees, next to the result it
// follows from. The status card is for the reader who stops: it says who has
// acted, what they did, and who acts next, in the sentence each step needs.

// HasStatusCard reports whether GenerateCards draws a status card for the
// group. It does whenever the group reports a process (votes.GroupProcess).
func HasStatusCard(group []votes.Vote) bool {
	return votes.GroupProcess(group) != nil
}

// ResultCardIndex is the carousel index of vote i's result card in a
// multi-vote group: after the overview and, where there is one, the status
// card. A lone vote has no result card of its own; its combined card is
// index 0.
func ResultCardIndex(group []votes.Vote, i int) int {
	if HasStatusCard(group) {
		return i + 2
	}
	return i + 1
}

// stepLabelSize is the size a status card sets each step's label at, in bold.
// Its detail is set below it in the 30pt face the Fraktion table uses.
const stepLabelSize = 38

// ---------------------------------------------------------------------------
// The footer

// cardFooter is the next-step line at the bottom of a card: a bold label and a
// sentence, centred, wrapped to at most footerMaxLines.
//
// It is reserved like the corner row rather than overlaid: card.inset charges
// it to the content above, so a card with a footer gives up title size, never a
// Fraktion row, to make room for it.
type cardFooter struct {
	label, text string
}

// footerMaxLines bounds the footer. Next-step sentences are short by design;
// one longer than this is cut rather than allowed to take the card's results.
const footerMaxLines = 2

// nextStepFooter is the footer a group's first card carries, or the zero
// footer when the group reports no process.
func nextStepFooter(group []votes.Vote) cardFooter {
	label, text := voteformat.NextStep(group)
	return cardFooter{label: label, text: text}
}

// footerWord is one word of the footer and whether it belongs to the label.
type footerWord struct {
	text string
	bold bool
}

// lines wraps the footer to the column, label words in bold. A line is a run
// of words, so each can be drawn in its own face.
func (f cardFooter) lines(fonts *fontSet) [][]footerWord {
	if f.text == "" {
		return nil
	}
	var words []footerWord
	for _, w := range strings.Fields(f.label) {
		words = append(words, footerWord{w, true})
	}
	for _, w := range strings.Fields(f.text) {
		words = append(words, footerWord{w, false})
	}

	maxWidth := imgWidth - 2*padding
	var lines [][]footerWord
	var line []footerWord
	for _, w := range words {
		if len(line) > 0 && footerWidth(append(line[:len(line):len(line)], w), fonts) > maxWidth {
			lines = append(lines, line)
			line = nil
		}
		line = append(line, w)
	}
	lines = append(lines, line)

	if len(lines) > footerMaxLines {
		lines = lines[:footerMaxLines]
		last := lines[footerMaxLines-1]
		for len(last) > 1 && footerWidth(append(last[:len(last):len(last)], footerWord{"…", false}), fonts) > maxWidth {
			last = last[:len(last)-1]
		}
		lines[footerMaxLines-1] = append(last[:len(last):len(last)], footerWord{"…", false})
	}
	return lines
}

func footerFace(w footerWord, fonts *fontSet) font.Face {
	if w.bold {
		return fonts.partyBold
	}
	return fonts.partyNum
}

// footerWidth is the width of a run of words joined by single spaces.
func footerWidth(words []footerWord, fonts *fontSet) int {
	w := 0
	for i, word := range words {
		if i > 0 {
			w += font.MeasureString(fonts.partyNum, " ").Ceil()
		}
		w += font.MeasureString(footerFace(word, fonts), word.text).Ceil()
	}
	return w
}

// footerGap is the space between a card's content and its footer, with a faint
// rule across its middle.
func footerGap(fonts *fontSet) int {
	return frac(fonts.partyNum, 1.2)
}

// height is the space the footer takes from the bottom of the content area:
// the gap above it and its lines. A footer with nothing to say reserves
// nothing.
func (f cardFooter) height(fonts *fontSet) int {
	n := len(f.lines(fonts))
	if n == 0 {
		return 0
	}
	return footerGap(fonts) + n*lineHeight(fonts.partyNum)
}

// draw prints the footer above the bottom margin, under a faint rule.
func (f cardFooter) draw(img *image.RGBA, fonts *fontSet, bg color.RGBA) {
	lines := f.lines(fonts)
	if img == nil || len(lines) == 0 {
		return
	}
	lh := lineHeight(fonts.partyNum)
	top := imgHeight - padding - len(lines)*lh
	drawBlendedLine(img, top-footerGap(fonts)/2, padding, imgWidth-padding, bg, rowRuleAlpha)

	space := font.MeasureString(fonts.partyNum, " ").Ceil()
	for i, line := range lines {
		x := (imgWidth - footerWidth(line, fonts)) / 2
		y := top + i*lh + fonts.partyNum.Metrics().Ascent.Ceil()
		for _, w := range line {
			face := footerFace(w, fonts)
			drawShadowedText(img, face, nil, x, y, w.text, bg)
			x += font.MeasureString(face, w.text).Ceil() + space
		}
	}
}

// ---------------------------------------------------------------------------
// The status card

// statusStep is one step as the status card draws it: its label, its detail
// wrapped to the column, and its state.
type statusStep struct {
	label  string
	detail []string
	state  votes.StepState
}

// stepMarkSize is the diameter of a step's marker, and stepMarkCol the column
// it is centred in; the step's text starts after the column.
const (
	stepMarkSize = 48
	stepMarkCol  = 68
	// stepMarkHalo is how far the ring around this vote's step stands out
	// from its disc. It has to fit the marker column, and the line between
	// two markers stops short of it.
	stepMarkHalo = 8
	stepTextGap  = 20
)

// statusMinTitle is the room a status card keeps for the business title: two
// lines at the smallest title size. A card whose steps would leave less sets
// each step's detail on one line rather than two — see layoutStatusCard.
var statusMinTitle = int(math.Ceil(2 * titleFontMin * 1.2 * (1 + titleLineGapFactor)))

// statusSteps wraps each step's detail to at most maxLines, cutting what does
// not fit with an ellipsis. Details are meant to be one short sentence; the
// bound keeps one that is not from crowding out the title.
func statusSteps(p *votes.Process, fonts *fontSet, maxLines int) []statusStep {
	width := imgWidth - 2*padding - stepMarkCol - stepTextGap
	steps := make([]statusStep, len(p.Steps))
	for i, s := range p.Steps {
		if s.State == votes.StepSkipped {
			steps[i] = statusStep{label: skippedLabel(s.Skipped), state: s.State}
			continue
		}
		detail := wrapText(fonts.partyNum, s.Detail, width)
		detail = ellipsizeLines(fonts.partyNum, detail, width, titleBlockHeight(fonts.partyNum, maxLines))
		steps[i] = statusStep{label: s.Label, detail: detail, state: s.State}
	}
	return steps
}

// skippedLabel is what the card prints where steps were left out: the count,
// so a reader knows the path was longer than the card shows.
func skippedLabel(n int) string {
	if n == 1 {
		return "1 weiterer Schritt"
	}
	return fmt.Sprintf("%d weitere Schritte", n)
}

// stepHeight is the space one step takes: its label line and its detail.
func stepHeight(s statusStep, fonts *fontSet) int {
	return lineHeight(fonts.stepLabel) + len(s.detail)*lineHeight(fonts.partyNum)
}

// stepGap is the space between two steps.
func stepGap(fonts *fontSet) int {
	return frac(fonts.partyNum, 0.8)
}

func stepsHeight(steps []statusStep, fonts *fontSet) int {
	h := 0
	for i, s := range steps {
		h += stepHeight(s, fonts)
		if i < len(steps)-1 {
			h += stepGap(fonts)
		}
	}
	return h
}

// statusHeading is the line above the business title naming the business as
// the source does: its type and number ("Motion 24.3004"), or the number alone
// where the source gives no type ("2026/519"), or "" when it gives neither.
//
// The card carries no heading of ours saying what it shows. The ticks, the ring
// and the next-step footer say that, and a label in our words would be the one
// text on the card that is not the parliament's.
func statusHeading(v votes.Vote) string {
	return strings.TrimSpace(strings.TrimSpace(v.Affair.Type) + " " + strings.TrimSpace(v.Affair.Number))
}

// statusHeadingHeight is the space the heading line takes, or 0 without one.
func statusHeadingHeight(v votes.Vote, fonts *fontSet) int {
	if statusHeading(v) == "" {
		return 0
	}
	return lineHeight(fonts.small) + frac(fonts.small, 0.6)
}

// statusReserve is the space the status card needs for everything but the
// title, mirroring layoutStatusCard.
func statusReserve(v votes.Vote, steps []statusStep, fonts *fontSet) int {
	reserve := statusHeadingHeight(v, fonts)
	reserve += titleTrailingGap() + frac(fonts.stepLabel, 0.75)
	reserve += stepsHeight(steps, fonts)
	return reserve + padding
}

// layoutStatusCard draws the business's path: its type and number, the title, a rule,
// then one row per step with a marker on a line that runs through them —
// solid through the steps taken, dashed through the ones to come.
func layoutStatusCard(img *image.RGBA, cur *layoutCursor, inset int, group []votes.Vote, bg color.RGBA, fonts *fontSet) error {
	p := votes.GroupProcess(group)
	if p == nil {
		return fmt.Errorf("status card for a group without a process")
	}
	v := group[0]
	maxTextWidth := imgWidth - 2*padding
	// Two lines per detail where the card has the room, one where it does
	// not: a business with votes.MaxProcessSteps steps, each detail running to
	// two lines, under a two-line footer would otherwise leave the title
	// nothing.
	steps := statusSteps(p, fonts, 2)
	if titleBudget(inset, statusReserve(v, steps, fonts)) < statusMinTitle {
		steps = statusSteps(p, fonts, 1)
	}

	titleFace, titleLines, err := fitTitle(voteformat.CleanVoteTitle(v.Title), maxTextWidth, titleBudget(inset, statusReserve(v, steps, fonts)))
	if err != nil {
		return err
	}

	if heading := statusHeading(v); heading != "" {
		if img != nil {
			drawCenteredText(img, fonts.small, nil, cur.baseline(fonts.small), heading, bg)
		}
		cur.advance(fonts.small)
		cur.gap(fonts.small, 0.6)
	}

	for _, line := range titleLines {
		if img != nil {
			drawCenteredText(img, titleFace, nil, cur.baseline(titleFace), line, bg)
		}
		cur.advance(titleFace)
		cur.gap(titleFace, titleLineGapFactor)
	}
	cur.gap(titleFace, 0.75)
	if img != nil {
		drawHLine(img, cur.y, padding, imgWidth-padding, semiWhite)
	}
	cur.gap(fonts.stepLabel, 0.75)

	markX := padding + (stepMarkCol-stepMarkSize)/2
	textX := padding + stepMarkCol + stepTextGap
	var prevMarkBottom int
	for i, s := range steps {
		base := cur.baseline(fonts.stepLabel)
		capTop := base - fonts.stepLabel.Metrics().Ascent.Ceil()*72/100
		markY := (capTop+base)/2 - stepMarkSize/2

		if img != nil {
			if i > 0 {
				// The line into this step: solid where the business has been,
				// dashed where it has yet to go.
				// Dotted, too, into and out of steps left out: the line does not
				// claim the steps on either side follow one another.
				dashed := s.state == votes.StepNext || s.state == votes.StepSkipped || steps[i-1].state == votes.StepSkipped
				drawStepConnector(img, markX+stepMarkSize/2, prevMarkBottom+stepMarkHalo+4, markY-stepMarkHalo-4, dashed, bg)
			}
			drawStepMark(img, markX, markY, s.state, bg)
			if s.state == votes.StepSkipped {
				drawTrackedText(img, fonts.partyNum, textX, base, s.label, 0, blend(bg, 0xB0))
			} else {
				drawShadowedText(img, fonts.stepLabel, nil, textX, base, s.label, bg)
			}
			for j, line := range s.detail {
				y := cur.y + lineHeight(fonts.stepLabel) + j*lineHeight(fonts.partyNum) + fonts.partyNum.Metrics().Ascent.Ceil()
				drawShadowedText(img, fonts.partyNum, nil, textX, y, line, bg)
			}
		}
		prevMarkBottom = markY + stepMarkSize
		cur.y += stepHeight(s, fonts)
		if i < len(steps)-1 {
			cur.y += stepGap(fonts)
		}
	}
	return nil
}

// drawStepConnector draws the vertical line between two markers.
func drawStepConnector(img *image.RGBA, x, y1, y2 int, dashed bool, bg color.RGBA) {
	c := color.RGBA{0xFF, 0xFF, 0xFF, 0xFF}
	if dashed {
		c = blend(bg, 0x80)
	}
	for y := y1; y <= y2; y++ {
		if dashed && (y-y1)%14 >= 8 {
			continue
		}
		for dx := -2; dx <= 1; dx++ {
			img.SetRGBA(x+dx, y, c)
		}
	}
}

// drawStepMark draws a step's marker: a white disc with a check for a step
// taken, and a dashed ring for a step to come. The step this vote took is a
// step taken too, so it gets the same disc — with a cross where it ended the
// business — and a ring around it, which is what marks it as this post's vote.
func drawStepMark(img *image.RGBA, x, y int, state votes.StepState, bg color.RGBA) {
	white := color.RGBA{0xFF, 0xFF, 0xFF, 0xFF}
	size := stepMarkSize
	switch state {
	case votes.StepDone, votes.StepCurrent, votes.StepEnded, votes.StepCompleted:
		if state != votes.StepDone {
			// The ring sits outside the disc, so the check inside is the same
			// size as on every other step taken.
			halo := size + 2*stepMarkHalo
			ring := circleMask(halo, 0.06, 0)
			drawMask(img, ring, x-stepMarkHalo+shadowOff, y-stepMarkHalo+shadowOff, darken(bg))
			drawMask(img, ring, x-stepMarkHalo, y-stepMarkHalo, white)
		}
		disc := circleMask(size, 0, 0)
		drawMask(img, disc, x+shadowOff, y+shadowOff, darken(bg))
		drawMask(img, disc, x, y, white)
		kind := markAccepted
		if state == votes.StepEnded {
			kind = markRejected
		}
		drawMask(img, strokeMask(kind, size), x, y, bg)
	case votes.StepNext:
		drawMask(img, circleMask(size, 0.09, 0, 12), x, y, blend(bg, 0x99))
	case votes.StepSkipped:
		// Three dots down the line, where a marker would stand.
		dot := circleMask(size/6, 0, 0)
		for i := -1; i <= 1; i++ {
			drawMask(img, dot, x+size/2-size/12, y+size/2-size/12+i*size/4, blend(bg, 0xB0))
		}
	}
}

// circleMask is a disc, or a ring of the given width, as an alpha mask the size
// of the marker; widths and inset are fractions of the diameter. With dashes,
// the ring is broken into that many dashes.
func circleMask(size int, ring, inset float64, dashes ...int) *image.Alpha {
	s := float64(size * markSupersample)
	hi := image.NewAlpha(image.Rect(0, 0, size*markSupersample, size*markSupersample))
	r := s/2 - s*inset
	inner := -1.0
	if ring > 0 {
		inner = r - s*ring
	}
	for py := 0; py < hi.Rect.Dy(); py++ {
		for px := 0; px < hi.Rect.Dx(); px++ {
			dx, dy := float64(px)+0.5-s/2, float64(py)+0.5-s/2
			d := math.Hypot(dx, dy)
			if d > r || d < inner {
				continue
			}
			if len(dashes) > 0 && dashes[0] > 0 {
				a := (math.Atan2(dy, dx) + math.Pi) / (2 * math.Pi) * float64(2*dashes[0])
				if int(a)%2 == 1 {
					continue
				}
			}
			hi.SetAlpha(px, py, color.Alpha{0xFF})
		}
	}
	return downsample(hi, size)
}

// ---------------------------------------------------------------------------
// Alt text

// statusAlt describes the status card: the heading and title, then the next
// step, then every step with its state.
//
// The next step comes before the steps, not after them as on the card: it is
// what the card's footer says, and a description cut for length cuts from the
// end. Six steps with long details can fill the cap on their own.
func statusAlt(group []votes.Vote) string {
	p := votes.GroupProcess(group)
	if p == nil {
		return ""
	}
	v := group[0]
	label := voteformat.CleanVoteTitle(v.Title)
	if heading := statusHeading(v); heading != "" {
		label = heading + "\n\n" + label
	}

	var lines []string
	for _, s := range p.Steps {
		var state string
		switch s.State {
		case votes.StepDone:
			state = "Erledigt"
		case votes.StepCurrent, votes.StepEnded, votes.StepCompleted:
			state = "Diese Abstimmung"
		case votes.StepNext:
			state = "Noch offen"
		case votes.StepSkipped:
			lines = append(lines, "… "+skippedLabel(s.Skipped))
			continue
		}
		line := state + ": " + s.Label
		if d := strings.TrimSpace(s.Detail); d != "" {
			line += " – " + d
		}
		lines = append(lines, line)
	}
	result := strings.Join(lines, "\n")
	if next := voteformat.NextStepLine(group); next != "" {
		result = next + "\n\n" + result
	}
	return assembleAlt(label, result)
}
