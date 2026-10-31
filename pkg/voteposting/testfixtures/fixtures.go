// Package testfixtures holds hand-built vote groups covering the shapes the
// formatters have to handle: single and multi-vote groups, Auswahl votes,
// titles long enough to truncate, and titles that trigger @mention tagging.
//
// Fixtures are votes.Vote values rather than raw API payloads, so they exercise
// the same neutral model every source produces. They are shaped after Stadt
// Zürich data — including its URLs — because that is the jurisdiction whose
// output the golden snapshot pins.
package testfixtures

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/siiitschiii/parlinfo/pkg/votes"
)

// sitzungDate is the sitting date shared by most fixtures.
var sitzungDate = date(2025, 6, 15)

func date(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func intPtr(i int) *int {
	return &i
}

// MustDate parses a YYYY-MM-DD date, panicking on malformed input. Intended for
// fixtures and tests, where a bad literal is a bug in the test itself.
func MustDate(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic("testfixtures: bad date " + s + ": " + err.Error())
	}
	return t
}

// The three URL shapes Stadt Zürich publishes. Spelled out here rather than
// imported from pkg/zurichapi: fixtures describe what an already-mapped vote
// looks like, and must not depend on any source package. pkg/zurichapi's own
// tests pin that these strings match what the mapper produces.
func VoteURL(guid string) string {
	return "https://www.gemeinderat-zuerich.ch/abstimmungen/detail.php?aid=" + guid
}

func TraktandumURL(sitzungGuid, traktandumGuid string) string {
	return "https://www.gemeinderat-zuerich.ch/sitzungen/sitzung/?gid=" + sitzungGuid + "#" + traktandumGuid
}

func GeschaeftURL(guid string) string {
	return "https://www.gemeinderat-zuerich.ch/geschaefte/detail.php?gid=" + guid
}

// stimmabgabe creates a single member vote with the given faction and choice.
func stimmabgabe(fraktion, choice string) votes.MemberVote {
	return votes.MemberVote{Fraktion: fraktion, Choice: choice}
}

// makeStimmabgaben creates member votes from faction vote distributions.
// Each entry is (fraktion, ja, nein, enthaltung, abwesend) for Ja/Nein votes.
func makeStimmabgaben(factions []struct {
	Name                string
	Ja, Nein, Enth, Abw int
}) []votes.MemberVote {
	var result []votes.MemberVote
	for _, f := range factions {
		for i := 0; i < f.Ja; i++ {
			result = append(result, stimmabgabe(f.Name, "Ja"))
		}
		for i := 0; i < f.Nein; i++ {
			result = append(result, stimmabgabe(f.Name, "Nein"))
		}
		for i := 0; i < f.Enth; i++ {
			result = append(result, stimmabgabe(f.Name, "Enthaltung"))
		}
		for i := 0; i < f.Abw; i++ {
			result = append(result, stimmabgabe(f.Name, "Abwesend"))
		}
	}
	return result
}

// makeAuswahlStimmabgaben creates member votes for Auswahl votes (A/B/C + Abwesend).
func makeAuswahlStimmabgaben(factions []struct {
	Name         string
	A, B, C, Abw int
}) []votes.MemberVote {
	var result []votes.MemberVote
	for _, f := range factions {
		for i := 0; i < f.A; i++ {
			result = append(result, stimmabgabe(f.Name, "A"))
		}
		for i := 0; i < f.B; i++ {
			result = append(result, stimmabgabe(f.Name, "B"))
		}
		for i := 0; i < f.C; i++ {
			result = append(result, stimmabgabe(f.Name, "C"))
		}
		for i := 0; i < f.Abw; i++ {
			result = append(result, stimmabgabe(f.Name, "Abwesend"))
		}
	}
	return result
}

// shareGroup gives every vote in a group the same session and Geschäft
// identity, as votes from one agenda item really have: one session ID, one
// Affair, and one shared group link.
func shareGroup(group []votes.Vote, stem string) {
	sessionGUID := "sitzung-" + stem
	affairGUID := "geschaeft-" + stem
	groupLink := TraktandumURL(sessionGUID, "trakt-"+stem)
	for i := range group {
		group[i].SessionID = sessionGUID
		group[i].GroupURL = groupLink
		group[i].Affair.ID = affairGUID
		group[i].Affair.URL = GeschaeftURL(affairGUID)
	}
}

// base builds a vote with the ID-derived URLs a Stadt Zürich mapping would
// produce, leaving counts and titles to the caller.
func base(guid, title, grNr string) votes.Vote {
	objGUID := fmt.Sprintf("objguid-%s", guid)
	sitzungGUID := fmt.Sprintf("sitzung-%s", guid)
	traktandumGUID := fmt.Sprintf("trakt-%s", guid)
	geschaeftGUID := fmt.Sprintf("geschaeft-%s", guid)

	return votes.Vote{
		SourceID:     objGUID,
		Jurisdiction: "zurich-city",
		Body:         "Gemeinderat ZH",
		SessionID:    sitzungGUID,
		Date:         sitzungDate,
		Title:        title,
		// A vote with no type can no longer be posted at all (see
		// voteformat.IsHandledVoteType). Such records do exist upstream —
		// Kanton Zürich serves a null type for attendance determinations — but
		// they are exactly what must never reach a reader, so fixtures here
		// model postable votes. "Normal" is the overwhelming majority in both
		// bodies; fixtures needing another set it.
		Type:        "Normal",
		SourceURL:   VoteURL(objGUID),
		GroupURL:    TraktandumURL(sitzungGUID, traktandumGUID),
		Attribution: "Daten: Open Data Zürich",
		Affair: votes.Affair{
			Number: grNr,
			Title:  title,
			ID:     geschaeftGUID,
			URL:    GeschaeftURL(geschaeftGUID),
		},
	}
}

// vote creates a standard Ja/Nein vote.
func vote(guid, title, grNr, result string, ja, nein, enth, abw int) votes.Vote {
	v := base(guid, title, grNr)
	v.Decision = result
	v.Yes = intPtr(ja)
	v.No = intPtr(nein)
	v.Abstention = intPtr(enth)
	v.Absent = intPtr(abw)
	return v
}

// SingleVoteAngenommen returns a single accepted Postulat (90/30/0/5).
func SingleVoteAngenommen() []votes.Vote {
	v := vote("angenommen-1", "Postulat von Reto Brüesch (SVP) und Martin Götzl (FDP) betreffend Anpassung der Mindest- und Höchstarealfläche bei der städtischen Liegenschaftenverwaltung", "2025/100", "angenommen", 90, 30, 0, 5)
	v.MemberVotes = makeStimmabgaben([]struct {
		Name                string
		Ja, Nein, Enth, Abw int
	}{
		{"SP", 0, 30, 0, 1},
		{"SVP", 22, 0, 0, 0},
		{"FDP", 20, 0, 0, 1},
		{"Grüne", 0, 0, 0, 2},
		{"GLP", 18, 0, 0, 0},
		{"Die Mitte", 15, 0, 0, 0},
		{"AL", 15, 0, 0, 1},
	})
	return []votes.Vote{v}
}

