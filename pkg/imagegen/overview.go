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
// votes it holds, then one row per vote with its number, a verdict mark, its
// label and its tally.
//
// It used to be the business title over a 28px list of "1. ✅ label" lines,
// and was the one card in the carousel with nothing to anchor it: every other
// card leads with a large verdict and a row of numbers between rules, while
// the overview floated a small list in the middle of an empty card and said
// nothing about how close any vote had been. The rows now carry the same rules
// and the same bold tallies as the result cards.

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

// A short group — a business with two or three votes — would leave half the
// card empty at the sizes that suit a long one. It gets larger rows to try
// first, and a title ladder that starts at overviewShortTitleMax rather than
// titleFontMax, so the card is filled by the same content set larger rather
// than by space.
const (
	overviewShortGroup    = 3
	overviewShortTitleMax = 52.0
)

var overviewShortRowSpecs = []overviewRowSpec{
	{42, 0.5},
	{38, 0.45},
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
	markNone     markKind = iota // no verdict to state: no mark at all
	markAccepted                 // white box, check
	markRejected                 // white box, cross
	markAuswahl                  // white box, the chosen option's letter
	markOmitted                  // no mark: the "… und N weitere" row
)

type overviewRow struct {
	num    string
	mark   markKind
	letter string
	lines  []string
	tally  []string // one line, or two for an Auswahl vote with three or more options
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

// overviewTallyLines is overviewTally as the row draws it. An Auswahl vote
// with three or more options is split over two lines, the first taking the
// larger half: such a vote's label nearly always wraps to two lines anyway,
// so the second tally line costs no height, and the label gets back the
// width a one-line "A 50 · B 24 · C 40" would take from it.
func overviewTallyLines(v votes.Vote) []string {
	c := voteformat.CountsOf(v)
	if parts := auswahlParts(c, " "); voteformat.IsAuswahlVote(c) && len(parts) >= 3 {
		split := (len(parts) + 1) / 2
		return []string{strings.Join(parts[:split], " · "), strings.Join(parts[split:], " · ")}
	}
	return []string{overviewTally(v)}
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
		return fmt.Sprintf("%s %s, %s %s", voteformat.FormatVoteCount(c.Ja), voteformat.T(c.Language, voteformat.MsgYes),
			voteformat.FormatVoteCount(c.Nein), voteformat.T(c.Language, voteformat.MsgNo))
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

// buildOverview sets the group's rows in faces, with labels cut to maxRunes
// (zero leaves them whole).
func buildOverview(group []votes.Vote, faces rowFaces, maxRunes int) overviewLayout {
	ol := overviewLayout{faces: faces, tallyRight: imgWidth - padding}
	ol.numW = font.MeasureString(faces.bold, strconv.Itoa(len(group))).Ceil()
	// A row that states no verdict draws no mark, so a group where no row
	// states one — most Kantonsrat votes — gives the mark column's width to
	// the labels rather than leaving it empty down the card.
	ol.labelX = padding + ol.numW + overviewNumGap
	for _, v := range group {
		if kind, _ := voteMark(v); kind != markNone {
			ol.markW = markSize(faces.text)
			ol.labelX += ol.markW + overviewMarkGap
			break
		}
	}

	tallyW := 0
	for _, v := range group {
		for _, line := range overviewTallyLines(v) {
			if w := font.MeasureString(faces.bold, line).Ceil(); w > tallyW {
				tallyW = w
			}
		}
	}
	ol.labelW = ol.tallyRight - tallyW - overviewTallyGap - ol.labelX
	if ol.labelW < overviewMinLabel {
		ol.labelW = overviewMinLabel
	}

	for i, v := range group {
		label := overviewLabel(v, i, len(group))
		if maxRunes > 0 {
			label = truncateWithEllipsis(label, maxRunes)
		}
		kind, letter := voteMark(v)
		ol.rows = append(ol.rows, overviewRow{
			num:    strconv.Itoa(i + 1),
			mark:   kind,
			letter: letter,
			lines:  wrapText(faces.text, label, ol.labelW),
			tally:  overviewTallyLines(v),
		})
	}
	return ol
}

// overviewLabel is the vote's label as its row prints it: SubVoteLabel without
// the " · Quorum" suffix, which the row's tally ("129 Zust. · 51 ohne")
// already says. Other type labels stay: an Ausgabenbremse is tallied the same
// way as a quorum vote but is a different procedure, and only the label says
// which.
func overviewLabel(v votes.Vote, index, groupSize int) string {
	label := voteformat.SubVoteLabel(v, index, groupSize)
	if strings.TrimSpace(v.Type) == "Quorum" {
		label = strings.TrimSuffix(label, " · "+voteformat.TypeLabel(v.Type))
	}
	return label
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
	lines := max(len(r.lines), len(r.tally), 1)
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
// first row's rule: the "N Abstimmungen" line and the gap under it.
//
// The line says how many votes the list holds and no more. A tally of how
// many passed and failed was tried here and dropped: "10 angenommen,
// 0 abgelehnt" says little about a business, where which vote passed matters
// and the rows already show that one by one.
func overviewHeadHeight(fonts *fontSet) int {
	return lineHeight(fonts.partyBold) + frac(fonts.statLabel, 0.9)
}

// fitOverview sizes the title and the rows against each other.
//
// The title is what matters most, so the rows give way to it, in three steps:
// first the rows shrink, through overviewRowSpecs, with every label whole;
// then the labels are cut to summarySubtitleMaxRunes; and then, for a group
// too long for both, rows are dropped from the end in favour of a closing
// "… und N weitere Abstimmungen" row (see squeezeOverview).
//
// "Whole" is not quite enough on its own: the ladder will happily set a long
// title at 26pt above rows at 28pt, and a card whose list outshouts its
// subject reads upside down. So at every step the title has to come out whole,
// visibly larger than the rows and at least at the largest row size; only a
// title too long for that even above a single row settles for less.
func fitOverview(group []votes.Vote, inset, maxWidth int, fonts *fontSet) (font.Face, []string, overviewLayout, error) {
	title := voteformat.CleanVoteTitle(group[0].Title)

	rows, titleMax := fonts.overviewRows, titleFontMax
	if len(group) <= overviewShortGroup {
		rows, titleMax = fonts.overviewRowsShort, overviewShortTitleMax
	}
	for _, maxRunes := range []int{0, summarySubtitleMaxRunes} {
		for _, faces := range rows {
			ol := buildOverview(group, faces, maxRunes)
			face, lines, whole, err := fitOverviewTitleFrom(group, title, ol, inset, maxWidth, fonts, titleMax)
			if err != nil {
				return nil, nil, overviewLayout{}, err
			}
			if whole && titleOutranksRows(face, ol, fonts) {
				return face, lines, ol, nil
			}
		}
	}
	return squeezeOverview(group, title, inset, maxWidth, fonts)
}

// fitOverviewTitle fits the title to the space ol leaves it.
func fitOverviewTitle(group []votes.Vote, title string, ol overviewLayout, inset, maxWidth int, fonts *fontSet) (font.Face, []string, bool, error) {
	return fitOverviewTitleFrom(group, title, ol, inset, maxWidth, fonts, titleFontMax)
}

// fitOverviewTitleFrom is fitOverviewTitle with the title's font ladder
// starting at maxSize.
func fitOverviewTitleFrom(group []votes.Vote, title string, ol overviewLayout, inset, maxWidth int, fonts *fontSet, maxSize float64) (font.Face, []string, bool, error) {
	reserve := titleTrailingGap() + overviewHeadHeight(fonts) + ol.height() + padding
	return cardTitleFullyFrom(group, title, maxWidth, titleBudget(inset, reserve), maxSize)
}

// overviewTitleFloor is the smallest size the overview sets the business title
// at while it still has rows to give up. The title is what the card is about,
// and dropping a few rows into "… und N weitere" costs less than setting it in
// small print; 38pt keeps it within a step or two of titleFontMax.
const overviewTitleFloor = 38.0

// titleOutranksRows reports whether the title is set visibly larger than the
// rows under it, and at overviewTitleFloor or above.
func titleOutranksRows(face font.Face, ol overviewLayout, fonts *fontSet) bool {
	h := lineHeight(face)
	return h*10 >= lineHeight(ol.faces.text)*11 && h >= lineHeight(fonts.overviewTitleFloor)
}

// overviewLabelFloor is the length labels are cut to once rows are being
// dropped: short enough to set on one line at the smallest size, so each row
// kept costs as little of the title's room as it can.
const overviewLabelFloor = 36

// overviewFits reports whether a title and rows fit between the card's inset
// and its bottom margin. Ellipsising the title cannot make room the rows
// themselves take, so a layout that overruns has to lose rows instead.
func overviewFits(face font.Face, lines []string, ol overviewLayout, inset int, fonts *fontSet) bool {
	h := titleBlockHeight(face, len(lines)) + titleTrailingGap() + overviewHeadHeight(fonts) + ol.height()
	return len(lines) > 0 && h <= imgHeight-inset-2*padding
}

// squeezeOverview is the last resort for a group whose rows leave the title too
// little room at any size: labels cut to one line, then rows dropped from the
// end in favour of a closing "… und N weitere Abstimmungen" row, keeping as
// many as leave the title whole and larger than them. Every vote still has its
// own result card, and the dropped rows' results are in the caption and the
// alt text.
//
// A title too long for that even above one row keeps the most rows that leave
// it whole at all, and failing that the most rows that fit with it cut.
func squeezeOverview(group []votes.Vote, title string, inset, maxWidth int, fonts *fontSet) (font.Face, []string, overviewLayout, error) {
	full := buildOverview(group, fonts.overviewRows[len(fonts.overviewRows)-1], overviewLabelFloor)
	type attempt struct {
		face  font.Face
		lines []string
		ol    overviewLayout
	}
	var whole, fits *attempt
	for keep := len(group); keep >= 1; keep-- {
		ol := full
		ol.rows = append([]overviewRow(nil), full.rows[:keep]...)
		if dropped := len(group) - keep; dropped > 0 {
			ol.rows = append(ol.rows, overviewRow{
				mark:  markOmitted,
				lines: []string{fmt.Sprintf(voteformat.T(group[0].Language, voteformat.MsgOverviewMore), dropped)},
			})
		}
		face, lines, isWhole, err := fitOverviewTitle(group, title, ol, inset, maxWidth, fonts)
		if err != nil {
			return nil, nil, overviewLayout{}, err
		}
		if isWhole && titleOutranksRows(face, ol, fonts) {
			return face, lines, ol, nil
		}
		a := &attempt{face, lines, ol}
		if isWhole && whole == nil {
			whole = a
		}
		if fits == nil && overviewFits(face, lines, ol, inset, fonts) {
			fits = a
		}
	}
	switch {
	case whole != nil:
		return whole.face, whole.lines, whole.ol, nil
	case fits != nil:
		return fits.face, fits.lines, fits.ol, nil
	}
	ol := full
	ol.rows = full.rows[:1]
	face, lines, _, err := fitOverviewTitle(group, title, ol, inset, maxWidth, fonts)
	return face, lines, ol, err
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
	if img != nil {
		drawCenteredText(img, fonts.partyBold, nil, cur.baseline(fonts.partyBold),
			fmt.Sprintf(voteformat.T(group[0].Language, voteformat.MsgCardOverview), len(group)), bg)
	}
	cur.advance(fonts.partyBold)
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

// drawOverviewRow draws one row: the number right-aligned in its column, the
// mark, the wrapped label and the tally right-aligned to the margin, all
// starting on the label's first line.
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
		if r.mark != markNone && r.mark != markOmitted {
			drawMark(img, markX, markY, ol.markW, r, bold, bg)
		}

		for i, line := range r.lines {
			drawShadowedText(img, text, nil, ol.labelX, base+i*lineHeight(text), line, bg)
		}

		for i, line := range r.tally {
			w := font.MeasureString(bold, line).Ceil()
			drawShadowedText(img, bold, nil, ol.tallyRight-w, base+i*lineHeight(text), line, bg)
		}
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
	box := markMask(size)
	drawMask(img, box, x+shadowOff, y+shadowOff, darken(bg))
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

const markSupersample = 4

// markMask is the filled rounded box of a mark as an alpha mask.
func markMask(size int) *image.Alpha {
	s := float64(size * markSupersample)
	hi := image.NewAlpha(image.Rect(0, 0, size*markSupersample, size*markSupersample))
	radius := s * 0.18
	for py := 0; py < hi.Rect.Dy(); py++ {
		for px := 0; px < hi.Rect.Dx(); px++ {
			if roundedBoxDist(float64(px)+0.5, float64(py)+0.5, s, radius) <= 0 {
				hi.SetAlpha(px, py, color.Alpha{0xFF})
			}
		}
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
