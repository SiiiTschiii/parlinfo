package instagram

import (
	"bytes"
	"image/jpeg"
	"strings"
	"testing"

	"github.com/siiitschiii/parlinfo/pkg/geschaeftposting"
	"github.com/siiitschiii/parlinfo/pkg/geschaeftposting/internal/testcontacts"
	"github.com/siiitschiii/parlinfo/pkg/geschaeftposting/testfixtures"
)

// maxCaptionChars is Instagram's caption limit.
const maxCaptionChars = 2200

func TestFormatEveryFixture(t *testing.T) {
	for name, g := range testfixtures.Supported() {
		t.Run(name, func(t *testing.T) {
			c, err := Format(g, nil)
			if err != nil {
				t.Fatal(err)
			}
			if len(c.Images) != 1 {
				t.Fatalf("%d images, want 1", len(c.Images))
			}
			cfg, err := jpeg.DecodeConfig(bytes.NewReader(c.Images[0]))
			if err != nil {
				t.Fatal(err)
			}
			if cfg.Width != 1080 || cfg.Height != 1350 {
				t.Errorf("card is %dx%d, want 1080x1350", cfg.Width, cfg.Height)
			}

			if n := len([]rune(c.Caption)); n > maxCaptionChars {
				t.Errorf("caption is %d characters, limit %d", n, maxCaptionChars)
			}
			// The caption is where nothing is cut: the full title and every
			// signer the text platforms may have shed.
			for _, want := range []string{
				geschaeftposting.Title(g),
				geschaeftposting.SignerLine(g, 0),
				geschaeftposting.Link(g),
				geschaeftposting.ArtenURL,
			} {
				if !strings.Contains(c.Caption, want) {
					t.Errorf("caption lacks %q", want)
				}
			}
		})
	}
}

func TestUnsupportedIsRefused(t *testing.T) {
	for name, g := range testfixtures.Unsupported() {
		if _, err := Format(g, nil); err == nil {
			t.Errorf("%s formatted, want an error", name)
		}
	}
}

func TestSignerIsTagged(t *testing.T) {
	caption := Caption(testfixtures.MotionMember(), testcontacts.Mapper(t))
	if !strings.Contains(caption, "@"+testcontacts.InstagramHandle) {
		t.Errorf("first signer not tagged:\n%s", caption)
	}
}