// SingleVoteAbgelehnt returns a single rejected Antrag (20/95/5/5).
func SingleVoteAbgelehnt() []votes.Vote {
	v := vote("abgelehnt-1", "Motion von Liv Mahrer (SP) vom 05.02.2025 betreffend Festsetzung der Selnaustrasse als Begegnungszone und Aufhebung des motorisierten Individualverkehrs", "2025/101", "abgelehnt", 20, 95, 5, 5)
	v.MemberVotes = makeStimmabgaben([]struct {
		Name                string
		Ja, Nein, Enth, Abw int
	}{
		{"SP", 0, 30, 2, 1},
		{"SVP", 0, 18, 0, 1},
		{"FDP", 0, 20, 0, 1},
		{"Grüne", 0, 15, 3, 0},
		{"GLP", 8, 7, 0, 1},
		{"Die Mitte", 12, 0, 0, 1},
		{"AL", 0, 5, 0, 0},
	})
	return []votes.Vote{v}
}

// SingleVoteDringlicherklaerung returns a single vote whose Abstimmungsgegenstand
// ("Dringlicherklärung") is not a Schlussabstimmung. This exercises the eyebrow/prefix
// that gets prepended above the title on single-vote posts and images.
func SingleVoteDringlicherklaerung() []votes.Vote {
	v := vote("dringlich-1", "Motion von Dr. Jonas Keller (SP), Pascal Lamprecht (SP) und Tanja Maag (AL) vom 27.05.2026: Erhalt kleinerer bis mittlerer Konzertlokale sowie Unterstützung der Kulturanbietenden bei der Suche nach Lokalitäten", "2026/244", "angenommen", 66, 0, 0, 59)
	v.Subtitle = "2026_0244 Dringlicherklärung"
	v.MemberVotes = makeStimmabgaben([]struct {
		Name                string
		Ja, Nein, Enth, Abw int
	}{
		{"SP", 37, 0, 0, 4},
		{"FDP", 0, 0, 0, 24},
		{"SVP", 0, 0, 0, 16},
		{"GLP", 0, 0, 0, 15},
		{"Grüne", 14, 0, 0, 0},
		{"AL", 8, 0, 0, 0},
		{"Die Mitte", 7, 0, 0, 0},
	})
	return []votes.Vote{v}
}

// LongTitleTruncation returns a vote with a ~300-char title that triggers truncation.
func LongTitleTruncation() []votes.Vote {
	longTitle := "Schlussabstimmung über die bereinigten Dispositivziffern " +
		"zum Objektkredit von 350 Millionen Franken für das Projekt Erweiterung " +
		"und Neugestaltung des Hauptbahnhofs Zürich mit unterirdischer Durchmesserlinie " +
		"und ergänzenden Massnahmen zur Verbesserung der Verkehrsinfrastruktur im Grossraum Zürich " +
		"inklusive der notwendigen Anpassungen an die bestehende urbane Planung"
	v := vote("longtrunc-1", longTitle, "2025/102", "angenommen", 80, 30, 5, 10)
	v.MemberVotes = makeStimmabgaben([]struct {
		Name                string
		Ja, Nein, Enth, Abw int
	}{
		{"SP", 30, 0, 2, 2},
		{"SVP", 0, 18, 0, 1},
		{"FDP", 18, 0, 0, 2},
		{"Grüne", 14, 0, 3, 1},
		{"GLP", 10, 5, 0, 2},
		{"Die Mitte", 8, 5, 0, 1},
		{"AL", 0, 2, 0, 1},
	})
	return []votes.Vote{v}
}

// TallTitleBandClearance is the 03.12.2025 Postulat: a title just long enough
// to fill the card at full size.
//
// It is the case between "fits comfortably" and "has to be cut". Its eight
// 42pt lines left the content 2px below the body band — the verdict emoji all
// but touching the chamber name — because the layout centred the content but
// only ever charged the title for the margin at the bottom. The card should
// give up a font size here, long before it gives up a line.
func TallTitleBandClearance() []votes.Vote {
	const title = "Postulat von Reto Brüesch (SVP) und Jean-Marc Jung (SVP) vom 03.12.2025: " +
		"Bau- und Entwicklungsprojekte sowie Erarbeitung und Genehmigung von Nutzungs- und " +
		"Sondernutzungsplanungen, Verankerung einer klaren, standardisierten und " +
		"nachvollziehbaren Interessenabwägung gemäss Art. 3 Raumplanungsverordnung (RPV)"

	v := vote("bandclearance-1", title, "2025/438", "angenommen", 117, 0, 0, 8)
	v.MemberVotes = makeStimmabgaben([]struct {
		Name                string
		Ja, Nein, Enth, Abw int
	}{
		{"SP", 37, 0, 0, 4},
		{"FDP", 23, 0, 0, 1},
		{"SVP", 16, 0, 0, 0},
		{"GLP", 14, 0, 0, 1},
		{"Grüne", 12, 0, 0, 2},
		{"AL", 8, 0, 0, 0},
		{"Die Mitte", 7, 0, 0, 0},
	})
	return []votes.Vote{v}
}

// ExtremeTitleFullRoster is the worst real title the Gemeinderat has produced,
// against a full nine-Fraktion roster.
//
// Two parliamentary initiatives, a court decision and its case number in one
// 560-character sentence. Rendered on a card the title used to take every line
// it asked for, and the Fraktion table below it was quietly cut off partway
// down the roster — the party breakdown, the one part of a vote a reader cannot
// reconstruct from the caption, lost four of its nine rows.
//
// The roster sums to the headline it is filed under — 60 Ja, 60 Nein, 5
// abwesend across the chamber's 125 seats. A fixture whose column did not add
// up would reproduce the very mismatch this PR exists to prevent, and the
// golden snapshot would pin it.
func ExtremeTitleFullRoster() []votes.Vote {
	const longTitle = "Parlamentarische Initiativen GR Nrn. 2022/144 und 2022/145 vom " +
		"13.04.2022: Rahmenkredit für ein dreijähriges Pilotprojekt zur Schaffung einer " +
		"Überbrückungshilfe für Ausländerinnen und Ausländer ohne gültigen Aufenthaltsstatus " +
		"und Rahmenkredit für ein dreijähriges Pilotprojekt «Wirtschaftliche Basishilfe für " +
		"Ausländerinnen und Ausländer mit gültigem Aufenthaltsstatus, die keinen risikofreien " +
		"Zugang zur Sozialhilfe haben», Beschluss des Verwaltungsgerichts des Kantons Zürich " +
		"(VB.2025.00517), Entscheid betreffend Einreichung einer Beschwerde an das Bundesgericht"

	v := vote("extremetitle-1", longTitle, "2022/144", "abgelehnt", 60, 60, 0, 5)
	v.MemberVotes = makeStimmabgaben([]struct {
		Name                string
		Ja, Nein, Enth, Abw int
	}{
		{"SP", 40, 0, 0, 1},
		{"SVP", 0, 20, 0, 1},
		{"FDP", 0, 17, 0, 1},
		{"Grüne", 14, 0, 0, 0},
		{"GLP", 0, 14, 0, 1},
		{"Die Mitte", 0, 5, 0, 0},
		{"AL", 6, 0, 0, 1},
		{"EVP", 0, 3, 0, 0},
		{"Fraktionslos", 0, 1, 0, 0},
	})
	return []votes.Vote{v}
}

