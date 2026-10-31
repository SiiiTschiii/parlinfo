package imagegen

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"strings"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
)

// GeschaeftCard is what the image announcing a newly filed Vorstoss shows. It
// is plain text rather than a Geschäft so this package stays ignorant of where
// the text came from, the way the vote cards only know votes.Vote.
type GeschaeftCard struct {
	// Kind is the Geschaeftsart ("Motion"). It picks the card's colours and is
	// the badge, upper-cased.
	Kind      string
	Number    string // GR number, top right
	Dringlich bool
	Title     string
	// Signers is the formatted list of who filed it, without an emoji prefix.
	Signers string
	// Date is the complete date line, e.g. "Eingereicht am 30.09.2026".
	Date string
	// Body names the chamber in the footer.
	Body string
}

// kindColours are a Geschäftsart's header bar and background.
//
// Keyed by type and nothing else, unlike a vote card's colour, which varies by
// business number: a reader scrolling the grid should be able to tell a Motion
// from a Postulat, and either from a vote result, before reading a word.
var kindColours = map[string]struct{ header, bg color.RGBA }{
	"Motion":   {header: color.RGBA{0x2E, 0x5B, 0xBA, 0xFF}, bg: color.RGBA{0x17, 0x25, 0x4A, 0xFF}},
	"Postulat": {header: color.RGBA{0x1F, 0x8A, 0x7A, 0xFF}, bg: color.RGBA{0x0E, 0x3B, 0x36, 0xFF}},
}

// fallbackKindColours is used for a type with no colours of its own, so a new
// type renders unbranded rather than not at all.
var fallbackKindColours = struct{ header, bg color.RGBA }{
	header: color.RGBA{0x55, 0x5B, 0x66, 0xFF}, bg: color.RGBA{0x2A, 0x2E, 0x35, 0xFF},
}

// dringlichRed is the urgent banner, the one colour no Geschäftsart uses.
var dringlichRed = color.RGBA{0xC0, 0x39, 0x2B, 0xFF}

const (
	// geschaeftBannerHeight is the DRINGLICH banner under the header bar.
	geschaeftBannerHeight = 80

	// The title is the card's subject, so it starts far larger than a vote
	// card's — there are no results competing for the room.
	geschaeftTitleMax  = 72.0
	geschaeftTitleMin  = 40.0
	geschaeftTitleStep = 4.0

	// geschaeftSignerLines caps the signer block. Five named signers and a
	// count is the most a recent Vorstoss carries, and three lines hold it;
	// anything longer is in the caption.
	geschaeftSignerLines = 3
)

