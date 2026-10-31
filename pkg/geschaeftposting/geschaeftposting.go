// Package geschaeftposting announces newly filed Vorstösse — Motionen and
// Postulate — when they are submitted, before the council has voted on them.
//
// It is the counterpart of voteposting: that package reports what the council
// decided, this one reports what was put in front of it. The two share the
// platforms' posting code and the log format, and nothing else — a Geschäft is
// one PARIS record rather than a group of votes, so there is no grouping, no
// tally and no source abstraction. Only Gemeinderat Stadt Zürich publishes
// filings this way, so the package works on zurichapi.Geschaeft directly.
package geschaeftposting

import (
	"fmt"
	"strings"

	"github.com/siiitschiii/parlinfo/pkg/zurichapi"
)

// Body names the chamber in every headline.
const Body = "Gemeinderat Zürich"

// ArtenURL is the council's own explanation of every Geschäftsart, linked
// beneath the explainer so a reader can check it against the source.
const ArtenURL = "https://www.gemeinderat-zuerich.ch/gemeinderat/geschaeftsarten/"

// Art is how one Geschäftsart is announced.
type Art struct {
	// Name is the Geschaeftsart exactly as PARIS spells it.
	Name  string
	Emoji string
	// New and NewDringlich are the headline nouns, inflected for the article
	// the noun takes: "Neue Motion" but "Neues Postulat".
	New          string
	NewDringlich string
	// Question is the hint pointing at the explainer reply.
	Question string
	// Explanation and NextSteps make up the explainer reply. They paraphrase
	// ArtenURL and the procedure PARIS records for this type: the Stadtrat
	// takes a position (Entgegennahme or Ablehnung), then the Gemeinderat
	// decides whether to refer it (Überweisung). No date is promised, because
	// none of the recorded Fristen reliably says when the council will vote.
	Explanation string
	NextSteps   string
}

// arten is the set of Geschäftsarten announced.
//
// A type is only added once at least three real filings of it have been
// studied and condensed into testfixtures, because PARIS populates fields
// differently per type. Schriftliche Anfragen are left out on purpose — they
// are the most frequent type and would drown the rest — and Erklärungen and
// Beschlussanträge are procedural.
var arten = map[string]Art{
	"Motion": {
		Name:         "Motion",
		Emoji:        "📋",
		New:          "Neue Motion",
		NewDringlich: "Neue dringliche Motion",
		Question:     "👇 Was ist eine Motion?",
		Explanation:  "ℹ️ Eine Motion verpflichtet den Stadtrat, einen Entwurf zum Erlass, zur Änderung oder Aufhebung eines Beschlusses vorzulegen.",
		NextSteps:    "⏭️ Erst nimmt der Stadtrat Stellung, dann entscheidet der Gemeinderat über die Überweisung. Danach hat der Stadtrat zwei Jahre für den Entwurf.",
	},
	"Postulat": {
		Name:         "Postulat",
		Emoji:        "📝",
		New:          "Neues Postulat",
		NewDringlich: "Neues dringliches Postulat",
		Question:     "👇 Was ist ein Postulat?",
		Explanation:  "ℹ️ Ein Postulat fordert den Stadtrat auf zu prüfen, ob und wie er ein Anliegen umsetzen kann.",
		NextSteps:    "⏭️ Erst nimmt der Stadtrat Stellung, dann entscheidet der Gemeinderat über die Überweisung. Danach hat der Stadtrat zwei Jahre, um das Ergebnis seiner Prüfung vorzulegen.",
	},
}

// DringlichEmoji replaces the type emoji on a Vorstoss declared urgent.
const DringlichEmoji = "🚨"

// ArtOf returns how g is announced, and false for a type that is not.
func ArtOf(g zurichapi.Geschaeft) (Art, bool) {
	a, ok := arten[g.Geschaeftsart]
	return a, ok
}

// Supported reports whether g is a type this package announces.
func Supported(g zurichapi.Geschaeft) bool {
	_, ok := ArtOf(g)
	return ok
}

// Headline is the first line of every post: "📋 Neue Motion | Gemeinderat
// Zürich".
//
// Dringlich is almost never set on a fresh filing — the Dringlicherklärung is
// requested at a later sitting — but a Vorstoss that is already urgent when it
// is first seen says so, because urgency shortens every deadline after it.
func Headline(g zurichapi.Geschaeft) string {
	a, ok := ArtOf(g)
	if !ok {
		return ""
	}
	if g.Dringlich {
		return fmt.Sprintf("%s %s | %s", DringlichEmoji, a.NewDringlich, Body)
	}
	return fmt.Sprintf("%s %s | %s", a.Emoji, a.New, Body)
}

// Title is the Geschäft title with PARIS's line breaks and runs of whitespace
// collapsed, so it wraps as one paragraph.
func Title(g zurichapi.Geschaeft) string {
	return cleanSpace(g.Titel)
}

// Link is the Geschäft's page on the council website.
func Link(g zurichapi.Geschaeft) string {
	return zurichapi.GeschaeftLink(g.OBJGUID)
}