// MultiVoteGroup returns 2 votes from the same Geschäft: Einleitungsartikel + Schlussabstimmung.
func MultiVoteGroup() []votes.Vote {
	const title = "Teilrevision der Gemeindeordnung der Stadt Zürich, Neuordnung der Kompetenzen im Bereich Stadtentwicklung"

	vote1 := vote("multi-1", title, "2025/103", "angenommen", 90, 20, 5, 10)
	vote1.Subtitle = "Einleitungsartikel"
	vote1.MemberVotes = makeStimmabgaben([]struct {
		Name                string
		Ja, Nein, Enth, Abw int
	}{
		{"SP", 32, 0, 0, 2},
		{"SVP", 0, 18, 0, 1},
		{"FDP", 20, 0, 0, 2},
		{"Grüne", 12, 0, 5, 1},
		{"GLP", 14, 0, 0, 2},
		{"Die Mitte", 12, 0, 0, 1},
		{"AL", 0, 2, 0, 1},
	})

	vote2 := vote("multi-2", title, "2025/103", "abgelehnt", 40, 70, 5, 10)
	vote2.Subtitle = "Schlussabstimmung"
	vote2.MemberVotes = makeStimmabgaben([]struct {
		Name                string
		Ja, Nein, Enth, Abw int
	}{
		{"SP", 0, 30, 2, 2},
		{"SVP", 18, 0, 0, 1},
		{"FDP", 0, 20, 0, 2},
		{"Grüne", 10, 0, 3, 1},
		{"GLP", 12, 0, 0, 2},
		{"Die Mitte", 0, 12, 0, 1},
		{"AL", 0, 8, 0, 1},
	})

	group := []votes.Vote{vote1, vote2}
	shareGroup(group, "multi")
	return group
}

// GenericAntragFallback returns a vote whose Traktandum title was the generic
// "Antrag 1.", so the mapper substituted the Geschäft title and pointed both
// links at the Geschäft page.
func GenericAntragFallback() []votes.Vote {
	v := vote("antrag-1", "Postulat von Max Müller (FDP) und Sarah Weber (Grüne) vom 12.11.2024 betreffend Verbesserung der Veloinfrastruktur entlang der Langstrasse und angrenzender Quartiere", "2025/200", "angenommen", 80, 35, 5, 5)
	// Only the generic-Antrag case sends both links to the Geschäft page.
	v.Affair.ID = "geschaeft-antrag"
	v.Affair.URL = GeschaeftURL(v.Affair.ID)
	v.SourceURL = v.Affair.URL
	v.GroupURL = v.Affair.URL
	v.MemberVotes = makeStimmabgaben([]struct {
		Name                string
		Ja, Nein, Enth, Abw int
	}{
		{"SP", 28, 0, 2, 1},
		{"SVP", 0, 20, 0, 0},
		{"FDP", 20, 0, 0, 1},
		{"Grüne", 12, 5, 3, 0},
		{"GLP", 12, 3, 0, 1},
		{"Die Mitte", 8, 5, 0, 1},
		{"AL", 0, 2, 0, 1},
	})
	return []votes.Vote{v}
}

// TenVoteStressTest returns 10 votes forcing multiple reply posts.
func TenVoteStressTest() []votes.Vote {
	const title = "Totalrevision der Bau- und Zonenordnung der Stadt Zürich, Anpassungen an das übergeordnete Recht"

	var group []votes.Vote
	for i := 0; i < 10; i++ {
		v := vote(fmt.Sprintf("stress-%d", i), title, "2025/104", "angenommen", 80+i, 30-i, 5, 10)
		v.Subtitle = fmt.Sprintf("Ziffer %c", 'A'+i)
		v.MemberVotes = makeStimmabgaben([]struct {
			Name                string
			Ja, Nein, Enth, Abw int
		}{
			{"SP", 25 + i, 0, 2, 2},
			{"SVP", 0, 15 - i, 0, 1},
			{"FDP", 18, 2, 0, 2},
			{"Grüne", 15, 0, 3, 1},
			{"GLP", 12 + i, 5 - i, 0, 2},
			{"Die Mitte", 10, 5, 0, 1},
			{"AL", 0, 3, 0, 1},
		})
		group = append(group, v)
	}
	shareGroup(group, "stress")
	return group
}

// InstagramLongMultiVoteTruncation returns a long multi-vote fixture that forces Instagram caption truncation.
func InstagramLongMultiVoteTruncation() []votes.Vote {
	const (
		longVoteTitleRepeatCount = 60
		longMainTitleRepeatCount = 120
	)

	group := TenVoteStressTest()
	for i := range group {
		group[i].Subtitle = strings.Repeat("Sehr langer Abstimmungstitel ", longVoteTitleRepeatCount)
	}
	longTitle := strings.Repeat("Sehr langes Traktandum ", longMainTitleRepeatCount)
	group[0].Title = longTitle
	group[0].Affair.Title = longTitle
	return group
}

// VoteWithMentions returns a vote with a politician name that triggers @mention matching.
func VoteWithMentions() []votes.Vote {
	v := vote("mention-1", "Postulat von Anna Graff (SP) vom 18.09.2024 betreffend Verbesserung der Sicherheit im öffentlichen Raum rund um den Hauptbahnhof", "2025/105", "angenommen", 80, 30, 5, 10)
	v.Affair.Title = "Verbesserung der Sicherheit im öffentlichen Raum rund um den Hauptbahnhof"
	v.MemberVotes = makeStimmabgaben([]struct {
		Name                string
		Ja, Nein, Enth, Abw int
	}{
		{"SP", 32, 0, 0, 2},
		{"SVP", 0, 18, 0, 1},
		{"FDP", 15, 5, 0, 2},
		{"Grüne", 16, 0, 2, 1},
		{"GLP", 10, 4, 3, 2},
		{"Die Mitte", 7, 3, 0, 1},
		{"AL", 0, 0, 0, 1},
	})
	return []votes.Vote{v}
}

