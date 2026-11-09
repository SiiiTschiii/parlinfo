package parlamentch

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/siiitschiii/parlinfo/pkg/votes"
)

// 17.523 was filed by Luzi Stamm, who left the Nationalrat in 2019, and taken
// over by Bruno Walliser. Both are named, the filer first.
func TestAuthorsOf_FilerThenTakeover(t *testing.T) {
	c, requests := newTestClient(t)

	got := c.authorsOf(20170523)
	want := []votes.Author{
		{Name: "Luzi Stamm", Party: "SVP", Role: votes.RoleSigned},
		{Name: "Bruno Walliser", Party: "SVP", Role: votes.RoleTookOver},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("authors = %v, want %v", got, want)
	}

	// Only the three roles a post names are asked for, on the server: a
	// business also lists the committees and cantons that dealt with it.
	var roleQuery string
	for _, q := range *requests {
		if strings.Contains(q, "BusinessNumber") {
			roleQuery = q
		}
	}
	if !strings.Contains(roleQuery, "Role+eq+7+or+Role+eq+4+or+Role+eq+3") && !strings.Contains(roleQuery, "Role%20eq%207%20or%20Role%20eq%204%20or%20Role%20eq%203") {
		t.Errorf("role query %q does not restrict to roles 7, 4 and 3", roleQuery)
	}
}

// 21.472 was filed by Stefania Prezioso Batou, taken over by Fabien Fivaz and
// co-signed by eleven. The service lists them in no particular order; posts
// name the filer, then the takeover, then the co-signers by surname.
func TestAuthorsOf_CoSignersBySurnameAfterTheTakeover(t *testing.T) {
	c, _ := newTestClient(t)

	got := c.authorsOf(20210472)
	var names []string
	for _, a := range got {
		names = append(names, a.Name)
	}
	want := []string{
		"Stefania Prezioso Batou", "Fabien Fivaz",
		"Denis de la Reussille", "Tamara Funiciello", "Balthasar Glättli", "Jean-Pierre Grin",
		"Greta Gysin", "Philipp Kutter", "Pierre-Yves Maillard", "Michel Matter",
		"Isabelle Moret", "Marie-France Roth Pasquier", "Christian Wasserfallen",
	}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("names = %v\nwant    %v", names, want)
	}
	if got[0].Role != votes.RoleSigned || got[1].Role != votes.RoleTookOver {
		t.Errorf("roles of filer and takeover = %v, %v", got[0].Role, got[1].Role)
	}
	for _, a := range got[2:] {
		if a.Role != votes.RoleCoSigned {
			t.Errorf("%s: role %v, want co-signed", a.Name, a.Role)
		}
	}
}

func TestAuthorsOf_AskedOncePerBusiness(t *testing.T) {
	c, requests := newTestClient(t)
	c.authorsOf(20170523)
	n := len(*requests)
	c.authorsOf(20170523)
	if len(*requests) != n {
		t.Errorf("second lookup sent %d more requests, want 0", len(*requests)-n)
	}
}

// Authorship is enrichment: a service failure leaves the vote without names
// rather than failing it.
func TestGroupByAffair_AuthorFailureKeepsTheVote(t *testing.T) {
	c, _ := newTestClient(t)
	inner := c.baseURL
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/BusinessRole" {
			http.Error(w, "down", http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, inner+r.URL.RequestURI(), http.StatusTemporaryRedirect)
	}))
	t.Cleanup(srv.Close)
	c.SetBaseURL(srv.URL)

	groups := fetchAndGroup(t, c)
	n := 0
	for _, g := range groups {
		for _, v := range g {
			n++
			if len(v.Affair.Authors) != 0 {
				t.Errorf("%s: authors %v despite the failure", v.SourceID, v.Affair.Authors)
			}
		}
	}
	if n != 4 {
		t.Errorf("%d votes, want 4", n)
	}
}

func TestAuthorOrder(t *testing.T) {
	id := func(n int64) *int64 { return &n }
	roles := []businessRoleDTO{
		{Role: roleCoSigned, MemberCouncilNumber: id(4268)},
		{Role: roleTakenOver, MemberCouncilNumber: id(3912)},
		{Role: roleAuthor, MemberCouncilNumber: nil}, // a committee
		{Role: roleAuthor, MemberCouncilNumber: id(1122)},
		{Role: roleTakenOver, MemberCouncilNumber: id(1122)},
		{Role: roleCoSigned, MemberCouncilNumber: id(3912)},
	}
	want := []heldRole{{1122, votes.RoleSigned}, {3912, votes.RoleTookOver}, {4268, votes.RoleCoSigned}}
	if got := authorOrder(roles); !reflect.DeepEqual(got, want) {
		t.Errorf("authorOrder = %v, want %v", got, want)
	}
}

