package imagegen

import (
	"bytes"
	_ "embed"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"

	"github.com/siiitschiii/parlinfo/pkg/voteposting/voteformat"
	"github.com/siiitschiii/parlinfo/pkg/votes"
)

//go:embed fonts/NotoEmoji-Regular.ttf
var notoEmojiTTF []byte

const (
	imgWidth                = 1080
	imgHeight               = 1350
	padding                 = 60
	shadowOff               = 2
	summarySubtitleMaxRunes = 90

	fraktionNameColWidth  = 200
	fraktionRowGapFactor  = 0.2
	fraktionColWidthScale = 0.6

	// The title shrinks between these two sizes to make room for the results,
	// and is ellipsised only once the smaller of them still does not fit.
	titleFontMax  = 42.0
	titleFontMin  = 26.0
	titleFontStep = 2.0

	// Visible whitespace between two wrapped title lines, as a fraction of the
	// line height. Shared by the measuring and the drawing so a title budgeted
	// for n lines really occupies n.
	titleLineGapFactor = 0.15
)

var palette = []color.RGBA{
	{0x1B, 0x4F, 0x72, 0xFF}, // dark blue
	{0x7B, 0x24, 0x1C, 0xFF}, // dark red
	{0x14, 0x5A, 0x32, 0xFF}, // dark green
	{0x4A, 0x23, 0x5A, 0xFF}, // dark purple
	{0x78, 0x4F, 0x0B, 0xFF}, // dark gold
	{0x1A, 0x5C, 0x5C, 0xFF}, // dark teal
	{0x6C, 0x3A, 0x0A, 0xFF}, // brown
	{0x2C, 0x3E, 0x6B, 0xFF}, // steel blue
}

// SelectColor returns a deterministic background colour for a vote group,
// keyed on the jurisdiction and the full business number (e.g. "2026/153").
//
// Hashing the whole string gives votes with the same trailing-number residue
// but a different year or prefix different colours. The jurisdiction is part of
// the key so that two bodies posting to one account do not land on the same
// colour for the same-numbered business. The background is decoration; which
// body voted is carried by the band.
func SelectColor(jurisdiction, affairNumber string) color.RGBA {
	seed := jurisdiction + "|" + affairNumber

	// FNV-inspired hash over the full string for good distribution
	h := uint32(2166136261)
	for i := 0; i < len(seed); i++ {
		h ^= uint32(seed[i])
		h *= 16777619
	}
	idx := int(h) % len(palette)
	if idx < 0 {
		idx += len(palette)
	}
	return palette[idx]
}

func darken(c color.RGBA) color.RGBA {
	return color.RGBA{c.R / 3, c.G / 3, c.B / 3, c.A}
}

func loadFace(fontData []byte, size float64) (font.Face, error) {
	f, err := opentype.Parse(fontData)
	if err != nil {
		return nil, err
	}
	return opentype.NewFace(f, &opentype.FaceOptions{
		Size:    size,
		DPI:     72,
		Hinting: font.HintingFull,
	})
}

// drawShadowedText draws text with a shadow offset for readability.
// It uses the emoji font face for emoji characters and the text font for everything else.
func drawShadowedText(img *image.RGBA, face font.Face, emojiFace font.Face, x, y int, text string, bg color.RGBA) {
	shadow := darken(bg)
	// Draw shadow pass then foreground pass
	for pass := 0; pass < 2; pass++ {
		var src *image.Uniform
		var ox, oy int
		if pass == 0 {
			src = image.NewUniform(shadow)
			ox, oy = x+shadowOff, y+shadowOff
		} else {
			src = image.NewUniform(color.White)
			ox, oy = x, y
		}
		dot := fixed.P(ox, oy)
		for _, seg := range splitEmojiText(text) {
			f := face
			segDot := dot
			if seg.isEmoji && emojiFace != nil {
				f = emojiFace
				// Shift emoji baseline so its visual center aligns with the text.
				// Text spans [Y-Ascent, Y+Descent]; center = Y - (Ascent-Descent)/2.
				// Same for emoji. Setting centers equal gives this offset:
				textM := face.Metrics()
				emojiM := emojiFace.Metrics()
				segDot.Y += ((emojiM.Ascent - emojiM.Descent) - (textM.Ascent - textM.Descent)) / 2
			}
			d := &font.Drawer{Dst: img, Src: src, Face: f, Dot: segDot}
			d.DrawString(seg.text)
			dot.X = d.Dot.X
		}
	}
}

// drawCenteredText draws text horizontally centered on the image.
func drawCenteredText(img *image.RGBA, face font.Face, emojiFace font.Face, y int, text string, bg color.RGBA) {
	w := measureMixedText(face, emojiFace, text)
	x := (imgWidth - w) / 2
	drawShadowedText(img, face, emojiFace, x, y, text, bg)
}

// measureMixedText returns the width drawShadowedText will actually occupy.
//
// It has to split the string the same way the drawing does, because the two
// faces are different sizes: the verdict emoji renders in NotoEmoji 72 while
// the text around it is gobold 64. Measuring the whole string with the text
// face alone reports the width of a glyph that is never drawn — 48px against
// the 91px the emoji really takes — and a card whose verdict is nothing but
// "❌" came out 21px right of centre, visibly off against the title below it.
func measureMixedText(face font.Face, emojiFace font.Face, text string) int {
	var total fixed.Int26_6
	for _, seg := range splitEmojiText(text) {
		f := face
		if seg.isEmoji && emojiFace != nil {
			f = emojiFace
		}
		total += font.MeasureString(f, seg.text)
	}
	return total.Ceil()
}

// drawHLine draws a thin horizontal separator line.
func drawHLine(img *image.RGBA, y, x1, x2 int, c color.Color) {
	for x := x1; x <= x2; x++ {
		img.Set(x, y, c)
		img.Set(x, y+1, c)
	}
}

// semiWhite is a semi-transparent white used for separator lines.
var semiWhite = color.RGBA{0xFF, 0xFF, 0xFF, 0x66}

// textSegment represents a run of text that is either all emoji or all non-emoji.
type textSegment struct {
	text    string
	isEmoji bool
}

// isEmojiRune returns true for Unicode codepoints that are emoji symbols.
func isEmojiRune(r rune) bool {
	// Variation selectors and zero-width joiners are part of emoji sequences
	if r == 0xFE0F || r == 0xFE0E || r == 0x200D {
		return true
	}
	// Common emoji ranges
	return (r >= 0x2600 && r <= 0x27BF) || // Misc Symbols, Dingbats
		(r >= 0x1F300 && r <= 0x1FAF8) || // Emoticons, Symbols, etc.
		(r >= 0x2300 && r <= 0x23FF) || // Misc Technical
		(r >= 0x2700 && r <= 0x27BF) || // Dingbats
		(r >= 0x1F100 && r <= 0x1F1FF) || // Enclosed Alphanumeric Supplement
		!unicode.IsGraphic(r)
}

