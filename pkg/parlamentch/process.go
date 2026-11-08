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
	if len(group) == 0 {
		return nil, nil
	}
	id, err := strconv.ParseInt(group[0].Affair.ID, 10, 64)
	if err != nil {
		return nil, nil
	}
	p, err := c.process(id, group)
	if errors.Is(err, errSeveralBills) {
		return nil, nil
	}
	return p, err
}

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
		v, ok := c.decisiveFiledVote(group)
		if !ok {
			return nil, nil
		}
		return c.filedProcess(id, t, v)
	case typeParlIv, typeStandesIv:
		if v, ok := c.decisiveFiledVote(group); ok && v.Outcome == outcomeNoLeave {
			return c.closingProcess(id, v, votes.ProcessStep{
				Label: "Nationalrat", Detail: "Keine Folge gegeben", State: votes.StepEnded})
		}
		return c.finalVoteProcess(id, group)
	case typeFederalCouncil:
		return c.finalVoteProcess(id, group)
	}
	return nil, nil
}

// decisiveFiledVote is the one vote of a group whose outcome the meanings
// state, or false where there is not exactly one, or where it adopts the
// business as amended.
func (c *Client) decisiveFiledVote(group []votes.Vote) (votes.Vote, bool) {
	var decisive []votes.Vote
	for _, v := range group {
		if v.Outcome != "" {
			decisive = append(decisive, v)
		}
	}
	if len(decisive) != 1 {
		return votes.Vote{}, false
	}
	v := decisive[0]
	l := c.labels[v.SourceID]
	if amended.MatchString(l.meaningYes) || amended.MatchString(l.meaningNo) {
		return votes.Vote{}, false
	}
	return v, true
}

// filedProcess is the process of a motion or postulate after the Nationalrat
// decided it.
func (c *Client) filedProcess(id int64, businessType string, v votes.Vote) (*votes.Process, error) {
	if v.Outcome == outcomeFailed {
		return c.closingProcess(id, v, votes.ProcessStep{Label: "Nationalrat", Detail: "Ablehnung", State: votes.StepEnded})
	}
	if v.Outcome != outcomeAdopted {
		return nil, nil
	}
	steps, b, otherChamber, err := c.history(id, v)
	if err != nil {
		return nil, err
	}
	steps = votes.TrimHistory(steps, votes.MaxProcessSteps-2)
	steps = append(steps, votes.ProcessStep{Label: "Nationalrat", Detail: "Annahme", State: votes.StepCurrent})

	if businessType == typePostulat || otherChamber {
		steps = append(steps, votes.ProcessStep{Label: "Überwiesen an den Bundesrat", State: votes.StepNext})
		return &votes.Process{Steps: steps, Next: "Überwiesen an den Bundesrat"}, nil
	}
	// A motion the Nationalrat adopts first goes to the Ständerat. One filed
	// in the Ständerat that reaches the Nationalrat first fits neither rule.
	if b.SubmissionCouncil != 1 {
		return nil, nil
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
		return nil, nil
	}
	v := group[0]
	if !isFinalVote(v.Subtitle) || !plainMeanings[strings.ToLower(c.labels[v.SourceID].meaningYes)] {
		return nil, nil
	}
	switch v.Decision {
	case outcomeAdopted:
		return c.closingProcess(id, v, votes.ProcessStep{
			Label: "Nationalrat", Detail: "Annahme in der Schlussabstimmung", State: votes.StepCompleted})
	case outcomeFailed:
		return c.closingProcess(id, v, votes.ProcessStep{
			Label: "Nationalrat", Detail: "Ablehnung in der Schlussabstimmung", State: votes.StepEnded})
	}
	return nil, nil
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
	return &votes.Process{Steps: append(steps, step), Next: processDone}, nil
}

// history is the business's steps before the vote: its filing, the
// Bundesrat's opinion where it gave one, and each earlier decision of either
// chamber. otherChamber reports whether the Ständerat decided it before.
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
		if strings.TrimSpace(s.BusinessStatusName) != "Stellungnahme zum Vorstoss liegt vor" || odataDay(s.BusinessStatusDate) >= day {
			continue
		}
		step := votes.ProcessStep{Label: "Stellungnahme zum Vorstoss liegt vor", State: votes.StepDone}
		if p := strings.TrimSpace(b.FederalCouncilProposalText); p != "" {
			step.Detail = "Antrag des Bundesrates: " + p
		}
		steps = append(steps, step)
		break
	}

	bill, err := c.billOf(id, c.labels[v.SourceID].billNumber)
	if err != nil {
		return
	}
	resolutions, err := c.resolutions(bill)
	if err != nil {
		return
	}
	// The first decision of each chamber is what a reader needs: a business
	// that went back and forth has many more, and the card has room for few.
	seen := map[string]bool{}
	for _, r := range resolutions {
		council := strings.TrimSpace(r.CouncilName)
		if council == "" || seen[council] || odataDay(r.ResolutionDate) >= day {
			continue
		}
		seen[council] = true
		steps = append(steps, votes.ProcessStep{Label: council, Detail: strings.TrimSpace(r.ResolutionText), State: votes.StepDone})
		if council == "Ständerat" {
			otherChamber = true
		}
	}
	return
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
