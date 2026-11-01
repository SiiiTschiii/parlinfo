package parlamentch

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/siiitschiii/parlinfo/pkg/votes"
)

// Client implements votes.Source.
var _ votes.Source = (*Client)(nil)

// voteDTO is one row of the Vote entity set: a single vote of the Nationalrat.
// It carries no totals; those exist only as the member rows in Voting.
type voteDTO struct {
	ID                  int64   `json:"ID"`
	RegistrationNumber  int64   `json:"RegistrationNumber"`
	BusinessNumber      int64   `json:"BusinessNumber"`
	BusinessShortNumber string  `json:"BusinessShortNumber"`
	BusinessTitle       string  `json:"BusinessTitle"`
	BillTitle           *string `json:"BillTitle"`
	IdLegislativePeriod int64   `json:"IdLegislativePeriod"`
	IdSession           int64   `json:"IdSession"`
	// Subject, MeaningYes and MeaningNo are served in French in the German
	// feed since the Wintersession 2025, and are published as the service
	// serves them. Subject is null on about a third of votes; none of those
	// is a final or overall vote.
	Subject    *string `json:"Subject"`
	MeaningYes *string `json:"MeaningYes"`
	VoteEnd    string  `json:"VoteEnd"`
}

// votingDTO is one member's row for one vote.
type votingDTO struct {
	FirstName                 string `json:"FirstName"`
	LastName                  string `json:"LastName"`
	ParlGroupCode             string `json:"ParlGroupCode"`
	ParlGroupNameAbbreviation string `json:"ParlGroupNameAbbreviation"`
	Decision                  int    `json:"Decision"`
	DecisionText              string `json:"DecisionText"`
}

// businessDTO is the part of a Business the posts use.
type businessDTO struct {
	BusinessTypeName string `json:"BusinessTypeName"`
}

// FetchRecent returns up to limit of the most recent votes that pass the
// subject filter, newest first.
//
// Member rows are not fetched here. They are 200 per vote, and almost every
// vote fetched has already been posted and is about to be discarded; the
// enrichment happens in GroupByAffair, which only sees what survived dedup.
func (c *Client) FetchRecent(limit int) ([]votes.Vote, error) {
	if limit <= 0 {
		return nil, nil
	}

	var rows []voteDTO
	err := c.get("/Vote", query{
		"$filter":  c.voteFilter(),
		"$orderby": "VoteEnd desc",
		"$top":     strconv.Itoa(limit),
		"$select":  "ID,RegistrationNumber,BusinessNumber,BusinessShortNumber,BusinessTitle,BillTitle,IdLegislativePeriod,IdSession,Subject,MeaningYes,VoteEnd",
	}, &rows)
	if err != nil {
		return nil, err
	}

	out := make([]votes.Vote, 0, len(rows))
	for _, r := range rows {
		v := c.toVote(r)
		c.labels[v.SourceID] = voteLabel{
			bill:       strings.TrimSpace(deref(r.BillTitle)),
			meaningYes: strings.TrimSpace(deref(r.MeaningYes)),
		}
		out = append(out, v)
	}
	return out, nil
}

// voteFilter builds the $filter for FetchRecent.
func (c *Client) voteFilter() string {
	filter := "Language eq 'DE'"
	if len(c.subjects) == 0 {
		return filter
	}
	clauses := make([]string, 0, len(c.subjects))
	for _, s := range c.subjects {
		// OData doubles a quote to escape it, and "Vote sur l'ensemble" has one.
		lit := strings.ReplaceAll(strings.ToLower(s), "'", "''")
		clauses = append(clauses, fmt.Sprintf("startswith(tolower(Subject),'%s') eq true", lit))
	}
	return filter + " and (" + strings.Join(clauses, " or ") + ")"
}

// GroupByAffair fills in each vote's member rows, totals and business type,
// and groups the votes by business and sitting day.
//
// Groups are not completed with further votes of the same business, unlike
// the other two sources. The subject filter already decides which votes of a
// business are posted, and widening a group would only pull back the article
// votes it exists to keep out.
func (c *Client) GroupByAffair(vs []votes.Vote) ([][]votes.Vote, error) {
	if len(vs) == 0 {
		return nil, nil
	}
	for i := range vs {
		if err := c.enrich(&vs[i]); err != nil {
			return nil, err
		}
	}
	groups := votes.GroupByAffairAndDate(vs)
	for _, g := range groups {
		c.label(g)
	}
	return groups, nil
}

