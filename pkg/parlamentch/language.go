package parlamentch

import (
	"strings"

	"github.com/siiitschiii/parlinfo/pkg/votes"
)

// The service publishes every entity once per language, under the same ID, and
// a query names the language it wants. What it actually translates is uneven,
// and this file holds what has to be done about that.
//
// Translated: business and bill titles, business types, the long names of the
// parliamentary groups.
//
// Not translated: Subject and MeaningYes, which every language's feed serves
// in French since the Wintersession 2025; DecisionText, which is German in
// every feed; and the groups' abbreviations, which are internal codes
// ("Groupe B" for the SVP) rather than anything a reader would recognise. So
// the group labels and the subjects are this package's own.

// odataLanguage is the language as the service's Language key spells it.
func (c *Client) odataLanguage() string {
	return strings.ToUpper(string(c.language()))
}

// language is the jurisdiction's post language.
func (c *Client) language() votes.Language {
	return c.jurisdiction.Language.OrGerman()
}

// attributions are the credit the Parlamentsdienste's open data terms require,
// in each language: the service's own name for itself in that language,
// introduced the way "Daten:" introduces it on every other body's posts.
var attributions = map[votes.Language]string{
	votes.German:  "Daten: Parlamentsdienste der Bundesversammlung, Bern",
	votes.French:  "Données : Les Services du Parlement",
	votes.Italian: "Dati: Servizi del Parlamento",
}

// AttributionFor is the "Daten: …" line for a post in l.
func AttributionFor(l votes.Language) string {
	return attributions[l.OrGerman()]
}

// fraktionLabels names each parliamentary group, by the service's group code,
// in each language. The German labels are the ones the city and canton posts
// already use, so the table reads the same on every German account; the French
// and Italian ones are the parties' own abbreviations in those languages, the
// way the Romand and Ticinese press print them.
var fraktionLabels = map[votes.Language]map[string]string{
	votes.German: {
		"V":   "SVP",
		"S":   "SP",
		"RL":  "FDP",
		"M-E": "Mitte-EVP",
		"G":   "Grüne",
		"GL":  "GLP",
	},
	votes.French: {
		"V":   "UDC",
		"S":   "PS",
		"RL":  "PLR",
		"M-E": "Centre-PEV",
		"G":   "Vert-e-s",
		"GL":  "PVL",
	},
	votes.Italian: {
		"V":   "UDC",
		"S":   "PS",
		"RL":  "PLR",
		"M-E": "Centro-PEV",
		"G":   "Verdi",
		"GL":  "PVL",
	},
}

// subjectLabels renames the Subjects the editorial filter admits into the post
// language, keyed by the Subject in lower case.
//
// German has no entry on purpose. Its feed serves these labels in French, and
// the German account has been publishing them as served; renaming them there
// is a change to a live account's copy that belongs in a change of its own.
var subjectLabels = map[votes.Language]map[string]string{
	votes.French: {
		"schlussabstimmung":       "Vote final",
		"gesamtabstimmung":        "Vote sur l'ensemble",
		"votazione finale":        "Vote final",
		"votazione sul complesso": "Vote sur l'ensemble",
	},
	votes.Italian: {
		"vote final":          "Votazione finale",
		"vote sur l'ensemble": "Votazione sul complesso",
		"schlussabstimmung":   "Votazione finale",
		"gesamtabstimmung":    "Votazione sul complesso",
	},
}

// subjectLabel returns the Subject as the post should print it. Only an exact
// match is renamed: a hand-typed variant ("Gesamtabstimmung zur Vorlage 2")
// is printed as served rather than half translated.
func subjectLabel(l votes.Language, subject string) string {
	if label, ok := subjectLabels[l][strings.ToLower(strings.TrimSpace(subject))]; ok {
		return label
	}
	return subject
}

// yesLabels introduce the meaning of Ja where it is not the plain one.
var yesLabels = map[votes.Language]string{
	votes.German:  "Ja",
	votes.French:  "Oui",
	votes.Italian: "Sì",
}

// repeatLabels mark a vote the council took again, after its first count was
// contested. See repeat.go.
var repeatLabels = map[votes.Language]string{
	votes.German:  "(Wiederholung)",
	votes.French:  "(répétition)",
	votes.Italian: "(ripetizione)",
}