// GenerateGeschaeftCard renders the single image of a Geschäft announcement,
// 1080×1350 like the vote cards:
//
//   - a header bar in the type's colour, the type badge left and the GR number
//     right — full-width, so the grid's 3:4 crop takes nothing that matters;
//   - a red DRINGLICH banner beneath it when the Vorstoss is urgent;
//   - the title, as large as fits;
//   - who filed it, the date, and the chamber, along the bottom.
func GenerateGeschaeftCard(c GeschaeftCard) ([]byte, error) {
	fonts, err := loadFontSet()
	if err != nil {
		return nil, err
	}

	colours, ok := kindColours[c.Kind]
	if !ok {
		colours = fallbackKindColours
	}
	bg := colours.bg
	img := newImage(bg)
	maxTextWidth := imgWidth - 2*padding

	// Header bar: badge left, number right.
	draw.Draw(img, image.Rect(0, 0, imgWidth, bandHeight), image.NewUniform(colours.header), image.Point{}, draw.Src)
	headerBaseline := centredBaseline(fonts.bandLabel, 0, bandHeight)
	drawTrackedText(img, fonts.bandLabel, padding, headerBaseline, strings.ToUpper(c.Kind), bandTracking, color.White)
	if c.Number != "" {
		x := imgWidth - padding - measureTracked(fonts.bandLabel, c.Number, 0)
		drawTrackedText(img, fonts.bandLabel, x, headerBaseline, c.Number, 0, color.White)
	}

	top := bandHeight
	if c.Dringlich {
		draw.Draw(img, image.Rect(0, top, imgWidth, top+geschaeftBannerHeight), image.NewUniform(dringlichRed), image.Point{}, draw.Src)
		label := "DRINGLICH"
		x := (imgWidth - measureTracked(fonts.bandLabel, label, bandTracking)) / 2
		drawTrackedText(img, fonts.bandLabel, x, centredBaseline(fonts.bandLabel, top, top+geschaeftBannerHeight), label, bandTracking, color.White)
		top += geschaeftBannerHeight
	}

	// Footer, laid out bottom-up so the title gets whatever is left.
	y := imgHeight - padding
	if c.Body != "" {
		label := strings.ToUpper(c.Body)
		x := (imgWidth - measureTracked(fonts.small, label, bandTracking)) / 2
		// Opaque, not semiWhite: a translucent glyph drawn one rune at a time
		// picks up colour fringes once the card is JPEG-encoded.
		drawTrackedText(img, fonts.small, x, y-fonts.small.Metrics().Descent.Ceil(), label, bandTracking, mix(bg, color.RGBA{0xFF, 0xFF, 0xFF, 0xFF}, 0.6))
		y -= lineHeight(fonts.small)
		y -= frac(fonts.small, 0.8)
	}
	if c.Date != "" {
		drawShadowedText(img, fonts.regular, fonts.emojiRegular, padding, y-fonts.regular.Metrics().Descent.Ceil(), "📅 "+c.Date, bg)
		y -= lineHeight(fonts.regular)
	}
	if c.Signers != "" {
		lines := wrapText(fonts.regular, "✍️ "+c.Signers, maxTextWidth)
		if len(lines) > geschaeftSignerLines {
			lines = lines[:geschaeftSignerLines]
			lines[len(lines)-1] = appendEllipsis(fonts.regular, lines[len(lines)-1], maxTextWidth)
		}
		for i := len(lines) - 1; i >= 0; i-- {
			drawShadowedText(img, fonts.regular, fonts.emojiRegular, padding, y-fonts.regular.Metrics().Descent.Ceil(), lines[i], bg)
			y -= lineHeight(fonts.regular)
		}
	}
	footerTop := y
	drawHLine(img, footerTop-frac(fonts.regular, 0.6), padding, imgWidth-padding, semiWhite)

	// Title: the largest size that fits between the header and the footer,
	// centred vertically in that space and set flush left like the footer.
	titleTop := top + padding
	available := footerTop - frac(fonts.regular, 1.2) - titleTop
	face, lines, err := fitGeschaeftTitle(c.Title, maxTextWidth, available)
	if err != nil {
		return nil, err
	}
	cur := newCursor(titleTop+(available-titleBlockHeight(face, len(lines)))/2, imgHeight)
	for _, line := range lines {
		drawShadowedText(img, face, nil, padding, cur.baseline(face), line, bg)
		cur.advance(face)
		cur.gap(face, titleLineGapFactor)
	}

	return encodeJPEG(img)
}

// fitGeschaeftTitle shrinks the bold title face until the wrapped title fits
// available, then ellipsises at the smallest size. Every title in a year of
// Motionen and Postulate fits whole; the ellipsis is for the one that does not.
func fitGeschaeftTitle(title string, maxWidth, available int) (font.Face, []string, error) {
	var face font.Face
	var lines []string
	for size := geschaeftTitleMax; size >= geschaeftTitleMin; size -= geschaeftTitleStep {
		f, err := loadFace(gobold.TTF, size)
		if err != nil {
			return nil, nil, fmt.Errorf("title font: %w", err)
		}
		face, lines = f, wrapText(f, title, maxWidth)
		if titleBlockHeight(f, len(lines)) <= available && widestWord(f, title) <= maxWidth {
			return face, lines, nil
		}
	}
	return face, ellipsizeLines(face, lines, maxWidth, available), nil
}

// mix blends a towards b by t (0 = a, 1 = b), opaque.
func mix(a, b color.RGBA, t float64) color.RGBA {
	lerp := func(x, y uint8) uint8 { return uint8(float64(x) + (float64(y)-float64(x))*t) }
	return color.RGBA{lerp(a.R, b.R), lerp(a.G, b.G), lerp(a.B, b.B), 0xFF}
}

// centredBaseline is the baseline that centres face's cap-to-descender box
// vertically between top and bottom.
func centredBaseline(face font.Face, top, bottom int) int {
	m := face.Metrics()
	return top + (bottom-top-m.Ascent.Ceil()-m.Descent.Ceil())/2 + m.Ascent.Ceil()
}