// splitEmojiText splits text into alternating emoji and non-emoji segments.
func splitEmojiText(text string) []textSegment {
	if text == "" {
		return nil
	}
	var segments []textSegment
	runes := []rune(text)
	start := 0
	curIsEmoji := isEmojiRune(runes[0])

	for i := 1; i < len(runes); i++ {
		ie := isEmojiRune(runes[i])
		if ie != curIsEmoji {
			segments = append(segments, textSegment{text: string(runes[start:i]), isEmoji: curIsEmoji})
			start = i
			curIsEmoji = ie
		}
	}
	segments = append(segments, textSegment{text: string(runes[start:]), isEmoji: curIsEmoji})
	return segments
}

// wrapText breaks text into lines that fit within maxWidth pixels.
//
// Wrapping happens at spaces, so a single word wider than the column has no
// break to wrap at: a German compound, or an identifier out of a court
// decision. Such a word is split across lines by splitToWidth rather than
// emitted as an over-wide line, because the callers draw what they are given
// and an over-wide line is drawn centred — bleeding past the text column on
// both sides, with the height accounting none the wiser.
func wrapText(face font.Face, text string, maxWidth int) []string {
	var words []string
	for _, w := range strings.Fields(text) {
		words = append(words, splitToWidth(face, w, maxWidth)...)
	}
	if len(words) == 0 {
		return nil
	}
	var lines []string
	line := words[0]
	for _, w := range words[1:] {
		test := line + " " + w
		adv := font.MeasureString(face, test)
		if adv.Ceil() > maxWidth {
			lines = append(lines, line)
			line = w
		} else {
			line = test
		}
	}
	lines = append(lines, line)
	return lines
}

// splitToWidth breaks one word too wide for the column into the widest pieces
// that fit. A word that already fits comes back untouched, so the wrapping of
// ordinary text is unchanged.
//
// The pieces are re-joined by the caller's own greedy wrapping, which cannot
// put two of them back on one line: every piece but the last is as wide as the
// column allows. A single rune wider than the whole column is kept as its own
// piece — there is nothing left to split it at.
func splitToWidth(face font.Face, word string, maxWidth int) []string {
	if font.MeasureString(face, word).Ceil() <= maxWidth {
		return []string{word}
	}
	runes := []rune(word)
	var pieces []string
	start := 0
	for i := range runes {
		if i > start && font.MeasureString(face, string(runes[start:i+1])).Ceil() > maxWidth {
			pieces = append(pieces, string(runes[start:i]))
			start = i
		}
	}
	return append(pieces, string(runes[start:]))
}

// titleLineStride returns the baseline-to-baseline distance the title drawing
// loop actually advances by: a line plus the gap that follows it.
func titleLineStride(face font.Face) int {
	return lineHeight(face) + int(float64(lineHeight(face))*titleLineGapFactor)
}

// titleBlockHeight returns the vertical space n wrapped title lines occupy.
func titleBlockHeight(face font.Face, n int) int {
	return n * titleLineStride(face)
}

// widestWord returns the width of the widest single word in text, measured
// before any wrapping splits it.
func widestWord(face font.Face, text string) int {
	widest := 0
	for _, w := range strings.Fields(text) {
		if adv := font.MeasureString(face, w).Ceil(); adv > widest {
			widest = adv
		}
	}
	return widest
}

// fitTitle sizes a vote title to the space the results leave it.
//
// It shrinks the face first, from titleFontMax down to titleFontMin, and
// ellipsises only when even the smallest size overruns the budget. Zurich
// business titles run to several hundred characters — one Gemeinderat title
// carries two parliamentary initiatives, a court decision and its case number
// in a single sentence — and letting such a title take all the room it asks
// for is what pushed the Fraktion table off the bottom of the card: the party
// breakdown silently lost rows, and the breakdown is the part a reader cannot
// reconstruct from anywhere else. The full title is in the caption directly
// under the image, so an ellipsis there costs nothing.
func fitTitle(title string, maxWidth, available int) (font.Face, []string, error) {
	face, lines, _, err := fitTitleFully(title, maxWidth, available)
	return face, lines, err
}

// fitTitleFully is fitTitle, and also reports whether the title got in whole.
// False means the font ladder ran out and the last lines were cut.
func fitTitleFully(title string, maxWidth, available int) (font.Face, []string, bool, error) {
	var face font.Face
	var lines []string
	for size := titleFontMax; size >= titleFontMin; size -= titleFontStep {
		f, err := loadFace(goregular.TTF, size)
		if err != nil {
			return nil, nil, false, err
		}
		face, lines = f, wrapText(f, title, maxWidth)
		// Width counts as well as height. wrapText now guarantees no line
		// overruns the column, but it buys that by breaking an over-wide word
		// mid-compound — so height alone would accept 42pt and hyphenate a word
		// that a step or two down the ladder sets whole. Measuring the widest
		// word before wrapping is what still sees the difference; after
		// wrapping, every line fits by construction.
		if titleBlockHeight(f, len(lines)) <= available && widestWord(f, title) <= maxWidth {
			return face, lines, true, nil
		}
	}
	return face, ellipsizeLines(face, lines, maxWidth, available), false, nil
}

// cardTitle sets the card's title line, naming as many signatories as the card
// can hold.
//
// Text posts name everyone who signed. The card cannot always: the type, the
// names and the subject share one column, and the font ladder is what absorbs a
// long line. Only when that runs out does this start dropping names — because
// what a reader cannot do without is the subject of the vote, so it is the last
// thing to give way. A card with room names everyone, exactly like the caption
// beside it.
func cardTitle(group []votes.Vote, subject string, maxWidth, available int) (font.Face, []string, error) {
	face, lines, _, err := cardTitleFully(group, subject, maxWidth, available)
	return face, lines, err
}

// cardTitleFully is cardTitle, and also reports whether the title got in whole,
// every signatory named and no line cut.
func cardTitleFully(group []votes.Vote, subject string, maxWidth, available int) (font.Face, []string, bool, error) {
	// Fullest first, then one name shorter each time, down to a lone name.
	caps := []int{0}
	for n := voteformat.CountAuthors(group) - 1; n >= 1; n-- {
		caps = append(caps, n)
	}

	var face font.Face
	var lines []string
	for _, max := range caps {
		title := subject
		if prefix := voteformat.GroupPrefixLineCapped(group, max); prefix != "" {
			title = prefix + ": " + subject
		}

		f, ls, whole, err := fitTitleFully(title, maxWidth, available)
		if err != nil {
			return nil, nil, false, err
		}
		face, lines = f, ls
		if whole {
			return face, lines, true, nil
		}
	}
	// Even one name does not fit: keep the shortest and let the cut fall where
	// it must.
	return face, lines, false, nil
}

