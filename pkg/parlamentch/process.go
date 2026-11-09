package parlamentch

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/siiitschiii/parlinfo/pkg/votes"
)

// The Nationalrat's process adapter: where a business stands after the vote a
// group reports, in parlament.ch's own words (see votes.Process).
//
// Each rule below covers one kind of vote whose next status the service
// records the same way every time. They were read off the Sommer- and
// Herbstsession 2026, every Nationalrat vote of the kind against the status
// its business got next:
//
//   - a motion adopted as first chamber: "In Kommission des Ständerats", 52 of 52
//   - a motion adopted as second chamber: "Überwiesen an den Bundesrat", 33 of 33
//   - a postulate adopted: "Überwiesen an den Bundesrat", 12 of 13
//   - a motion or postulate rejected: "Erledigt", 77 of 77
//   - an initiative given no leave ("Keine Folge gegeben"): "Erledigt", 15 of 15
//   - a Schlussabstimmung, adopted or rejected: "Erledigt", 15 of 15
//
// Everything else gets no process: a business with several bills, a motion
// adopted as amended (2 of its 3
// went back to the Ständerat), an initiative given leave and a Gesamtabstimmung
// (each continues under more than one status), a group whose outcome the
// meanings do not settle, and any business type not named here.

var _ votes.ProcessSource = (*Client)(nil)

// typeFederalCouncil is the service's name for a bill the Bundesrat put
// before parliament.
const typeFederalCouncil = "Geschäft des Bundesrates"

// processDone is the status the service gives a business nothing follows.
const processDone = "Erledigt"

// amended matches a meaning that adopts a motion as amended, which sends it
// back to the other chamber rather than on to the Bundesrat.
var amended = regexp.MustCompile(`(?i)modifi|geändert|änderung`)

type processBusinessDTO struct {
	SubmissionCouncil          int    `json:"SubmissionCouncil"`
	FederalCouncilProposalText string `json:"FederalCouncilProposalText"`
	SubmissionDate             string `json:"SubmissionDate"`
}

type statusDTO struct {
	BusinessStatusName string `json:"BusinessStatusName"`
	BusinessStatusDate string `json:"BusinessStatusDate"`
}

type billDTO struct {
	ID         string `json:"ID"`
	BillNumber int64  `json:"BillNumber"`
}

type resolutionDTO struct {
	ResolutionDate string `json:"ResolutionDate"`
	ResolutionText string `json:"ResolutionText"`
	CouncilName    string `json:"CouncilName"`
}

type preconsultationDTO struct {
	CommitteeName string `json:"CommitteeName"`
	Abbreviation1 string `json:"Abbreviation1"`
}

// Process implements votes.ProcessSource.
func (c *Client) Process(group []votes.Vote) (*votes.Process, error) {
	// Without the vote's date there is no telling which decisions came before
	// it.
	if len(group) == 0 || group[0].DateString() == "" {
		return nil, none("Abstimmung ohne Datum")
	}
	id, err := strconv.ParseInt(group[0].Affair.ID, 10, 64)
	if err != nil {
		return nil, none("Geschäftsnummer nicht lesbar")
	}
	p, err := c.process(id, group)
	if errors.Is(err, errSeveralBills) {
		return nil, none("mehrere Entwürfe, und die Abstimmung nennt keinen")
	}
	return p, err
}

func none(reason string) error { return votes.NoProcess{Reason: reason} }

// errSeveralBills stops a business with more than one bill when the vote does
// not say which it was on. Such a business — a Volksinitiative and its
// counter-proposal, a law and its decree — has a history for each, and a card
// that merged them listed one bill's rejection on the way to the other's
// adoption: the service files decisions by bill, not by business. So the
// history is read from the vote's own bill, and without it there is none.
var errSeveralBills = errors.New("business has several bills, and the vote names none")

