package testfixtures

import (
	"time"

	"github.com/siiitschiii/parlinfo/pkg/votes"
)

// The fixtures in this file carry a votes.Process, so the snapshot pins the
// next-step line and the status card everywhere they appear.
//
// The first three are real businesses, with their real tallies and the
// sources' own texts as of the day they were looked up: every Label and Detail
// below is what parlament.ch or the PARIS API publishes, and the steps for the
// vote itself and after it are taken from the same vocabulary, as an adapter
// would choose them (see votes.Process). The last is the constructed
// multi-vote group from MultiVoteGroup, given a process so the snapshot covers
// a status card among several result cards.
//
// The Kantonsrat has none: OpenParlData does not say what a Kantonsrat Ja
// stood for, so no adapter can derive a process for it.

func withProcess(group []votes.Vote, p votes.Process) []votes.Vote {
	for i := range group {
		cp := p
		cp.Steps = append([]votes.ProcessStep(nil), p.Steps...)
		group[i].Affair.Process = &cp
	}
	return group
}

// ProcessNationalratMotion is motion 24.3004, which the Nationalrat adopted on
// 07.03.2024 against the Bundesrat's proposal; parlament.ch then listed it "In
// Kommission des Ständerats". The motion is the committee's, so it names no
// member.
func ProcessNationalratMotion() []votes.Vote {
	const title = "Abschaffung der Alterskinderrenten und gleichzeitige Erhöhung der Ergänzungsleistungen für Eltern mit Unterhaltspflichten"
	const affairURL = "https://www.parlament.ch/de/ratsbetrieb/suche-curia-vista/geschaeft?AffairId=20243004"

	v := votes.Vote{
		SourceID:     "32084",
		Jurisdiction: "nationalrat",
		Body:         "Nationalrat",
		SessionID:    "5202",
		Sequence:     "28344",
		Date:         time.Date(2024, 3, 7, 13, 51, 42, 0, time.UTC),
		Title:        title,
		Type:         "Normal",
		Decision:     "angenommen",
		// "Antrag der Mehrheit (Annahme der Motion)" against "Antrag der
		// Minderheit Weichelt und des Bundesrates (Ablehnung der Motion)".
		Outcome:     "angenommen",
		Yes:         intPtr(117),
		No:          intPtr(62),
		Abstention:  intPtr(8),
		Absent:      intPtr(12),
		SourceURL:   affairURL,
		GroupURL:    affairURL,
		ArchiveURL:  "https://www.parlament.ch/poly/Abstimmung/52/out/vote_52_28344.pdf",
		Attribution: "Daten: Parlamentsdienste der Bundesversammlung, Bern",
		Affair: votes.Affair{
			Number: "24.3004",
			Title:  title,
			ID:     "20243004",
			URL:    affairURL,
			Type:   "Motion",
		},
	}
	v.MemberVotes = makeStimmabgaben([]struct {
		Name                string
		Ja, Nein, Enth, Abw int
	}{
		{"SVP", 58, 1, 4, 4},
		{"SP", 0, 38, 0, 2},
		{"Mitte-EVP", 27, 2, 1, 1},
		{"FDP", 22, 0, 3, 3},
		{"Grüne", 0, 21, 0, 2},
		{"GLP", 10, 0, 0, 0},
	})

	return withProcess([]votes.Vote{v}, votes.Process{
		Steps: []votes.ProcessStep{
			{Label: "Eingereicht", Detail: "18.01.2024", State: votes.StepDone},
			{Label: "Stellungnahme zum Vorstoss liegt vor", Detail: "26.02.2024 · Antrag des Bundesrates: Ablehnung", State: votes.StepDone},
			{Label: "Nationalrat", Detail: "07.03.2024 · Annahme", State: votes.StepCurrent},
			{Label: "In Kommission des Ständerats", Detail: "Kommission für soziale Sicherheit und Gesundheit Ständerat (SGK-S)", State: votes.StepNext},
		},
		Next: "In Kommission des Ständerats (SGK-S)",
	})
}

