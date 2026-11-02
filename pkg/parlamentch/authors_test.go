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
	got := namedAuthors([]heldRole{{1, votes.RoleSigned}, {2, votes.RoleSigned}, {3, votes.RoleSigned}, {4, votes.RoleSigned}}, []authorDTO{
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
	for in, want := range map[string]string{
		"M-E": "Mitte", "FDP-Liberale": "FDP", "GRÜNE": "Grüne", "glp": "GLP", "SVP": "SVP", "": "",
	} {
		if got := partyLabel(in); got != want {
			t.Errorf("partyLabel(%q) = %q, want %q", in, got, want)
		}
	}
}