// ellipsizeLines drops the title lines that do not fit and marks the cut.
func ellipsizeLines(face font.Face, lines []string, maxWidth, available int) []string {
	stride := titleLineStride(face)
	if stride <= 0 {
		return lines
	}
	maxLines := available / stride
	if maxLines < 1 {
		maxLines = 1
	}
	if maxLines >= len(lines) {
		return lines
	}
	kept := append([]string(nil), lines[:maxLines]...)
	kept[len(kept)-1] = appendEllipsis(face, kept[len(kept)-1], maxWidth)
	return kept
}

// appendEllipsis ends a line with "…", dropping trailing words until the
// ellipsis itself fits within maxWidth.
func appendEllipsis(face font.Face, line string, maxWidth int) string {
	const ellipsis = "…"
	fits := func(s string) bool {
		return font.MeasureString(face, s+ellipsis).Ceil() <= maxWidth
	}
	words := strings.Fields(line)
	for len(words) > 1 {
		if candidate := trimForEllipsis(strings.Join(words, " ")); fits(candidate) {
			return candidate + ellipsis
		}
		words = words[:len(words)-1]
	}
	// A single word wider than the line has no word boundary left to cut on,
	// so cut it mid-word rather than returning a bare ellipsis.
	runes := []rune(trimForEllipsis(strings.Join(words, " ")))
	for len(runes) > 1 && !fits(string(runes)) {
		runes = runes[:len(runes)-1]
	}
	return string(runes) + ellipsis
}

// trimForEllipsis strips punctuation the ellipsis would otherwise follow, so a
// cut lands on "… Aufenthaltsstatus…" rather than "… Aufenthaltsstatus,…".
func trimForEllipsis(s string) string {
	return strings.TrimRight(s, " ,;:.·-–—«»(/")
}

// GenerateCarousel produces carousel JPEG images for a vote group.
// Returns [][]byte (JPEG-encoded images).
func GenerateCarousel(group []votes.Vote) ([][]byte, error) {
	if len(group) == 0 {
		return nil, fmt.Errorf("no votes provided")
	}

	bgColor := SelectColor(group[0].Jurisdiction, group[0].Affair.Number)

	fonts, err := loadFontSet()
	if err != nil {
		return nil, fmt.Errorf("loading fonts: %w", err)
	}

	var images [][]byte
	for _, c := range carouselCards(group, bgColor, fonts) {
		img, err := renderCard(c, bgColor, fonts)
		if err != nil {
			return nil, fmt.Errorf("rendering %s: %w", c.name, err)
		}
		images = append(images, img)
	}
	return images, nil
}

// card is one image of a carousel: the vote whose band and corner it carries,
// what its corners say, and how it lays out its content below a given inset.
//
// Every card goes through renderCard, so the band, the corner row and the
// centring are decided once for all of them, and carouselCards is the one
// place that says which card gets which corner.
type card struct {
	name   string
	vote   votes.Vote
	corner cardCorner
	layout func(img *image.RGBA, cur *layoutCursor, inset int) error
}

// inset is the Y the card's content starts below: the band, then the corner
// row.
func (c card) inset(fonts *fontSet) int {
	return bandInset(c.vote) + c.corner.height(fonts)
}

// carouselCards returns the cards GenerateCarousel draws for a group, in order.
//
// An election business gets one text-only card, not the usual stats
// dashboard: its only votes are attendance roll calls, and none of their
// counts mean anything here. Checked on the whole group, because a second
// ballot round adds a second roll call — see votes.IsWahlgeschaeftGroup.
//
// A single vote combines title and results on one card. A multi-vote group
// gets an overview card, then one result card per vote.
func carouselCards(group []votes.Vote, bg color.RGBA, fonts *fontSet) []card {
	if votes.IsWahlgeschaeftGroup(group) {
		v := group[0]
		return []card{{"Wahlgeschäft card", v, dateCorner(v), func(img *image.RGBA, cur *layoutCursor, inset int) error {
			layoutWahlgeschaeftCard(img, cur, inset, &v, bg, fonts)
			return nil
		}}}
	}

	if len(group) == 1 {
		v := group[0]
		return []card{{"combined card", v, voteCorner(v, ""), func(img *image.RGBA, cur *layoutCursor, inset int) error {
			_, _, err := layoutCombinedCard(img, cur, inset, &v, bg, fonts)
			return err
		}}}
	}

	cards := []card{{"title card", group[0], dateCorner(group[0]), func(img *image.RGBA, cur *layoutCursor, inset int) error {
		layoutTitleCard(img, cur, inset, group, bg, fonts)
		return nil
	}}}
	for i := range group {
		v := group[i]
		cards = append(cards, card{fmt.Sprintf("result card %d", i+1), v, voteCorner(v, formatProgressBadge(i+1, len(group))), func(img *image.RGBA, cur *layoutCursor, inset int) error {
			layoutResultCard(img, cur, inset, &v, bg, fonts, i+1, len(group))
			return nil
		}})
	}
	return cards
}

// renderCard lays a card out twice: a dry run at its inset to measure it, then
// the real run, centred in the space the band and the corner row leave.
func renderCard(c card, bg color.RGBA, fonts *fontSet) ([]byte, error) {
	inset := c.inset(fonts)

	dry := newCursor(inset, imgHeight)
	if err := c.layout(nil, dry, inset); err != nil {
		return nil, err
	}

	startY := centredStart(inset, dry.contentHeight())
	img := newImage(bg)
	drawBodyBand(img, fonts.bandLabel, c.vote)
	c.corner.draw(img, fonts, c.vote, bg)

	cur := newCursor(startY, imgHeight)
	if err := c.layout(img, cur, inset); err != nil {
		return nil, err
	}
	return encodeJPEG(img)
}

// centredStart returns the Y the card's content starts at, centring a block of
// contentHeight between the margin under the body band and the one above the
// bottom edge.
//
// The clamps are the point, and they rank. drawFraktionTable stops drawing at
// imgHeight-padding, so a card centred without regard for its bottom edge comes
// back missing Fraktionen, with nothing on it to say any were dropped: the
// bottom edge is held first. The margin under the band is held second, which is
// the order that matters only for a card too tall to satisfy both — it gives up
// its top margin, down to the band edge at worst, rather than a Fraktion.
//
// Cards that fit keep both margins, because titleBudget charges the title for
// them. The two margins are equal, so centring between them is the same
// arithmetic as centring in the raw frame: a card with room to spare sits
// exactly where it always did.
func centredStart(inset, contentHeight int) int {
	top := inset + padding
	start := top + (imgHeight-padding-top-contentHeight)/2
	if lowest := imgHeight - padding - contentHeight; start > lowest {
		start = lowest
	}
	if start < inset {
		start = inset
	}
	return start
}