func (c *Client) process(id int64, group []votes.Vote) (*votes.Process, error) {
	switch t := group[0].Affair.Type; t {
	case typeMotion, typePostulat:
		v, why := c.decisiveFiledVote(group)
		if why != "" {
			return nil, none(why)
		}
		return c.filedProcess(id, t, v)
	case typeParlIv, typeStandesIv:
		if v, why := c.decisiveFiledVote(group); why == "" {
			switch v.Outcome {
			case outcomeNoLeave:
				return c.closingProcess(id, v, votes.ProcessStep{
					Label: "Nationalrat", Detail: "Keine Folge gegeben", State: votes.StepEnded})
			case outcomeLeave:
				return nil, none("Folge gegeben: nächster Status nicht eindeutig (\"Vorprüfung - in Kommission des Ständerates\" oder \"In Kommission des Ständerats\")")
			}
		}
		return c.finalVoteProcess(id, group)
	case typeFederalCouncil:
		return c.finalVoteProcess(id, group)
	case "":
		return nil, none("Geschäftsart unbekannt")
	default:
		return nil, none("Geschäftsart " + t + ": keine Regel")
	}
}

// decisiveFiledVote is the one vote of a group whose outcome the meanings
// state, or the reason there is none: not exactly one such vote, or one that
// adopts the business as amended.
func (c *Client) decisiveFiledVote(group []votes.Vote) (votes.Vote, string) {
	var decisive []votes.Vote
	for _, v := range group {
		if v.Outcome != "" {
			decisive = append(decisive, v)
		}
	}
	switch len(decisive) {
	case 0:
		return votes.Vote{}, "keine Abstimmung, deren Ausgang die Quelle nennt (z. B. über einzelne Punkte, Abschreibung oder Fristverlängerung)"
	case 1:
	default:
		return votes.Vote{}, fmt.Sprintf("%d Abstimmungen mit Ausgang am selben Tag: welche entscheidet, ist nicht eindeutig", len(decisive))
	}
	v := decisive[0]
	l := c.labels[v.SourceID]
	if amended.MatchString(l.meaningYes) || amended.MatchString(l.meaningNo) {
		return votes.Vote{}, "Annahme mit Änderung: geht zurück an den anderen Rat, nächster Status nicht eindeutig"
	}
	return v, ""
}

// filedProcess is the process of a motion or postulate after the Nationalrat
// decided it.
func (c *Client) filedProcess(id int64, businessType string, v votes.Vote) (*votes.Process, error) {
	if v.Outcome == outcomeFailed {
		return c.closingProcess(id, v, votes.ProcessStep{Label: "Nationalrat", Detail: "Ablehnung", State: votes.StepEnded})
	}
	if v.Outcome != outcomeAdopted {
		return nil, none("Ausgang " + v.Outcome + ": keine Regel")
	}
	steps, b, otherChamber, err := c.history(id, v)
	if err != nil {
		return nil, err
	}
	steps = votes.TrimHistory(steps, votes.MaxProcessSteps-2)
	steps = append(steps, votes.ProcessStep{Label: "Nationalrat", Detail: dated(v.DateString(), "Annahme"), State: votes.StepCurrent})

	if businessType == typePostulat || otherChamber {
		steps = append(steps, votes.ProcessStep{Label: "Überwiesen an den Bundesrat", State: votes.StepNext})
		return &votes.Process{Steps: steps, Next: "Überwiesen an den Bundesrat"}, nil
	}
	// A motion the Nationalrat adopts first goes to the Ständerat. One filed
	// in the Ständerat that reaches the Nationalrat first fits neither rule.
	if b.SubmissionCouncil != 1 {
		return nil, none("im Ständerat eingereichte Motion, über die der Ständerat noch nicht entschieden hat")
	}
	next := votes.ProcessStep{Label: "In Kommission des Ständerats", State: votes.StepNext}
	text := "In Kommission des Ständerats"
	if name, abbr, err := c.standeratCommittee(id); err != nil {
		return nil, err
	} else if name != "" {
		next.Detail = name
		if abbr != "" {
			next.Detail += " (" + abbr + ")"
			text += " (" + abbr + ")"
		}
	}
	steps = append(steps, next)
	return &votes.Process{Steps: steps, Next: text}, nil
}