// DateLine says when the Vorstoss was filed. Beginn is the sitting at which it
// was submitted; empty when PARIS has no date.
func DateLine(g zurichapi.Geschaeft) string {
	if g.Beginn.Text == "" {
		return ""
	}
	return "📅 Eingereicht am " + g.Beginn.Text
}

// Explainer is the reply under the root post: what this type of Vorstoss does,
// then what happens to it next.
func Explainer(g zurichapi.Geschaeft) string {
	a, ok := ArtOf(g)
	if !ok {
		return ""
	}
	return a.Explanation + "\n\n" + a.NextSteps
}

// Signer is one name on a Vorstoss.
type Signer struct {
	Name  string
	Party string
	// Fraktion marks a Vorstoss filed by a whole parliamentary group, which
	// PARIS records as a signer whose Name is the party.
	Fraktion bool
}

// Label is how a signer is written in a post: "Brigitte Fürer (Grüne)", or
// "Fraktion SP" for a group.
func (s Signer) Label() string {
	switch {
	case s.Fraktion:
		return "Fraktion " + s.Name
	case s.Party != "":
		return fmt.Sprintf("%s (%s)", s.Name, s.Party)
	default:
		return s.Name
	}
}

// Signers returns the first signer followed by the named co-signers, in the
// order PARIS lists them.
//
// Names are normalised because PARIS joins first and last name with a
// non-breaking space: left in, it would stop contacts.Mapper from recognising
// the member, and nobody would be tagged.
func Signers(g zurichapi.Geschaeft) []Signer {
	all := append(append([]zurichapi.KontaktGremium{}, g.Erstunterzeichner...), g.Mitunterzeichner...)
	out := make([]Signer, 0, len(all))
	for _, k := range all {
		name := cleanSpace(k.Name)
		if name == "" {
			continue
		}
		out = append(out, Signer{Name: name, Party: cleanSpace(k.Partei), Fraktion: k.IstGremium})
	}
	return out
}

// Additional is the number of co-signers PARIS counts but does not name.
func Additional(g zurichapi.Geschaeft) int {
	if g.AnzahlMitunterzeichnende == nil || *g.AnzahlMitunterzeichnende < 0 {
		return 0
	}
	return *g.AnzahlMitunterzeichnende
}

// SignerLine names the first shown signers and counts the rest:
// "✍️ Brigitte Fürer (Grüne), Tanja Maag (AL) +31 weitere".
//
// A shown of zero or less, or at least the number of signers, names everyone.
// The count folds the unnamed co-signers in with any names left out for
// length, because to a reader both are simply more people behind it.
func SignerLine(g zurichapi.Geschaeft, shown int) string {
	signers := Signers(g)
	if shown <= 0 || shown > len(signers) {
		shown = len(signers)
	}
	rest := len(signers) - shown + Additional(g)

	labels := make([]string, 0, shown)
	for _, s := range signers[:shown] {
		labels = append(labels, s.Label())
	}

	line := strings.Join(labels, ", ")
	if rest > 0 {
		if line == "" {
			line = fmt.Sprintf("%d Mitunterzeichnende", rest)
		} else {
			line += fmt.Sprintf(" +%d weitere", rest)
		}
	}
	if line == "" {
		return ""
	}
	return "✍️ " + line
}

// FitRoot builds the root post within limit as measure counts it.
//
// The headline, the dates, the hint and linkLine are fixed; the signer line
// and the title give way, in that order. Names are shed first, down to the
// first signer, because they are counted in "+N weitere" and named again in
// the caption — the title is the one thing a reader cannot get back from the
// post. Only then is the title cut, with an ellipsis.
//
// tag, when not nil, is applied to the signer line before it is measured: on
// X it inserts @handles, which cost characters the budget has to see.
func FitRoot(g zurichapi.Geschaeft, limit int, measure func(string) int, tag func(string) string, linkLine string) string {
	a, _ := ArtOf(g)
	render := func(title string, shown int) string {
		signers := SignerLine(g, shown)
		if tag != nil {
			signers = tag(signers)
		}
		var meta []string
		for _, l := range []string{signers, DateLine(g)} {
			if l != "" {
				meta = append(meta, l)
			}
		}
		parts := []string{Headline(g), title}
		if len(meta) > 0 {
			parts = append(parts, strings.Join(meta, "\n"))
		}
		tail := a.Question
		if linkLine != "" {
			tail += "\n" + linkLine
		}
		parts = append(parts, tail)
		return strings.Join(parts, "\n\n")
	}

	title := Title(g)
	n := len(Signers(g))
	if n < 1 {
		n = 1
	}
	for shown := n; shown >= 1; shown-- {
		if text := render(title, shown); measure(text) <= limit {
			return text
		}
	}

	runes := []rune(title)
	for cut := len(runes) - 1; cut > 0; cut-- {
		text := render(strings.TrimRight(string(runes[:cut]), " ,;:–-")+"…", 1)
		if measure(text) <= limit {
			return text
		}
	}
	return render("…", 1)
}

// cleanSpace collapses every run of whitespace, including PARIS's non-breaking
// spaces and line breaks, into a single space.
func cleanSpace(s string) string {
	return strings.Join(strings.Fields(strings.ReplaceAll(s, " ", " ")), " ")
}
