package contacts

import (
	"regexp"
	"sort"
	"strings"
)

// partyAliases maps every spelling of a party that a Fraktion phrase uses to
// the name its organization entry carries. "Grünen" and "Mitte" are the
// declined and shortened forms: titles say "der Grünen-Fraktion" and
// "Mitte-Fraktion" as readily as "Grüne" and "Die Mitte".
var partyAliases = map[string]string{
	"SP":           "SP",
	"FDP":          "FDP",
	"SVP":          "SVP",
	"GLP":          "GLP",
	"AL":           "AL",
	"EVP":          "EVP",
	"EDU":          "EDU",
	"Grüne":        "Grüne",
	"Grünen":       "Grüne",
	"Die Mitte":    "Die Mitte",
	"Mitte":        "Die Mitte",
	"Grünliberale": "GLP",
	"CVP":          "Die Mitte",
}

// partyLevels are the organization-entry suffixes consulted for a
// jurisdiction, most specific first. A Gemeinderat vote tags the city party and
// falls back to the canton party on a platform where the city one has no
// account; a Kantonsrat vote has nobody to fall back to above it.
var partyLevels = map[string][]string{
	"zurich-city":   {"Stadt Zürich", "Kanton Zürich"},
	"zurich-canton": {"Kanton Zürich"},
}

// partyFallbacks are the jurisdictions whose contacts file a jurisdiction's
// party tagging reads beyond its own. partyLevels names the entries to consult;
// this is what makes sure their file is loaded when the canton itself is not
// being posted, previewed or checked.
var partyFallbacks = map[string][]string{
	"zurich-city": {"zurich-canton"},
}

// authorFallbacks are the jurisdictions whose members can file business that
// another jurisdiction votes on. A parliamentary initiative filed in the
// Ständerat goes to the Nationalrat too, and its post names the Ständerat
// member who filed it: 23.439, Andrea Caroni's, came to the Nationalrat's
// overall vote in September 2026. Without the Ständerat's file that name goes
// untagged however well it is curated.
var authorFallbacks = map[string][]string{
	"nationalrat": {"staenderat"},
}

// PathsFor returns the contacts files needed to tag the given jurisdictions:
// their own, then the files their party accounts and their business's authors
// fall back to, each once.
func PathsFor(jurisdictions ...string) []string {
	var paths []string
	seen := map[string]bool{}
	add := func(key string) {
		if !seen[key] {
			seen[key] = true
			paths = append(paths, PathFor(key))
		}
	}
	for _, key := range jurisdictions {
		add(key)
	}
	for _, key := range jurisdictions {
		for _, fb := range partyFallbacks[key] {
			add(fb)
		}
		for _, fb := range authorFallbacks[key] {
			add(fb)
		}
	}
	return paths
}

var (
	fraktionRe = buildFraktionRegexp()
	prefixRe   = buildPrefixRegexp()
	aliasRe    = buildAliasRegexp()
)

func aliasAlternation() string {
	aliases := make([]string, 0, len(partyAliases))
	for a := range partyAliases {
		aliases = append(aliases, a)
	}
	// Longest first, so "Die Mitte" is not read as "Mitte" and "Grünen" not as
	// "Grün" — alternation takes the first branch that matches.
	sort.Slice(aliases, func(i, j int) bool { return len(aliases[i]) > len(aliases[j]) })
	for i, a := range aliases {
		aliases[i] = regexp.QuoteMeta(a)
	}
	return strings.Join(aliases, "|")
}

// buildFraktionRegexp matches a run of parties joined by "-", ",", "/", "und" or
// "sowie" that ends in "-Fraktion(en)": "SP-, Grüne- und AL-Fraktion".
//
// Requiring the closing Fraktion is what keeps "in der Mitte" or "AL" in
// another word from being read as a party.
func buildFraktionRegexp() *regexp.Regexp {
	alias := `(?:` + aliasAlternation() + `)`
	sep := `(?:\s*[-/,]+\s*(?:(?:und|sowie)\s+)?|\s+(?:und|sowie)\s+)`
	return regexp.MustCompile(`(?:^|[^\p{L}])(` + alias + `(?:` + sep + alias + `)*)-?\s?Fraktion(?:en)?\b`)
}

// buildPrefixRegexp matches the older form where the word comes first and the
// parties follow: "Fraktionen SP, SVP, FDP und AL", "Parlamentsgruppe EVP".
// Spaces only between the word and the list, so a line break never reads the
// next line as a party.
func buildPrefixRegexp() *regexp.Regexp {
	alias := `(?:` + aliasAlternation() + `)`
	sep := `(?:\s*[-/,]+\s*(?:(?:und|sowie)\s+)?|\s+(?:und|sowie)\s+)`
	return regexp.MustCompile(`(?:^|[^\p{L}])(?:Fraktionen?|Parlamentsgruppe)[ ]+(` + alias + `(?:` + sep + alias + `)*)(?:[^\p{L}-]|$)`)
}