// ProcessGemeinderatPostulat is Postulat 2026/519, überwiesen on 21.10.2026
// after an Ablehnungsantrag. The PARIS step names are verbatim; the next step
// is the one composed text votes.Process allows: "Stadtrat, Weisung", the step
// every überwiesene Postulat records next, with the deadline the "Überweisung,
// Frist 24 Monate" step set.
func ProcessGemeinderatPostulat() []votes.Vote {
	// Title, Subtitle and Decision as zurichapi.ToVote maps the real record.
	v := vote("2026-519", "2026/519\nPostulat von Rahel Hofstetter (GLP) und Christoph Riedweg (GLP) vom 16.09.2026:\nKontrakte mit den Nachbarschaftshilfe-Vereinen, Ausweisung zusätzlicher Kennzahlen in der nächsten Beitragsvorlage", "2026/519", "Ja", 101, 16, 0, 8)
	v.Affair.Title = "Kontrakte mit den Nachbarschaftshilfe-Vereinen, Ausweisung zusätzlicher Kennzahlen in der nächsten Beitragsvorlage"
	v.Date = date(2026, 10, 21)
	v.Subtitle = "2026_0519 Schlussabstimmung"
	v.MemberVotes = makeStimmabgaben([]struct {
		Name                string
		Ja, Nein, Enth, Abw int
	}{
		{"SP", 40, 0, 0, 1},
		{"FDP", 22, 0, 0, 2},
		{"SVP", 0, 16, 0, 0},
		{"GLP", 12, 0, 0, 3},
		{"Grüne", 14, 0, 0, 0},
		{"AL", 8, 0, 0, 0},
		{"Die Mitte", 5, 0, 0, 2},
	})

	return withProcess([]votes.Vote{v}, votes.Process{
		Steps: []votes.ProcessStep{
			{Label: "Eingang, Frist 3 Monate", Detail: "16.09.2026", State: votes.StepDone},
			{Label: "Stadtrat, Entgegennahme", Detail: "23.09.2026", State: votes.StepDone},
			{Label: "Ablehnung, beantragt", Detail: "30.09.2026", State: votes.StepDone},
			{Label: "Gemeinderat", Detail: "21.10.2026 · Überweisung", State: votes.StepCurrent},
			{Label: "Stadtrat, Weisung", Detail: "Frist 24 Monate", State: votes.StepNext},
		},
		Next: "Stadtrat, Weisung, Frist 24 Monate",
	})
}

// ProcessGemeinderatAbgelehnt is Postulat 2026/392, which the Gemeinderat
// rejected on 26.08.2026 although the Stadtrat would have taken it: the vote
// ends the business, which PARIS then lists as "Abgeschlossen".
func ProcessGemeinderatAbgelehnt() []votes.Vote {
	v := vote("2026-392", "2026/392\nPostulat der SP- und Grüne-Fraktion vom 08.07.2026:\nUmsetzung einer stationsgebundenen Mikromobilitätsstrategie auf dem gesamten Stadtgebiet", "2026/392", "Nein", 51, 69, 0, 5)
	v.Affair.Title = "Umsetzung einer stationsgebundenen Mikromobilitätsstrategie auf dem gesamten Stadtgebiet"
	v.Date = date(2026, 8, 26)
	v.Subtitle = "2026_0392 Schlussabstimmung"
	v.MemberVotes = makeStimmabgaben([]struct {
		Name                string
		Ja, Nein, Enth, Abw int
	}{
		{"SP", 39, 0, 0, 2},
		{"FDP", 0, 24, 0, 0},
		{"SVP", 0, 16, 0, 0},
		{"GLP", 0, 14, 0, 1},
		{"Grüne", 12, 0, 0, 2},
		{"AL", 0, 8, 0, 0},
		{"Die Mitte", 0, 7, 0, 0},
	})

	return withProcess([]votes.Vote{v}, votes.Process{
		Steps: []votes.ProcessStep{
			{Label: "Eingang, Frist 3 Monate", Detail: "08.07.2026", State: votes.StepDone},
			{Label: "Stadtrat, Entgegennahme", Detail: "26.08.2026", State: votes.StepDone},
			{Label: "Gemeinderat", Detail: "26.08.2026 · Ablehnung", State: votes.StepEnded},
		},
		Next: "Abgeschlossen",
	})
}

// ProcessMultiVote is the constructed MultiVoteGroup with a process, so the
// snapshot covers a status card among several result cards. Its first two
// step names are the ones PARIS gives a Weisung; the business itself is not
// real.
func ProcessMultiVote() []votes.Vote {
	return withProcess(MultiVoteGroup(), votes.Process{
		Steps: []votes.ProcessStep{
			{Label: "Stadtrat, Weisung", State: votes.StepDone},
			{Label: "Überweisung, SK PRD/SSD", Detail: "Sachkommission Präsidialdepartement, Schul- und Sportdepartement", State: votes.StepDone},
			{Label: "Gemeinderat", Detail: "Schlussabstimmung: Nein", State: votes.StepEnded},
		},
		Next: "Abgeschlossen",
	})
}
