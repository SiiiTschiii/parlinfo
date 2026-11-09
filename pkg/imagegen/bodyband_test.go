package imagegen

import (
	"bytes"
	"image/color"
	"image/jpeg"
	"testing"

	"github.com/siiitschiii/parlinfo/pkg/config"
	"github.com/siiitschiii/parlinfo/pkg/voteposting/testfixtures"
	"github.com/siiitschiii/parlinfo/pkg/votes"
)

// TestBandColoursDifferPerBody pins the property the band exists for: two
// bodies posting to one account render visibly different cards.
func TestBandColoursDifferPerBody(t *testing.T) {
	city := testfixtures.SingleVoteAngenommen()
	canton := testfixtures.KantonsratVote()

	cityBand := bandPixel(t, city)
	cantonBand := bandPixel(t, canton)

	if cityBand == cantonBand {
		t.Fatalf("both bodies rendered the same band colour %v", cityBand)
	}
	if want := bandAccent(city[0].Jurisdiction); !closeTo(cityBand, want) {
		t.Errorf("city band = %v, want ~%v", cityBand, want)
	}
	if want := bandAccent(canton[0].Jurisdiction); !closeTo(cantonBand, want) {
		t.Errorf("canton band = %v, want ~%v", cantonBand, want)
	}
}

// TestEveryJurisdictionHasABandColour guards the one silent failure mode: a
// body without a colour of its own shares the fallback with every other such
// body, leaving their cards indistinguishable. A body with no vote source
// renders no cards, so it needs a colour only once it gets one.
func TestEveryJurisdictionHasABandColour(t *testing.T) {
	for _, key := range config.JurisdictionKeys() {
		j, err := config.LookupJurisdiction(key)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := j.Source(); err != nil {
			continue
		}
		if _, ok := bodyAccents[key]; !ok {
			t.Errorf("jurisdiction %q has no band colour in bodyAccents", key)
		}
	}
}

func TestBandInset_SkippedWithoutBodyName(t *testing.T) {
	v := testfixtures.SingleVoteAngenommen()[0]
	if got := bandInset(v); got != bandHeight {
		t.Errorf("bandInset with body = %d, want %d", got, bandHeight)
	}
	v.Body = ""
	if got := bandInset(v); got != 0 {
		t.Errorf("bandInset without body = %d, want 0", got)
	}
}

// TestCardContentClearsBand checks the layout reserves the band's space rather
// than centring content over it.
func TestCardContentClearsBand(t *testing.T) {
	fonts, err := loadFontSet()
	if err != nil {
		t.Fatalf("loadFontSet failed: %v", err)
	}
	v := testfixtures.SingleVoteAngenommen()[0]
	bg := SelectColor(v.Jurisdiction, v.Affair.Number)

	cur := newCursor(bandInset(v), imgHeight)
	if _, _, err := layoutCombinedCard(nil, cur, bandInset(v), &v, bg, fonts); err != nil {
		t.Fatalf("layoutCombinedCard failed: %v", err)
	}
	if cur.startY < bandHeight {
		t.Fatalf("content starts at y=%d, inside the %dpx band", cur.startY, bandHeight)
	}
}

// bandPixel samples the band of a group's first rendered card, above the
// centred label so that a change in font metrics cannot move text under the
// sample point.
func bandPixel(t *testing.T, group []votes.Vote) color.RGBA {
	t.Helper()
	images, err := GenerateCarousel(group)
	if err != nil {
		t.Fatalf("GenerateCarousel failed: %v", err)
	}
	img, err := jpeg.Decode(bytes.NewReader(images[0]))
	if err != nil {
		t.Fatalf("decoding JPEG: %v", err)
	}
	r, g, b, _ := img.At(20, 12).RGBA()
	return color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), 0xFF}
}

// closeTo compares a sampled pixel with the intended colour, allowing for JPEG
// quantisation.
func closeTo(got, want color.RGBA) bool {
	const tolerance = 8
	diff := func(a, b uint8) int {
		if a > b {
			return int(a - b)
		}
		return int(b - a)
	}
	return diff(got.R, want.R) <= tolerance &&
		diff(got.G, want.G) <= tolerance &&
		diff(got.B, want.B) <= tolerance
}
