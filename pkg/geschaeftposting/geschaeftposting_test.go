package geschaeftposting

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/siiitschiii/parlinfo/pkg/geschaeftposting/testfixtures"
	"github.com/siiitschiii/parlinfo/pkg/zurichapi"
)

func TestSupportedMatchesFixtures(t *testing.T) {
	for name, g := range testfixtures.Supported() {
		if !Supported(g) {
			t.Errorf("%s (%s) not supported", name, g.Geschaeftsart)
		}
	}
	for name, g := range testfixtures.Unsupported() {
		if Supported(g) {
			t.Errorf("%s (%s) supported, should be left out", name, g.Geschaeftsart)
		}
	}
}

// The issue's criterion for a supported type: grounded in at least three real
// filings.
func TestEverySupportedTypeHasThreeFixtures(t *testing.T) {
	count := map[string]int{}
	for _, g := range testfixtures.Supported() {
		count[g.Geschaeftsart]++
	}
	for art := range arten {
		if count[art] < 3 {
			t.Errorf("%s has %d fixtures, want at least 3 real filings", art, count[art])
		}
	}
}

func TestHeadline(t *testing.T) {
	tests := []struct {
		g    zurichapi.Geschaeft
		want string
	}{
		{testfixtures.MotionMember(), "📋 Neue Motion | Gemeinderat Zürich"},
		{testfixtures.MotionDringlich(), "🚨 Neue dringliche Motion | Gemeinderat Zürich"},
		{testfixtures.PostulatCounted(), "📝 Neues Postulat | Gemeinderat Zürich"},
		{testfixtures.PostulatDringlich(), "🚨 Neues dringliches Postulat | Gemeinderat Zürich"},
		{testfixtures.Weisung(), ""},
	}
	for _, tt := range tests {
		if got := Headline(tt.g); got != tt.want {
			t.Errorf("Headline(%s) = %q, want %q", tt.g.GRNr, got, tt.want)
		}
	}
}

// PARIS separates first and last name with U+00A0. Left in, the name matches
// no contact and nobody is tagged.
func TestSignersNormaliseNonBreakingSpace(t *testing.T) {
	for _, s := range Signers(testfixtures.MotionMember()) {
		if strings.ContainsRune(s.Name, '\u00a0') {
			t.Errorf("signer %q keeps a non-breaking space", s.Name)
		}
	}
	if got := Signers(testfixtures.MotionMember())[0].Name; got != "Brigitte Fürer" {
		t.Errorf("first signer = %q", got)
	}
}

func TestSignerLine(t *testing.T) {
	noSigners := testfixtures.MotionCounted()
	noSigners.Erstunterzeichner, noSigners.Mitunterzeichner = nil, nil

	tests := []struct {
		name  string
		g     zurichapi.Geschaeft
		shown int
		want  string
	}{
		{"everyone", testfixtures.MotionMember(), 0,
			"✍️ Brigitte Fürer (Grüne), Tanja Maag (AL), Pascal Lamprecht (SP)"},
		{"names shed into the count", testfixtures.MotionMember(), 1,
			"✍️ Brigitte Fürer (Grüne) +2 weitere"},
		{"unnamed co-signers counted", testfixtures.MotionCounted(), 0,
			"✍️ Selina Frey (GLP), Guy Krayenbühl (GLP), Marisa Kappeler-Schudel (GLP) +8 weitere"},
		{"shed and unnamed add up", testfixtures.MotionCounted(), 1,
			"✍️ Selina Frey (GLP) +10 weitere"},
		{"Fraktionen", testfixtures.PostulatFraktionen(), 0,
			"✍️ Fraktion SP, Fraktion AL, Fraktion Grüne"},
		{"only a count", noSigners, 0, "✍️ 8 Mitunterzeichnende"},
		{"shown beyond the list names everyone", testfixtures.MotionMember(), 99,
			"✍️ Brigitte Fürer (Grüne), Tanja Maag (AL), Pascal Lamprecht (SP)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SignerLine(tt.g, tt.shown); got != tt.want {
				t.Errorf("got  %q\nwant %q", got, tt.want)
			}
		})
	}

	none := testfixtures.MotionMember()
	none.Erstunterzeichner, none.Mitunterzeichner = nil, nil
	if got := SignerLine(none, 0); got != "" {
		t.Errorf("no signers and no count: got %q, want empty", got)
	}
}