func newImage(bg color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, imgWidth, imgHeight))
	draw.Draw(img, img.Bounds(), image.NewUniform(bg), image.Point{}, draw.Src)
	return img
}

func encodeJPEG(img *image.RGBA) ([]byte, error) {
	var buf bytes.Buffer
	err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90})
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// layoutCursor tracks the Y position as the top of the next text region.
// Unlike raw baseline tracking, gap calculations represent visible whitespace
// between the bottom of one glyph and the top of the next.
type layoutCursor struct {
	y         int
	startY    int
	imgHeight int
}

// newCursor creates a layout cursor starting at the given Y position.
func newCursor(startY, imgHeight int) *layoutCursor {
	return &layoutCursor{y: startY, startY: startY, imgHeight: imgHeight}
}

// contentHeight returns the total vertical space consumed since cursor creation.
func (c *layoutCursor) contentHeight() int {
	return c.y - c.startY
}

// baseline returns the baseline Y for drawing text with the given face.
// Since the cursor tracks the top of the text region, the baseline = y + ascent.
func (c *layoutCursor) baseline(face font.Face) int {
	return c.y + face.Metrics().Ascent.Ceil()
}

// lineHeight returns the recommended baseline-to-baseline distance for a face.
func lineHeight(face font.Face) int {
	return face.Metrics().Height.Ceil()
}

// advance moves the cursor down by one line at the given font's line height.
func (c *layoutCursor) advance(face font.Face) {
	c.y += lineHeight(face)
}

// gap adds vertical space equal to a fraction of the face's line height.
func (c *layoutCursor) gap(face font.Face, fraction float64) {
	c.y += int(float64(lineHeight(face)) * fraction)
}

// fontSet holds all preloaded font faces needed for image generation.
type fontSet struct {
	verdict     font.Face // gobold 64
	verdictSm   font.Face // gobold 56 (result card)
	statNum     font.Face // gobold 48
	statLabel   font.Face // goregular 26
	partyBold   font.Face // gobold 30
	partyNum    font.Face // goregular 30
	regular     font.Face // goregular 36
	small       font.Face // goregular 28
	boldHeading font.Face // gobold 48 (= statNum, shared)
	bandLabel   font.Face // gobold 44 (body band)

	emojiRegular font.Face // notoEmoji 36
	emojiSmall   font.Face // notoEmoji 28
	emojiLarge   font.Face // notoEmoji 48
	emojiVerdict font.Face // notoEmoji 72
}

func loadFontSet() (*fontSet, error) {
	var fs fontSet
	var err error

	load := func(data []byte, size float64) (font.Face, error) {
		return loadFace(data, size)
	}

	if fs.verdict, err = load(gobold.TTF, 64); err != nil {
		return nil, fmt.Errorf("verdict font: %w", err)
	}
	if fs.verdictSm, err = load(gobold.TTF, 56); err != nil {
		return nil, fmt.Errorf("verdict-sm font: %w", err)
	}
	if fs.statNum, err = load(gobold.TTF, 48); err != nil {
		return nil, fmt.Errorf("statNum font: %w", err)
	}
	if fs.statLabel, err = load(goregular.TTF, 26); err != nil {
		return nil, fmt.Errorf("statLabel font: %w", err)
	}
	if fs.partyBold, err = load(gobold.TTF, 30); err != nil {
		return nil, fmt.Errorf("partyBold font: %w", err)
	}
	if fs.partyNum, err = load(goregular.TTF, 30); err != nil {
		return nil, fmt.Errorf("partyNum font: %w", err)
	}
	if fs.regular, err = load(goregular.TTF, 36); err != nil {
		return nil, fmt.Errorf("regular font: %w", err)
	}
	if fs.small, err = load(goregular.TTF, 28); err != nil {
		return nil, fmt.Errorf("small font: %w", err)
	}
	if fs.bandLabel, err = load(gobold.TTF, 44); err != nil {
		return nil, fmt.Errorf("bandLabel font: %w", err)
	}
	fs.boldHeading = fs.statNum // same face, gobold 48

	if fs.emojiRegular, err = load(notoEmojiTTF, 36); err != nil {
		return nil, fmt.Errorf("emojiRegular font: %w", err)
	}
	if fs.emojiSmall, err = load(notoEmojiTTF, 28); err != nil {
		return nil, fmt.Errorf("emojiSmall font: %w", err)
	}
	if fs.emojiLarge, err = load(notoEmojiTTF, 48); err != nil {
		return nil, fmt.Errorf("emojiLarge font: %w", err)
	}
	if fs.emojiVerdict, err = load(notoEmojiTTF, 72); err != nil {
		return nil, fmt.Errorf("emojiVerdict font: %w", err)
	}

	return &fs, nil
}