// voteLabel is what the listing says about a vote beyond its Subject, kept
// until the vote's group is known because whether it is worth printing
// depends on the group. See Client.label.
type voteLabel struct {
	bill       string
	meaningYes string
}

// plainMeanings are the ways the service says that Ja adopts the bill, in both
// the languages it has used. A vote whose Ja means exactly that needs no
// explanation; any other meaning is printed.
var plainMeanings = map[string]bool{
	"adopter le projet":    true,
	"annahme der vorlage":  true,
	"annahme des entwurfs": true,
}

// label sets each vote's Subtitle to its Subject plus whatever a reader needs
// to tell it apart and to read its Ja correctly.
//
// The bill is named only when its business holds several. A business often
// carries more than one draft — a Volksinitiative and the indirect
// counter-proposal beside it — and the Nationalrat takes a final vote on each
// under the same Subject, so cards labelled by Subject alone cannot be told
// apart. For the usual business of a single bill the bill title only restates
// the business title at greater length, and on a card it pushed the real
// subject below four lines of it.
//
// Whether a business has several is asked of the service rather than read off
// the group, because which siblings a group still holds depends on what earlier
// runs posted and what the fetch window reached: a label that changed with them
// would leave the vote on the counter-proposal reading as the initiative
// whenever it arrived alone. See hasSeveralBills.
//
// The meaning of Ja is added when it is anything but "adopt the bill", because
// then the verdict is easy to misread. On the Pelz-Initiative the bill adopted
// is the one recommending that the people reject the initiative, so
// "Angenommen" on its own reads as the opposite of what happened.
//
// Both are published as the service serves them, French included; see
// voteDTO.
func (c *Client) label(group []votes.Vote) {
	for i := range group {
		l := c.labels[group[i].SourceID]
		parts := []string{}
		if group[i].Subtitle != "" {
			parts = append(parts, group[i].Subtitle)
		}
		if l.bill != "" && !strings.EqualFold(l.bill, group[i].Affair.Title) && c.hasSeveralBills(group[i].Affair.ID) {
			parts = append(parts, l.bill)
		}
		sub := strings.Join(parts, ": ")
		if l.meaningYes != "" && !plainMeanings[strings.ToLower(l.meaningYes)] {
			sub += " (Ja: " + l.meaningYes + ")"
		}
		group[i].Subtitle = strings.TrimSpace(sub)
	}
}

// hasSeveralBills reports whether a business holds more than one bill, which
// is whether any of its votes belongs to a bill numbered above the first.
//
// When the service cannot say, it answers true. The two mistakes are not
// symmetric: a bill named needlessly costs a longer label, while one left out
// where it was needed leaves two cards on the same business that nothing
// tells apart.
func (c *Client) hasSeveralBills(affairID string) bool {
	id, err := strconv.ParseInt(affairID, 10, 64)
	if err != nil {
		return false
	}
	if several, ok := c.multiBill[id]; ok {
		return several
	}
	n, err := c.count("/Vote", fmt.Sprintf("Language eq 'DE' and BusinessNumber eq %d and BillNumber gt 1", id))
	if err != nil {
		log.Printf("⚠️  parlamentch: could not count the bills of business %d, naming the bill: %v", id, err)
		return true
	}
	c.multiBill[id] = n > 0
	return n > 0
}

// enrich reads one vote's member rows and derives everything that rests on
// them.
func (c *Client) enrich(v *votes.Vote) error {
	var rows []votingDTO
	err := c.get("/Voting", query{
		"$filter": fmt.Sprintf("Language eq 'DE' and IdVote eq %s", v.SourceID),
		"$select": "FirstName,LastName,ParlGroupCode,ParlGroupNameAbbreviation,Decision,DecisionText",
	}, &rows)
	if err != nil {
		return err
	}
	applyMemberVotes(v, rows)

	if id, err := strconv.ParseInt(v.Affair.ID, 10, 64); err == nil {
		v.Affair.Type = c.businessType(id)
		v.Affair.Authors = c.authorsOf(id)
	}
	return nil
}

