package zurichapi

import (
	"encoding/xml"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"

	"github.com/siiitschiii/parlinfo/pkg/votes"
)

// The Gemeinderat's process adapter: where a business stands after the vote a
// group reports, in the PARIS step names (see votes.Process).
//
// The rules were read off every Postulat and Motion of 2021 to 2026 in PARIS,
// against the step each Schlussabstimmung was recorded as:
//
//   - a Postulat: Ja is its Überweisung, Nein is "Ablehnung", and an
//     überwiesenes Postulat always continues with "Stadtrat, Weisung"
//   - a Motion: Nein is "Ablehnung". Ja is its Überweisung only
//     where the Stadtrat was ready to take it as a Motion and a member moved
//     to reject it; where the Stadtrat would take it only as a Postulat, or not
//     at all, a Ja was as often an "Umwandlung in Postulat", and gets no
//     process
//
// An Überweisung is recorded as "Überweisung, Frist 24 Monate" or, for about
// one business in seven, "Überweisung mit Textänderung, Frist 24 Monate", and
// the change of wording is not always announced by a step before the vote. So
// the post says "Überweisung", which both are, and the deadline only in the
// next step, which both set. A business whose history already holds a
// Textänderung gets no process. Weisungen get none either: a Ja is
// "Zustimmung" or "Zustimmung, fakultatives Referendum, Frist 60 Tage"
// depending on the Weisung, which the vote does not say.

var _ votes.ProcessSource = (*Client)(nil)

// reSchlussabstimmung matches the title PARIS gives the vote that decides a
// business, "2026_0351 Schlussabstimmung", and nothing else: it gives a
// deadline extension "2021_0183 Schlussabstimmung Fristverlängerung 2", whose
// Ja is no Überweisung.
var reSchlussabstimmung = regexp.MustCompile(`^\d{4}_\d+ Schlussabstimmung$`)

const (
	stepUeberweisung = "Überweisung"
	stepAblehnung    = "Ablehnung"
	// processClosed is PARIS's Geschaeftsstatus for a business nothing
	// follows. It is set some days after the vote; the post says it at once.
	processClosed = "Abgeschlossen"
)

// processGeschaeft is the part of a PARIS Geschäft the adapter reads.
type processGeschaeft struct {
	GRNr  string        `xml:"GRNr"`
	Art   string        `xml:"Geschaeftsart"`
	Steps []processStep `xml:"Ablaufschritte>Aufgabe"`
}

type processStep struct {
	Name  string `xml:"AblaufschrittName"`
	Datum struct {
		Start string `xml:"Start"`
		Text  string `xml:"Text"`
	} `xml:"Datum"`
}

// Process implements votes.ProcessSource.
func (c *Client) Process(group []votes.Vote) (*votes.Process, error) {
	if len(group) != 1 {
		return nil, none(fmt.Sprintf("%d Abstimmungen am selben Tag: nur eine Schlussabstimmung allein hat eine Regel", len(group)))
	}
	if !reSchlussabstimmung.MatchString(strings.TrimSpace(group[0].Subtitle)) {
		return nil, none("keine Schlussabstimmung über das Geschäft (" + strings.TrimSpace(group[0].Subtitle) + ")")
	}
	g, err := c.fetchProcessGeschaeft(group[0].Affair.Number)
	if err != nil {
		return nil, err
	}
	if g == nil {
		return nil, none("Geschäft " + group[0].Affair.Number + " nicht in PARIS gefunden")
	}
	p, why := processFor(group[0], *g)
	if p == nil {
		return nil, none(why)
	}
	return p, nil
}

func none(reason string) error { return votes.NoProcess{Reason: reason} }

// fetchProcessGeschaeft fetches a business by its GR number, or nil when PARIS
// has none.
func (c *Client) fetchProcessGeschaeft(grNr string) (*processGeschaeft, error) {
	if strings.TrimSpace(grNr) == "" {
		return nil, nil
	}
	// The query is spelled with %20 and %22, not url.QueryEscape's "+": the
	// API reads a plus as a literal.
	q := strings.NewReplacer("+", "%20").Replace(url.QueryEscape(fmt.Sprintf(`grnr adj "%s"`, grNr)))
	body, err := c.makeRequest(GeschaeftBaseURL + "/searchdetails/?q=" + q + "&l=de-CH")
	if err != nil {
		return nil, err
	}
	return parseProcessGeschaeft(body, grNr)
}

