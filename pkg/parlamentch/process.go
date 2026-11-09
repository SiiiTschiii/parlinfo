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
//
// The rules read the service's codes — the business type, the council, the
// decision — which are the same in every language's feed, and the history is
// read from the feed of the post's language. So a French or Italian post gets
// the process the German one does, in the words parlament.ch uses in that
// language; the few the adapter states itself are in processWords.

var _ votes.ProcessSource = (*Client)(nil)

// typeFederalCouncil is the service's name for a bill the Bundesrat put
// before parliament.
const typeFederalCouncil = "Geschäft des Bundesrates"

// businessKinds are the types the rules below know, by the service's
// BusinessType code, under their German names: the code is the same in every
// language's feed, where the name is not ("Objet du Conseil fédéral",
// "Mozione").
var businessKinds = map[int]string{1: typeFederalCouncil, 3: typeStandesIv, 4: typeParlIv, 5: typeMotion, 6: typePostulat}

// The service's codes the rules read, the same in every language's feed: the
// councils (Council) and the decisions they record (ResolutionId).
const (
	councilNational = 1
	councilStates   = 2

	resolutionAdopted  = 20
	resolutionRejected = 22
	resolutionLeave    = 16
	resolutionNoLeave  = 51
)

// amended matches a meaning that adopts a motion as amended, which sends it
// back to the other chamber rather than on to the Bundesrat.
var amended = regexp.MustCompile(`(?i)modifi|geändert|änderung`)

type processBusinessDTO struct {
	SubmissionCouncil          int    `json:"SubmissionCouncil"`
	FederalCouncilProposalText string `json:"FederalCouncilProposalText"`
	FederalCouncilProposalDate string `json:"FederalCouncilProposalDate"`
	SubmissionDate             string `json:"SubmissionDate"`
}

type billDTO struct {
	ID         string `json:"ID"`
	BillNumber int64  `json:"BillNumber"`
}

type resolutionDTO struct {
	ResolutionDate         string `json:"ResolutionDate"`
	ResolutionID           int    `json:"ResolutionId"`
	Council                int    `json:"Council"`
	ResolutionText         string `json:"ResolutionText"`
	CouncilName            string `json:"CouncilName"`
	CommitteeName          string `json:"CommitteeName"`
	CommitteeAbbreviation1 string `json:"CommitteeAbbreviation1"`
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
	// A vote the chamber repeated says nothing any more; the repetition does.
	group = c.standing(group)
	p, err := c.process(id, group)
	if errors.Is(err, errSeveralBills) {
		return nil, none("mehrere Entwürfe, und die Abstimmung nennt keinen")
	}
	if p != nil {
		p.About = typeAbout[c.language()][c.businessKind(group[0])]
		if n, bill := c.billCounts[id], c.labels[group[0].SourceID].billNumber; n > 1 && bill > 0 {
			p.Bill = fmt.Sprintf(c.processWords().bill, bill, n)
		}
	}
	return p, err
}