// applyMemberVotes sets a vote's member list, totals and decision from its
// member rows.
//
// The totals are counted from the member rows, which the votes package
// otherwise forbids. Here there is nothing else to take them from: the Vote
// entity carries no counts, and the member rows are the record — the
// Parlamentsdienste's own published tally is the same count of the same rows.
// So the completeness gate passes by construction, and what it guards against
// elsewhere is guarded here, against the one number that does not come from
// these rows: the chamber's seats. Every vote has a row for every seat, the
// President's included, so a response with fewer is a partial snapshot and
// would publish a wrong result with a confident verdict. It is refused, as is a
// row this adapter cannot classify.
func applyMemberVotes(v *votes.Vote, rows []votingDTO) {
	if len(rows) != Seats {
		// Not published yet, cut short, or a vacant seat. Leaving the type
		// empty refuses the vote loudly rather than posting a tally that
		// cannot be checked; a vacancy is rare and short enough to be worth
		// a red run over a wrong post.
		log.Printf("⚠️  parlamentch: vote %s has %d member rows for %d seats", v.SourceID, len(rows), Seats)
		v.Type = ""
		return
	}

	var ja, nein, enth, abw int
	members := make([]votes.MemberVote, 0, len(rows))
	for _, r := range rows {
		choice, counted := choiceFor(r.Decision)
		if !counted {
			continue
		}
		if choice == "" {
			// A decision code nobody has mapped. Guessing which bucket it
			// belongs in would misstate a tally; refusing the vote fails the
			// run and links it, so somebody looks.
			log.Printf("⚠️  parlamentch: vote %s has unknown decision %d (%q) for %s %s",
				v.SourceID, r.Decision, r.DecisionText, r.FirstName, r.LastName)
			v.Type = ""
			return
		}
		switch choice {
		case "Ja":
			ja++
		case "Nein":
			nein++
		case "Enthaltung":
			enth++
		case "Abwesend":
			abw++
		}
		members = append(members, votes.MemberVote{
			Name:     strings.TrimSpace(r.FirstName + " " + r.LastName),
			Fraktion: fraktionLabel(r.ParlGroupCode, r.ParlGroupNameAbbreviation),
			Choice:   choice,
		})
	}

	v.MemberVotes = members
	v.Yes, v.No, v.Abstention, v.Absent = &ja, &nein, &enth, &abw
	v.Decision = decisionFor(ja, nein)
}

// choiceFor maps the service's decision code onto the formatters' vocabulary.
// counted is false for a row that belongs in no column; choice is "" for a
// code nobody has mapped.
func choiceFor(decision int) (choice string, counted bool) {
	switch decision {
	case 1:
		return "Ja", true
	case 2:
		return "Nein", true
	case 3:
		return "Enthaltung", true
	// 5 "Hat nicht teilgenommen" and 6 "Entschuldigt gemäss Art. 57 Abs. 4".
	// The service keeps the two apart; the table has one column for both.
	case 5, 6:
		return "Abwesend", true
	// 7 "Die Präsidentin/der Präsident stimmt nicht". The chair is in the room
	// and votes only to break a tie, so filing them as absent would be wrong
	// and as an abstention would be worse. They are left out of the count, as
	// the Nationalrat's own tally leaves them out.
	case 7:
		return "", false
	}
	return "", true
}

// decisionFor states the outcome of a final or overall vote, which the
// service does not publish.
//
// It is derived, which the formatters otherwise refuse to do, and it is safe
// for exactly the votes this adapter is configured to fetch. A Gesamt- or
// Schlussabstimmung is decided by a simple majority of the votes cast, Ja is
// always "adopt the bill", and a tie is broken by the chair, whose vote then
// appears as a row. Votes that need more than a majority of those voting — the
// Ausgabenbremse and the Dringlichkeitsklausel — are taken under their own
// subjects and never reach here. An unbroken tie states nothing.
func decisionFor(ja, nein int) string {
	switch {
	case ja > nein:
		return "angenommen"
	case nein > ja:
		return "abgelehnt"
	}
	return ""
}