// AuswahlVote returns a single Auswahl vote with A/B/C counts (no ✅/❌ prefix).
func AuswahlVote() []votes.Vote {
	const title = "Weisung des Stadtrats betreffend Objektkredit für die Erneuerung der Jugendwohnsiedlung Buchegg und Erweiterung des Betreuungsangebots"

	v := base("auswahl-1", title, "2025/106")
	v.Date = date(2026, 3, 4)
	v.Type = "Gleichgerichtete Anträge mit 3 Optionen"
	v.Decision = "Auswahl A"
	v.Absent = intPtr(10)
	v.ChoiceA = intPtr(74)
	v.ChoiceB = intPtr(28)
	v.ChoiceC = intPtr(13)
	v.MemberVotes = makeAuswahlStimmabgaben([]struct {
		Name         string
		A, B, C, Abw int
	}{
		{"SP", 30, 0, 0, 2},
		{"FDP", 18, 2, 0, 2},
		{"SVP", 0, 18, 0, 2},
		{"GLP", 14, 0, 0, 1},
		{"Grüne", 0, 0, 13, 1},
		{"Die Mitte", 12, 0, 0, 1},
		{"AL", 0, 8, 0, 1},
	})
	return []votes.Vote{v}
}

// AuswahlVoteAllFraktionen returns an Auswahl vote in which every option and
// "Abw." has a column in the Fraktionen table. Four columns is the tightest
// the table gets: with "Auswahl A" as a header they ran into one another.
func AuswahlVoteAllFraktionen() []votes.Vote {
	const title = "Weisung des Stadtrats betreffend Teilrevision der Verordnung über die Schulraumplanung"

	v := base("auswahl-all-fraktionen-1", title, "2025/108")
	v.Date = date(2026, 9, 30)
	v.Type = "Gleichgerichtete Anträge mit 3 Optionen"
	v.Subtitle = "Änderungsanträge 1–2 zum Dispositivpunkt B2 und zu den Dispositivpunkten C1–C2"
	v.Decision = "Auswahl A"
	v.Absent = intPtr(7)
	v.ChoiceA = intPtr(76)
	v.ChoiceB = intPtr(29)
	v.ChoiceC = intPtr(13)
	v.MemberVotes = makeAuswahlStimmabgaben([]struct {
		Name         string
		A, B, C, Abw int
	}{
		{"SP", 38, 0, 0, 3},
		{"FDP", 0, 22, 0, 2},
		{"SVP", 16, 0, 0, 0},
		{"GLP", 0, 0, 13, 2},
		{"Grüne", 14, 0, 0, 0},
		{"AL", 8, 0, 0, 0},
		{"Die Mitte", 0, 7, 0, 0},
	})
	// The Gemeinderat API reports the option as "Auswahl A", not "A"; the other
	// fixtures use the bare letter and so never produced the long header.
	for i, mv := range v.MemberVotes {
		if mv.Choice != "Abwesend" {
			v.MemberVotes[i].Choice = "Auswahl " + mv.Choice
		}
	}
	return []votes.Vote{v}
}

// MixedMultiVote returns one Ja/Nein vote + one Auswahl vote in the same group.
func MixedMultiVote() []votes.Vote {
	const title = "Weisung des Stadtrats betreffend Revision der Bau- und Zonenordnung, Anpassung der Bestimmungen für Gewerbe- und Industriezonen"

	v1 := vote("mixed-1", title, "2025/107", "angenommen", 62, 51, 0, 12)
	v1.Date = date(2026, 2, 25)
	v1.Subtitle = "Änderungsantrag 9"
	v1.MemberVotes = makeStimmabgaben([]struct {
		Name                string
		Ja, Nein, Enth, Abw int
	}{
		{"SP", 30, 0, 0, 2},
		{"SVP", 0, 18, 0, 2},
		{"FDP", 0, 20, 0, 2},
		{"Grüne", 14, 0, 0, 1},
		{"GLP", 10, 5, 0, 2},
		{"Die Mitte", 8, 5, 0, 2},
		{"AL", 0, 3, 0, 1},
	})

	v2 := base("mixed-2", title, "2025/107")
	v2.Date = date(2026, 2, 25)
	v2.Type = "Gleichgerichtete Anträge mit 3 Optionen"
	v2.Subtitle = "Änderungsantrag 17, 1. Abstimmung Weisung des Stadtrats betreffend Revision der Bau- und Zonenordnung"
	v2.Decision = "Auswahl A"
	v2.Absent = intPtr(11)
	v2.ChoiceA = intPtr(50)
	v2.ChoiceB = intPtr(24)
	v2.ChoiceC = intPtr(40)
	v2.MemberVotes = makeAuswahlStimmabgaben([]struct {
		Name         string
		A, B, C, Abw int
	}{
		{"SP", 28, 0, 0, 2},
		{"SVP", 0, 0, 20, 2},
		{"FDP", 0, 18, 0, 2},
		{"Grüne", 10, 0, 5, 1},
		{"GLP", 8, 6, 0, 2},
		{"Die Mitte", 4, 0, 10, 1},
		{"AL", 0, 0, 5, 1},
	})

	group := []votes.Vote{v1, v2}
	shareGroup(group, "mixed")
	return group
}

// PostulatWithGrNrPrefix returns a Postulat whose title starts with
// "2025/100 Postulat von ...", exercising Geschäft-number stripping.
func PostulatWithGrNrPrefix() []votes.Vote {
	v := vote("grnr-1", "2025/100 Postulat von Reto Brüesch (SVP) vom 05.03.2025: Anpassung der Mindest- und Höchstarealfläche", "2025/100", "abgelehnt", 21, 38, 56, 10)
	v.Date = date(2025, 11, 26)
	v.Affair.Title = "Anpassung der Mindest- und Höchstarealfläche"
	v.MemberVotes = makeStimmabgaben([]struct {
		Name                string
		Ja, Nein, Enth, Abw int
	}{
		{"SP", 0, 0, 30, 2},
		{"SVP", 18, 0, 0, 1},
		{"FDP", 0, 20, 0, 2},
		{"Grüne", 0, 0, 15, 1},
		{"GLP", 3, 10, 5, 2},
		{"Die Mitte", 0, 8, 6, 1},
		{"AL", 0, 0, 0, 1},
	})
	return []votes.Vote{v}
}