// typeAbout is parlament.ch's own sentence on what each business type asks
// for, the first of its page on the type ("Parlamentsporträt",
// "Beratungsgegenstände und parlamentarische Verfahren"), read 09.11.2026.
// The French and Italian are the first sentence of each type's section on
// the page's French and Italian versions ("Initiatives parlementaires,
// initiatives déposées par des cantons et interventions"; "Iniziative
// parlamentari, iniziative cantonali e interventi parlamentari"), read
// 09.11.2026.
var typeAbout = map[votes.Language]map[string]string{
	votes.German: {
		typeMotion:    "Mit einer Motion wird der Bundesrat beauftragt, einen Entwurf zu einem Erlass der Bundesversammlung vorzulegen oder eine Massnahme zu treffen.",
		typePostulat:  "Ein Postulat beauftragt den Bundesrat zu prüfen und zu berichten, ob ein Entwurf zu einem Erlass der Bundesversammlung vorgelegt oder eine Massnahme getroffen werden muss.",
		typeParlIv:    "Mit einer parlamentarischen Initiative kann ein Ratsmitglied, eine Fraktion oder eine Kommission den Entwurf zu einem Erlass oder die Grundzüge eines solchen Erlasses vorschlagen.",
		typeStandesIv: "Jeder Kanton kann mit einer Standesinitiative vorschlagen, dass eine Kommission einen Entwurf für einen Erlass der Bundesversammlung ausarbeitet.",
	},
	votes.French: {
		typeMotion:    "La motion charge le Conseil fédéral de déposer un projet d’acte de l’Assemblée fédérale ou de prendre une mesure.",
		typePostulat:  "Le postulat charge le Conseil fédéral d’examiner l’opportunité, soit de déposer un projet d’acte de l’Assemblée fédérale, soit de prendre une mesure et de présenter un rapport à ce sujet.",
		typeParlIv:    "Par la voie d’une initiative, un député, un groupe parlementaire ou une commission peut proposer qu’une commission élabore un projet d’acte de l’Assemblée fédérale.",
		typeStandesIv: "Tout canton peut proposer, au moyen d’une initiative, qu’une commission élabore un projet d’acte de l’Assemblée fédérale.",
	},
	votes.Italian: {
		typeMotion:    "La mozione incarica il Consiglio federale di presentare un disegno di atto legislativo dell’Assemblea federale o di prendere un provvedimento.",                                                  //nolint:misspell // Italian, as parlament.ch words it
		typePostulat:  "Il postulato incarica il Consiglio federale di esaminare l’opportunità di proporre un disegno di atto legislativo dell’Assemblea federale o di prendere un provvedimento.",                       //nolint:misspell // Italian, as parlament.ch words it
		typeParlIv:    "Mediante un’iniziativa parlamentare un parlamentare, un gruppo o una commissione parlamentare può proporre che una commissione elabori un progetto di atto legislativo dell’Assemblea federale.", //nolint:misspell // Italian, as parlament.ch words it
		typeStandesIv: "Ogni Cantone può proporre mediante un’iniziativa che una commissione elabori un progetto di atto legislativo dell'Assemblea federale.",                                                           //nolint:misspell // Italian, as parlament.ch words it
	},
}