// urlLanguages are the path segments parlament.ch serves each language under.
// They match the codes, but the site is the authority on its own paths.
var urlLanguages = map[votes.Language]string{
	votes.German:  "de",
	votes.French:  "fr",
	votes.Italian: "it",
}

// processWords are the words a process card states that the vote itself does
// not yet carry, in each language: the step this vote took and the one after
// it are not in the service at posting time (see votes.Process), so they are
// taken from its vocabulary rather than read off the business.
//
// Each is the service's own name for a code, the same in every language's
// feed — a BusinessStatusId, a ResolutionId, a council — or, where the
// service has no entity for it, the business page's own label, from
// parlament.ch's resource file (ScriptResx.ashx, name itsystemsPdInternet,
// cultures de-DE, fr-FR and it-IT), read 09.11.2026.
type processWords struct {
	council       string // Council 1, the Nationalrat, as CouncilName
	submitted     string // BusinessStatusId 202
	proposal      string // "Antrag" + "des Bundesrates": affairPageLabelProposal, affairPageLabelFederalCouncil
	adopted       string // ResolutionId 20
	rejected      string // ResolutionId 22
	noLeave       string // ResolutionId 51
	finalAdopted  string // ResolutionId 36
	finalRejected string // ResolutionId 56
	transmitted   string // BusinessStatusId 209
	inCommittee   string // BusinessStatusId 206
	done          string // BusinessStatusId 229
	statusHeading string // businessPageStateOfConsulting
	committee     string // before a committee's abbreviation: the start of its CommitteeName
	bill          string // fmt verbs: businessPageDraft, the bill's number, how many there are; the "of" between is ours
	// standerat ends the CommitteeName of every Ständerat committee.
	standerat string
}

var processWordsByLanguage = map[votes.Language]processWords{
	votes.German: {
		council:       "Nationalrat",
		submitted:     "Eingereicht",
		proposal:      "Antrag des Bundesrates",
		adopted:       "Annahme",
		rejected:      "Ablehnung",
		noLeave:       "Keine Folge gegeben",
		finalAdopted:  "Annahme in der Schlussabstimmung",
		finalRejected: "Ablehnung in der Schlussabstimmung",
		transmitted:   "Überwiesen an den Bundesrat",
		inCommittee:   "In Kommission des Ständerats",
		done:          "Erledigt",
		statusHeading: "Stand der Beratungen:",
		committee:     "Kommission",
		bill:          "Entwurf %d von %d",
		standerat:     "Ständerat",
	},
	votes.French: {
		council:       "Conseil national",
		submitted:     "Déposé",
		proposal:      "Proposition du Conseil fédéral",
		adopted:       "Adoption",
		rejected:      "Rejet",
		noLeave:       "Refusé de donner suite",
		finalAdopted:  "Adoption (vote final)",
		finalRejected: "Rejet (vote final)",
		transmitted:   "Transmis au Conseil fédéral",
		inCommittee:   "En commission du Conseil des Etats",
		done:          "Liquidé",
		statusHeading: "Etat des délibérations:",
		committee:     "Commission",
		bill:          "Projet %d sur %d",
		standerat:     "Conseil des États",
	},
	votes.Italian: {
		council:       "Consiglio nazionale",
		submitted:     "Depositato",
		proposal:      "Proposta del Consiglio federale",
		adopted:       "Adozione",
		rejected:      "Reiezione",
		noLeave:       "Non è dato seguito",
		finalAdopted:  "Adozione nella votazione finale",
		finalRejected: "Reiezione nella votazione finale",
		transmitted:   "Trasmesso al Consiglio federale",
		inCommittee:   "Nella Commissione del Consiglio degli Stati",
		done:          "Liquidato",
		statusHeading: "Stato delle deliberazioni:",
		committee:     "Commissione",
		bill:          "Disegno %d di %d",
		standerat:     "Consiglio degli Stati",
	},
}

// processWords is the process vocabulary of the jurisdiction's language.
func (c *Client) processWords() processWords {
	if w, ok := processWordsByLanguage[c.language()]; ok {
		return w
	}
	return processWordsByLanguage[votes.German]
}