// layoutCombinedCard lays out a single vote with visual hierarchy: large
// verdict, bold title, dashboard stats, and grouped party breakdown.
func layoutCombinedCard(img *image.RGBA, cur *layoutCursor, inset int, v *votes.Vote, bg color.RGBA, fonts *fontSet) (font.Face, []string, error) {
	maxTextWidth := imgWidth - 2*padding

	// Build counts
	counts := voteformat.CountsOf(*v)
	isAuswahl := voteformat.IsAuswahlVote(counts)

	// Title first: bold, wrapped, centered
	title := voteformat.CleanVoteTitle(v.Title)

	// When the vote was taken goes in the corner, as on every other card. The
	// line above the counts is left to say what kind of ballot it was, and is
	// dropped for an ordinary one.
	countsLabel := voteformat.TypeLabel(v.Type)

	// Everything below the title has to fit, all of it: the reserve below
	// mirrors the layout that follows line for line, and the title gets what is
	// left over.
	//
	// The budget is measured from the band inset rather than from cur.y,
	// because renderCard lays the card out twice — a dry run at the
	// inset, then the real run at the centred start — and a title that sized
	// itself against each run's own cur.y would come out at two different
	// sizes, leaving the measured height wrong for the card actually drawn.
	fraktionCounts := voteformat.AggregateFraktionCounts(*v)
	bottomReserved := combinedBottomReserve(fonts, len(fraktionCounts), countsLabel != "")
	availableForTitle := titleBudget(inset, bottomReserved)

	titleFace, titleLines, err := cardTitle([]votes.Vote{*v}, title, maxTextWidth, availableForTitle)
	if err != nil {
		return nil, nil, err
	}

	// Verdict first: large centered emoji above the title
	var verdictText string
	switch {
	case isAuswahl:
		verdictText = strings.ToUpper(v.Decision)
	case voteformat.HasVerdict(*v):
		verdictText = voteformat.GetVoteResultEmoji(v.Decision)
	}
	if img != nil {
		drawCenteredText(img, fonts.verdict, fonts.emojiVerdict, cur.baseline(fonts.verdict), verdictText, bg)
	}
	cur.advance(fonts.verdict)

	cur.gap(fonts.verdict, 0.75)

	// Title: bold, wrapped, centered
	for _, line := range titleLines {
		if img != nil {
			drawCenteredText(img, titleFace, nil, cur.baseline(titleFace), line, bg)
		}
		cur.advance(titleFace)
		cur.gap(titleFace, titleLineGapFactor)
	}

	cur.gap(titleFace, 0.75)

	// Horizontal separator
	if img != nil {
		drawHLine(img, cur.y, padding, imgWidth-padding, semiWhite)
	}
	cur.gap(fonts.statNum, 0.75)

	if countsLabel != "" {
		if img != nil {
			drawCenteredText(img, fonts.statLabel, nil, cur.baseline(fonts.statLabel), countsLabel, bg)
		}
		cur.advance(fonts.statLabel)
		cur.gap(fonts.statLabel, 0.3)
	}

	// Stats dashboard: large numbers with small labels in columns
	switch {
	case isAuswahl:
		drawAuswahlStatsDashboard(img, cur, counts, bg, fonts.statNum, fonts.statLabel)
	case voteformat.IsQuorumVote(counts):
		drawQuorumStatsDashboard(img, cur, counts, bg, fonts.statNum, fonts.statLabel)
	default:
		drawStandardStatsDashboard(img, cur, counts, bg, fonts.statNum, fonts.statLabel)
	}

	cur.gap(fonts.statNum, 0.75)

	// Horizontal separator
	if img != nil {
		drawHLine(img, cur.y, padding, imgWidth-padding, semiWhite)
	}
	cur.gap(fonts.partyBold, 1.25)

	// Party breakdown table
	drawFraktionTable(img, cur, fraktionCounts, bg, fonts.partyBold, fonts.partyNum)

	return titleFace, titleLines, nil
}

// frac returns a fraction of a face's line height, matching layoutCursor.gap.
func frac(face font.Face, f float64) int {
	return int(float64(lineHeight(face)) * f)
}

// titleTrailingGap reserves the gap drawn after a title's last line.
//
// The title's own face is not known while its budget is being computed, so the
// gap is reserved at the largest size the title can take: a goregular line box
// runs about 1.2× the point size.
func titleTrailingGap() int {
	maxTitleLine := titleFontMax * 1.2
	return int(maxTitleLine * 0.75)
}

// fraktionTableHeight returns the space drawFraktionTable occupies: its column
// header and gap, then one row per Fraktion with a gap between rows.
func fraktionTableHeight(fonts *fontSet, numParties int) int {
	rowHeight := lineHeight(fonts.partyNum)
	height := rowHeight + int(float64(rowHeight)*fraktionRowGapFactor)
	height += numParties * rowHeight
	if numParties > 1 {
		height += int(float64((numParties-1)*rowHeight) * fraktionRowGapFactor)
	}
	return height
}

// statsBlockHeight returns the space between a card's title and its Fraktion
// table: the separator gap, the stats dashboard, and the gaps around both.
func statsBlockHeight(fonts *fontSet) int {
	return frac(fonts.statNum, 0.75) +
		lineHeight(fonts.statNum) + lineHeight(fonts.statLabel) +
		frac(fonts.statNum, 0.75) + frac(fonts.partyBold, 1.25)
}

// titleBudget returns the vertical space a card's title may take: the frame
// below the card's inset (band and corner row), less the margin that has to
// stay under it, less what the results below the title need.
//
// The top margin is budgeted rather than hoped for. Centring only ever
// protected the bottom edge, so a title long enough to fill the card was
// accepted at full size and the top paid for it: on the 03.12.2025 Postulat the
// verdict emoji came out 2px under the band, all but touching it. Charging the
// title for the margin makes it give up a font size instead.
func titleBudget(inset, reserve int) int {
	return imgHeight - inset - padding - reserve
}

// combinedBottomReserve returns the space layoutCombinedCard needs for
// everything that is not the title, so the title can be given what is left.
//
// It mirrors the advances and gaps of the layout, which is the point: a reserve
// that came up short would hand the title room the Fraktion table then has to
// give up, and the table gives it up by dropping rows.
func combinedBottomReserve(fonts *fontSet, numParties int, hasCountsLabel bool) int {
	// The verdict sits above the title, but its space is just as unavailable.
	reserve := lineHeight(fonts.verdict) + frac(fonts.verdict, 0.75)
	reserve += titleTrailingGap()
	if hasCountsLabel {
		reserve += lineHeight(fonts.statLabel) + frac(fonts.statLabel, 0.3)
	}
	reserve += statsBlockHeight(fonts)
	reserve += fraktionTableHeight(fonts, numParties)
	return reserve + padding
}

// resultBottomReserve returns the space layoutResultCard needs below its
// sub-vote heading, mirroring the advances and gaps of the layout that follows.
func resultBottomReserve(fonts *fontSet, numParties int) int {
	reserve := frac(fonts.boldHeading, 0.5)
	reserve += lineHeight(fonts.verdictSm) + frac(fonts.verdictSm, 0.75)
	reserve += statsBlockHeight(fonts)
	reserve += fraktionTableHeight(fonts, numParties)
	return reserve + padding
}

// statCol holds a value/label pair for dashboard-style stat columns.
type statCol struct {
	value string
	label string
}

// drawStandardStatsDashboard draws Ja/Nein/Enthaltung as large centered numbers.
func drawStandardStatsDashboard(img *image.RGBA, cur *layoutCursor, counts voteformat.VoteCounts, bg color.RGBA, numFace, labelFace font.Face) {
	cols := []statCol{
		{voteformat.FormatVoteCount(counts.Ja), "Ja"},
		{voteformat.FormatVoteCount(counts.Nein), "Nein"},
		{voteformat.FormatVoteCount(counts.Enthaltung), "Enth."},
	}
	drawStatColumns(img, cur, cols, bg, numFace, labelFace)
}

