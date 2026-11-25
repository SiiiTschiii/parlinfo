package parlamentch

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFetchMemberDetails_JoinsEmailsToMembers(t *testing.T) {
	var filters []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		filters = append(filters, r.URL.Query().Get("$filter"))
		switch r.URL.Path {
		case "/MemberCouncil":
			_, _ = w.Write([]byte(`{"d":{"results":[
				{"PersonNumber":2,"FirstName":"Zoé","LastName":"Zweite","GenderAsString":"f","CantonAbbreviation":"VD","PartyName":"GRÜNE Schweiz","DateOfBirth":"/Date(-228873600000)/"},
				{"PersonNumber":1,"FirstName":"Adam","LastName":"Erster","GenderAsString":"m","CantonAbbreviation":"ZH","PartyName":"SVP","DateOfBirth":null}
			]}}`))
		case "/PersonCommunication":
			// The ordered-query shape, {"d": [...]}, which get also reads.
			_, _ = w.Write([]byte(`{"d":[
				{"PersonNumber":2,"Address":"zoe@parl.ch"},
				{"PersonNumber":2,"Address":" zoe@example.org "},
				{"PersonNumber":99,"Address":"former@parl.ch"}
			]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := New(testJurisdiction)
	c.SetBaseURL(srv.URL)
	c.retryDelay = 0

	got, err := c.FetchMemberDetails()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Name() != "Adam Erster" || got[1].Name() != "Zoé Zweite" {
		t.Fatalf("members = %+v, want both, sorted by name", got)
	}
	if len(got[0].Emails) != 0 || !got[0].BirthDate.IsZero() {
		t.Errorf("Adam = %+v, want no email and no birth date", got[0])
	}
	zoe := got[1]
	if strings.Join(zoe.Emails, ",") != "zoe@parl.ch,zoe@example.org" {
		t.Errorf("Zoé's emails = %q", zoe.Emails)
	}
	if zoe.Gender != "f" || zoe.Canton != "VD" || zoe.BirthDate.Year() != 1962 {
		t.Errorf("Zoé = %+v", zoe)
	}
	if !strings.Contains(filters[0], "Active eq true and Council eq 1") ||
		!strings.Contains(filters[1], "CommunicationType eq 3") {
		t.Errorf("filters = %q", filters)
	}
}

func TestSpeechLanguages_CountsByLanguage(t *testing.T) {
	var filter string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		filter = r.URL.Query().Get("$filter")
		_, _ = w.Write([]byte(`{"d":[{"LanguageOfText":"IT"},{"LanguageOfText":"de"},{"LanguageOfText":"IT"},{"LanguageOfText":""}]}`))
	}))
	defer srv.Close()

	c := New(testJurisdiction)
	c.SetBaseURL(srv.URL)

	got, err := c.SpeechLanguages(4222, 20)
	if err != nil {
		t.Fatal(err)
	}
	if got["IT"] != 2 || got["DE"] != 1 || len(got) != 2 {
		t.Errorf("counts = %v", got)
	}
	if !strings.Contains(filter, "PersonNumber eq 4222 and Type eq 1") {
		t.Errorf("filter = %q", filter)
	}
}
