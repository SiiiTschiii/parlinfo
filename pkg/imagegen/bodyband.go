package imagegen

import (
	"image"
	"image/color"
	"image/draw"
	"strings"

	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"

	"github.com/siiitschiii/parlinfo/pkg/votes"
)

const (
	// bandHeight is the vertical space the body band claims at the top of a
	// card. Content starts below it.
	bandHeight = 132
	// bandTracking is the extra spacing between the label's letters. Wide
	// tracking is what makes an all-caps label read as a wordmark rather than
	// as shouting; the font has none built in.
	bandTracking = 6
)

// bodyAccents is the band colour each body's cards carry, keyed by
// votes.Vote.Jurisdiction.
//
// Several bodies post to the same accounts, and a reader scrolling the feed
// takes in colour long before they read anything, so each body's colour has to
// be its own and always the same. These are the administrations' official
// corporate-design colours.
//
// A body added here without a colour falls back to an unbranded band rather
// than an unlabelled card; TestEveryJurisdictionHasABandColour keeps that from
// going unnoticed.
var bodyAccents = map[string]color.RGBA{
	"zurich-city":   {0x0F, 0x05, 0xA0, 0xFF},
	"zurich-canton": {0x00, 0x70, 0xB4, 0xFF},
	// The Bund's red. The Nationalrat posts to accounts of its own, but its
	// cards follow the same rule.
	"nationalrat":    {0xDC, 0x00, 0x18, 0xFF},
	"nationalrat-fr": {0xDC, 0x00, 0x18, 0xFF},
	"nationalrat-it": {0xDC, 0x00, 0x18, 0xFF},
}

// fallbackAccent is the band colour for a body with no assigned colour.
var fallbackAccent = color.RGBA{0x3A, 0x3F, 0x47, 0xFF}

// bandAccent returns the band colour for a jurisdiction.
func bandAccent(jurisdiction string) color.RGBA {
	if c, ok := bodyAccents[jurisdiction]; ok {
		return c
	}
	return fallbackAccent
}

// bandInset is the vertical space the band takes on this card, and so the Y the
// card's content has to start below. A vote with no body name gets no band.
func bandInset(v votes.Vote) int {
	if v.Body == "" {
		return 0
	}
	return bandHeight
}

// drawBodyBand draws the full-width bar naming the chamber that voted.
//
// It spans the full width deliberately: Instagram's profile grid crops a 4:5
// post to 3:4, losing ~34px off each side, so anything that has to survive the
// grid cannot live in a corner.
func drawBodyBand(img *image.RGBA, face font.Face, v votes.Vote) {
	if img == nil || v.Body == "" {
		return
	}

	accent := bandAccent(v.Jurisdiction)
	draw.Draw(img, image.Rect(0, 0, imgWidth, bandHeight), image.NewUniform(accent), image.Point{}, draw.Src)

	label := strings.ToUpper(v.Body)
	m := face.Metrics()
	baseline := (bandHeight-m.Ascent.Ceil()-m.Descent.Ceil())/2 + m.Ascent.Ceil()
	x := (imgWidth - measureTracked(face, label, bandTracking)) / 2
	drawTrackedText(img, face, x, baseline, label, bandTracking, color.White)
}

// measureTracked returns the width of s drawn with extra spacing between
// glyphs. font.MeasureString already accounts for kerning, which is why
// drawTrackedText has to reapply it.
func measureTracked(face font.Face, s string, tracking int) int {
	w := font.MeasureString(face, s).Ceil()
	if n := len([]rune(s)); n > 1 {
		w += (n - 1) * tracking
	}
	return w
}

// drawTrackedText draws s one glyph at a time, adding tracking between them.
//
// Drawing per glyph means the kerning font.Drawer would apply within a single
// DrawString has to be applied by hand. Without it the drawn label is wider
// than measureTracked reports and the centring drifts by that difference.
func drawTrackedText(img *image.RGBA, face font.Face, x, y int, s string, tracking int, fg color.Color) {
	d := &font.Drawer{Dst: img, Src: image.NewUniform(fg), Face: face, Dot: fixed.P(x, y)}
	first := true
	var prev rune
	for _, r := range s {
		if !first {
			d.Dot.X += face.Kern(prev, r) + fixed.I(tracking)
		}
		d.DrawString(string(r))
		prev, first = r, false
	}
}
