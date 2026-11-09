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
