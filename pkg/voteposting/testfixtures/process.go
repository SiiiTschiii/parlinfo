package testfixtures

import (
	"time"

	"github.com/siiitschiii/parlinfo/pkg/votes"
)

// The fixtures in this file carry a votes.Process, one per body, so the
// snapshot pins the next-step line and the status card everywhere they appear:
// a lone vote on a business that goes on, in each of the three bodies, and a
// multi-vote group whose last vote ends its business.
//
// The processes are written out by hand, as an adapter would produce them from
// its source's history and the vote's own result. They are shaped after real
// paths seen in the sources — parlament.ch's status history, the PARIS
// Ablaufschritte, the OpenParlData events — but no adapter fills them yet.

func withProcess(group []votes.Vote, p votes.Process) []votes.Vote {
	for i := range group {
		cp := p
		cp.Steps = append([]votes.ProcessStep(nil), p.Steps...)
		group[i].Affair.Process = &cp
	}
	return group
}

// ProcessNationalratMotion is a Nationalrat motion the Nationalrat adopts as
// the first chamber: the Ständerat decides next.
func ProcessNationalratMotion() []votes.Vote {
	const title = "Zugang zu Kinderbetreuung im ländlichen Raum verbessern"
	const affairURL = "https://www.parlament.ch/de/ratsbetrieb/suche-curia-vista/geschaeft?AffairId=20244305"

	v := votes.Vote{
		SourceID:     "38412",
		Jurisdiction: "nationalrat",
		Body:         "Nationalrat",
		SessionID:    "5215",
		Sequence:     "33002",
		Date:         time.Date(2026, 9, 14, 16, 52, 0, 0, time.UTC),
		Title:        title,
		Type:         "Normal",
		Decision:     "angenommen",
		Outcome:      "angenommen",
		Yes:          intPtr(102),
		No:           intPtr(88),
		Abstention:   intPtr(3),
		Absent:       intPtr(7),
		SourceURL:    affairURL,
		GroupURL:     affairURL,
		ArchiveURL:   "https://www.parlament.ch/poly/Abstimmung/52/out/vote_52_33002.pdf",
		Attribution:  "Daten: Parlamentsdienste der Bundesversammlung, Bern",
		Affair: votes.Affair{
			Number:  "24.4305",
			Title:   title,
			ID:      "20244305",
			URL:     affairURL,
			Type:    "Motion",
			Authors: []votes.Author{{Name: "Andrea Muster", Party: "SP", Role: votes.RoleSigned}},
		},
	}
	v.MemberVotes = makeStimmabgaben([]struct {
		Name                string
		Ja, Nein, Enth, Abw int
	}{
		{"SVP", 0, 66, 0, 2},
		{"SP", 40, 0, 0, 1},
		{"Mitte", 26, 2, 2, 1},
		{"FDP-Liberale", 4, 20, 1, 2},
		{"Grüne", 23, 0, 0, 0},
		{"GLP", 9, 0, 0, 1},
	})

	return withProcess([]votes.Vote{v}, votes.Process{
		Steps: []votes.ProcessStep{
			{Label: "Eingereicht", Detail: "Im Nationalrat, 13.12.2024", State: votes.StepDone},
			{Label: "Bundesrat", Detail: "Beantragt Ablehnung", State: votes.StepDone},
			{Label: "Nationalrat", Detail: "Angenommen", State: votes.StepCurrent},
			{Label: "Ständerat", Detail: "Entscheidet als Zweitrat", State: votes.StepNext},
			{Label: "Umsetzung", Detail: "Der Bundesrat setzt die Motion um", State: votes.StepNext},
		},
		Next: "Der Ständerat entscheidet.",
	})
}

// ProcessKantonsratPostulat is a Kantonsrat Postulat the chamber überweist
// against the Regierungsrat's advice: the Regierungsrat reports next.
func ProcessKantonsratPostulat() []votes.Vote {
	return withProcess(KantonsratMemberBusiness(), votes.Process{
		Steps: []votes.ProcessStep{
			{Label: "Eingereicht", Detail: "15.12.2025", State: votes.StepDone},
			{Label: "Regierungsrat", Detail: "Beantragt Ablehnung", State: votes.StepDone},
			{Label: "Kantonsrat", Detail: "Überwiesen", State: votes.StepCurrent},
			{Label: "Regierungsrat", Detail: "Erstattet Bericht", State: votes.StepNext},
		},
		Next: "Der Regierungsrat erstattet Bericht.",
	})
}

// ProcessGemeinderatPostulat is a Gemeinderat Postulat überwiesen after a
// member moved to reject it, with the deadline the source states.
func ProcessGemeinderatPostulat() []votes.Vote {
	return withProcess(SingleVoteAngenommen(), votes.Process{
		Steps: []votes.ProcessStep{
			{Label: "Eingereicht", Detail: "12.02.2025", State: votes.StepDone},
			{Label: "Stadtrat", Detail: "Ist bereit, das Postulat entgegenzunehmen", State: votes.StepDone},
			{Label: "Ablehnung beantragt", Detail: "Von einem Ratsmitglied", State: votes.StepDone},
			{Label: "Gemeinderat", Detail: "Überwiesen", State: votes.StepCurrent},
			{Label: "Stadtrat", Detail: "Bericht, Frist 24 Monate", State: votes.StepNext},
		},
		Next: "Der Stadtrat hat 24 Monate für seinen Bericht.",
	})
}

// ProcessEndedMultiVote is a Gemeinderat Weisung whose final vote fails: the
// business ends, and no step follows.
func ProcessEndedMultiVote() []votes.Vote {
	return withProcess(MultiVoteGroup(), votes.Process{
		Steps: []votes.ProcessStep{
			{Label: "Weisung", Detail: "Antrag des Stadtrats", State: votes.StepDone},
			{Label: "Kommission", Detail: "Vorberatung abgeschlossen", State: votes.StepDone},
			{Label: "Gemeinderat", Detail: "In der Schlussabstimmung abgelehnt", State: votes.StepEnded},
		},
	})
}
