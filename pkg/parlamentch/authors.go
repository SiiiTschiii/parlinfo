package parlamentch

import (
	"fmt"
	"log"
	"sort"
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
// 3, the Mitunterzeichner(-in), signed beside the filer. A federal initiative
// carries up to twenty, which would fill half the card, so they keep
// votes.RoleCoSigned and the formatters name them only in the text, where each
// is a handle that can be tagged.
const (
	roleCoSigned  = 3
	roleTakenOver = 4
	roleAuthor    = 7
)

// authorRoles maps the service's codes onto the roles posts distinguish, in
// the order they are named: who filed, who took over, who co-signed.
var authorRoles = []struct {
	code int
	role votes.AuthorRole
}{
	{roleAuthor, votes.RoleSigned},
	{roleTakenOver, votes.RoleTookOver},
	{roleCoSigned, votes.RoleCoSigned},
}

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

// partyLabels maps the service's party abbreviations, as each language's feed
// spells them, onto the names posts use elsewhere. Unlisted abbreviations
// (SVP, SP, EVP, EDU, Lega, MCG in German; UDC, PLR, PEV, UDF in French and
// Italian) already are those names and pass through.
var partyLabels = map[votes.Language]map[string]string{
	votes.German: {
		"M-E":          "Mitte",
		"FDP-Liberale": "FDP",
		"GRÜNE":        "Grüne",
		"glp":          "GLP",
		"-":            "parteilos",
	},
	votes.French: {
		"M-E":      "Centre",
		"PSS":      "PS",
		"VERT-E-S": "Vert-e-s",
		"pvl":      "PVL",
		"-":        "sans parti",
	},
	votes.Italian: {
		"M-E":   "Centro",
		"PSS":   "PS",
		"VERDI": "Verdi",
		"pvl":   "PVL",
		"-":     "senza partito",
	},
}

func partyLabel(l votes.Language, abbreviation string) string {
	abbreviation = strings.TrimSpace(abbreviation)
	if label, ok := partyLabels[l.OrGerman()][abbreviation]; ok {
		return label
	}
	return abbreviation
}

// authorsOf returns the members who filed, took over or co-signed a business,
// remembering them for the
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

// businessAuthors asks who filed a business, who took it over and who
// co-signed it, then looks their names and parties up in one request.
//
// Most business the Nationalrat takes a final vote on comes from the Federal
// Council or a committee, and has no member in either role: in the year to
// November 2026, 11 of 105. Those cost one request that returns nothing.
func (c *Client) businessAuthors(businessNumber int64) ([]votes.Author, error) {
	var roles []businessRoleDTO
	err := c.get("/BusinessRole", query{
		"$filter": fmt.Sprintf("Language eq '%s' and BusinessNumber eq %d and (Role eq %d or Role eq %d or Role eq %d)",
			c.odataLanguage(), businessNumber, roleAuthor, roleTakenOver, roleCoSigned),
		"$select": "Role,MemberCouncilNumber",
	}, &roles)
	if err != nil {
		return nil, err
	}
	held := authorOrder(roles)
	if len(held) == 0 {
		return nil, nil
	}

	clauses := make([]string, len(held))
	for i, h := range held {
		clauses[i] = fmt.Sprintf("ID eq %d", h.id)
	}
	var members []authorDTO
	err = c.get("/MemberCouncil", query{
		"$filter": "Language eq '" + c.odataLanguage() + "' and (" + strings.Join(clauses, " or ") + ")",
		"$select": "ID,FirstName,LastName,PartyAbbreviation",
	}, &members)
	if err != nil {
		return nil, err
	}
	return namedAuthors(c.language(), held, members), nil
}

// heldRole is a member and the role they hold on a business.
type heldRole struct {
	id   int64
	role votes.AuthorRole
}

// authorOrder returns the members holding a role, filer first: the business is
// theirs, then whoever took it over, then the co-signers. A member holding two
// roles is named once, in the first. Rows held by a committee or a canton have
// no member number and are skipped; a committee's business is posted as the
// committee's, without names.
func authorOrder(roles []businessRoleDTO) []heldRole {
	var held []heldRole
	seen := map[int64]bool{}
	for _, want := range authorRoles {
		for _, r := range roles {
			if r.Role != want.code || r.MemberCouncilNumber == nil || seen[*r.MemberCouncilNumber] {
				continue
			}
			seen[*r.MemberCouncilNumber] = true
			held = append(held, heldRole{*r.MemberCouncilNumber, want.role})
		}
	}
	return held
}

// namedAuthors puts the looked-up members in the order of held, with the
// co-signers sorted by surname: the service returns them in no stated order,
// and a list that reshuffled between runs would read as a different list. A
// member the lookup did not return, or one without a name or party, is left
// out: the formatters treat the party as what marks a member acting in the
// chamber, and a filer who has left is still a member of record with their
// party.
func namedAuthors(l votes.Language, held []heldRole, members []authorDTO) []votes.Author {
	byID := make(map[int64]authorDTO, len(members))
	for _, m := range members {
		byID[m.ID] = m
	}
	type named struct {
		author  votes.Author
		surname string
	}
	list := make([]named, 0, len(held))
	for _, h := range held {
		m, ok := byID[h.id]
		if !ok {
			continue
		}
		first, last := strings.TrimSpace(m.FirstName), strings.TrimSpace(m.LastName)
		name := strings.TrimSpace(first + " " + last)
		party := partyLabel(l, m.PartyAbbreviation)
		if name == "" || party == "" {
			continue
		}
		list = append(list, named{votes.Author{Name: name, Party: party, Role: h.role}, strings.ToLower(last + " " + first)})
	}
	// held puts the co-signers last, so only that tail is sorted.
	tail := len(list)
	for tail > 0 && list[tail-1].author.Role == votes.RoleCoSigned {
		tail--
	}
	coSigners := list[tail:]
	sort.SliceStable(coSigners, func(i, j int) bool { return coSigners[i].surname < coSigners[j].surname })

	authors := make([]votes.Author, len(list))
	for i, n := range list {
		authors[i] = n.author
	}
	return authors
}
