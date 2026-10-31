package x

import (
	"strings"
	"testing"
)

func TestWeightedLen(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want int
	}{
		{"ascii", "Ja", 2},
		// Umlauts are inside the weight-1 range, so German text costs its rune
		// count — not its byte count, which is what the packer used to charge.
		{"umlaut", "Grünliberale", 12},
		{"emoji", "📊", 2},
		// A variation selector is a code point of its own and X charges for it.
		{"emoji with variation selector", "🏛️", 4},
		{"url replaces its own length", "https://zh.recapp.ch/shareparl?agendaItemUid=82166c96-87f8-4fdb-8fd7-20af55278ec4&segmentUid=e815525c-45ef-475e-a2db-3f644e0f0c0b", urlWeightedLen},
		{"short url costs the same", "https://a.ch", urlWeightedLen},
		{"url after text", "🔗 https://a.ch", 2 + 1 + urlWeightedLen},
		{"two urls", "https://a.ch https://b.ch", urlWeightedLen + 1 + urlWeightedLen},
		// Only URLs at a boundary count as URLs; anything else is charged per
		// rune, which overcounts rather than under.
		{"not a url", "xhttps://a.ch", 13},
		{"ellipsis", "…", 2},
		// X linkifies bare domains. This one is on every Kanton Zürich post.
		{"attribution line", "Daten: OpenParlData.ch", 7 + urlWeightedLen},
		{"bare domain with path", "kantonsrat.zh.ch/geschaefte", urlWeightedLen},
		// A date is not a link, and charging it 23 would waste header room.
		{"date is not a domain", "15.06.2026", 10},
		{"abbreviation is not a domain", "z.B.", 4},
		{"decimal is not a domain", "3.5", 3},
		{"word is not a domain", "Verkehr", 7},
		// Sentence punctuation is not part of the link X makes, so it is charged
		// separately. Swallowing it into the flat 23 undercounts.
		{"url followed by a comma", "https://a.ch,", urlWeightedLen + 1},
		{"bare domain followed by a full stop", "OpenParlData.ch.", urlWeightedLen + 1},
		{"attribution at the end of a sentence", "Daten: OpenParlData.ch.", 7 + urlWeightedLen + 1},
		{"bare domain in parentheses", "(OpenParlData.ch)", 1 + urlWeightedLen + 1},
		{"url with a query string keeps its punctuation", "https://a.ch/x?id=1&y=2", urlWeightedLen},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := weightedLen(tc.in); got != tc.want {
				t.Errorf("weightedLen(%q) = %d, want %d", tc.in, got, tc.want)
			}
		})
	}
}

func TestTruncateToWeighted(t *testing.T) {
	// Truncation has to respect the same weights, or a title trimmed to "fit"
	// still overshoots and the API rejects the root post.
	if got := truncateToWeighted("Grünliberale", 5); got != "Grünl" {
		t.Errorf("truncateToWeighted() = %q, want %q", got, "Grünl")
	}
	if got := truncateToWeighted("📊📊📊", 4); got != "📊📊" {
		t.Errorf("truncateToWeighted() = %q, want %q", got, "📊📊")
	}
	if got := truncateToWeighted("kurz", 99); got != "kurz" {
		t.Errorf("truncateToWeighted() = %q, want %q", got, "kurz")
	}
}

// The guarantee truncation exists for: whatever comes back must actually fit,
// or a "truncated" title still overshoots and the API rejects the root post.
//
// Cutting a string can raise its cost per character. A title cut through a URL
// leaves something like "https://aver", which still reads as a link and is
// still charged 23 — more than the twelve runes it now holds.
func TestTruncateToWeightedAlwaysFits(t *testing.T) {
	inputs := []string{
		"Teilrevision 2022 des kantonalen Richtplans, Kapitel 4 «Verkehr»",
		"Details: https://www.kantonsrat.zh.ch/geschaefte/geschaeft/?id=6b13c255e8c94b10adfde33dede18c8c und weiter",
		"https://averylongdomainname.example.ch/with/a/path",
		"Quelle OpenParlData.ch, Stand 15.06.2026",
		"🗳️ 🏛️ 📊 Abstimmung über die Änderung",
	}

	for _, in := range inputs {
		for maxLen := 0; maxLen <= weightedLen(in)+2; maxLen++ {
			got := truncateToWeighted(in, maxLen)
			if cost := weightedLen(got); cost > maxLen {
				t.Errorf("truncateToWeighted(%q, %d) = %q, which costs %d", in, maxLen, got, cost)
			}
			if !strings.HasPrefix(in, got) {
				t.Errorf("truncateToWeighted(%q, %d) = %q, not a prefix", in, maxLen, got)
			}
		}
	}
}

// TestWeightedLenNeverUndercounts is the property that matters operationally:
// a post the formatter believes fits must fit, because the alternative is a
// published root post whose replies were rejected mid-thread.
func TestWeightedLenNeverUndercounts(t *testing.T) {
	for _, s := range []string{
		"🗳️  Kantonsrat ZH | Abstimmung vom 15.06.2026",
		"🏛️ Fraktionen (Ja/Nein/Enth/Abw):\nGrünliberale 23/0/0/0",
		"📊 130 Ja | 44 Nein | 0 Enthaltung | 6 Abwesend",
		"✅ Angenommen: Änderungsantrag zu Dispositivziffer 1",
	} {
		if got, runes := weightedLen(s), len([]rune(s)); got < runes {
			t.Errorf("weightedLen(%q) = %d, below rune count %d", s, got, runes)
		}
	}
}
