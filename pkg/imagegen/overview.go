package imagegen

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
	"strconv"
	"strings"

	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"

	"github.com/siiitschiii/parlinfo/pkg/voteposting/voteformat"
	"github.com/siiitschiii/parlinfo/pkg/votes"
)

// The overview card opens a multi-vote carousel: the business title, how many
// of its votes passed and failed, then one row per vote with its number, a
// verdict mark, its label and its tally.
//
// It used to be the business title over a 28px list of "1. ✅ label" lines,
// and was the one card in the carousel with nothing to anchor it: every other
// card leads with a large verdict and a row of numbers between rules, while
// the overview floated a small list in the middle of an empty card and said
// nothing about how close any vote had been. The rows now carry the same rules
// and the same bold tallies as the result cards, and the counts above them give
// the card the large figures the others have.

// overviewRowSpec is one size the rows can be set at, largest first: the face
// size and the space above and below each row as a fraction of its line
// height. The largest that leaves the title whole is used, so a two-vote card
// gets generous rows and a ten-vote card dense ones.
type overviewRowSpec struct {
	size float64
	pad  float64
}

var overviewRowSpecs = []overviewRowSpec{
	{34, 0.45},
	{32, 0.38},
	{30, 0.30},
	{28, 0.24},
}

// rowFaces is one overviewRowSpec loaded: the label face, the bold face the
// number and the tally are set in, and the row padding.
type rowFaces struct {
	text, bold font.Face
	pad        float64
}

const (
	overviewNumGap   = 16 // number column to mark
	overviewMarkGap  = 18 // mark to label
	overviewTallyGap = 24 // label to tally
	// overviewMinLabel keeps a column for the label however wide the tallies:
	// an Auswahl vote with five options has a long one.
	overviewMinLabel = 260
)

// rowRule is the faint rule between rows. It is blended against the
// background rather than drawn with alpha, because the card is encoded as an
// opaque JPEG and draw's alpha would be dropped with it.
const rowRuleAlpha = 0x48

// markKind is what an overview row's mark says.
type markKind int

const (
	markNone     markKind = iota // no verdict to state: an outline with a dash
	markAccepted                 // white box, check
	markRejected                 // white box, cross
	markAuswahl                  // white box, the chosen option's letter
)

type overviewRow struct {
	num    string
	mark   markKind
	letter string
	lines  []string
	tally  string
}

// overviewLayout is the rows set at one size: the faces, the columns and the
// wrapped labels.
type overviewLayout struct {
	faces               rowFaces
	numW, markW, labelX int
	labelW, tallyRight  int
	rows                []overviewRow
}

// voteMark classifies a vote for its row's mark.
func voteMark(v votes.Vote) (markKind, string) {
	if voteformat.IsAuswahlVote(voteformat.CountsOf(v)) {
		label := strings.Trim(auswahlResultLabel(v.Decision), "[]")
		if label == "" {
			return markNone, ""
		}
		// The letter is what fits in the box; anything longer is a decision
		// string we do not recognise, and claims nothing.
		if len([]rune(label)) > 1 {
			return markNone, ""
		}
		return markAuswahl, label
	}
	if !voteformat.HasVerdict(v) {
		return markNone, ""
	}
	if voteformat.GetVoteResultEmoji(v.Decision) == "✅" {
		return markAccepted, ""
	}
	return markRejected, ""
}

// overviewTally is a vote's counts as its overview row prints them: Ja to Nein
// for an ordinary vote, the two numbers QuorumTally gives for a quorum vote,
// and each option that drew votes for an Auswahl vote. Abstentions and
// absences are left to the vote's own card; the row is for the comparison.
func overviewTally(v votes.Vote) string {
	c := voteformat.CountsOf(v)
	switch {
	case voteformat.IsAuswahlVote(c):
		return strings.Join(auswahlParts(c, " "), " · ")
	case voteformat.IsQuorumVote(c):
		support, without := voteformat.QuorumTally(c)
		return fmt.Sprintf("%d Zust. · %d ohne", support, without)
	default:
		return voteformat.FormatVoteCount(c.Ja) + " : " + voteformat.FormatVoteCount(c.Nein)
	}
}

