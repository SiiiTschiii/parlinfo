package testfixtures

import (
	"fmt"
	"time"

	"github.com/siiitschiii/parlinfo/pkg/votes"
)

// The Nationalrat fixtures are real votes, in the shape pkg/parlamentch maps
// them to, once per post language. The titles, bill titles and business types
// are the service's own in each language; the Subject and the meaning of Ja are
// French in every feed, as the service serves them, and renamed only where the
// adapter renames them. The tallies are the Abstimmungsprotokoll's.
//
// They exist so the golden files pin what each language's accounts would post,
// which the Stadt Zürich fixtures cannot: those are German by construction.

// NationalratFixtureNames lists the Nationalrat fixtures in golden-file order.
var NationalratFixtureNames = []string{
	"nationalrat-single-vote",
	"nationalrat-two-bills",
}

// NationalratFixtures returns the Nationalrat fixtures as posted in l, keyed
// by name.
func NationalratFixtures(l votes.Language) map[string][]votes.Vote {
	return map[string][]votes.Vote{
		"nationalrat-single-vote": nationalratSingleVote(l),
		"nationalrat-two-bills":   nationalratTwoBills(l),
	}
}

// nationalratText is what a fixture says in each language.
type nationalratText map[votes.Language]string

func (t nationalratText) in(l votes.Language) string { return t[l.OrGerman()] }

var (
	nationalratBody = nationalratText{
		votes.German:  "Nationalrat",
		votes.French:  "Conseil national",
		votes.Italian: "Consiglio nazionale",
	}
	nationalratKey = nationalratText{
		votes.German:  "nationalrat",
		votes.French:  "nationalrat-fr",
		votes.Italian: "nationalrat-it",
	}
	nationalratAttribution = nationalratText{
		votes.German:  "Daten: Parlamentsdienste der Bundesversammlung, Bern",
		votes.French:  "Données : Les Services du Parlement",
		votes.Italian: "Dati: Servizi del Parlamento",
	}
	federalCouncilBusiness = nationalratText{
		votes.German:  "Geschäft des Bundesrates",
		votes.French:  "Objet du Conseil fédéral",
		votes.Italian: "Oggetto del Consiglio federale",
	}
	// The overall-vote Subject: French in the German feed, as served;
	// renamed into Italian by the adapter.
	overallVote = nationalratText{
		votes.German:  "Vote sur l'ensemble",
		votes.French:  "Vote sur l'ensemble",
		votes.Italian: "Votazione sul complesso",
	}
	finalVote = nationalratText{
		votes.German:  "Vote final",
		votes.French:  "Vote final",
		votes.Italian: "Votazione finale",
	}
	yesLabel = nationalratText{
		votes.German:  "Ja",
		votes.French:  "Oui",
		votes.Italian: "Sì",
	}
)

// nationalratGroups are the parliamentary groups as the adapter labels them,
// by group code.
var nationalratGroups = map[string]nationalratText{
	"V":   {votes.German: "SVP", votes.French: "UDC", votes.Italian: "UDC"},
	"S":   {votes.German: "SP", votes.French: "PS", votes.Italian: "PS"},
	"M-E": {votes.German: "Mitte-EVP", votes.French: "Centre-PEV", votes.Italian: "Centro-PEV"},
	"RL":  {votes.German: "FDP", votes.French: "PLR", votes.Italian: "PLR"},
	"G":   {votes.German: "Grüne", votes.French: "Vert-e-s", votes.Italian: "Verdi"},
	"GL":  {votes.German: "GLP", votes.French: "PVL", votes.Italian: "PVL"},
}

// groupTally is one group's row of the Abstimmungsprotokoll.
type groupTally struct {
	code                string
	ja, nein, enth, abw int
}

// nationalratVote builds a vote as pkg/parlamentch maps it, with its totals
// counted from the group rows, as the adapter counts them.
func nationalratVote(l votes.Language, id, registration int, businessNumber int, shortNumber string,
	at time.Time, title, subtitle, businessType string, tallies []groupTally) votes.Vote {
	affairURL := fmt.Sprintf("https://www.parlament.ch/%s/ratsbetrieb/suche-curia-vista/geschaeft?AffairId=%d",
		l.OrGerman(), businessNumber)

	var members []votes.MemberVote
	var ja, nein, enth, abw int
	for _, g := range tallies {
		members = append(members, makeStimmabgaben([]struct {
			Name                string
			Ja, Nein, Enth, Abw int
		}{{nationalratGroups[g.code].in(l), g.ja, g.nein, g.enth, g.abw}})...)
		ja, nein, enth, abw = ja+g.ja, nein+g.nein, enth+g.enth, abw+g.abw
	}
	decision := ""
	switch {
	case ja > nein:
		decision = "angenommen"
	case nein > ja:
		decision = "abgelehnt"
	}

	return votes.Vote{
		SourceID:     fmt.Sprint(id),
		Jurisdiction: nationalratKey.in(l),
		Body:         nationalratBody.in(l),
		Language:     l,
		SessionID:    "5215",
		Sequence:     fmt.Sprint(registration),
		Date:         at,
		Title:        title,
		Subtitle:     subtitle,
		Type:         "Normal",
		SourceURL:    affairURL,
		GroupURL:     affairURL,
		ArchiveURL:   fmt.Sprintf("https://www.parlament.ch/poly/Abstimmung/52/out/vote_52_%d.pdf", registration),
		Attribution:  nationalratAttribution.in(l),
		Yes:          intPtr(ja),
		No:           intPtr(nein),
		Abstention:   intPtr(enth),
		Absent:       intPtr(abw),
		Decision:     decision,
		Affair: votes.Affair{
			Number: shortNumber,
			Title:  title,
			ID:     fmt.Sprint(businessNumber),
			URL:    affairURL,
			Type:   businessType,
		},
		MemberVotes: members,
	}
}