func buildAliasRegexp() *regexp.Regexp {
	return regexp.MustCompile(`(?:^|[^\p{L}])(` + aliasAlternation() + `)(?:[^\p{L}]|$)`)
}

// fraktionSpan is one Fraktion phrase and the parties named in it.
type fraktionSpan struct {
	end     int // byte offset just past "Fraktion"
	parties []partyMention
}

type partyMention struct {
	party      string
	start, end int // the party token in the text
}

func findFraktionSpans(text string) []fraktionSpan {
	var spans []fraktionSpan
	add := func(loc []int, end int) {
		listStart, listEnd := loc[2], loc[3]
		span := fraktionSpan{end: end}
		// Re-scan the list for the parties in it. The alias regexp consumes the
		// character after a token, so step by hand to catch adjacent ones.
		for pos := listStart; pos < listEnd; {
			m := aliasRe.FindStringSubmatchIndex(text[pos:listEnd])
			if m == nil {
				break
			}
			s, e := pos+m[2], pos+m[3]
			span.parties = append(span.parties, partyMention{party: partyAliases[text[s:e]], start: s, end: e})
			pos = e
		}
		spans = append(spans, span)
	}
	for _, loc := range fraktionRe.FindAllStringSubmatchIndex(text, -1) {
		add(loc, loc[1])
	}
	for _, loc := range prefixRe.FindAllStringSubmatchIndex(text, -1) {
		add(loc, loc[3])
	}
	// tagFraktionen inserts from the back, so it needs them in text order.
	sort.Slice(spans, func(i, j int) bool { return spans[i].end < spans[j].end })
	return spans
}

// partyHandle returns the verified account a party has on a platform at the
// jurisdiction's level, falling back to the next level up.
func (m *Mapper) partyHandle(party, platform, jurisdiction string, extract func(string) string) string {
	for _, level := range partyLevels[jurisdiction] {
		contact, ok := m.GetContact(party + " " + level)
		if !ok || !contact.IsOrganization() {
			continue
		}
		for _, url := range contact.Verified(platform) {
			if h := extract(url); h != "" {
				return h
			}
		}
	}
	return ""
}

// tagFraktionen appends the handles of the parties named in a Fraktion phrase
// after the phrase: "… der SP-, Grüne- und AL-Fraktion @spzuerich @gruene …".
func (m *Mapper) tagFraktionen(text, platform, jurisdiction string, extract func(string) string) string {
	spans := findFraktionSpans(text)
	// Back to front, so an insertion does not shift the offsets still to come.
	for i := len(spans) - 1; i >= 0; i-- {
		var handles []string
		seen := map[string]bool{}
		for _, p := range spans[i].parties {
			h := m.partyHandle(p.party, platform, jurisdiction, extract)
			if h == "" || seen[h] {
				continue
			}
			seen[h] = true
			handles = append(handles, h)
		}
		if len(handles) == 0 {
			continue
		}
		end := spans[i].end
		text = text[:end] + " " + strings.Join(handles, " ") + text[end:]
	}
	return text
}

// TagXFraktionen tags the parties of a Fraktion phrase with their X handles.
func (m *Mapper) TagXFraktionen(text, jurisdiction string) string {
	return m.tagFraktionen(text, "x", jurisdiction, ExtractXHandleFromURL)
}

// TagInstagramFraktionen tags the parties of a Fraktion phrase with their
// Instagram handles.
func (m *Mapper) TagInstagramFraktionen(text, jurisdiction string) string {
	return m.tagFraktionen(text, "instagram", jurisdiction, ExtractInstagramHandleFromURL)
}

// FindBlueskyFraktionMentions returns a mention over each party token of a
// Fraktion phrase whose party has a Bluesky account. Like FindBlueskyMentions
// it leaves the text alone: the party name itself becomes the link.
func (m *Mapper) FindBlueskyFraktionMentions(text, jurisdiction string) []BlueskyMention {
	var out []BlueskyMention
	for _, span := range findFraktionSpans(text) {
		for _, p := range span.parties {
			h := m.partyHandle(p.party, "bluesky", jurisdiction, ExtractBlueskyHandleFromURL)
			if h == "" {
				continue
			}
			out = append(out, BlueskyMention{Handle: h, ByteStart: p.start, ByteEnd: p.end})
		}
	}
	return out
}

// NewMapperForTest builds a mapper from contacts in memory, so tests in other
// packages can exercise tagging without a file on disk.
func NewMapperForTest(cs []Contact) *Mapper { return newMapper(cs) }
