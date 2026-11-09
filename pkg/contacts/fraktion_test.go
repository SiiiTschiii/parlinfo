package contacts

import (
	"reflect"
	"testing"
)

func partyMapper() *Mapper {
	org := func(name string, platform map[string][]string) Contact {
		c := Contact{Name: name, Kind: KindOrganization}
		for p, urls := range platform {
			accounts := VerifiedAccounts(urls...)
			switch p {
			case "x":
				c.X = accounts
			case "instagram":
				c.Instagram = accounts
			case "bluesky":
				c.Bluesky = accounts
			}
		}
		return c
	}
	return newMapper([]Contact{
		org("SP Stadt Zürich", map[string][]string{
			"instagram": {"https://www.instagram.com/spzuerich/"},
			"bluesky":   {"https://bsky.app/profile/spzuerich.ch"},
		}),
		org("Grüne Stadt Zürich", map[string][]string{
			"x":         {"https://x.com/GrueneZuerich"},
			"instagram": {"https://www.instagram.com/gruenezuerich/"},
		}),
		org("AL Stadt Zürich", map[string][]string{
			"x":         {"https://x.com/al_zuerich"},
			"instagram": {"https://www.instagram.com/alternativelinkezh/"},
		}),
		org("SP Kanton Zürich", map[string][]string{
			"x":         {"https://x.com/spkantonzh"},
			"instagram": {"https://www.instagram.com/spkantonzh/"},
		}),
		org("FDP Kanton Zürich", map[string][]string{
			"x": {"https://x.com/fdpzh"},
		}),
		{Name: "Mitte Person", X: VerifiedAccounts("https://x.com/mitteperson")},
	})
}

const fraktionTitle = "Postulat der Grüne-, SP- und AL-Fraktion vom 18.03.2026: Vergabe von Schiffsplätzen"

func TestTagXFraktionen(t *testing.T) {
	m := partyMapper()

	t.Run("city tags city accounts and falls back to the canton", func(t *testing.T) {
		got := m.TagXFraktionen(fraktionTitle, "zurich-city")
		want := "Postulat der Grüne-, SP- und AL-Fraktion @GrueneZuerich @spkantonzh @al_zuerich vom 18.03.2026: Vergabe von Schiffsplätzen"
		if got != want {
			t.Errorf("got  %q\nwant %q", got, want)
		}
	})

	t.Run("canton never uses a city account", func(t *testing.T) {
		got := m.TagXFraktionen(fraktionTitle, "zurich-canton")
		want := "Postulat der Grüne-, SP- und AL-Fraktion @spkantonzh vom 18.03.2026: Vergabe von Schiffsplätzen"
		if got != want {
			t.Errorf("got  %q\nwant %q", got, want)
		}
	})

	t.Run("declined and shortened forms", func(t *testing.T) {
		got := m.TagXFraktionen("Motion der Grünen-Fraktion und der FDP-Fraktion", "zurich-canton")
		want := "Motion der Grünen-Fraktion und der FDP-Fraktion @fdpzh"
		if got != want {
			t.Errorf("got %q want %q", got, want)
		}
		got = m.TagXFraktionen("Motion der Grünen-Fraktion", "zurich-city")
		if got != "Motion der Grünen-Fraktion @GrueneZuerich" {
			t.Errorf("got %q", got)
		}
	})

	t.Run("no false positives outside a Fraktion phrase", func(t *testing.T) {
		for _, in := range []string{
			"Postulat von Anna Muster (SP) vom 01.01.2026: Parkplätze in der Mitte der Stadt",
			"Die SP hat zugestimmt",
			"Verkehr AL-Strasse",
		} {
			if got := m.TagXFraktionen(in, "zurich-city"); got != in {
				t.Errorf("changed %q to %q", in, got)
			}
		}
	})
}

func TestTagInstagramFraktionen(t *testing.T) {
	m := partyMapper()
	got := m.TagInstagramFraktionen(fraktionTitle, "zurich-city")
	want := "Postulat der Grüne-, SP- und AL-Fraktion @gruenezuerich @spzuerich @alternativelinkezh vom 18.03.2026: Vergabe von Schiffsplätzen"
	if got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}

