package parlamentch

import (
	"fmt"
	"log"
	"strings"

	"github.com/siiitschiii/parlinfo/pkg/votes"
)

// BusinessRole codes for the members a post names.
//
// 7, the Urheber(-in), filed the business. 4, the Übernehmer(-in), took it over
// when the filer left the chamber; a parliamentary initiative routinely
// outlives its author's term — 17.523 was filed by Luzi Stamm in 2017 and came
// to its final vote in 2026 under Bruno Walliser.
//
// 3, the Mitunterzeichner(-in), is left out, unlike in Kanton Zürich where every
// signatory is named. A cantonal Postulat carries a handful of names; a federal
// initiative carries up to eighteen, which fill half the card and push the
// filer of the business into a reply on X. The filer is the fact a reader
// needs; the co-signers are on the business page the post links.
const (
	roleTakenOver = 4
	roleAuthor    = 7
)

// businessRoleDTO is one row of BusinessRole: a person, group, committee or
// canton with a role on a business. MemberCouncilNumber is null unless the
// role is held by a member of a council.
type businessRoleDTO struct {
	Role                int    `json:"Role"`
	MemberCouncilNumber *int64 `json:"MemberCouncilNumber"`
}

// authorDTO is the part of a MemberCouncil row a post names.
type authorDTO struct {
	ID                int64  `json:"ID"`
	FirstName         string `json:"FirstName"`
	LastName          string `json:"LastName"`
	PartyAbbreviation string `json:"PartyAbbreviation"`
}

// partyLabels maps the service's party abbreviations onto the names posts use
// elsewhere. Unlisted abbreviations (SVP, SP, EVP, EDU, Lega, MCG) already are
// those names and pass through.
var partyLabels = map[string]string{
	"M-E":          "Mitte",
	"FDP-Liberale": "FDP",
	"GRÜNE":        "Grüne",
	"glp":          "GLP",
	"-":            "parteilos",
}

func partyLabel(abbreviation string) string {
	abbreviation = strings.TrimSpace(abbreviation)
	if label, ok := partyLabels[abbreviation]; ok {
		return label
	}
	return abbreviation
}

// authorsOf returns the members who filed a business, remembering them for the
// rest of the run, since every vote of a group asks.
//
// Authorship is enrichment, so a failure costs the names and nothing else: the
// post still carries the subject, the counts and the link.
func (c *Client) authorsOf(businessNumber int64) []votes.Author {
	if authors, ok := c.authors[businessNumber]; ok {
		return authors
	}
	authors, err := c.businessAuthors(businessNumber)
	if err != nil {
		log.Printf("⚠️  parlamentch: could not fetch the authors of business %d: %v", businessNumber, err)
		// Cached as absent, so one outage is not retried once per vote.
		authors = nil
	}
	c.authors[businessNumber] = authors
	return authors
}

// businessAuthors asks who filed a business and who took it over, then looks
// their names and parties up.
//
// Most business the Nationalrat takes a final vote on comes from the Federal
// Council or a committee, and has no member in either role: in the year to
// November 2026, 11 of 105. Those cost one request that returns nothing.
func (c *Client) businessAuthors(businessNumber int64) ([]votes.Author, error) {
	var roles []businessRoleDTO
	err := c.get("/BusinessRole", query{
		"$filter": fmt.Sprintf("Language eq 'DE' and BusinessNumber eq %d and (Role eq %d or Role eq %d)",
			businessNumber, roleAuthor, roleTakenOver),
		"$select": "Role,MemberCouncilNumber",
	}, &roles)
	if err != nil {
		return nil, err
	}
	ids := authorOrder(roles)
	if len(ids) == 0 {
		return nil, nil
	}

	clauses := make([]string, len(ids))
	for i, id := range ids {
		clauses[i] = fmt.Sprintf("ID eq %d", id)
	}
	var members []authorDTO
	err = c.get("/MemberCouncil", query{
		"$filter": "Language eq 'DE' and (" + strings.Join(clauses, " or ") + ")",
		"$select": "ID,FirstName,LastName,PartyAbbreviation",
	}, &members)
	if err != nil {
		return nil, err
	}
	return namedAuthors(ids, members), nil
}

// authorOrder returns the members holding a role, filer first: the business is
// theirs, and the one who took it over is named after them. Rows held by a
// committee or a canton have no member number and are skipped; a committee's
// business is posted as the committee's, without names.
func authorOrder(roles []businessRoleDTO) []int64 {
	var ids []int64
	seen := map[int64]bool{}
	for _, want := range []int{roleAuthor, roleTakenOver} {
		for _, r := range roles {
			if r.Role != want || r.MemberCouncilNumber == nil || seen[*r.MemberCouncilNumber] {
				continue
			}
			seen[*r.MemberCouncilNumber] = true
			ids = append(ids, *r.MemberCouncilNumber)
		}
	}
	return ids
}

// namedAuthors puts the looked-up members in the order of ids. A member the
// lookup did not return, or one without a name or party, is left out: the
// formatters treat the party as what marks a member acting in the chamber, and
// a filer who has left is still a member of record with their party.
func namedAuthors(ids []int64, members []authorDTO) []votes.Author {
	byID := make(map[int64]authorDTO, len(members))
	for _, m := range members {
		byID[m.ID] = m
	}
	authors := make([]votes.Author, 0, len(ids))
	for _, id := range ids {
		m, ok := byID[id]
		if !ok {
			continue
		}
		name := strings.TrimSpace(strings.TrimSpace(m.FirstName) + " " + strings.TrimSpace(m.LastName))
		party := partyLabel(m.PartyAbbreviation)
		if name == "" || party == "" {
			continue
		}
		authors = append(authors, votes.Author{Name: name, Party: party})
	}
	return authors
}
