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
	want := []votes.Author{{Name: "Luzi Stamm", Party: "SVP"}, {Name: "Bruno Walliser", Party: "SVP"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("authors = %v, want %v", got, want)
	}

	// Co-signers (role 3) are left out on the server, not after the fetch.
	var roleQuery string
	for _, q := range *requests {
		if strings.Contains(q, "BusinessNumber") {
			roleQuery = q
		}
	}
	if !strings.Contains(roleQuery, "Role+eq+7+or+Role+eq+4") && !strings.Contains(roleQuery, "Role%20eq%207%20or%20Role%20eq%204") {
		t.Errorf("role query %q does not restrict to roles 7 and 4", roleQuery)
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
		{Role: roleTakenOver, MemberCouncilNumber: id(3912)},
		{Role: roleAuthor, MemberCouncilNumber: nil}, // a committee
		{Role: roleAuthor, MemberCouncilNumber: id(1122)},
		{Role: roleTakenOver, MemberCouncilNumber: id(1122)},
	}
	if got, want := authorOrder(roles), []int64{1122, 3912}; !reflect.DeepEqual(got, want) {
		t.Errorf("authorOrder = %v, want %v", got, want)
	}
}

func TestNamedAuthors_DropsWhoCannotBeNamed(t *testing.T) {
	got := namedAuthors([]int64{1, 2, 3, 4}, []authorDTO{
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