// overviewTallyLong is overviewTally with its labels spelled out, for the alt
// text, where "62 : 51" would leave a listener to guess which side is which.
func overviewTallyLong(v votes.Vote) string {
	c := voteformat.CountsOf(v)
	switch {
	case voteformat.IsAuswahlVote(c):
		return strings.Join(auswahlParts(c, ": "), ", ")
	case voteformat.IsQuorumVote(c):
		support, without := voteformat.QuorumTally(c)
		return fmt.Sprintf("%d Zustimmungen, %d ohne Zustimmung", support, without)
	default:
		return fmt.Sprintf("%s Ja, %s Nein", voteformat.FormatVoteCount(c.Ja), voteformat.FormatVoteCount(c.Nein))
	}
}

func auswahlParts(c voteformat.VoteCounts, sep string) []string {
	var parts []string
	for i, p := range []*int{c.A, c.B, c.C, c.D, c.E} {
		if p != nil && *p > 0 {
			parts = append(parts, string(rune('A'+i))+sep+strconv.Itoa(*p))
		}
	}
	return parts
}

// verdictTotals counts the group's accepted and rejected votes. ok is false
// when no vote states a verdict, and the card then says how many votes there
// were instead of printing two zeros it has no grounds for.
func verdictTotals(group []votes.Vote) (accepted, rejected int, ok bool) {
	for _, v := range group {
		switch k, _ := voteMark(v); k {
		case markAccepted:
			accepted++
		case markRejected:
			rejected++
		}
	}
	return accepted, rejected, accepted+rejected > 0
}

// buildOverview sets the group's rows in faces, with labels cut to maxRunes
// (zero leaves them whole).
func buildOverview(group []votes.Vote, faces rowFaces, maxRunes int) overviewLayout {
	ol := overviewLayout{faces: faces, tallyRight: imgWidth - padding}
	ol.numW = font.MeasureString(faces.bold, strconv.Itoa(len(group))).Ceil()
	ol.markW = markSize(faces.text)
	ol.labelX = padding + ol.numW + overviewNumGap + ol.markW + overviewMarkGap

	tallyW := 0
	for _, v := range group {
		if w := font.MeasureString(faces.bold, overviewTally(v)).Ceil(); w > tallyW {
			tallyW = w
		}
	}
	ol.labelW = ol.tallyRight - tallyW - overviewTallyGap - ol.labelX
	if ol.labelW < overviewMinLabel {
		ol.labelW = overviewMinLabel
	}

	for i, v := range group {
		label := voteformat.SubVoteLabel(v, i, len(group))
		if maxRunes > 0 {
			label = truncateWithEllipsis(label, maxRunes)
		}
		kind, letter := voteMark(v)
		ol.rows = append(ol.rows, overviewRow{
			num:    strconv.Itoa(i + 1),
			mark:   kind,
			letter: letter,
			lines:  wrapText(faces.text, label, ol.labelW),
			tally:  overviewTally(v),
		})
	}
	return ol
}

// markSize is the side of a row's mark: a little taller than the label's
// capitals, so it reads as an icon rather than as a letter.
func markSize(face font.Face) int {
	return int(float64(face.Metrics().Ascent.Ceil()) * 1.12)
}

func (ol overviewLayout) rowPad() int {
	return frac(ol.faces.text, ol.faces.pad)
}

func (ol overviewLayout) rowHeight(r overviewRow) int {
	lines := len(r.lines)
	if lines == 0 {
		lines = 1
	}
	return 2*ol.rowPad() + lines*lineHeight(ol.faces.text)
}

func (ol overviewLayout) height() int {
	h := 0
	for _, r := range ol.rows {
		h += ol.rowHeight(r)
	}
	return h
}

// overviewHeadHeight is the space between the gap after the title and the
// first row's rule: the counts (or the "N Abstimmungen" line that stands in for
// them) and the gap under them.
func overviewHeadHeight(fonts *fontSet, withTotals bool) int {
	h := lineHeight(fonts.partyBold)
	if withTotals {
		h = lineHeight(fonts.statNum) + lineHeight(fonts.statLabel)
	}
	return h + frac(fonts.statLabel, 0.9)
}