// drawQuorumStatsDashboard draws a quorum vote's two real numbers.
//
// The standard dashboard is actively misleading here. It shows Ja/Nein/Enth.
// and omits Abwesend, so a quorum vote renders as "129 / 0 / 0" — the 51
// members who did not support it vanish from the card entirely, and the two
// zeros are positions no one could have taken.
func drawQuorumStatsDashboard(img *image.RGBA, cur *layoutCursor, counts voteformat.VoteCounts, bg color.RGBA, numFace, labelFace font.Face) {
	// Shared with the caption and the Fraktion table, which state the same two
	// numbers: a card that disagreed with the text under it would be worse than
	// either being wrong alone.
	support, without := voteformat.QuorumTally(counts)
	cols := []statCol{
		{strconv.Itoa(support), "Zust."},
		{strconv.Itoa(without), "ohne"},
	}
	drawStatColumns(img, cur, cols, bg, numFace, labelFace)
}

// drawAuswahlStatsDashboard draws A/B/C/D/E option counts as large centered numbers.
func drawAuswahlStatsDashboard(img *image.RGBA, cur *layoutCursor, counts voteformat.VoteCounts, bg color.RGBA, numFace, labelFace font.Face) {
	var cols []statCol
	options := []struct {
		ptr   *int
		label string
	}{
		{counts.A, "A"}, {counts.B, "B"}, {counts.C, "C"},
		{counts.D, "D"}, {counts.E, "E"},
	}
	for _, o := range options {
		if o.ptr != nil && *o.ptr > 0 {
			cols = append(cols, statCol{fmt.Sprintf("%d", *o.ptr), o.label})
		}
	}
	if len(cols) == 0 {
		return
	}
	drawStatColumns(img, cur, cols, bg, numFace, labelFace)
}

// drawStatColumns draws stat values in evenly-spaced centered columns.
func drawStatColumns(img *image.RGBA, cur *layoutCursor, cols []statCol, bg color.RGBA, numFace, labelFace font.Face) {
	if len(cols) == 0 {
		return
	}
	colWidth := (imgWidth - 2*padding) / len(cols)

	// Draw large numbers
	for i, col := range cols {
		cx := padding + colWidth*i + colWidth/2
		w := font.MeasureString(numFace, col.value).Ceil()
		if img != nil {
			drawShadowedText(img, numFace, nil, cx-w/2, cur.baseline(numFace), col.value, bg)
		}
	}
	cur.advance(numFace)

	// Draw small labels
	for i, col := range cols {
		cx := padding + colWidth*i + colWidth/2
		w := font.MeasureString(labelFace, col.label).Ceil()
		if img != nil {
			drawShadowedText(img, labelFace, nil, cx-w/2, cur.baseline(labelFace), col.label, bg)
		}
	}
	cur.advance(labelFace)
}

// fraktionEntry holds a faction name and its vote counts for sorting.
type fraktionEntry struct {
	name   string
	counts map[string]int
	total  int
}

// drawFraktionTable draws a simple party breakdown table sorted by faction size descending.
func drawFraktionTable(img *image.RGBA, cur *layoutCursor, fraktionCounts map[string]*voteformat.FraktionCounts, bg color.RGBA, nameFace, numFace font.Face) {
	if len(fraktionCounts) == 0 {
		return
	}

	// Build faction entries
	var entries []fraktionEntry
	for name, fc := range fraktionCounts {
		total := 0
		for _, v := range fc.Counts {
			total += v
		}
		entries = append(entries, fraktionEntry{
			name: name, counts: fc.Counts, total: total,
		})
	}

	// Sort by total members descending, ties alphabetically
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].total != entries[j].total {
			return entries[i].total > entries[j].total
		}
		return entries[i].name < entries[j].name
	})

	// Determine vote columns from the data
	keySet := make(map[string]bool)
	for _, e := range entries {
		for k := range e.counts {
			keySet[k] = true
		}
	}

	// Build columns: primary vote keys first, then Enthaltung, then Abwesend
	var primary []string
	hasEnth, hasAbw := false, false
	for k := range keySet {
		switch k {
		case "Enthaltung":
			hasEnth = true
		case "Abwesend":
			hasAbw = true
		default:
			primary = append(primary, k)
		}
	}
	sort.Strings(primary)

	// Abbreviate column headers
	var allCols []string // display headers
	var colKeys []string // actual map keys
	for _, k := range primary {
		// "Auswahl A" is too wide for a column and runs into its neighbours;
		// the option letter alone is unambiguous under the "AUSWAHL" heading.
		allCols = append(allCols, strings.TrimPrefix(k, "Auswahl "))
		colKeys = append(colKeys, k)
	}
	if hasEnth {
		allCols = append(allCols, "Enth.")
		colKeys = append(colKeys, "Enthaltung")
	}
	if hasAbw {
		allCols = append(allCols, "Abw.")
		colKeys = append(colKeys, "Abwesend")
	}
	if len(allCols) == 0 {
		return
	}

	// Layout
	nameColWidth := fraktionNameColWidth
	maxNumColsWidth := imgWidth - 2*padding - nameColWidth
	numColWidth := int(float64(maxNumColsWidth) / float64(len(allCols)) * fraktionColWidthScale)
	if numColWidth <= 0 {
		return
	}
	totalTableWidth := nameColWidth + numColWidth*len(allCols)
	tableStartX := (imgWidth - totalTableWidth) / 2
	numStartX := tableStartX + nameColWidth

	// Draw column headers
	for i, col := range allCols {
		cx := numStartX + numColWidth*i + numColWidth/2
		w := font.MeasureString(numFace, col).Ceil()
		if img != nil {
			drawShadowedText(img, numFace, nil, cx-w/2, cur.baseline(numFace), col, bg)
		}
	}
	cur.advance(numFace)
	cur.gap(numFace, fraktionRowGapFactor)

	rowHeight := lineHeight(numFace)
	if rowHeight <= 0 {
		return
	}
	rowGap := int(float64(rowHeight) * fraktionRowGapFactor)

	// The measuring pass takes every row. It has to: renderCard centres
	// the card on the height this pass reports, so a measurement that had
	// already dropped rows would report a short card, centre it lower, and make
	// the drawing pass drop them for real — which is how a long title used to
	// cost the card its last Fraktionen.
	maxRows := len(entries)
	if img != nil {
		tableBottom := imgHeight - padding
		if cur.imgHeight > 0 {
			tableBottom = cur.imgHeight - padding
		}
		// n rows occupy n*rowHeight plus the n-1 gaps between them; the last
		// row is not followed by one. Dividing by the full stride would demand
		// a trailing gap that is never drawn and drop a row that fits.
		if fit := (tableBottom - cur.y + rowGap) / (rowHeight + rowGap); fit < maxRows {
			maxRows = fit
		}
	}
	if maxRows < 0 {
		maxRows = 0
	}

	// Draw party rows
	for i := range maxRows {
		e := entries[i]
		if img != nil {
			// Bold party name
			drawShadowedText(img, nameFace, nil, tableStartX, cur.baseline(numFace), e.name, bg)
			// Numbers in columns
			for i, key := range colKeys {
				cx := numStartX + numColWidth*i + numColWidth/2
				numStr := fmt.Sprintf("%d", e.counts[key])
				w := font.MeasureString(numFace, numStr).Ceil()
				drawShadowedText(img, numFace, nil, cx-w/2, cur.baseline(numFace), numStr, bg)
			}
		}
		cur.advance(numFace)
		if i < maxRows-1 {
			cur.gap(numFace, fraktionRowGapFactor)
		}
	}
}

