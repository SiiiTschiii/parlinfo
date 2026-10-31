package imagegen

import (
	"bytes"
	"image/jpeg"
	"strings"
	"testing"
)

func TestGenerateGeschaeftCard(t *testing.T) {
	cases := map[string]GeschaeftCard{
		"motion": {Kind: "Motion", Number: "2026/541", Title: "Ausscheidung einer Freihaltezone auf dem Schlachthofareal",
			Signers: "Brigitte Fürer (Grüne), Tanja Maag (AL)", Date: "Eingereicht am 30.09.2026", Body: "Gemeinderat Zürich"},
		"dringlich postulat": {Kind: "Postulat", Number: "2026/109", Dringlich: true, Title: "Strassenraum entlang der Tramtangente Nord",
			Signers: "Fraktion Grüne", Date: "Eingereicht am 11.03.2026", Body: "Gemeinderat Zürich"},
		// More title than the card can hold at the smallest size, and more
		// signers than the block's three lines: both must be cut, not overflow.
		"overlong": {Kind: "Motion", Number: "2026/1", Title: strings.Repeat("Sehr langer Titel mit vielen Wörtern ", 40),
			Signers: strings.Repeat("Vorname Nachname (Partei), ", 30), Date: "Eingereicht am 01.01.2026"},
		// A type with no colours of its own renders in the fallback.
		"unknown kind": {Kind: "Interpellation", Title: "Kurz"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			data, err := GenerateGeschaeftCard(c)
			if err != nil {
				t.Fatal(err)
			}
			cfg, err := jpeg.DecodeConfig(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if cfg.Width != imgWidth || cfg.Height != imgHeight {
				t.Errorf("card is %dx%d", cfg.Width, cfg.Height)
			}
		})
	}
}

// A title that fits must fit whole; only one that cannot gets an ellipsis.
func TestFitGeschaeftTitle(t *testing.T) {
	_, lines, err := fitGeschaeftTitle("Kurzer Titel", imgWidth-2*padding, 600)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(lines, " ") != "Kurzer Titel" {
		t.Errorf("short title changed: %q", lines)
	}

	face, lines, err := fitGeschaeftTitle(strings.Repeat("Wort ", 400), imgWidth-2*padding, 600)
	if err != nil {
		t.Fatal(err)
	}
	if titleBlockHeight(face, len(lines)) > 600 {
		t.Errorf("%d lines overflow the 600px budget", len(lines))
	}
	if !strings.HasSuffix(lines[len(lines)-1], "…") {
		t.Errorf("cut title not marked: %q", lines[len(lines)-1])
	}
}