// fitOverview sizes the title and the rows against each other.
//
// The title is what matters most, so the rows give way to it, in two steps:
// first the rows shrink, through overviewRowSpecs, with every label whole;
// only if the title still cannot be shown whole at the smallest are the labels
// cut to summarySubtitleMaxRunes. If even that leaves no room, the last
// attempt is used, title ellipsised.
//
// "Whole" is not quite enough on its own: the ladder will happily set a long
// title at 26pt above rows at 34pt, and a card whose list outshouts its subject
// reads upside down. So the first choice is the largest rows that leave the
// title whole and visibly larger than they are; failing that, the largest that
// leave it whole at all.
func fitOverview(group []votes.Vote, inset, maxWidth int, fonts *fontSet) (font.Face, []string, overviewLayout, error) {
	title := voteformat.CleanVoteTitle(group[0].Title)
	_, _, withTotals := verdictTotals(group)

	type attempt struct {
		face  font.Face
		lines []string
		ol    overviewLayout
	}
	var fallback, last *attempt
	for _, maxRunes := range []int{0, summarySubtitleMaxRunes} {
		for _, faces := range fonts.overviewRows {
			ol := buildOverview(group, faces, maxRunes)
			reserve := titleTrailingGap() + overviewHeadHeight(fonts, withTotals) + ol.height() + padding

			face, lines, whole, err := cardTitleFully(group, title, maxWidth, titleBudget(inset, reserve))
			if err != nil {
				return nil, nil, overviewLayout{}, err
			}
			a := &attempt{face, lines, ol}
			last = a
			if !whole {
				continue
			}
			if lineHeight(face)*10 >= lineHeight(faces.text)*11 {
				return a.face, a.lines, a.ol, nil
			}
			if fallback == nil {
				fallback = a
			}
		}
	}
	if fallback == nil {
		fallback = last
	}
	return fallback.face, fallback.lines, fallback.ol, nil
}

func layoutTitleCard(img *image.RGBA, cur *layoutCursor, inset int, group []votes.Vote, bg color.RGBA, fonts *fontSet) {
	titleFace, titleLines, ol, err := fitOverview(group, inset, imgWidth-2*padding, fonts)
	if err != nil {
		return
	}

	for _, line := range titleLines {
		if img != nil {
			drawCenteredText(img, titleFace, nil, cur.baseline(titleFace), line, bg)
		}
		cur.advance(titleFace)
		cur.gap(titleFace, titleLineGapFactor)
	}

	// The gap titleTrailingGap reserves.
	cur.gap(fonts.regular, 0.75)
	accepted, rejected, withTotals := verdictTotals(group)
	if withTotals {
		drawVerdictTotals(img, cur, accepted, rejected, bg, fonts)
	} else {
		if img != nil {
			drawCenteredText(img, fonts.partyBold, nil, cur.baseline(fonts.partyBold),
				fmt.Sprintf("%d Abstimmungen", len(group)), bg)
		}
		cur.advance(fonts.partyBold)
	}
	cur.gap(fonts.statLabel, 0.9)

	if img != nil {
		drawHLine(img, cur.y, padding, imgWidth-padding, semiWhite)
	}
	for i, r := range ol.rows {
		drawOverviewRow(img, cur, ol, r, bg)
		if img != nil && i < len(ol.rows)-1 {
			drawBlendedLine(img, cur.y, padding, imgWidth-padding, bg, rowRuleAlpha)
		}
	}
	if img != nil {
		drawHLine(img, cur.y, padding, imgWidth-padding, semiWhite)
	}
}

// drawVerdictTotals draws "N angenommen | M abgelehnt" as two stat columns
// either side of a short vertical rule, in the faces the result cards use for
// their Ja/Nein/Enth. dashboard.
func drawVerdictTotals(img *image.RGBA, cur *layoutCursor, accepted, rejected int, bg color.RGBA, fonts *fontSet) {
	const halfSpan = 170
	cols := []struct {
		cx           int
		value, label string
	}{
		{imgWidth/2 - halfSpan, strconv.Itoa(accepted), "angenommen"},
		{imgWidth/2 + halfSpan, strconv.Itoa(rejected), "abgelehnt"},
	}
	top := cur.y
	if img != nil {
		for _, c := range cols {
			w := font.MeasureString(fonts.statNum, c.value).Ceil()
			drawShadowedText(img, fonts.statNum, nil, c.cx-w/2, cur.baseline(fonts.statNum), c.value, bg)
		}
	}
	cur.advance(fonts.statNum)
	if img != nil {
		for _, c := range cols {
			w := font.MeasureString(fonts.statLabel, c.label).Ceil()
			drawShadowedText(img, fonts.statLabel, nil, c.cx-w/2, cur.baseline(fonts.statLabel), c.label, bg)
		}
	}
	cur.advance(fonts.statLabel)
	if img != nil {
		inset := (cur.y - top) / 8
		drawBlendedVLine(img, imgWidth/2, top+inset, cur.y-inset, bg, 0xA0)
	}
}