// businessKind is the German name of a vote's business type, read from its
// type code where that was fetched and otherwise from the name the vote
// carries, which is the German one in the German feed.
func (c *Client) businessKind(v votes.Vote) string {
	if id, err := strconv.ParseInt(v.Affair.ID, 10, 64); err == nil {
		if code, ok := c.businessCodes[id]; ok {
			if kind, ok := businessKinds[code]; ok {
				return kind
			}
		}
	}
	return v.Affair.Type
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
	if l := c.labels[group[0].SourceID]; len(group) == 1 && group[0].Outcome == "" && c.filedKind(group[0]) != "" && l.meaningYes == "" && l.meaningNo == "" {
		v, err := c.recordedOutcome(id, group[0])
		if err != nil {
			return nil, err
		}
		group = []votes.Vote{v}
	}
	w := c.processWords()
	switch t := c.businessKind(group[0]); t {
	case typeMotion, typePostulat:
		v, why := c.decisiveFiledVote(group)
		if why != "" {
			return nil, none(why)
		}
		return c.filedProcess(id, t, v)
	case typeParlIv, typeStandesIv:
		v, why := c.decisiveFiledVote(group)
		if why == "" {
			switch v.Outcome {
			case outcomeNoLeave:
				return c.closingProcess(id, v, votes.ProcessStep{
					Label: w.council, Detail: w.noLeave, State: votes.StepEnded})
			case outcomeLeave:
				return nil, none("Folge gegeben: nächster Status nicht eindeutig (\"Vorprüfung - in Kommission des Ständerates\" oder \"In Kommission des Ständerats\")")
			}
		}
		// A vote on leave whose meaning cannot be read says so, rather than
		// being reported as no Schlussabstimmung, which it never claimed to be.
		if len(group) == 1 && !isFinalVote(group[0].Subtitle) && strings.TrimSpace(group[0].Subtitle) == "" && why != "" {
			return nil, none(why)
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
	w := c.processWords()
	if v.Outcome == outcomeFailed {
		return c.closingProcess(id, v, votes.ProcessStep{Label: w.council, Detail: w.rejected, State: votes.StepEnded})
	}
	if v.Outcome != outcomeAdopted {
		return nil, none("Ausgang " + v.Outcome + ": keine Regel")
	}
	steps, b, otherChamber, err := c.history(id, v)
	if err != nil {
		return nil, err
	}
	steps = votes.TrimHistory(steps, votes.MaxProcessSteps-2)
	steps = append(steps, votes.ProcessStep{Label: w.council, Detail: dated(v.DateString(), w.adopted), State: votes.StepCurrent})

	if businessType == typePostulat || otherChamber {
		steps = append(steps, votes.ProcessStep{Label: w.transmitted, State: votes.StepNext})
		return &votes.Process{Steps: steps, Next: w.transmitted}, nil
	}
	// A motion the Nationalrat adopts first goes to the Ständerat. One filed
	// in the Ständerat that reaches the Nationalrat first fits neither rule.
	if b.SubmissionCouncil != 1 {
		return nil, none("im Ständerat eingereichte Motion, über die der Ständerat noch nicht entschieden hat")
	}
	next := votes.ProcessStep{Label: w.inCommittee, State: votes.StepNext}
	text := w.inCommittee
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
	w := c.processWords()
	switch v.Decision {
	case outcomeAdopted:
		return c.closingProcess(id, v, votes.ProcessStep{
			Label: w.council, Detail: w.finalAdopted, State: votes.StepCompleted})
	case outcomeFailed:
		return c.closingProcess(id, v, votes.ProcessStep{
			Label: w.council, Detail: w.finalRejected, State: votes.StepEnded})
	}
	return nil, none("kein Ausgang (Stimmengleichheit)")
}

// isFinalVote reports whether a vote's subtitle names a Schlussabstimmung, in
// either language the service has used, or as the Italian post labels it (see
// subjectLabels).
func isFinalVote(subtitle string) bool {
	s := strings.ToLower(strings.TrimSpace(subtitle))
	return strings.HasPrefix(s, "vote final") || strings.HasPrefix(s, "schlussabstimmung") || strings.HasPrefix(s, "votazione finale")
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
	w := c.processWords()
	return &votes.Process{Steps: append(steps, step), Next: w.done, StatusLabel: w.statusHeading}, nil
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
		"$filter": fmt.Sprintf("Language eq '%s' and ID eq %d", c.odataLanguage(), id),
		"$select": "SubmissionCouncil,FederalCouncilProposalText,FederalCouncilProposalDate,SubmissionDate",
	}, &bs); err != nil {
		return
	}
	if len(bs) == 0 {
		err = fmt.Errorf("business %d not found", id)
		return
	}
	b = bs[0]

	if d := odataDay(b.SubmissionDate); d != "" {
		steps = append(steps, votes.ProcessStep{Label: c.processWords().submitted, Detail: dayDE(d), State: votes.StepDone})
	}
	// The Bundesrat's proposal on a Vorstoss, dated, as the business page
	// prints it under "Antrag des Bundesrates". It is read off the business
	// itself: the status "Stellungnahme zum Vorstoss liegt vor", which this
	// used to date it by, is missing from businesses filed before 2022.
	if p, d := strings.TrimSpace(b.FederalCouncilProposalText), odataDay(b.FederalCouncilProposalDate); p != "" && d != "" && d < day {
		steps = append(steps, votes.ProcessStep{Label: c.processWords().proposal, Detail: dated(d, p), State: votes.StepDone})
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
		// the adapter states itself.
		if d > day || (d == day && r.Council == councilNational) {
			continue
		}
		// A committee's decision carries no council but the committee: on an
		// initiative, "Folge geben (Erstrat)" and the other committee's
		// "Zustimmung" are the steps before the chamber votes, as the
		// business page's chronology lists them (20.454). It is labelled by
		// the committee's abbreviation, since its full name ("Kommission für
		// Wissenschaft, Bildung und Kultur Nationalrat") does not fit a
		// step's line. A decision with neither — "Abgeschrieben, weil nicht
		// innert zwei Jahren …", "Zurückgezogen", "Diskussion" — is the
		// service's bookkeeping, not a step.
		label := council
		if label == "" {
			abbr := strings.TrimSpace(r.CommitteeAbbreviation1)
			if abbr == "" || strings.TrimSpace(r.CommitteeName) == "" {
				continue
			}
			label = c.processWords().committee + " " + abbr
		}
		steps = append(steps, votes.ProcessStep{Label: label, Detail: dated(d, strings.TrimSpace(r.ResolutionText)), State: votes.StepDone})
		if r.Council == councilStates {
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
		"$filter": fmt.Sprintf("Language eq '%s' and BusinessNumber eq %d", c.odataLanguage(), id),
		"$select": "ID,BillNumber",
	}, &bills); err != nil {
		return billDTO{}, err
	}
	numbered := 0
	for _, b := range bills {
		if b.BillNumber > 0 {
			numbered++
		}
	}
	if c.billCounts == nil {
		c.billCounts = map[int64]int{}
	}
	c.billCounts[id] = numbered
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
		"$filter": fmt.Sprintf("Language eq '%s' and IdBill eq guid'%s'", c.odataLanguage(), bl.ID),
	}, &rs); err != nil {
		return nil, err
	}
	sort.SliceStable(rs, func(i, j int) bool { return odataDay(rs[i].ResolutionDate) < odataDay(rs[j].ResolutionDate) })
	return rs, nil
}

