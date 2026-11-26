package imagegen

import "testing"

// TestSiteMarkStaysInTheBottomMargin checks that the mark is drawn, and only
// where no content can be: in the bottom margin, clear of the strip the
// Instagram grid crops off each side.
func TestSiteMarkStaysInTheBottomMargin(t *testing.T) {
	fonts, err := loadFontSet()
	if err != nil {
		t.Fatalf("loadFontSet failed: %v", err)
	}
	bg := palette[0]
	img := newImage(bg)
	drawSiteMark(img, fonts.siteMark, bg)

	const gridCrop = 34 // (1080 - 1350*3/4) / 2
	drawn := 0
	for y := 0; y < imgHeight; y++ {
		for x := 0; x < imgWidth; x++ {
			if img.RGBAAt(x, y) == bg {
				continue
			}
			drawn++
			if y < imgHeight-padding || x < gridCrop || x >= imgWidth-gridCrop {
				t.Fatalf("mark drew at (%d,%d), outside the bottom margin", x, y)
			}
		}
	}
	if drawn == 0 {
		t.Fatal("mark drew nothing")
	}
}
