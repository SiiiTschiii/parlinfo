package main

import (
	"strings"
	"testing"
	"time"

	"github.com/siiitschiii/parlinfo/pkg/parlamentch"
	"gopkg.in/yaml.v3"
)

func TestChooseLanguage(t *testing.T) {
	for _, tc := range []struct {
		name    string
		counts  map[string]int
		canton  string
		want    string
		flagged bool
		whyHas  string
	}{
		{"German speaker with the odd French speech", map[string]int{"DE": 19, "FR": 1}, "ZH", "de", false, "19 DE, 1 FR"},
		{"French speaker", map[string]int{"DE": 1, "FR": 19}, "VD", "fr", false, "Reden"},
		// Italian speakers give much of their speaking in German.
		{"Italian speaker who speaks mostly German", map[string]int{"DE": 14, "IT": 6}, "GR", "it", true, "6 IT"},
		{"Italian speaker", map[string]int{"FR": 1, "IT": 19}, "TI", "it", false, "19 IT"},
		{"French speaker with a German share", map[string]int{"DE": 8, "FR": 12}, "BE", "fr", true, "gemischt"},
		{"no speeches, French-speaking canton", nil, "GE", "fr", false, "keine Reden"},
		{"no speeches, Ticino", nil, "TI", "it", false, "Kanton TI"},
		{"no speeches, bilingual canton", nil, "VS", "fr", true, "zweisprachig"},
		{"too few speeches", map[string]int{"FR": 1}, "FR", "fr", true, "Reden unklar"},
		{"even German–French split goes by canton", map[string]int{"DE": 5, "FR": 5}, "ZH", "de", false, "Kanton ZH"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, why := chooseLanguage(tc.counts, tc.canton)
			if got != tc.want {
				t.Errorf("language = %q, want %q (%s)", got, tc.want, why)
			}
			if flagged := strings.Contains(why, "prüfen"); flagged != tc.flagged {
				t.Errorf("flagged = %v, want %v (%s)", flagged, tc.flagged, why)
			}
			if !strings.Contains(why, tc.whyHas) {
				t.Errorf("reason %q lacks %q", why, tc.whyHas)
			}
		})
	}
}

func TestChooseFormal(t *testing.T) {
	born := func(y int) time.Time { return time.Date(y, 6, 1, 0, 0, 0, 0, time.UTC) }
	for _, tc := range []struct {
		lang string
		born time.Time
		want bool
	}{
		{"fr", born(1990), true},
		{"it", born(1990), true},
		{"de", born(1969), true},
		{"de", born(1970), false},
		{"de", time.Time{}, false},
	} {
		if got, why := chooseFormal(tc.lang, tc.born); got != tc.want {
			t.Errorf("chooseFormal(%s, %v) = %v (%s), want %v", tc.lang, tc.born.Year(), got, why, tc.want)
		}
	}
}

// The file is read back by send_campaign_emails, so it has to parse, quotes and
// comments included, into the fields that tool reads.
func TestEntryParsesAsARecipient(t *testing.T) {
	m := parlamentch.MemberDetails{
		FirstName: "Anna-Béatrice", LastName: "de Quelque \"Chose\"", Gender: "f",
		Canton: "VD", Party: "GRÜNE Schweiz", BirthDate: time.Date(1960, 1, 1, 0, 0, 0, 0, time.UTC),
		Emails: []string{"a@parl.ch", "b@example.org"},
	}
	doc := header(time.Date(2026, 11, 22, 0, 0, 0, 0, time.UTC)) + entry(m, map[string]int{"FR": 20})

	var f struct {
		Recipients []struct {
			Name, Lastname, Email, Gender, Role, Party, Canton, Language string
			Formal                                                       bool
		} `yaml:"recipients"`
	}
	if err := yaml.Unmarshal([]byte(doc), &f); err != nil {
		t.Fatalf("does not parse: %v\n%s", err, doc)
	}
	if len(f.Recipients) != 1 {
		t.Fatalf("got %d recipients", len(f.Recipients))
	}
	r := f.Recipients[0]
	if r.Name != `Anna-Béatrice de Quelque "Chose"` || r.Lastname != `de Quelque "Chose"` {
		t.Errorf("name = %q / %q", r.Name, r.Lastname)
	}
	if r.Email != "a@parl.ch" || !strings.Contains(doc, "# auch: b@example.org") {
		t.Errorf("email = %q; the second address should be a comment:\n%s", r.Email, doc)
	}
	if r.Gender != "weiblich" || r.Role != "Nationalrätin" || r.Canton != "VD" || r.Language != "fr" || !r.Formal {
		t.Errorf("parsed %+v", r)
	}
}