// KantonsratVote is a Kanton Zürich vote as the OpenParlData adapter delivers
// it, for rendering next to a Gemeinderat post.
//
// The two bodies share an account, so the fixtures have to cover both — this is
// what makes "can a reader tell these apart?" a question the golden snapshot
// answers rather than one that gets checked once by eye and then drifts.
//
// It also captures how cantonal data differs: no agenda item (hence no
// subtitle), a decision derived from the counts rather than reported, and a
// business number in the canton's DD/YYYY form.
//
// Both URLs are the Geschäft permalink, matching what the adapter produces: the
// canton's own page is used for a single vote as well as a group, rather than
// the zh.recapp.ch deep link the source supplies per vote. See
// openparldata.applyAffair.
// withCantonLinks fills the archive and sitting links every Kanton Zürich vote
// carries beside its Geschäft permalink: the recapp segment for this one vote,
// the agenda item for a post covering several of them, and the sitting list
// narrowed to the day. config.cantonLinks derives all three in production, so a
// fixture that left them empty would render a link block no cantonal post has.
func withCantonLinks(v *votes.Vote, agendaItemUID, segmentUID string) {
	item := "https://zh.recapp.ch/shareparl?agendaItemUid=" + agendaItemUID
	v.VideoURL = item + "&segmentUid=" + segmentUID
	v.VideoGroupURL = item

	day := v.Date.Format("2006-01-02")
	q := url.Values{}
	q.Set("startDate", day)
	q.Set("endDate", day)
	v.SessionURL = "https://www.kantonsrat.zh.ch/ratsbetrieb/sitzungenundprotokolle/?" + q.Encode()
}

func KantonsratVote() []votes.Vote {
	const title = "Einzelinitiative betreffend Ausbau des Angebots an Tagesschulen und familienergänzender Betreuung im Kanton Zürich"

	v := votes.Vote{
		SourceID:     "EBA24B53-B404-3BCB-9A1B-4E7E01C1ACAC",
		Jurisdiction: "zurich-canton",
		Body:         "Kantonsrat ZH",
		Date:         time.Date(2026, 7, 6, 10, 21, 43, 0, time.UTC),
		Sequence:     "1783333303",
		Title:        title,
		// "Normal" is the overwhelming majority of cantonal votes and must stay
		// unlabelled, or the label on a quorum vote stops standing out.
		Type: "Normal",
		// Kanton Zürich reports no decision, and none is derived, so cantonal
		// posts state the counts and no outcome.
		Decision:   "",
		Yes:        intPtr(83),
		No:         intPtr(87),
		Abstention: intPtr(0),
		Absent:     intPtr(10),
		SourceURL:  "https://www.kantonsrat.zh.ch/geschaefte/geschaeft/?id=3e9a314a447f42f6bc8ed5995d9ae47e",
		// OpenParlData is CC BY 4.0; the credit is a licence obligation, so it
		// must actually appear in the rendered post.
		Attribution: "Daten: OpenParlData.ch",
		GroupURL:    "https://www.kantonsrat.zh.ch/geschaefte/geschaeft/?id=3e9a314a447f42f6bc8ed5995d9ae47e",
		Affair: votes.Affair{
			Number: "6087",
			Title:  title,
			ID:     "313093",
			URL:    "https://www.kantonsrat.zh.ch/geschaefte/geschaeft/?id=3e9a314a447f42f6bc8ed5995d9ae47e",
		},
	}
	v.MemberVotes = makeStimmabgaben([]struct {
		Name                string
		Ja, Nein, Enth, Abw int
	}{
		{"SVP", 44, 0, 0, 3},
		{"SP", 0, 35, 0, 1},
		{"FDP", 27, 0, 0, 3},
		{"Grünliberale", 0, 22, 0, 1},
		{"Grüne", 0, 19, 0, 0},
		{"Die Mitte", 11, 0, 0, 1},
		{"EVP", 0, 6, 0, 1},
		{"AL", 0, 5, 0, 0},
	})
	// One member of the chamber is not mapped to a faction, as ~1% are in the
	// real data: they must stay out of the table without being dropped from the
	// totals, which the source reports independently.
	v.MemberVotes = append(v.MemberVotes, votes.MemberVote{Name: "Fraktionslos", Choice: "Ja"})

	withCantonLinks(&v, "89f5a593-67ad-4ff5-82b4-818a2bab741a", "921ebc9c-1891-4d4e-bd8f-12d0ab2a7353")

	return []votes.Vote{v}
}

// KantonsratMultiVote is the real 15.06.2026 Glattalbahn group: two substantive
// votes interleaved with three Ausgabenbremse (spending-brake) quorum votes.
//
// It is here because it is the shape that reads worst, and the snapshot should
// show that honestly. Kanton Zürich publishes no per-vote title, so the entries
// can only be numbered; and the quorum votes show a lopsided 129:0 with ~51
// "Abwesend", which reads as near-unanimous agreement when it is really a
// procedural vote most of the opposition sits out.
//
// The type is what rescues that reading, and it arrives from the source: after
// we reported it, OpenParlData began populating type_de for the canton too, so
// the quorum entries carry a label. The Types below are the real served values.
//
// One gap remains upstream and is not ours to invent: "Abwesend" still conflates
// members who were absent with those who were present and did not vote — for
// this vote the official record says 6 and 46, where the API says 52.
func KantonsratMultiVote() []votes.Vote {
	const title = "Staatsbeitrag Bau Verlängerung Glattalbahn, Flughafen bis Kloten Industrie, Objektkredite Velohauptverbindung und Hochwasserschutzmassnahmen in Kloten"
	const agendaItemUID = "c2c4b880-e83b-4ecc-aadb-5895d0f80f13"
	const geschaeft = "https://www.kantonsrat.zh.ch/geschaefte/geschaeft/?id=89ddd67395d74b70bb1015edac49b7e2"

	type sub struct {
		id                  string
		hour, minute        int
		segment             string
		voteType            string
		ja, nein, enth, abw int
	}
	subs := []sub{
		{"C73B20F8-BE10-9CC8-70BA-B3510FCBA125", 11, 29, "95cfdd0d-453d-467b-9e57-6e1afddb766e", "Normal", 130, 44, 0, 6},
		{"1E159CE0-5FF2-6550-3DD8-6AC38D15BECC", 11, 30, "fc47bda1-dbb3-447f-bd3e-ce1d95ccea59", "Quorum", 129, 0, 0, 51},
		{"D8C48612-302B-0CEB-C7C7-A8474CDD2C21", 11, 40, "94915e31-5be7-41db-911e-123f216f9ee9", "Normal", 129, 44, 0, 7},
		{"A8D4D59E-0756-D4F6-AE63-D6CCAD573A79", 11, 41, "c3422c5f-3dac-495d-855c-d26552ef5601", "Quorum", 129, 0, 0, 51},
		{"5D0CFBDB-1691-F36C-1C75-48CE4002036C", 11, 42, "aa6d241e-bfdc-4ca1-beea-236337c16f0e", "Quorum", 128, 0, 0, 52},
	}

	var group []votes.Vote
	for _, sv := range subs {
		at := time.Date(2026, 6, 15, sv.hour, sv.minute, 0, 0, time.UTC)
		v := votes.Vote{
			SourceID:     sv.id,
			Jurisdiction: "zurich-canton",
			Body:         "Kantonsrat ZH",
			Date:         at,
			Sequence:     fmt.Sprintf("%d", at.Unix()),
			Title:        title,
			Type:         sv.voteType,
			// Empty for the same reason as KantonsratVote: the source reports
			// no outcome for any cantonal vote.
			Decision:    "",
			Yes:         intPtr(sv.ja),
			No:          intPtr(sv.nein),
			Abstention:  intPtr(sv.enth),
			Absent:      intPtr(sv.abw),
			SourceURL:   geschaeft,
			GroupURL:    geschaeft,
			Attribution: "Daten: OpenParlData.ch",
			Affair: votes.Affair{
				Number: "6031",
				Title:  title,
				ID:     "247676",
				URL:    geschaeft,
			},
		}
		withCantonLinks(&v, agendaItemUID, sv.segment)
		v.MemberVotes = kantonsratRoster(sv.ja, sv.nein, sv.abw)
		group = append(group, v)
	}
	return group
}