func TestFindBlueskyFraktionMentions(t *testing.T) {
	m := partyMapper()
	got := m.FindBlueskyFraktionMentions(fraktionTitle, "zurich-city")
	// Only SP has a Bluesky account.
	if len(got) != 1 || got[0].Handle != "spzuerich.ch" {
		t.Fatalf("got %+v", got)
	}
	if tok := fraktionTitle[got[0].ByteStart:got[0].ByteEnd]; tok != "SP" {
		t.Errorf("facet covers %q, want SP", tok)
	}
	if !reflect.DeepEqual(m.FindBlueskyFraktionMentions("keine Fraktion hier", "zurich-city"), []BlueskyMention(nil)) {
		t.Error("expected no mentions")
	}
}

func TestPathsForLoadsCantonFallbackForCityOnly(t *testing.T) {
	got := PathsFor("zurich-city")
	want := []string{PathFor("zurich-city"), PathFor("zurich-canton")}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("city only: got %v, want %v", got, want)
	}
	if got := PathsFor("zurich-city", "zurich-canton"); len(got) != 2 {
		t.Errorf("both: got %v, want each file once", got)
	}
	if got := PathsFor("zurich-canton"); !reflect.DeepEqual(got, []string{PathFor("zurich-canton")}) {
		t.Errorf("canton only: got %v", got)
	}
}

// A Nationalrat post names Ständerat members who filed the business, so their
// file is loaded too; a Ständerat preview needs nothing from the Nationalrat.
func TestPathsForLoadsStaenderatForNationalrat(t *testing.T) {
	if got, want := PathsFor("nationalrat"), []string{PathFor("nationalrat"), PathFor("staenderat")}; !reflect.DeepEqual(got, want) {
		t.Errorf("nationalrat: got %v, want %v", got, want)
	}
	if got, want := PathsFor("staenderat"), []string{PathFor("staenderat")}; !reflect.DeepEqual(got, want) {
		t.Errorf("staenderat: got %v, want %v", got, want)
	}
}

// Titles as the Gemeinderat has actually written them, so the patterns are
// pinned to the real spellings rather than the ones we would have guessed.
func TestFraktionPhrasesFromRealTitles(t *testing.T) {
	m := newMapper([]Contact{
		{Name: "SP Stadt Zürich", Kind: KindOrganization, X: VerifiedAccounts("https://x.com/sp")},
		{Name: "SVP Stadt Zürich", Kind: KindOrganization, X: VerifiedAccounts("https://x.com/svp")},
		{Name: "FDP Stadt Zürich", Kind: KindOrganization, X: VerifiedAccounts("https://x.com/fdp")},
		{Name: "Grüne Stadt Zürich", Kind: KindOrganization, X: VerifiedAccounts("https://x.com/gruene")},
		{Name: "GLP Stadt Zürich", Kind: KindOrganization, X: VerifiedAccounts("https://x.com/glp")},
		{Name: "AL Stadt Zürich", Kind: KindOrganization, X: VerifiedAccounts("https://x.com/al")},
		{Name: "Die Mitte Stadt Zürich", Kind: KindOrganization, X: VerifiedAccounts("https://x.com/mitte")},
		{Name: "EVP Kanton Zürich", Kind: KindOrganization, X: VerifiedAccounts("https://x.com/evp")},
	})
	tests := []struct{ title, want string }{
		{"Motion der SP-Fraktion vom 22.10.2025", "Motion der SP-Fraktion @sp vom 22.10.2025"},
		{"Postulat der SP-, Grüne- und Die Mitte/EVP-Fraktion vom 05.02.2025",
			"Postulat der SP-, Grüne- und Die Mitte/EVP-Fraktion @sp @gruene @mitte @evp vom 05.02.2025"},
		{"Dringliche Motion der SP-, Grünen- und AL-Fraktionen betreffend X",
			"Dringliche Motion der SP-, Grünen- und AL-Fraktionen @sp @gruene @al betreffend X"},
		{"Postulat der SP- und Grüne-Fraktionen betreffend X", "Postulat der SP- und Grüne-Fraktionen @sp @gruene betreffend X"},
		{"Motion der Fraktionen SP, SVP, FDP, Grüne, GLP, CVP und AL betreffend X",
			"Motion der Fraktionen SP, SVP, FDP, Grüne, GLP, CVP und AL @sp @svp @fdp @gruene @glp @mitte @al betreffend X"},
		{"Dringliche Motion der SP-, GLP-Fraktionen und der Parlamentsgruppe EVP betreffend X",
			"Dringliche Motion der SP-, GLP-Fraktionen @sp @glp und der Parlamentsgruppe EVP @evp betreffend X"},
		{"2025/354 Antrag der AL-Fraktion zu Dispositivziffer 1", "2025/354 Antrag der AL-Fraktion @al zu Dispositivziffer 1"},
		// No party named: nothing to tag.
		{"Digitalisierung der Prozesse für Vorstösse, Anträge und Fraktionserklärungen",
			"Digitalisierung der Prozesse für Vorstösse, Anträge und Fraktionserklärungen"},
		{"Fraktionserklärung", "Fraktionserklärung"},
		// A party outside a Fraktion phrase is a member's affiliation, not a Fraktion.
		{"Motion von Karin Stepinski (Die Mitte), Roger Föhn (EVP) vom 17.12.2025",
			"Motion von Karin Stepinski (Die Mitte), Roger Föhn (EVP) vom 17.12.2025"},
	}
	for _, tt := range tests {
		if got := m.TagXFraktionen(tt.title, "zurich-city"); got != tt.want {
			t.Errorf("%q\n got  %q\n want %q", tt.title, got, tt.want)
		}
	}
}