// recordedOutcomes are the Nationalrat's decisions on filed business by the
// service's ResolutionId, and the outcome each stands for: "Annahme",
// "Ablehnung", "Folge gegeben" and "Keine Folge gegeben" in the German feed.
var recordedOutcomes = map[int]string{
	resolutionAdopted:  outcomeAdopted,
	resolutionRejected: outcomeFailed,
	resolutionLeave:    outcomeLeave,
	resolutionNoLeave:  outcomeNoLeave,
}

// recordedOutcome is v with the outcome the service records for the
// Nationalrat on the vote's day, for a vote on filed business whose meanings
// it does not serve: on 25.301 (28.04.2026) neither Ja nor Nein had one, and
// the decision "Keine Folge gegeben" was recorded the next morning. Until it
// is, or where the day holds anything but one such decision, v is returned
// as it is and gets no process.
func (c *Client) recordedOutcome(id int64, v votes.Vote) (votes.Vote, error) {
	bill, err := c.billOf(id, c.labels[v.SourceID].billNumber)
	if errors.Is(err, errSeveralBills) {
		return v, nil
	}
	if err != nil {
		return v, err
	}
	rs, err := c.resolutions(bill)
	if err != nil {
		return v, err
	}
	var found []int
	for _, r := range rs {
		if r.Council == councilNational && odataDay(r.ResolutionDate) == v.DateString() {
			found = append(found, r.ResolutionID)
		}
	}
	if len(found) == 1 {
		v.Outcome = recordedOutcomes[found[0]]
	}
	return v, nil
}

// standeratCommittee is the Ständerat committee that will deliberate the
// business, as the service names it, or "" where it names none yet.
func (c *Client) standeratCommittee(id int64) (name, abbreviation string, err error) {
	var pcs []preconsultationDTO
	if err := c.get("/Preconsultation", query{
		"$filter": fmt.Sprintf("Language eq '%s' and BusinessNumber eq %d", c.odataLanguage(), id),
	}, &pcs); err != nil {
		return "", "", err
	}
	for _, p := range pcs {
		if strings.HasSuffix(strings.TrimSpace(p.CommitteeName), c.processWords().standerat) {
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