// KantonsratDecisionReported is the first two votes of the same 15.06.2026
// Glattalbahn group — one Normal, one Quorum — as they would arrive if
// OpenParlData populated `decision` for the canton.
//
// Upstream now does so for binary votes (MR !1264); a threshold vote arrives
// without one in production, since the client clears it (see applyDetails).
// Other votes of the canton still carry counts and no outcome, and the adapter's
// decision() never derives one.
//
// The fixture pins how the formatters render a reported verdict: a quorum vote
// keeps its own counts rendering while gaining the outcome line. It bypasses the
// client, so it shows rendering only, not what production would let through.
func KantonsratDecisionReported() []votes.Vote {
	group := KantonsratMultiVote()[:2]
	for i := range group {
		group[i].Decision = "Ja"
	}
	return group
}

// KantonsratMemberBusiness is the real 31.08.2026 vote on Postulat 367/2025,
// filed by Tobias Weidmann (SVP).
//
// It is the case the canton has and the city does not: business put forward by
// members, whose names appear nowhere in the title. Stadt Zürich's titles carry
// their submitters — "Postulat von Reto Brüesch (SVP) und …" — while Kanton
// Zürich's are bare subject lines and the authorship arrives separately, from
// the affair's contributor list.
//
// So this is what proves the two chambers read alike, and that the mapping does
// anything at all for the canton: a post that names nobody tags nobody, however
// well curated the file is. Roughly a third of recent cantonal business has
// named authors; the rest comes from the Regierungsrat or a committee and
// renders exactly as the other cantonal fixtures do.
func KantonsratMemberBusiness() []votes.Vote {
	const title = "Verbindliche Richtlinien – keine Smartphones an Volksschulen"

	v := votes.Vote{
		SourceID:     "FBA39D0F-37F4-9741-5EFE-D6F42E280FC4",
		Jurisdiction: "zurich-canton",
		Body:         "Kantonsrat ZH",
		Date:         time.Date(2026, 8, 31, 10, 21, 8, 0, time.UTC),
		Sequence:     "1788171668",
		Title:        title,
		Type:         "Normal",
		Decision:     "",
		Yes:          intPtr(92),
		No:           intPtr(82),
		Abstention:   intPtr(2),
		Absent:       intPtr(4),
		SourceURL:    "https://www.kantonsrat.zh.ch/geschaefte/geschaeft/?id=2904da3a36db41d8bcae242155d64ae1",
		GroupURL:     "https://www.kantonsrat.zh.ch/geschaefte/geschaeft/?id=2904da3a36db41d8bcae242155d64ae1",
		Attribution:  "Daten: OpenParlData.ch",
		Affair: votes.Affair{
			Number: "367/2025",
			Title:  title,
			ID:     "284945",
			URL:    "https://www.kantonsrat.zh.ch/geschaefte/geschaeft/?id=2904da3a36db41d8bcae242155d64ae1",
			Type:   "Postulat",
			// One first signatory, which is what the canton's record almost
			// always has: the two co-signatories are not named, and the
			// Direktion the business was assigned to is not an author.
			Authors: []votes.Author{{Name: "Tobias Weidmann", Party: "SVP"}},
		},
	}

	// Spelled out rather than spread mechanically, because a post whose author
	// is named needs the table beside it to be readable as the same event: a
	// Postulat filed by an SVP member and carried by the bourgeois factions.
	v.MemberVotes = makeStimmabgaben([]struct {
		Name                string
		Ja, Nein, Enth, Abw int
	}{
		{"SVP", 46, 0, 0, 1},
		{"SP", 0, 35, 0, 1},
		{"FDP", 29, 0, 0, 1},
		{"Grünliberale", 0, 23, 0, 0},
		{"Grüne", 0, 19, 0, 0},
		{"Die Mitte", 10, 0, 1, 1},
		{"EVP", 6, 0, 1, 0},
		{"AL", 0, 5, 0, 0},
	})
	// The unattached member, as in KantonsratVote: about 1% of the chamber.
	v.MemberVotes = append(v.MemberVotes, votes.MemberVote{Name: "Fraktionslos", Choice: "Ja"})

	withCantonLinks(&v, "d7dc2b4b-51b2-4c4f-bf9b-9d9b0e1a2c73", "4f0a1d6e-2a83-4b02-9f52-1c6d9f39a5b1")

	return []votes.Vote{v}
}

