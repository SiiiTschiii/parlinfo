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
	ParlGroupNumber     *int64 `json:"ParlGroupNumber"`
	CantonNumber        *int64 `json:"CantonNumber"`
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

// fraktionNames are the Fraktionen of the current legislature by
// ParlGroupNumber, as a post names them after "der". Each name lists the
// parties it is tagged for, because the tagger reads them off the text: the
// Mitte-Fraktion is "Die Mitte-Fraktion. Die Mitte. EVP." and seats both, so it
// is written with both. The SVP-Fraktion also seats EDU, Lega and MCG, but its
// name is the SVP's alone; the tagger adds those from its own table, see
// contacts.fraktionParties.
//
// A group not listed — a Fraktion of an earlier legislature — is not named.
// Business filed by one and still on the agenda is rare enough that the post
// does without the name rather than guess at a group that has since
// dissolved or merged.
//
// The French and Italian names are what those languages' posts print after
// "du" and "del"; the tagger reads only the German ones, so they carry no
// party list.
var fraktionNames = map[votes.Language]map[int64]string{
	votes.German: {
		1:   "FDP-Fraktion",
		2:   "SP-Fraktion",
		3:   "Mitte-EVP-Fraktion",
		4:   "SVP-Fraktion",
		6:   "Grüne-Fraktion",
		137: "GLP-Fraktion",
	},
	votes.French: {
		1:   "groupe PLR",
		2:   "groupe PS",
		3:   "groupe Centre-PEV",
		4:   "groupe UDC",
		6:   "groupe des Vert-e-s",
		137: "groupe PVL",
	},
	votes.Italian: {
		1:   "gruppo PLR",
		2:   "gruppo PS",
		3:   "gruppo Centro-PEV",
		4:   "gruppo UDC",
		6:   "gruppo dei Verdi",
		137: "gruppo PVL",
	},
}

// cantonNames are the cantons by the service's CantonNumber, in each language,
// as a post names them after "des Kantons" or in brackets after the type. The
// numbers follow the federal order except Tessin, which the service numbers
// 27; the French and Italian names are the service's own.
var cantonNames = map[votes.Language]map[int64]string{
	votes.German: {
		1: "Zürich", 2: "Bern", 3: "Luzern", 4: "Uri", 5: "Schwyz", 6: "Obwalden", 7: "Nidwalden",
		8: "Glarus", 9: "Zug", 10: "Freiburg", 11: "Solothurn", 12: "Basel-Stadt", 13: "Basel-Landschaft",
		14: "Schaffhausen", 15: "Appenzell Ausserrhoden", 16: "Appenzell Innerrhoden", 17: "St. Gallen",
		18: "Graubünden", 19: "Aargau", 20: "Thurgau", 22: "Waadt", 23: "Wallis",
		24: "Neuenburg", 25: "Genf", 26: "Jura", 27: "Tessin",
	},
	votes.French: {
		1: "Zurich", 2: "Berne", 3: "Lucerne", 4: "Uri", 5: "Schwyz", 6: "Obwald", 7: "Nidwald",
		8: "Glaris", 9: "Zoug", 10: "Fribourg", 11: "Soleure", 12: "Bâle-Ville", 13: "Bâle-Campagne",
		14: "Schaffhouse", 15: "Appenzell Rh.-Ext.", 16: "Appenzell Rh.-Int.", 17: "St-Gall",
		18: "Grisons", 19: "Argovie", 20: "Thurgovie", 22: "Vaud", 23: "Valais",
		24: "Neuchâtel", 25: "Genève", 26: "Jura", 27: "Tessin",
	},
	votes.Italian: {
		1: "Zurigo", 2: "Berna", 3: "Lucerna", 4: "Uri", 5: "Svitto", 6: "Obvaldo", 7: "Nidvaldo",
		8: "Glarona", 9: "Zugo", 10: "Friburgo", 11: "Soletta", 12: "Basilea-Città", 13: "Basilea-Campagna",
		14: "Sciaffusa", 15: "Appenzello Esterno", 16: "Appenzello Interno", 17: "San Gallo",
		18: "Grigioni", 19: "Argovia", 20: "Turgovia", 22: "Vaud", 23: "Vallese",
		24: "Neuchâtel", 25: "Ginevra", 26: "Giura", 27: "Ticino",
	},
}

// authorship is who filed a business: members, a Fraktion, or a canton.
type authorship struct {
	authors  []votes.Author
	fraktion string
	canton   string
}

// authorsOf returns the members who filed, took over or co-signed a business.
func (c *Client) authorsOf(businessNumber int64) []votes.Author {
	return c.authorshipOf(businessNumber).authors
}

// authorshipOf returns who filed a business, remembering it for the rest of
// the run, since every vote of a group asks.
//
// Authorship is enrichment, so a failure costs the names and nothing else: the
// post still carries the subject, the counts and the link.
func (c *Client) authorshipOf(businessNumber int64) authorship {
	if a, ok := c.authors[businessNumber]; ok {
		return a
	}
	a, err := c.businessAuthors(businessNumber)
	if err != nil {
		log.Printf("⚠️  parlamentch: could not fetch the authors of business %d: %v", businessNumber, err)
		// Cached as absent, so one outage is not retried once per vote.
		a = authorship{}
	}
	c.authors[businessNumber] = a
	return a
}

// businessAuthors asks who filed a business, who took it over and who
// co-signed it, then looks their names and parties up in one request.
//
// Most business the Nationalrat takes a final vote on comes from the Federal
// Council or a committee, and has no member in either role: in the year to
// November 2026, 11 of 105. Those cost one request that returns nothing.
//
// A Fraktion files business too, and the service records it as the filer with
// a group number and no member. Of the 1,481 businesses that reached a final or
// overall vote from 2007 to November 2026, ten were a Fraktion's — "Ehe für
// alle", the GLP's, among them — and those are named by the group.
func (c *Client) businessAuthors(businessNumber int64) (authorship, error) {
	var roles []businessRoleDTO
	err := c.get("/BusinessRole", query{
		"$filter": fmt.Sprintf("Language eq '%s' and BusinessNumber eq %d and (Role eq %d or Role eq %d or Role eq %d)",
			c.odataLanguage(), businessNumber, roleAuthor, roleTakenOver, roleCoSigned),
		"$select": "Role,MemberCouncilNumber,ParlGroupNumber,CantonNumber",
	}, &roles)
	if err != nil {
		return authorship{}, err
	}
	a := authorship{fraktion: filingFraktion(c.language(), roles), canton: filingCanton(c.language(), roles)}
	held := authorOrder(roles)
	if len(held) == 0 {
		return a, nil
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
		return authorship{}, err
	}
	a.authors = namedAuthors(c.language(), held, members)
	return a, nil
}

// filingCanton returns the name of the canton that filed a business, a
// Standesinitiative, in l, or "" when no canton did.
func filingCanton(l votes.Language, roles []businessRoleDTO) string {
	for _, r := range roles {
		if r.Role == roleAuthor && r.CantonNumber != nil {
			return cantonNames[l.OrGerman()][*r.CantonNumber]
		}
	}
	return ""
}

// filingFraktion returns the name of the Fraktion that filed a business, in l,
// or "" when no Fraktion did or the one that did is not in fraktionNames.
func filingFraktion(l votes.Language, roles []businessRoleDTO) string {
	for _, r := range roles {
		if r.Role == roleAuthor && r.ParlGroupNumber != nil {
			return fraktionNames[l.OrGerman()][*r.ParlGroupNumber]
		}
	}
	return ""
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