// A Nationalrat post names the Fraktion that filed the business, and tags every
// party it seats: the Mitte-EVP-Fraktion names both, the SVP-Fraktion names the
// SVP alone and seats EDU, Lega and MCG beside it. Lega and MCG are cantonal
// parties, found under their own names.
func TestTagXFraktionenNationalrat(t *testing.T) {
	m := newMapper([]Contact{
		{Name: "SVP Schweiz", Kind: KindOrganization, X: VerifiedAccounts("https://x.com/SVPch")},
		{Name: "EDU Schweiz", Kind: KindOrganization, X: VerifiedAccounts("https://x.com/EDUSchweiz")},
		{Name: "Lega dei Ticinesi", Kind: KindOrganization, X: VerifiedAccounts("https://x.com/lega")},
		{Name: "Die Mitte Schweiz", Kind: KindOrganization, X: VerifiedAccounts("https://x.com/Mitte_Centre")},
		{Name: "EVP Schweiz", Kind: KindOrganization, X: VerifiedAccounts("https://x.com/evp")},
		{Name: "GLP Schweiz", Kind: KindOrganization, X: VerifiedAccounts("https://x.com/grunliberale")},
		{Name: "Grüne Schweiz", Kind: KindOrganization, X: VerifiedAccounts("https://x.com/GrueneCH")},
		{Name: "SP Kanton Zürich", Kind: KindOrganization, X: VerifiedAccounts("https://x.com/spkantonzh")},
	})
	tests := []struct{ in, want string }{
		// MCG has no account here, so it is skipped rather than guessed.
		{"Parlamentarische Initiative der SVP-Fraktion", "Parlamentarische Initiative der SVP-Fraktion @SVPch @EDUSchweiz @lega"},
		{"Parlamentarische Initiative der Mitte-EVP-Fraktion", "Parlamentarische Initiative der Mitte-EVP-Fraktion @Mitte_Centre @evp"},
		{"Parlamentarische Initiative der GLP-Fraktion", "Parlamentarische Initiative der GLP-Fraktion @grunliberale"},
		{"Parlamentarische Initiative der Grüne-Fraktion", "Parlamentarische Initiative der Grüne-Fraktion @GrueneCH"},
		// The national level only: a cantonal account is never a federal tag.
		{"Parlamentarische Initiative der SP-Fraktion", "Parlamentarische Initiative der SP-Fraktion"},
	}
	for _, tt := range tests {
		if got := m.TagXFraktionen(tt.in, "nationalrat"); got != tt.want {
			t.Errorf("%q\n got  %q\n want %q", tt.in, got, tt.want)
		}
	}

	// The SVP-Fraktion of the Gemeinderat is the SVP's alone, and a cantonal
	// party is not looked up for a jurisdiction that has none of its own.
	if got := m.TagXFraktionen("Motion der SVP-Fraktion", "zurich-city"); got != "Motion der SVP-Fraktion" {
		t.Errorf("zurich-city: got %q", got)
	}
}