// KantonsratCoSignedBusiness is the real 08.06.2026 vote on Postulat 377/2020,
// filed by Hannah Pfalzgraf (SP) and co-signed across four other parties.
//
// It is the shape that decides how much of a signatory list a post can carry.
// One name says who filed it; all five say the thing the lead name hides, that
// a Postulat on LGBTI equality was carried into the chamber by SP, Die Mitte,
// GLP, Grüne and AL together. Text posts name all five. The card names as many
// as its column holds and drops from the end only when the font ladder has run
// out, because the subject of the vote is what a reader cannot do without.
//
// Five is the long end of normal: recent cantonal business runs one to five
// signatories, and this is the only fixture where naming everyone costs the
// 280-character X root its full title.
func KantonsratCoSignedBusiness() []votes.Vote {
	const title = "Gleichstellung von LGBTI-Menschen und Massnahmen gegen Hasskommunikation"

	v := votes.Vote{
		SourceID:     "08E65A77-2230-1903-47C7-3295F674EF43",
		Jurisdiction: "zurich-canton",
		Body:         "Kantonsrat ZH",
		Date:         time.Date(2026, 6, 8, 9, 15, 20, 0, time.UTC),
		Sequence:     "1780909-520",
		Title:        title,
		Type:         "Normal",
		Decision:     "",
		Yes:          intPtr(99),
		No:           intPtr(74),
		Abstention:   intPtr(0),
		Absent:       intPtr(7),
		SourceURL:    "https://www.kantonsrat.zh.ch/geschaefte/geschaeft/?id=d463146e155b46589279b3228ea48c01",
		GroupURL:     "https://www.kantonsrat.zh.ch/geschaefte/geschaeft/?id=d463146e155b46589279b3228ea48c01",
		Attribution:  "Daten: OpenParlData.ch",
		Affair: votes.Affair{
			Number: "377/2020",
			Title:  title,
			ID:     "91231",
			URL:    "https://www.kantonsrat.zh.ch/geschaefte/geschaeft/?id=d463146e155b46589279b3228ea48c01",
			Type:   "Postulat",
			// First signatory first, then the co-signatories in the order the
			// parliament lists them. The Direktion the business was assigned to
			// is on the same contributor list and is not an author.
			Authors: []votes.Author{
				{Name: "Hannah Pfalzgraf", Party: "SP"},
				{Name: "Yvonne Bürgin", Party: "Die Mitte"},
				{Name: "Andrea Gisler", Party: "GLP"},
				{Name: "Silvia Rigoni", Party: "Grüne"},
				{Name: "Judith Anna Stofer", Party: "AL"},
			},
		},
	}

	v.MemberVotes = makeStimmabgaben([]struct {
		Name                string
		Ja, Nein, Enth, Abw int
	}{
		{"SVP", 0, 46, 0, 2},
		{"SP", 36, 0, 0, 0},
		{"FDP", 26, 0, 0, 4},
		{"Grünliberale", 23, 0, 0, 0},
		{"Grüne", 19, 0, 0, 0},
		{"Die Mitte", 0, 11, 0, 1},
		{"EVP", 0, 7, 0, 0},
		{"AL", 5, 0, 0, 0},
	})
	v.MemberVotes = append(v.MemberVotes, votes.MemberVote{Name: "Fraktionslos", Choice: "Ja"})

	withCantonLinks(&v, "1a6f4c92-77ad-4e05-8b3c-52b1a0f4d8e6", "8c31b5d0-9e64-4a71-b0d8-3f2a7c6e15b9")

	return []votes.Vote{v}
}

// KantonsratLoneQuorumVote is a spending-brake vote standing on its own, which
// is how they usually arrive: the 17.08.2026 Rahmenkredit Energiegesetz was a
// single vote posted alone.
//
// It is the case the type label matters most for and the one the group fixtures
// cannot cover. A lone threshold vote has no sibling beside it to make 129:0
// with 51 "Abwesend" look unusual, and no per-vote heading either — SubVoteLabel
// only numbers the entries of a group — so the label has to reach the reader
// from the counts themselves, on every platform and on the card.
//
// The counts are the Glattalbahn group's first threshold vote; the type is set
// to Ausgabenbremse, which is what the archive calls these ("Abstimmung
// Ausgabenbremse"). The group fixture keeps the plainer "Quorum" it is served,
// so both labels stay covered by the snapshot.
func KantonsratLoneQuorumVote() []votes.Vote {
	group := KantonsratMultiVote()[1:2]
	group[0].Type = "Ausgabenbremse"
	return group
}

// KantonsratCupVote is the real 15.12.2025 Steuerfuss vote (voting 98765), the
// canton's equivalent of an Auswahl vote: a Cup-Abstimmung, one knockout round
// between more than two competing proposals.
//
// It is here to pin that we refuse it, because the source's version of it cannot
// be published truthfully, and for two independent reasons:
//
//   - Every aggregate count is null. results_yes, results_no and
//     results_abstention are all absent while only results_absent is populated,
//     so there is no result to render at all. Stadt Zürich populates these for
//     the same kind of vote.
//   - The per-member records are duplicated. The API returns 296 rows for a
//     180-seat chamber: each of the 116 members who chose an option also appears
//     under vote_display_de "Präsidium", harmonised to abstention — 175 members
//     carry that value, which cannot mean what it says. The duplication is
//     confined to Cup-Abstimmungen; Normal and Quorum votes return exactly 180.
//
// Both are reported upstream (issue 180). Until they are fixed the vote type is
// off the handled list, so a group containing this is skipped and the run
// reports it rather than publishing a knockout round as an ordinary tally.
//
// The member list below is deliberately short. Nothing about the refusal depends
// on its length — the counts and the type decide it — so it carries just enough
// to show the shape, including one member recorded twice.
func KantonsratCupVote() []votes.Vote {
	const title = "Steuerfuss für die Jahre 2026 und 2027"

	v := votes.Vote{
		SourceID:     "FFD3E6F2-E781-6937-515C-A54A2288E7CA",
		Jurisdiction: "zurich-canton",
		Body:         "Kantonsrat ZH",
		Date:         time.Date(2025, 12, 15, 10, 30, 44, 0, time.UTC),
		Sequence:     "1765794644",
		Title:        title,
		Type:         "Cup-Abstimmung",
		// Null, exactly as served: only the absent count survives.
		Yes: nil, No: nil, Abstention: nil,
		Absent:      intPtr(5),
		Decision:    "",
		SourceURL:   "https://www.kantonsrat.zh.ch/geschaefte/geschaeft/?id=250382",
		GroupURL:    "https://www.kantonsrat.zh.ch/geschaefte/geschaeft/?id=250382",
		Attribution: "Daten: OpenParlData.ch",
		Affair: votes.Affair{
			Number: "250382",
			Title:  title,
			ID:     "250382",
			URL:    "https://www.kantonsrat.zh.ch/geschaefte/geschaeft/?id=250382",
		},
		MemberVotes: []votes.MemberVote{
			{Name: "Astrid Furrer", Fraktion: "FDP", Choice: "Auswahl A"},
			{Name: "Astrid Furrer", Fraktion: "FDP", Choice: "Enthaltung"},
			{Name: "Beat Hauser", Fraktion: "Grünliberale", Choice: "Auswahl D"},
			{Name: "Beat Hauser", Fraktion: "Grünliberale", Choice: "Enthaltung"},
			{Name: "Ein Abwesender", Fraktion: "SP", Choice: "Abwesend"},
		},
	}

	withCantonLinks(&v, "6b2e8f14-3c57-4d90-a1e6-0d84b7f2c395", "e5470a2d-1b68-4c3f-9a07-6d2e83f14bc0")

	return []votes.Vote{v}
}