// fitTitleAndSummary sizes the title card's title and its Übersicht against
// each other.
//
// The title is what matters most, so the summary gives way to it. The
// subtitles are set in full first, wrapped over as many lines as they need;
// only if the title cannot then be shown whole are they cut to
// summarySubtitleMaxRunes and the title fitted again. The summary is built
// first either way: it is what the title has to leave room for, and it does not
// depend on the size the title ends up at.
func fitTitleAndSummary(group []votes.Vote, inset, maxWidth int, fonts *fontSet) (font.Face, []string, []string, error) {
	title := voteformat.CleanVoteTitle(group[0].Title)

	var titleFace font.Face
	var titleLines, summaryLines []string
	for _, maxRunes := range []int{0, summarySubtitleMaxRunes} {
		summaryLines = nil
		for i, sv := range group {
			line, ok := formatSummaryLine(i+1, sv, len(group), maxRunes)
			if ok {
				summaryLines = append(summaryLines, wrapText(fonts.small, line, maxWidth)...)
			}
		}

		// Reserve: gap after the title, the "Übersicht" header and its gap, one
		// line per summary line, and the bottom padding.
		summaryReserve := titleTrailingGap() +
			lineHeight(fonts.small) + frac(fonts.small, 0.4) +
			len(summaryLines)*lineHeight(fonts.small) + padding

		var whole bool
		var err error
		titleFace, titleLines, whole, err = cardTitleFully(group, title, maxWidth, titleBudget(inset, summaryReserve))
		if err != nil {
			return nil, nil, nil, err
		}
		if whole {
			break
		}
	}
	return titleFace, titleLines, summaryLines, nil
}

func layoutTitleCard(img *image.RGBA, cur *layoutCursor, inset int, group []votes.Vote, bg color.RGBA, fonts *fontSet) {
	maxTextWidth := imgWidth - 2*padding

	titleFace, titleLines, summaryLines, err := fitTitleAndSummary(group, inset, maxTextWidth, fonts)
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

	// Summary: list each sub-vote with number + emoji + short subtitle.
	cur.gap(fonts.regular, 0.75)
	if img != nil {
		header := fmt.Sprintf("Übersicht (%d Teilabstimmungen)", len(group))
		drawCenteredText(img, fonts.small, nil, cur.baseline(fonts.small), header, bg)
	}
	cur.advance(fonts.small)
	cur.gap(fonts.small, 0.4)

	// Find widest line and center the block, then left-align all lines within it
	maxW := 0
	for _, line := range summaryLines {
		// Measured with the same pair of faces it is drawn with below; a
		// summary line carrying an emoji would otherwise centre off its true
		// width.
		w := measureMixedText(fonts.small, fonts.emojiSmall, line)
		if w > maxW {
			maxW = w
		}
	}
	blockX := (imgWidth - maxW) / 2
	for _, line := range summaryLines {
		if img != nil {
			drawShadowedText(img, fonts.small, fonts.emojiSmall, blockX, cur.baseline(fonts.small), line, bg)
		}
		cur.advance(fonts.small)
	}
}

// layoutWahlgeschaeftCard draws a large 🗳️, then the business title, a
// divider, and the sentence saying the result is not in our data.
//
// The marker is a ballot box rather than the ✅ an accepted vote carries:
// nothing on this card is a decision. The shape still follows
// layoutCombinedCard's — marker, title, divider, text — rather than
// layoutTitleCard's plainer one, so the card reads as the same kind of thing
// as everything else on the timeline instead of a visibly thinner one. There
// is no stats dashboard and no Fraktion table, because the only recorded vote
// is the roll call taken before the ballot (see votes.IsWahlgeschaeft).
func layoutWahlgeschaeftCard(img *image.RGBA, cur *layoutCursor, inset int, v *votes.Vote, bg color.RGBA, fonts *fontSet) {
	maxTextWidth := imgWidth - 2*padding

	// The sentence is built first: it is what the title has to leave room for,
	// and it does not depend on the size the title ends up at. The card draws
	// the title itself, so this is the sentence alone rather than
	// voteformat.WahlgeschaeftBody, which carries both.
	bodyLines := wrapText(fonts.regular, voteformat.WahlgeschaeftNoResult, maxTextWidth)

	// Reserve, mirroring combinedBottomReserve: the marker sits above the
	// title but its space is just as unavailable, so it is charged here too,
	// then the gap and divider after the title, one line per sentence line,
	// and the bottom padding.
	bodyReserve := lineHeight(fonts.verdict) + frac(fonts.verdict, 0.75)
	bodyReserve += titleTrailingGap() + frac(fonts.regular, 0.75)
	for _, line := range bodyLines {
		if line == "" {
			bodyReserve += frac(fonts.regular, 0.5)
			continue
		}
		bodyReserve += lineHeight(fonts.regular)
	}
	bodyReserve += padding

	title := voteformat.CleanVoteTitle(v.Title)

	titleFace, titleLines, err := fitTitle(title, maxTextWidth, titleBudget(inset, bodyReserve))
	if err != nil {
		return
	}

	// Marker: large centered 🗳️ above the title, where an ordinary vote card
	// carries its verdict.
	if img != nil {
		drawCenteredText(img, fonts.verdict, fonts.emojiVerdict, cur.baseline(fonts.verdict), "🗳️", bg)
	}
	cur.advance(fonts.verdict)
	cur.gap(fonts.verdict, 0.75)

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
	cur.gap(fonts.regular, 0.75)

	// The sentence: centered, plain text.
	for _, line := range bodyLines {
		if line == "" {
			cur.gap(fonts.regular, 0.5)
			continue
		}
		if img != nil {
			drawCenteredText(img, fonts.regular, fonts.emojiRegular, cur.baseline(fonts.regular), line, bg)
		}
		cur.advance(fonts.regular)
	}
}

