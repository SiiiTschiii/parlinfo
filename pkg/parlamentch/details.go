package parlamentch

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// communicationEmail is PersonCommunication's CommunicationType for an email
// address. The others are phone numbers and homepages.
const communicationEmail = 3

// MemberDetails is what an outreach campaign needs to know about a sitting
// member: how to address them, in which language, and where to write.
//
// It is kept apart from votes.Member because nothing that posts needs it, and
// an email address has no business travelling with the roster the bots load.
type MemberDetails struct {
	PersonNumber int64
	FirstName    string
	LastName     string
	// Gender is "m" or "f", as the service writes it.
	Gender string
	// Canton is the abbreviation, e.g. "VD".
	Canton string
	Party  string
	// BirthDate is zero when the service has none.
	BirthDate time.Time
	// Emails are the addresses the member publishes, in the service's order.
	// Empty for the few who publish none.
	Emails []string
}

// Name is the display name in the form the vote rows and contacts files use.
func (m MemberDetails) Name() string {
	return strings.TrimSpace(m.FirstName + " " + m.LastName)
}

type memberDetailsDTO struct {
	PersonNumber       int64  `json:"PersonNumber"`
	FirstName          string `json:"FirstName"`
	LastName           string `json:"LastName"`
	GenderAsString     string `json:"GenderAsString"`
	CantonAbbreviation string `json:"CantonAbbreviation"`
	PartyName          string `json:"PartyName"`
	DateOfBirth        string `json:"DateOfBirth"`
}

type communicationDTO struct {
	PersonNumber int64  `json:"PersonNumber"`
	Address      string `json:"Address"`
}

// FetchMemberDetails returns the members sitting in the client's chamber with
// their published email addresses, sorted by name.
//
// The addresses come in one query for every person at once rather than one per
// member: the service holds a few hundred email rows in all, not thousands,
// because it keeps them only for people currently in office.
func (c *Client) FetchMemberDetails() ([]MemberDetails, error) {
	var rows []memberDetailsDTO
	err := c.get("/MemberCouncil", query{
		"$filter": fmt.Sprintf("Language eq 'DE' and Active eq true and Council eq %d", c.council),
		"$select": "PersonNumber,FirstName,LastName,GenderAsString,CantonAbbreviation,PartyName,DateOfBirth",
	}, &rows)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("parlamentch: no sitting members found")
	}

	var comms []communicationDTO
	err = c.get("/PersonCommunication", query{
		"$filter": fmt.Sprintf("Language eq 'DE' and CommunicationType eq %d", communicationEmail),
		"$select": "PersonNumber,Address",
		"$top":    "10000",
	}, &comms)
	if err != nil {
		return nil, err
	}
	emails := make(map[int64][]string)
	for _, cm := range comms {
		if a := strings.TrimSpace(cm.Address); a != "" {
			emails[cm.PersonNumber] = append(emails[cm.PersonNumber], a)
		}
	}

	out := make([]MemberDetails, 0, len(rows))
	for _, r := range rows {
		var born time.Time
		if r.DateOfBirth != "" {
			born = parseODataDate(r.DateOfBirth)
		}
		out = append(out, MemberDetails{
			PersonNumber: r.PersonNumber,
			FirstName:    strings.TrimSpace(r.FirstName),
			LastName:     strings.TrimSpace(r.LastName),
			Gender:       strings.TrimSpace(r.GenderAsString),
			Canton:       strings.TrimSpace(r.CantonAbbreviation),
			Party:        strings.TrimSpace(r.PartyName),
			BirthDate:    born,
			Emails:       emails[r.PersonNumber],
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name() < out[j].Name() })
	return out, nil
}

type speechDTO struct {
	LanguageOfText string `json:"LanguageOfText"`
}

// SpeechLanguages counts the languages of a member's latest n speeches in the
// chamber, keyed "DE", "FR", "IT" (and "RM" on the rare occasion). It is the
// best published sign of which language a member works in: the service has a
// NativeLanguage field, but it is empty for all but one person.
func (c *Client) SpeechLanguages(personNumber int64, n int) (map[string]int, error) {
	var rows []speechDTO
	err := c.get("/Transcript", query{
		// Type 1 is a speech; the others are the chair's procedural lines.
		"$filter":  fmt.Sprintf("Language eq 'DE' and PersonNumber eq %d and Type eq 1", personNumber),
		"$orderby": "Start desc",
		"$top":     fmt.Sprint(n),
		"$select":  "LanguageOfText",
	}, &rows)
	if err != nil {
		return nil, err
	}
	counts := make(map[string]int)
	for _, r := range rows {
		if l := strings.ToUpper(strings.TrimSpace(r.LanguageOfText)); l != "" {
			counts[l]++
		}
	}
	return counts, nil
}