// zurichTime is a vote's timestamp as the adapter reads it: the wall clock
// of the Abstimmungsprotokoll, in Zürich. See parlamentch.parseODataDate.
func zurichTime(s string) time.Time {
	loc, err := time.LoadLocation("Europe/Zurich")
	if err != nil {
		panic("testfixtures: " + err.Error())
	}
	t, err := time.ParseInLocation("2006-01-02 15:04:05", s, loc)
	if err != nil {
		panic("testfixtures: bad time " + s + ": " + err.Error())
	}
	return t
}

// nationalratSingleVote is the overall vote on 26.022, support for the
// victims of the Crans-Montana fire, 17.09.2026 (Ref. 32936): a lone vote on
// a single-bill business, so the Subject is its whole label.
func nationalratSingleVote(l votes.Language) []votes.Vote {
	title := nationalratText{
		votes.German:  "Bundesgesetz über die Unterstützung der Opfer der Brandkatastrophe von Crans-Montana vom 1. Januar 2026",
		votes.French:  "Loi fédérale sur le soutien aux victimes de l'incendie de Crans-Montana du 1er janvier 2026", //nolint:misspell // French, as the service serves it
		votes.Italian: "Legge federale sul sostegno alle vittime dell’incendio di Crans-Montana del 1° gennaio 2026",
	}
	return []votes.Vote{nationalratVote(l, 38674, 32936, 20260022, "26.022",
		zurichTime("2026-09-17 09:54:17"), title.in(l), overallVote.in(l), federalCouncilBusiness.in(l),
		[]groupTally{
			{"V", 1, 65, 0, 0},
			{"S", 41, 0, 0, 0},
			{"M-E", 28, 1, 0, 2},
			{"RL", 3, 21, 3, 0},
			{"G", 22, 0, 1, 0},
			{"GL", 11, 0, 0, 0},
		})}
}

// nationalratTwoBills is the pair of final votes on 25.059, the
// Pelzinitiative, 02.10.2026 (Ref. 33215 and 33216): one on the federal
// decree recommending that the people reject the initiative, one on the
// indirect counter-proposal. Each vote names its bill, and the first spells
// out its Ja, because "accepted" alone reads as the initiative carrying.
func nationalratTwoBills(l votes.Language) []votes.Vote {
	title := nationalratText{
		votes.German:  "«Ja zum Importverbot für tierquälerisch erzeugte Pelzprodukte (Pelzinitiative)». Volksinitiative und indirekter Gegenvorschlag (Änderung des Tierschutzgesetzes)",
		votes.French:  "«Oui à l'interdiction d'importer des produits en fourrure provenant d’animaux ayant subi de mauvais traitements (initiative fourrure)». Initiative populaire et contre-projet indirect (modification de la loi fédérale sur la protection des animaux)",
		votes.Italian: "«Sì al divieto di importare prodotti di pellicceria ottenuti infliggendo sofferenze agli animali (Iniziativa pellicce)». Iniziativa popolare e controprogetto indiretto (Modifica della legge federale sulla protezione degli animali)",
	}
	decree := nationalratText{
		votes.German:  "Bundesbeschluss über die Volksinitiative «Ja zum Importverbot für tierquälerisch erzeugte Pelzprodukte (Pelz-Initiative)»",
		votes.French:  "Arrêté fédéral relatif à l’initiative populaire «Oui à l’interdiction d’importer des produits en fourrure provenant d’animaux ayant subi de mauvais traitements (initiative fourrure)»",
		votes.Italian: "Decreto federale concernente l’iniziativa popolare federale «Sì al divieto di importare prodotti di pellicceria ottenuti infliggendo sofferenze agli animali (Iniziativa pellicce)»",
	}
	act := nationalratText{
		votes.German:  "Tierschutzgesetz (TSchG)",
		votes.French:  "Loi fédérale sur la protection des animaux (LPA)",
		votes.Italian: "Legge federale sulla protezione degli animali (LPAn)",
	}
	meaning := "Adopter le projet (recommandation de rejeter l'initiative)" //nolint:misspell // French, as the service serves it

	return []votes.Vote{
		nationalratVote(l, 38953, 33215, 20250059, "25.059",
			zurichTime("2026-10-02 09:29:18"), title.in(l),
			finalVote.in(l)+": "+decree.in(l)+" ("+yesLabel.in(l)+": "+meaning+")",
			federalCouncilBusiness.in(l),
			[]groupTally{
				{"V", 57, 4, 5, 0},
				{"S", 36, 4, 1, 0},
				{"M-E", 29, 1, 0, 1},
				{"RL", 26, 1, 0, 0},
				{"G", 0, 22, 0, 1},
				{"GL", 8, 1, 1, 1},
			}),
		nationalratVote(l, 38954, 33216, 20250059, "25.059",
			zurichTime("2026-10-02 09:30:14"), title.in(l),
			finalVote.in(l)+": "+act.in(l),
			federalCouncilBusiness.in(l),
			[]groupTally{
				{"V", 10, 52, 4, 0},
				{"S", 41, 0, 0, 0},
				{"M-E", 30, 0, 0, 1},
				{"RL", 26, 0, 1, 0},
				{"G", 22, 0, 0, 1},
				{"GL", 10, 0, 0, 1},
			}),
	}
}