// KantonsratWahlgeschaeft is the real 14.09.2026 roll call (external id
// EA7109F3-...) on KR-Nr. 20/2026, the Baurekursgericht seat.
//
// It is the fact pattern an election leaves behind: a Wahl business whose only
// recorded vote is the roll call taken before the ballot. Its 157/0 counts are
// the headcount, and the post built from it reports no result at all — the
// ballot elected Marco Bühler with 145 votes, which appears in none of the
// data and only in the linked Bulletin. The title names Adrian Bergmann, the
// member being replaced, and is published exactly as the parliament wrote it.
// See votes.IsWahlgeschaeft and voteformat.WahlgeschaeftBody.
func KantonsratWahlgeschaeft() []votes.Vote {
	const title = "Wahl Mitglied Baurekursgericht (BRG) für Adrian Bergmann"
	const geschaeftURL = "https://www.kantonsrat.zh.ch/geschaefte/geschaeft/?id=5fdb30c929a04ac2a30fdd07977045e7"

	v := votes.Vote{
		SourceID:     "EA7109F3-A57B-F1FB-5617-EFCE652A7545",
		Jurisdiction: "zurich-canton",
		Body:         "Kantonsrat ZH",
		Date:         time.Date(2026, 9, 14, 6, 19, 39, 0, time.UTC),
		Sequence:     "1789366779",
		Title:        title,
		Type:         votes.AttendanceType,
		// Withheld for every Wahl affair, same as KantonsratVote — see
		// openparldata.affairStatesItsOutcome.
		Decision: "",
		// The roll call's own tally: who was in the chamber, not a ballot on
		// any candidate. Nothing on this path renders them.
		Yes:         intPtr(157),
		No:          intPtr(0),
		Abstention:  intPtr(0),
		Absent:      intPtr(23),
		SourceURL:   geschaeftURL,
		GroupURL:    geschaeftURL,
		BulletinURL: "https://parlzhcdws.cmicloud.ch/parlzh3/cdws/Files/6e3f5290635f44f596a3f967499979bb-332/1/pdf",
		Attribution: "Daten: OpenParlData.ch",
		Affair: votes.Affair{
			Number: "20/2026",
			Title:  title,
			ID:     "337845",
			URL:    geschaeftURL,
			Type:   votes.WahlAffairType,
		},
	}

	withCantonLinks(&v, "5bb43fa5-18bd-4f5d-b501-28ee1eca8f2f", "287041e6-dc9a-428b-9c29-a37570e5f7ff")

	return []votes.Vote{v}
}

// KantonsratWahlgeschaeftTwoRounds is KantonsratWahlgeschaeft with a second
// roll call on the same business and day, as a second ballot round would add.
// The second record is synthetic; no such round happened on 14.09.2026.
//
// It exists because votes are grouped by business and sitting day, and a
// formatter that recognised a Wahlgeschäft only as a single-vote group would
// send this one down the ordinary multi-vote path, publishing both headcounts
// as results. See votes.IsWahlgeschaeftGroup.
func KantonsratWahlgeschaeftTwoRounds() []votes.Vote {
	first := KantonsratWahlgeschaeft()[0]

	second := first
	second.SourceID = "EA7109F3-0000-0000-0000-000000000002"
	second.Date = first.Date.Add(40 * time.Minute)
	second.Sequence = "1789369179"
	second.Yes = intPtr(151)
	second.Absent = intPtr(29)
	withCantonLinks(&second, "5bb43fa5-18bd-4f5d-b501-28ee1eca8f2f", "00000000-0000-0000-0000-000000000002")

	return []votes.Vote{first, second}
}

// kantonsratRoster spreads a tally across the chamber's factions in roughly
// their real proportions. Exact per-faction figures are not the point here —
// the group exists to exercise labelling and layout — but the total must match
// the reported counts or the completeness gate would drop the breakdown.
func kantonsratRoster(ja, nein, abw int) []votes.MemberVote {
	shares := []struct {
		name string
		size int
	}{
		{"SVP", 47}, {"SP", 36}, {"FDP", 30}, {"Grünliberale", 23},
		{"Grüne", 19}, {"Die Mitte", 12}, {"EVP", 7}, {"AL", 5},
	}

	var out []votes.MemberVote
	remaining := map[string]int{"Ja": ja, "Nein": nein, "Abwesend": abw}
	order := []string{"Nein", "Abwesend", "Ja"}

	for _, f := range shares {
		seats := f.size
		for _, choice := range order {
			for seats > 0 && remaining[choice] > 0 {
				out = append(out, votes.MemberVote{Fraktion: f.name, Choice: choice})
				remaining[choice]--
				seats--
			}
		}
	}
	return out
}

// FixtureNames returns fixture names in definition order.
var FixtureNames = []string{
	"single-vote-angenommen",
	"single-vote-abgelehnt",
	"single-vote-dringlicherklaerung",
	"long-title-truncation",
	"tall-title-band-clearance",
	"extreme-title-full-roster",
	"multi-vote-group",
	"generic-antrag-fallback",
	"ten-vote-stress-test",
	"vote-with-mentions",
	"auswahl-vote",
	"auswahl-vote-all-fraktionen",
	"mixed-multi-vote",
	"postulat-with-grnr-prefix",
	"kantonsrat-vote",
	"kantonsrat-multi-vote",
	"kantonsrat-lone-quorum-vote",
	"kantonsrat-decision-reported",
	"kantonsrat-member-business",
	"kantonsrat-co-signed-business",
	"kantonsrat-wahlgeschaeft",
	"kantonsrat-wahlgeschaeft-two-rounds",
}

// AllFixtures returns all fixtures keyed by kebab-case name.
func AllFixtures() map[string][]votes.Vote {
	return map[string][]votes.Vote{
		"single-vote-angenommen":              SingleVoteAngenommen(),
		"single-vote-abgelehnt":               SingleVoteAbgelehnt(),
		"single-vote-dringlicherklaerung":     SingleVoteDringlicherklaerung(),
		"long-title-truncation":               LongTitleTruncation(),
		"tall-title-band-clearance":           TallTitleBandClearance(),
		"extreme-title-full-roster":           ExtremeTitleFullRoster(),
		"multi-vote-group":                    MultiVoteGroup(),
		"generic-antrag-fallback":             GenericAntragFallback(),
		"ten-vote-stress-test":                TenVoteStressTest(),
		"vote-with-mentions":                  VoteWithMentions(),
		"auswahl-vote":                        AuswahlVote(),
		"auswahl-vote-all-fraktionen":         AuswahlVoteAllFraktionen(),
		"mixed-multi-vote":                    MixedMultiVote(),
		"postulat-with-grnr-prefix":           PostulatWithGrNrPrefix(),
		"kantonsrat-vote":                     KantonsratVote(),
		"kantonsrat-multi-vote":               KantonsratMultiVote(),
		"kantonsrat-lone-quorum-vote":         KantonsratLoneQuorumVote(),
		"kantonsrat-decision-reported":        KantonsratDecisionReported(),
		"kantonsrat-member-business":          KantonsratMemberBusiness(),
		"kantonsrat-co-signed-business":       KantonsratCoSignedBusiness(),
		"kantonsrat-wahlgeschaeft":            KantonsratWahlgeschaeft(),
		"kantonsrat-wahlgeschaeft-two-rounds": KantonsratWahlgeschaeftTwoRounds(),
	}
}