func runeLen(s string) int { return utf8.RuneCountInString(s) }

// The title is the subject; names are shed into "+N weitere" before a single
// character of it goes.
func TestFitRootShedsNamesBeforeTitle(t *testing.T) {
	g := testfixtures.MotionCounted()
	full := FitRoot(g, 10000, runeLen, nil, "🔗 link")
	if !strings.Contains(full, Title(g)) || !strings.Contains(full, "Marisa Kappeler-Schudel") {
		t.Fatalf("unbounded root lost content:\n%s", full)
	}

	// Room for the whole title but not for every name.
	limit := runeLen(full) - runeLen("Marisa Kappeler-Schudel (GLP), ")
	got := FitRoot(g, limit, runeLen, nil, "🔗 link")
	if runeLen(got) > limit {
		t.Fatalf("root is %d runes, limit %d", runeLen(got), limit)
	}
	if !strings.Contains(got, Title(g)) {
		t.Errorf("title cut while names could still be shed:\n%s", got)
	}
	if !strings.Contains(got, "+9 weitere") {
		t.Errorf("shed name not counted:\n%s", got)
	}
}

func TestFitRootCutsTitleLastAndKeepsTheRest(t *testing.T) {
	g := testfixtures.PostulatLongestTitle()
	const limit = 280
	got := FitRoot(g, limit, runeLen, nil, "🔗 GR Nr. 2026/48")
	if runeLen(got) > limit {
		t.Fatalf("root is %d runes, limit %d", runeLen(got), limit)
	}
	for _, want := range []string{Headline(g), "✍️ Fraktion SP", DateLine(g), "👇 Was ist ein Postulat?", "🔗 GR Nr. 2026/48", "…"} {
		if !strings.Contains(got, want) {
			t.Errorf("root lost %q:\n%s", want, got)
		}
	}
}

// The tag function's output is what gets measured: an @handle inserted after
// measuring would push the post past the limit the API enforces.
func TestFitRootMeasuresTaggedLine(t *testing.T) {
	g := testfixtures.MotionMember()
	tag := func(s string) string {
		return strings.ReplaceAll(s, "Brigitte Fürer", "Brigitte Fürer @"+strings.Repeat("x", 60))
	}
	untagged := FitRoot(g, 10000, runeLen, nil, "")
	limit := runeLen(untagged) + 10
	got := FitRoot(g, limit, runeLen, tag, "")
	if runeLen(got) > limit {
		t.Errorf("tagged root is %d runes, limit %d:\n%s", runeLen(got), limit, got)
	}
	if !strings.Contains(got, "@"+strings.Repeat("x", 60)) {
		t.Errorf("tag dropped:\n%s", got)
	}
}

func TestTitleCollapsesWhitespace(t *testing.T) {
	g := zurichapi.Geschaeft{Titel: "  Erstens,\r\n zweitens\u00a0 drittens "}
	if got := Title(g); got != "Erstens, zweitens drittens" {
		t.Errorf("Title = %q", got)
	}
}

// The explainer is written to fit one Bluesky reply with its link label, the
// tightest of the platforms. A third post would put the source link below the
// fold.
func TestExplainerCopyFitsOneReply(t *testing.T) {
	const bluesky, linkLabel = 300, "\n\n📖 Geschäftsarten erklärt"
	for name, a := range arten {
		text := a.Explanation + "\n\n" + a.NextSteps + linkLabel
		if n := runeLen(text); n > bluesky {
			t.Errorf("%s explainer is %d runes, limit %d", name, n, bluesky)
		}
	}
}