// drawOverviewRow draws one row: the number right-aligned in its column, the
// mark, the wrapped label and the tally right-aligned to the margin, all on
// the label's first line.
func drawOverviewRow(img *image.RGBA, cur *layoutCursor, ol overviewLayout, r overviewRow, bg color.RGBA) {
	top := cur.y
	cur.y += ol.rowPad()
	if img != nil {
		text, bold := ol.faces.text, ol.faces.bold
		base := cur.baseline(text)

		numW := font.MeasureString(bold, r.num).Ceil()
		drawShadowedText(img, bold, nil, padding+ol.numW-numW, base, r.num, bg)

		// The mark is centred on the label's capitals rather than on its line
		// box, which the descender space would pull low.
		capTop := base - text.Metrics().Ascent.Ceil()*72/100
		markX := padding + ol.numW + overviewNumGap
		markY := (capTop+base)/2 - ol.markW/2
		drawMark(img, markX, markY, ol.markW, r, bold, bg)

		for i, line := range r.lines {
			drawShadowedText(img, text, nil, ol.labelX, base+i*lineHeight(text), line, bg)
		}

		tallyW := font.MeasureString(bold, r.tally).Ceil()
		drawShadowedText(img, bold, nil, ol.tallyRight-tallyW, base, r.tally, bg)
	}
	cur.y = top + ol.rowHeight(r)
}

// drawMark draws a row's verdict mark in a size×size square at (x, y).
//
// The shapes are drawn at markSupersample times their size and scaled down,
// which is the cheapest way to anti-alias a check stroke with the standard
// library's rasterless image types. The emoji face is not used: Noto Emoji's
// ✅ and ❌ are outline glyphs of different weights, and the rows read better
// with one solid shape that differs only in its stroke.
func drawMark(img *image.RGBA, x, y, size int, r overviewRow, letterFace font.Face, bg color.RGBA) {
	box := markMask(r.mark, size)
	drawMask(img, box, x+shadowOff, y+shadowOff, darken(bg))

	switch r.mark {
	case markNone:
		drawMask(img, box, x, y, blend(bg, rowNoneAlpha))
	default:
		drawMask(img, box, x, y, color.RGBA{0xFF, 0xFF, 0xFF, 0xFF})
		if r.mark == markAuswahl {
			w := font.MeasureString(letterFace, r.letter).Ceil()
			m := letterFace.Metrics()
			capH := m.Ascent.Ceil() * 72 / 100
			d := &font.Drawer{Dst: img, Src: image.NewUniform(bg), Face: letterFace,
				Dot: fixed.P(x+(size-w)/2, y+(size+capH)/2)}
			d.DrawString(r.letter)
			return
		}
		drawMask(img, strokeMask(r.mark, size), x, y, bg)
	}
}

// rowNoneAlpha is how strongly the no-verdict outline shows: dimmer than a
// verdict, so a row that claims nothing does not look like one that does.
const rowNoneAlpha = 0xA8

const markSupersample = 4

// markMask is the box of a mark as an alpha mask: filled for a verdict, an
// outline with a dash for markNone.
func markMask(kind markKind, size int) *image.Alpha {
	s := float64(size * markSupersample)
	hi := image.NewAlpha(image.Rect(0, 0, size*markSupersample, size*markSupersample))
	radius := s * 0.18
	stroke := s * 0.08
	for py := 0; py < hi.Rect.Dy(); py++ {
		for px := 0; px < hi.Rect.Dx(); px++ {
			fx, fy := float64(px)+0.5, float64(py)+0.5
			d := roundedBoxDist(fx, fy, s, radius)
			in := d <= 0
			if kind == markNone {
				in = in && d > -stroke
			}
			if in {
				hi.SetAlpha(px, py, color.Alpha{0xFF})
			}
		}
	}
	if kind == markNone {
		// The dash.
		strokeLine(hi, s*0.32, s*0.5, s*0.68, s*0.5, s*0.11)
	}
	return downsample(hi, size)
}