func TestNamedAuthors_DropsWhoCannotBeNamed(t *testing.T) {
	got := namedAuthors(votes.German, []heldRole{{1, votes.RoleSigned}, {2, votes.RoleSigned}, {3, votes.RoleSigned}, {4, votes.RoleSigned}}, []authorDTO{
		{ID: 4, FirstName: "Thomas", LastName: "Minder", PartyAbbreviation: "-"},
		{ID: 2, FirstName: "Ohne", LastName: "Partei"},
		{ID: 1, FirstName: "Andrea", LastName: "Caroni", PartyAbbreviation: "FDP-Liberale"},
	})
	want := []votes.Author{{Name: "Andrea Caroni", Party: "FDP"}, {Name: "Thomas Minder", Party: "parteilos"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("namedAuthors = %v, want %v", got, want)
	}
}

func TestPartyLabel(t *testing.T) {
	for _, c := range []struct {
		lang     votes.Language
		in, want string
	}{
		{votes.German, "M-E", "Mitte"}, {votes.German, "FDP-Liberale", "FDP"}, {votes.German, "GRÜNE", "Grüne"},
		{votes.German, "glp", "GLP"}, {votes.German, "SVP", "SVP"}, {votes.German, "", ""},
		// Each feed spells the parties its own way; the labels match the
		// group table's for that language.
		{votes.French, "PSS", "PS"}, {votes.French, "VERT-E-S", "Vert-e-s"}, {votes.French, "UDC", "UDC"},
		{votes.Italian, "VERDI", "Verdi"}, {votes.Italian, "M-E", "Centro"}, {votes.Italian, "pvl", "PVL"},
	} {
		if got := partyLabel(c.lang, c.in); got != c.want {
			t.Errorf("partyLabel(%s, %q) = %q, want %q", c.lang, c.in, got, c.want)
		}
	}
}

// A Fraktion filing is a role-7 row with a group and no member. Only the
// current legislature's groups are named.
func TestFilingFraktion(t *testing.T) {
	n := func(v int64) *int64 { return &v }
	tests := []struct {
		name  string
		roles []businessRoleDTO
		want  string
	}{
		{"the GLP-Fraktion, 13.468", []businessRoleDTO{{Role: roleAuthor, ParlGroupNumber: n(137)}}, "GLP-Fraktion"},
		{"the Mitte-Fraktion names both its parties", []businessRoleDTO{{Role: roleAuthor, ParlGroupNumber: n(3)}}, "Mitte-EVP-Fraktion"},
		{"a member filed it", []businessRoleDTO{{Role: roleAuthor, MemberCouncilNumber: n(214)}}, ""},
		{"a group in another role", []businessRoleDTO{{Role: roleCoSigned, ParlGroupNumber: n(4)}}, ""},
		{"a dissolved group", []businessRoleDTO{{Role: roleAuthor, ParlGroupNumber: n(136)}}, ""},
		{"nobody", nil, ""},
	}
	for _, tt := range tests {
		if got := filingFraktion(votes.German, tt.roles); got != tt.want {
			t.Errorf("%s: got %q, want %q", tt.name, got, tt.want)
		}
	}
	// The other languages name the group in their own words.
	glp := []businessRoleDTO{{Role: roleAuthor, ParlGroupNumber: n(137)}}
	if got := filingFraktion(votes.French, glp); got != "groupe PVL" {
		t.Errorf("French: got %q, want groupe PVL", got)
	}
	if got := filingFraktion(votes.Italian, glp); got != "gruppo PVL" {
		t.Errorf("Italian: got %q, want gruppo PVL", got)
	}
}

// Every language names the same Fraktionen, so a business filed by one reads
// the same in every language's posts.
func TestFraktionNames_EveryLanguageNamesEveryGroup(t *testing.T) {
	for l, names := range fraktionNames {
		for n := range fraktionNames[votes.German] {
			if names[n] == "" {
				t.Errorf("%s has no name for group %d", l, n)
			}
		}
	}
}

// 13.468, "Ehe für alle", was filed by the GLP-Fraktion: the service's one
// row names the group and no member. Recorded from the service in November
// 2026. The Fraktion reaches the vote, nobody is looked up, and the second
// vote of the business is served from the cache.
func TestEnrich_FraktionFiling(t *testing.T) {
	c, requests := newTestClient(t)
	inner := c.baseURL
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/BusinessRole" && strings.Contains(r.URL.Query().Get("$filter"), "BusinessNumber eq 20130468 ") {
			*requests = append(*requests, r.URL.RawQuery)
			http.ServeFile(w, r, "testdata/roles_20130468.json")
			return
		}
		http.Redirect(w, r, inner+r.URL.RequestURI(), http.StatusTemporaryRedirect)
	}))
	t.Cleanup(srv.Close)
	c.SetBaseURL(srv.URL)

	for i, id := range []string{"38953", "38954"} {
		before := len(*requests)
		v := votes.Vote{SourceID: id, Affair: votes.Affair{ID: "20130468"}}
		if err := c.enrich(&v); err != nil {
			t.Fatal(err)
		}
		if v.Affair.Fraktion != "GLP-Fraktion" || len(v.Affair.Authors) != 0 {
			t.Errorf("vote %s: Fraktion %q, Authors %v; want GLP-Fraktion and nobody", id, v.Affair.Fraktion, v.Affair.Authors)
		}
		var roles, members int
		for _, q := range (*requests)[before:] {
			if strings.Contains(q, "Role") {
				roles++
			}
			if strings.Contains(q, "ID+eq") || strings.Contains(q, "ID%20eq") {
				members++
			}
		}
		if members != 0 {
			t.Errorf("vote %s: %d member lookups, want none", id, members)
		}
		if want := 1 - i; roles != want {
			t.Errorf("vote %s: %d BusinessRole requests, want %d", id, roles, want)
		}
	}
}
