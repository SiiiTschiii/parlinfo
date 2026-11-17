package contacts

import (
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
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

// Every body loads every roster, its own first and its party fallback next:
// a Gemeinderat Weisung can name an author who has since moved to the
// Nationalrat, and a Nationalrat vote one who sits in the Ständerat.
func TestPathsForLoadsEveryRoster(t *testing.T) {
	tests := []struct {
		keys []string
		want []string
	}{
		{[]string{"zurich-city"}, []string{"zurich-city", "zurich-canton", "nationalrat", "staenderat"}},
		{[]string{"zurich-canton"}, []string{"zurich-canton", "zurich-city", "nationalrat", "staenderat"}},
		{[]string{"nationalrat"}, []string{"nationalrat", "zurich-city", "zurich-canton", "staenderat"}},
		{[]string{"staenderat"}, []string{"staenderat", "zurich-city", "zurich-canton", "nationalrat"}},
		{[]string{"zurich-city", "zurich-canton"}, []string{"zurich-city", "zurich-canton", "nationalrat", "staenderat"}},
	}
	for _, tt := range tests {
		var want []string
		for _, k := range tt.want {
			want = append(want, PathFor(k))
		}
		if got := PathsFor(tt.keys...); !reflect.DeepEqual(got, want) {
			t.Errorf("PathsFor(%v) = %v, want %v", tt.keys, got, want)
		}
	}
}

// Loading every roster into one tagger is only safe while a name means one
// person. A name in two files is accepted when one side has no verified
// accounts yet (a dual mandate still being curated there) or when both share
// at least one: that shared account is the evidence they are the same person.
// Otherwise the merged contact would carry one person's handles into the
// other's posts, on whichever platforms each happens to have.
func TestRostersShareNoHomonyms(t *testing.T) {
	type entry struct {
		roster string
		c      Contact
	}
	byName := map[string][]entry{}
	for _, key := range Rosters() {
		path := filepath.Join("..", "..", PathFor(key))
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		var m ContactMapping
		if err := yaml.Unmarshal(data, &m); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, c := range m.Contacts {
			byName[NameKey(c.Name)] = append(byName[NameKey(c.Name)], entry{key, c})
		}
	}
	for _, es := range byName {
		for i := range es {
			for j := i + 1; j < len(es); j++ {
				if es[i].roster == es[j].roster {
					continue
				}
				a, b := verifiedURLs(es[i].c), verifiedURLs(es[j].c)
				if len(a) > 0 && len(b) > 0 && !overlaps(a, b) {
					t.Errorf("%q is in %s and %s with no verified account in common (%v, %v): two people, or one account to add to both",
						es[i].c.Name, es[i].roster, es[j].roster, a, b)
				}
			}
		}
	}
}

func verifiedURLs(c Contact) []string {
	var out []string
	for _, p := range []string{"x", "facebook", "instagram", "linkedin", "bluesky", "tiktok"} {
		out = append(out, c.Verified(p)...)
	}
	return out
}

// overlaps compares accounts by host and path, so "www.", "m.", a query string
// or a trailing slash written differently in two files is still one account.
// ValidateURL guarantees every entry is a full http(s) URL.
func overlaps(a, b []string) bool {
	norm := func(raw string) string {
		u, err := url.Parse(strings.TrimSpace(raw))
		if err != nil {
			return strings.ToLower(raw)
		}
		host := strings.TrimPrefix(strings.TrimPrefix(strings.ToLower(u.Host), "www."), "m.")
		return host + strings.ToLower(strings.TrimRight(u.Path, "/"))
	}
	for _, x := range a {
		for _, y := range b {
			if norm(x) == norm(y) {
				return true
			}
		}
	}
	return false
}