// finalVoteProcess is the process of a business whose group is its
// Schlussabstimmung: the last vote it takes, after which it is "Erledigt"
// whichever way the vote went. The group must be that one vote, with Ja meaning
// to adopt the bill, so the verdict says what the vote did.
func (c *Client) finalVoteProcess(id int64, group []votes.Vote) (*votes.Process, error) {
	if len(group) != 1 {
		return nil, none(fmt.Sprintf("%d Abstimmungen am selben Tag (z. B. Detailberatung, Gesamtabstimmung): nur eine Schlussabstimmung allein hat eine Regel", len(group)))
	}
	v := group[0]
	if !isFinalVote(v.Subtitle) {
		return nil, none("keine Schlussabstimmung (" + strings.TrimSpace(v.Subtitle) + "): nächster Status nicht eindeutig")
	}
	if !plainMeanings[strings.ToLower(c.labels[v.SourceID].meaningYes)] {
		return nil, none("Ja bedeutet nicht \"Annahme des Entwurfs\" (" + c.labels[v.SourceID].meaningYes + ")")
	}
	switch v.Decision {
	case outcomeAdopted:
		return c.closingProcess(id, v, votes.ProcessStep{
			Label: "Nationalrat", Detail: "Annahme in der Schlussabstimmung", State: votes.StepCompleted})
	case outcomeFailed:
		return c.closingProcess(id, v, votes.ProcessStep{
			Label: "Nationalrat", Detail: "Ablehnung in der Schlussabstimmung", State: votes.StepEnded})
	}
	return nil, none("kein Ausgang (Stimmengleichheit)")
}

// isFinalVote reports whether a vote's subtitle names a Schlussabstimmung, in
// either language the service has used.
func isFinalVote(subtitle string) bool {
	s := strings.ToLower(strings.TrimSpace(subtitle))
	return strings.HasPrefix(s, "vote final") || strings.HasPrefix(s, "schlussabstimmung")
}

// closingProcess is a process this vote ends: the history, the vote's own
// step, and "Erledigt".
func (c *Client) closingProcess(id int64, v votes.Vote, step votes.ProcessStep) (*votes.Process, error) {
	steps, _, _, err := c.history(id, v)
	if err != nil {
		return nil, err
	}
	steps = votes.TrimHistory(steps, votes.MaxProcessSteps-1)
	step.Detail = dated(v.DateString(), step.Detail)
	return &votes.Process{Steps: append(steps, step), Next: processDone}, nil
}

// history is the business's steps before the vote, as parlament.ch dates and
// words them.
//
// It opens with the filing, the "Einreichungsdatum" the business page heads
// every business with, and for a Vorstoss the Bundesrat's opinion, from the
// business's status history. Then every decision of either chamber on the vote's bill, in order: those of
// the days before, and those the other chamber took on the vote's own day,
// which the service dates but does not time. otherChamber reports whether the
// Ständerat decided it before.
func (c *Client) history(id int64, v votes.Vote) (steps []votes.ProcessStep, b processBusinessDTO, otherChamber bool, err error) {
	day := v.DateString()

	var bs []processBusinessDTO
	if err = c.get("/Business", query{
		"$filter": fmt.Sprintf("Language eq 'DE' and ID eq %d", id),
		"$select": "SubmissionCouncil,FederalCouncilProposalText,SubmissionDate",
	}, &bs); err != nil {
		return
	}
	if len(bs) == 0 {
		err = fmt.Errorf("business %d not found", id)
		return
	}
	b = bs[0]

	{
		var statuses []statusDTO
		if err = c.get("/BusinessStatus", query{
			"$filter": fmt.Sprintf("Language eq 'DE' and BusinessNumber eq %d", id),
		}, &statuses); err != nil {
			return
		}
		if d := odataDay(b.SubmissionDate); d != "" {
			steps = append(steps, votes.ProcessStep{Label: "Eingereicht", Detail: dayDE(d), State: votes.StepDone})
		}
		for _, s := range statuses {
			d := odataDay(s.BusinessStatusDate)
			if strings.TrimSpace(s.BusinessStatusName) != "Stellungnahme zum Vorstoss liegt vor" || d >= day {
				continue
			}
			text := ""
			if p := strings.TrimSpace(b.FederalCouncilProposalText); p != "" {
				text = "Antrag des Bundesrates: " + p
			}
			steps = append(steps, votes.ProcessStep{Label: "Stellungnahme zum Vorstoss liegt vor", Detail: dated(d, text), State: votes.StepDone})
			break
		}
	}

	bill, err := c.billOf(id, c.labels[v.SourceID].billNumber)
	if err != nil {
		return
	}
	resolutions, err := c.resolutions(bill)
	if err != nil {
		return
	}
	for _, r := range resolutions {
		council := strings.TrimSpace(r.CouncilName)
		d := odataDay(r.ResolutionDate)
		// The Nationalrat's own decisions of the day are this vote's, which
		// the adapter states itself. Committees' decisions carry no council
		// and are not the chambers' chronology.
		if council == "" || d > day || (d == day && council == "Nationalrat") {
			continue
		}
		steps = append(steps, votes.ProcessStep{Label: council, Detail: dated(d, strings.TrimSpace(r.ResolutionText)), State: votes.StepDone})
		if council == "Ständerat" {
			otherChamber = true
		}
	}
	return
}

