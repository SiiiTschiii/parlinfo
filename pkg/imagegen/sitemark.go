package imagegen

import (
	"image"
	"image/color"

	"golang.org/x/image/font"
)

const (
	// siteMark is printed on every card, so an image saved, screenshotted or
	// reposted away from the account that posted it still says where it is
	// from. The account handle would not: it names a channel, not the project,
	// and is gone the moment the image leaves the platform.
	siteMark = "parlinfo.ch"
	// siteMarkSize is small enough to read as a signature rather than content.
	siteMarkSize = 22.0
	// siteMarkAlpha is how far the mark is mixed from the background towards
	// white: legible on every palette colour, but quieter than the white text.
	siteMarkAlpha = 0xA8
	// siteMarkTracking spaces the letters a little, as the band's are, so the
	// two read as one family.
	siteMarkTracking = 2
)

// drawSiteMark prints siteMark centred in the bottom margin.
//
// That margin is empty on every card: centredStart and drawFraktionTable both
// stop the content at imgHeight-padding, and the footer is drawn above it too.
// Centred, it survives the Instagram grid's side crop, as the body band does.
func drawSiteMark(img *image.RGBA, face font.Face, bg color.RGBA) {
	if img == nil {
		return
	}
	m := face.Metrics()
	ascent, descent := m.Ascent.Ceil(), m.Descent.Ceil()
	baseline := imgHeight - padding + (padding-ascent-descent)/2 + ascent
	x := (imgWidth - measureTracked(face, siteMark, siteMarkTracking)) / 2
	drawTrackedText(img, face, x, baseline, siteMark, siteMarkTracking, blend(bg, siteMarkAlpha))
}