func parseProcessGeschaeft(body []byte, grNr string) (*processGeschaeft, error) {
	var resp struct {
		Hits []struct {
			G processGeschaeft `xml:"Geschaeft"`
		} `xml:"Hit"`
	}
	if err := xml.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("zurichapi: decoding Geschäft %s: %w", grNr, err)
	}
	for _, h := range resp.Hits {
		if h.G.GRNr == grNr {
			g := h.G
			return &g, nil
		}
	}
	return nil, nil
}

// processFor builds the process of a Postulat or Motion from its PARIS record
// and its Schlussabstimmung, or nil and the reason where no rule covers it.
func processFor(v votes.Vote, g processGeschaeft) (*votes.Process, string) {
	switch g.Art {
	case "Postulat", "Motion":
	case "Weisung":
		return nil, "Weisung: ein Ja ist \"Zustimmung\" oder \"Zustimmung, fakultatives Referendum, Frist 60 Tage\", und die Abstimmung sagt nicht, welches"
	case "":
		return nil, "Geschäftsart unbekannt"
	default:
		return nil, "Geschäftsart " + g.Art + ": keine Regel"
	}
	day := v.DateString()
	// Without the vote's date there is no telling which steps came before it.
	if day == "" {
		return nil, "Abstimmung ohne Datum"
	}
	// PARIS lists steps in the order they were entered, which is not always
	// the order they happened in.
	sorted := append([]processStep(nil), g.Steps...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Datum.Start < sorted[j].Datum.Start })
	var steps []votes.ProcessStep
	for _, s := range sorted {
		start := strings.TrimSpace(s.Datum.Start)
		name := strings.TrimSpace(s.Name)
		// Steps of the voting day count unless they record the vote's own
		// result: the Stadtrat's answer often comes the same day, and on
		// 2026/392 it did.
		if len(start) < 10 || start[:10] > day || (start[:10] == day && isResultStep(name)) {
			continue
		}
		if strings.Contains(name, "Textänderung") {
			return nil, "Textänderung im Verlauf (\"" + name + "\"): der Schritt der Abstimmung hat dann eine eigene Form"
		}
		steps = append(steps, votes.ProcessStep{Label: name, Detail: strings.TrimSpace(s.Datum.Text), State: votes.StepDone})
	}

	switch strings.TrimSpace(v.Decision) {
	case "Nein":
		steps = votes.TrimHistory(steps, votes.MaxProcessSteps-1)
		steps = append(steps, votes.ProcessStep{Label: "Gemeinderat", Detail: v.Date.Format("02.01.2006") + " · " + stepAblehnung, State: votes.StepEnded})
		return &votes.Process{Steps: steps, Next: processClosed}, ""
	case "Ja":
		if g.Art == "Motion" && !motionGoesToStadtrat(steps) {
			return nil, "Motion, die der Stadtrat nur als Postulat oder gar nicht entgegennehmen will: ein Ja ist ebenso oft eine \"Umwandlung in Postulat\" wie eine Überweisung"
		}
		steps = votes.TrimHistory(steps, votes.MaxProcessSteps-2)
		steps = append(steps,
			votes.ProcessStep{Label: "Gemeinderat", Detail: v.Date.Format("02.01.2006") + " · " + stepUeberweisung, State: votes.StepCurrent},
			votes.ProcessStep{Label: "Stadtrat, Weisung", Detail: "Frist 24 Monate", State: votes.StepNext})
		return &votes.Process{Steps: steps, Next: "Stadtrat, Weisung, Frist 24 Monate"}, ""
	}
	return nil, "Schlussresultat \"" + v.Decision + "\": keine Regel"
}

// isResultStep reports whether a PARIS step records the result of a
// Schlussabstimmung, which the adapter states itself, rather than something
// that led up to it.
func isResultStep(name string) bool {
	return strings.HasPrefix(name, "Überweisung") || name == stepAblehnung || strings.HasPrefix(name, "Umwandlung")
}

// motionGoesToStadtrat reports whether a Motion's history is the one where a
// Ja is always its Überweisung: the Stadtrat ready to take it as a Motion, and
// a member moving to reject it.
func motionGoesToStadtrat(steps []votes.ProcessStep) bool {
	var accepted, contested bool
	for _, s := range steps {
		switch s.Label {
		case "Stadtrat, Entgegennahme":
			accepted = true
		case "Ablehnung, beantragt":
			contested = true
		case "Stadtrat, Entgegennahme als Postulat", "Stadtrat, Ablehnung":
			return false
		}
	}
	return accepted && contested
}