// dated prefixes a step's text with the day it was taken, as the business
// page prints it beside each decision.
func dated(day, text string) string {
	if day == "" {
		return text
	}
	if text == "" {
		return dayDE(day)
	}
	return dayDE(day) + " · " + text
}

// billOf is the bill a vote was on: the one numbered billNumber, or the
// business's only bill where the vote names none.
func (c *Client) billOf(id, billNumber int64) (billDTO, error) {
	var bills []billDTO
	if err := c.get("/Bill", query{
		"$filter": fmt.Sprintf("Language eq 'DE' and BusinessNumber eq %d", id),
		"$select": "ID,BillNumber",
	}, &bills); err != nil {
		return billDTO{}, err
	}
	for _, b := range bills {
		if billNumber > 0 && b.BillNumber == billNumber {
			return b, nil
		}
	}
	if billNumber == 0 && len(bills) == 1 {
		return bills[0], nil
	}
	return billDTO{}, errSeveralBills
}

// resolutions are the chambers' decisions on a bill, oldest first.
func (c *Client) resolutions(bl billDTO) ([]resolutionDTO, error) {
	var rs []resolutionDTO
	if err := c.get("/Resolution", query{
		"$filter": fmt.Sprintf("Language eq 'DE' and IdBill eq guid'%s'", bl.ID),
	}, &rs); err != nil {
		return nil, err
	}
	sort.SliceStable(rs, func(i, j int) bool { return odataDay(rs[i].ResolutionDate) < odataDay(rs[j].ResolutionDate) })
	return rs, nil
}

// standeratCommittee is the Ständerat committee that will deliberate the
// business, as the service names it, or "" where it names none yet.
func (c *Client) standeratCommittee(id int64) (name, abbreviation string, err error) {
	var pcs []preconsultationDTO
	if err := c.get("/Preconsultation", query{
		"$filter": fmt.Sprintf("Language eq 'DE' and BusinessNumber eq %d", id),
	}, &pcs); err != nil {
		return "", "", err
	}
	for _, p := range pcs {
		if strings.HasSuffix(strings.TrimSpace(p.CommitteeName), "Ständerat") {
			return strings.TrimSpace(p.CommitteeName), strings.TrimSpace(p.Abbreviation1), nil
		}
	}
	return "", "", nil
}

// odataDay is an OData date as YYYY-MM-DD in Swiss time, or "" when there is
// none.
func odataDay(s string) string {
	t := parseODataDate(s)
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02")
}

// dayDE turns YYYY-MM-DD into the DD.MM.YYYY the cards print.
func dayDE(day string) string {
	if len(day) != 10 {
		return day
	}
	return day[8:10] + "." + day[5:7] + "." + day[0:4]
}