// strokeMask is the check or the cross a verdict mark carries, as an alpha
// mask the size of the box.
func strokeMask(kind markKind, size int) *image.Alpha {
	s := float64(size * markSupersample)
	hi := image.NewAlpha(image.Rect(0, 0, size*markSupersample, size*markSupersample))
	w := s * 0.14
	switch kind {
	case markAccepted:
		strokeLine(hi, s*0.24, s*0.53, s*0.42, s*0.71, w)
		strokeLine(hi, s*0.42, s*0.71, s*0.76, s*0.30, w)
	case markRejected:
		strokeLine(hi, s*0.30, s*0.30, s*0.70, s*0.70, w)
		strokeLine(hi, s*0.70, s*0.30, s*0.30, s*0.70, w)
	}
	return downsample(hi, size)
}

// roundedBoxDist is the signed distance from (x, y) to the edge of an s×s box
// with corners of radius r: negative inside.
func roundedBoxDist(x, y, s, r float64) float64 {
	h := s / 2
	qx := math.Abs(x-h) - (h - r)
	qy := math.Abs(y-h) - (h - r)
	outside := math.Hypot(math.Max(qx, 0), math.Max(qy, 0))
	return outside + math.Min(math.Max(qx, qy), 0) - r
}

// strokeLine sets every pixel within w/2 of the segment, with round caps.
func strokeLine(m *image.Alpha, x1, y1, x2, y2, w float64) {
	dx, dy := x2-x1, y2-y1
	l2 := dx*dx + dy*dy
	r := w / 2
	minX, maxX := int(math.Min(x1, x2)-r), int(math.Max(x1, x2)+r)+1
	minY, maxY := int(math.Min(y1, y2)-r), int(math.Max(y1, y2)+r)+1
	for py := minY; py <= maxY; py++ {
		for px := minX; px <= maxX; px++ {
			fx, fy := float64(px)+0.5, float64(py)+0.5
			t := ((fx-x1)*dx + (fy-y1)*dy) / l2
			t = math.Max(0, math.Min(1, t))
			if math.Hypot(fx-(x1+t*dx), fy-(y1+t*dy)) <= r {
				m.SetAlpha(px, py, color.Alpha{0xFF})
			}
		}
	}
}

func downsample(hi *image.Alpha, size int) *image.Alpha {
	lo := image.NewAlpha(image.Rect(0, 0, size, size))
	xdraw.BiLinear.Scale(lo, lo.Rect, hi, hi.Rect, draw.Src, nil)
	return lo
}

// drawMask paints c through mask with its top-left corner at (x, y).
func drawMask(img *image.RGBA, mask *image.Alpha, x, y int, c color.RGBA) {
	r := mask.Rect.Add(image.Pt(x, y))
	draw.DrawMask(img, r, image.NewUniform(c), image.Point{}, mask, image.Point{}, draw.Over)
}

// blend mixes white into bg at alpha a, giving the opaque colour a
// translucent white line would show on this background.
func blend(bg color.RGBA, a uint8) color.RGBA {
	mix := func(c uint8) uint8 { return uint8((int(c)*(255-int(a)) + 255*int(a)) / 255) }
	return color.RGBA{mix(bg.R), mix(bg.G), mix(bg.B), 0xFF}
}

func drawBlendedLine(img *image.RGBA, y, x1, x2 int, bg color.RGBA, a uint8) {
	c := blend(bg, a)
	for x := x1; x <= x2; x++ {
		img.SetRGBA(x, y, c)
	}
}

func drawBlendedVLine(img *image.RGBA, x, y1, y2 int, bg color.RGBA, a uint8) {
	c := blend(bg, a)
	for y := y1; y <= y2; y++ {
		img.SetRGBA(x, y, c)
		img.SetRGBA(x+1, y, c)
	}
}