// fraktionLabels names each parliamentary group the way the city and canton
// posts already name theirs, so the table reads the same on every account. The
// keys are the service's group codes, which are stable across legislatures.
var fraktionLabels = map[string]string{
	"V":   "SVP",
	"S":   "SP",
	"RL":  "FDP",
	"M-E": "Mitte-EVP",
	"G":   "Grüne",
	"GL":  "GLP",
}

// fraktionLabel returns the table label for a member's group. A code nobody
// has mapped falls back to the service's own abbreviation, minus the
// "Fraktion " it opens with: a new group should appear under its official
// short name rather than vanish from the table.
func fraktionLabel(code, abbreviation string) string {
	if label, ok := fraktionLabels[code]; ok {
		return label
	}
	if rest, ok := strings.CutPrefix(abbreviation, "Fraktion "); ok {
		return rest
	}
	if abbreviation != "" {
		return abbreviation
	}
	return code
}

// businessType returns the kind of business ("Geschäft des Bundesrates",
// "Parlamentarische Initiative"), or "" when it cannot be fetched. A failure
// costs the label, not the post.
func (c *Client) businessType(id int64) string {
	if t, ok := c.businessTypes[id]; ok {
		return t
	}
	var b businessDTO
	if err := c.getOne(fmt.Sprintf("/Business(ID=%d,Language='DE')", id), query{
		"$select": "BusinessTypeName",
	}, &b); err != nil {
		log.Printf("⚠️  parlamentch: could not fetch business %d: %v", id, err)
		return ""
	}
	t := strings.TrimSpace(b.BusinessTypeName)
	c.businessTypes[id] = t
	return t
}

// toVote maps a Vote row. Totals and member votes are filled in later, by
// enrich.
func (c *Client) toVote(r voteDTO) votes.Vote {
	id := strconv.FormatInt(r.ID, 10)
	affairURL := BusinessURL(r.BusinessNumber)
	title := strings.TrimSpace(r.BusinessTitle)
	protocol := ""
	if r.IdLegislativePeriod > 0 {
		protocol = ProtocolURL(r.IdLegislativePeriod, r.RegistrationNumber)
	}

	return votes.Vote{
		SourceID:     id,
		Jurisdiction: c.jurisdiction.Key,
		Body:         c.jurisdiction.ShortName,
		SessionID:    strconv.FormatInt(r.IdSession, 10),
		Sequence:     strconv.FormatInt(r.RegistrationNumber, 10),
		Date:         parseODataDate(r.VoteEnd),
		Title:        title,
		Subtitle:     strings.TrimSpace(deref(r.Subject)),
		// Every vote this adapter is configured to fetch is a plain Ja/Nein
		// ballot decided by a simple majority — see decisionFor.
		Type:        "Normal",
		SourceURL:   affairURL,
		GroupURL:    affairURL,
		ArchiveURL:  protocol,
		Attribution: Attribution,
		Affair: votes.Affair{
			Number: r.BusinessShortNumber,
			Title:  title,
			ID:     strconv.FormatInt(r.BusinessNumber, 10),
			URL:    affairURL,
		},
	}
}

// deref reads a nullable string, treating null as empty.
func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// Seats is the Nationalrat's size, fixed by Art. 149 of the Federal
// Constitution.
const Seats = 200

// BusinessURL is the business's page in Curia Vista, the durable link for
// every vote taken on it.
func BusinessURL(businessNumber int64) string {
	return fmt.Sprintf("https://www.parlament.ch/de/ratsbetrieb/suche-curia-vista/geschaeft?AffairId=%d", businessNumber)
}

// ProtocolURL is a vote's Abstimmungsprotokoll: the Parlamentsdienste's PDF of
// the tally and every member's vote, headed "Ref." with the vote's
// RegistrationNumber. It is built rather than looked up, because the service
// names no document for it; the path is the one parlament.ch links from the
// Amtliches Bulletin, and the file is there within seconds of the vote closing.
func ProtocolURL(legislativePeriod, registrationNumber int64) string {
	return fmt.Sprintf("https://www.parlament.ch/poly/Abstimmung/%d/out/vote_%d_%d.pdf", legislativePeriod, legislativePeriod, registrationNumber)
}