func layoutResultCard(img *image.RGBA, cur *layoutCursor, inset int, v *votes.Vote, bg color.RGBA, fonts *fontSet, idx, total int) {
	// Heading naming this vote within the group. Without it the cards of a
	// group whose source publishes no per-vote title are indistinguishable.
	fraktionCounts := voteformat.AggregateFraktionCounts(*v)

	if sub := voteformat.CardSubVoteLabel(*v, idx-1, total); sub != "" {
		maxTextWidth := imgWidth - 2*padding
		subLines := wrapText(fonts.boldHeading, sub, maxTextWidth)
		// Same bargain as the combined card's title: the heading gives way to
		// the results rather than pushing Fraktionen off the bottom. It keeps
		// its face — these labels are short enough that shrinking would only
		// make the common card worse — and is cut with an ellipsis instead.
		available := titleBudget(inset, resultBottomReserve(fonts, len(fraktionCounts)))
		subLines = ellipsizeLines(fonts.boldHeading, subLines, maxTextWidth, available)
		for _, line := range subLines {
			if img != nil {
				drawCenteredText(img, fonts.boldHeading, fonts.emojiLarge, cur.baseline(fonts.boldHeading), line, bg)
			}
			cur.advance(fonts.boldHeading)
			cur.gap(fonts.boldHeading, 0.15)
		}
		cur.gap(fonts.boldHeading, 0.5)
	}

	// Vote counts
	counts := voteformat.CountsOf(*v)

	// Verdict: large centered
	isAuswahl := voteformat.IsAuswahlVote(counts)
	var verdictText string
	switch {
	case isAuswahl:
		verdictText = strings.ToUpper(v.Decision)
	case voteformat.HasVerdict(*v):
		verdictText = voteformat.GetVoteResultEmoji(v.Decision)
	}
	if img != nil {
		drawCenteredText(img, fonts.verdictSm, fonts.emojiVerdict, cur.baseline(fonts.verdictSm), verdictText, bg)
	}
	cur.advance(fonts.verdictSm)
	cur.gap(fonts.verdictSm, 0.75)

	// Horizontal separator
	if img != nil {
		drawHLine(img, cur.y, padding, imgWidth-padding, semiWhite)
	}
	cur.gap(fonts.statNum, 0.75)

	// Stats dashboard
	switch {
	case isAuswahl:
		drawAuswahlStatsDashboard(img, cur, counts, bg, fonts.statNum, fonts.statLabel)
	case voteformat.IsQuorumVote(counts):
		drawQuorumStatsDashboard(img, cur, counts, bg, fonts.statNum, fonts.statLabel)
	default:
		drawStandardStatsDashboard(img, cur, counts, bg, fonts.statNum, fonts.statLabel)
	}

	cur.gap(fonts.statNum, 0.75)

	// Horizontal separator
	if img != nil {
		drawHLine(img, cur.y, padding, imgWidth-padding, semiWhite)
	}
	cur.gap(fonts.partyBold, 1.25)

	// Party breakdown table
	drawFraktionTable(img, cur, fraktionCounts, bg, fonts.partyBold, fonts.partyNum)
}

// formatSummaryLine builds one entry of the Übersicht. A maxSubtitleRunes above
// zero cuts the subtitle to that length; zero leaves it whole.
func formatSummaryLine(index int, vote votes.Vote, groupSize, maxSubtitleRunes int) (string, bool) {
	subtitle := voteformat.SubVoteLabel(vote, index-1, groupSize)
	if subtitle == "" {
		return "", false
	}
	if maxSubtitleRunes > 0 {
		subtitle = truncateWithEllipsis(subtitle, maxSubtitleRunes)
	}

	counts := voteformat.CountsOf(vote)
	var verdict string
	switch {
	case voteformat.IsAuswahlVote(counts):
		verdict = auswahlResultLabel(vote.Decision)
	case voteformat.HasVerdict(vote):
		verdict = voteformat.GetVoteResultEmoji(vote.Decision)
	}
	if verdict == "" {
		return fmt.Sprintf("%d. %s", index, subtitle), true
	}
	return fmt.Sprintf("%d. %s %s", index, verdict, subtitle), true
}

// auswahlResultLabel converts a Schlussresultat like "Auswahl A" to a bracket
// notation like "[A]" for display in summary lines.
func auswahlResultLabel(schlussresultat string) string {
	upper := strings.ToUpper(strings.TrimSpace(schlussresultat))
	letter := upper
	if strings.HasPrefix(upper, "AUSWAHL ") {
		letter = upper[len("AUSWAHL "):]
	}
	return "[" + letter + "]"
}

// cardCorner is what a card prints in its top corners: a timestamp on the
// left and, on a result card, the progress badge on the right.
//
// Every card carries a timestamp there, in the same place and the same form,
// so a reader finds it without learning each card's layout, and a card shared
// on its own still says when the vote was taken.
//
// The row is reserved rather than overlaid: renderCard starts the content
// below it and titleBudget charges the title for it. A heading wrapped to the
// full width would otherwise share the row with the date on any card tall
// enough for centredStart to clamp it to the margin.
type cardCorner struct {
	stamp, badge string
}

// voteCorner stamps a card that reports one vote with its date and time
// ("15.06.2026 11:29"), or the date alone where the source has no clock.
func voteCorner(v votes.Vote, badge string) cardCorner {
	return cardCorner{stamp: voteformat.FormatVoteDateTime(v.Date), badge: badge}
}

// dateCorner stamps the date alone. The overview card spans votes taken at
// several times, which its list already names; a Wahlgeschäft card's only
// timestamp is the roll call's, and printing it would date an election to the
// minute the chamber was counted.
func dateCorner(v votes.Vote) cardCorner {
	return cardCorner{stamp: voteformat.FormatVoteDate(v.Date)}
}

// height is the space the corner row takes from the top of the content area:
// one line of the small face and a gap under it. A corner with nothing to
// print reserves nothing.
func (c cardCorner) height(fonts *fontSet) int {
	if c.stamp == "" && c.badge == "" {
		return 0
	}
	return lineHeight(fonts.small) + frac(fonts.small, 0.5)
}

// draw prints the stamp and the badge on one line under the band.
func (c cardCorner) draw(img *image.RGBA, fonts *fontSet, v votes.Vote, bg color.RGBA) {
	if img == nil {
		return
	}
	y := bandInset(v) + padding + fonts.small.Metrics().Ascent.Ceil()
	if c.stamp != "" {
		drawShadowedText(img, fonts.small, nil, padding, y, c.stamp, bg)
	}
	if c.badge != "" {
		badgeW := font.MeasureString(fonts.small, c.badge).Ceil()
		drawShadowedText(img, fonts.small, nil, imgWidth-padding-badgeW, y, c.badge, bg)
	}
}

func formatProgressBadge(index, total int) string {
	if total <= 1 || index <= 0 || index > total {
		return ""
	}
	return fmt.Sprintf("%d/%d", index, total)
}

func truncateWithEllipsis(text string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	runes := []rune(text)
	if len(runes) <= maxRunes {
		return text
	}
	if maxRunes == 1 {
		return "…"
	}
	return string(runes[:maxRunes-1]) + "…"
}
