package config

import (
	"net/url"
	"testing"
	"time"

	"github.com/siiitschiii/parlinfo/pkg/votes"
)

func TestValidate(t *testing.T) {
	if err := Validate(); err != nil {
		t.Errorf("the shipped configuration must be valid: %v", err)
	}
}

// update_contacts -prune asks every other jurisdiction who sits there, to tell
// a member who moved chambers from one who left. A body that cannot answer
// stops it for every other body, so each one must be able to.
func TestEveryJurisdictionCanSayWhoSits(t *testing.T) {
	for _, key := range JurisdictionKeys() {
		j, err := LookupJurisdiction(key)
		if err != nil {
			t.Fatal(err)
		}
		if j.NewMemberSource == nil {
			t.Errorf("%s has no roster", key)
			continue
		}
		if _, ok := j.NewMemberSource().(votes.SittingLister); !ok {
			t.Errorf("%s's roster cannot say who sits", key)
		}
	}
}

func TestEnvKey(t *testing.T) {
	tests := map[string]string{
		"zurich":        "ZURICH",
		"zurich-city":   "ZURICH_CITY",
		"zurich-canton": "ZURICH_CANTON",
	}
	for in, want := range tests {
		if got := envKey(in); got != want {
			t.Errorf("envKey(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestChannelEnv(t *testing.T) {
	zurich := Channel{Key: "zurich"}

	t.Setenv("ZURICH_X_API_KEY", "prefixed")
	if got := zurich.Env("X_API_KEY"); got != "prefixed" {
		t.Errorf("Env = %q, want the channel-scoped value", got)
	}

	// An unprefixed variable belongs to no channel. Honouring it would let a
	// channel with a missing secret post to another channel's account.
	t.Setenv("ZURICH_X_API_KEY", "")
	t.Setenv("X_API_KEY", "unscoped")
	if got := zurich.Env("X_API_KEY"); got != "" {
		t.Errorf("Env = %q, want empty — unprefixed names are not channel credentials", got)
	}
}

func TestChannelEnvInt(t *testing.T) {
	c := Channel{Key: "zurich"}

	if got := c.EnvInt("X_MAX_POSTS_PER_RUN", 10); got != 10 {
		t.Errorf("unset should give the fallback, got %d", got)
	}

	t.Setenv("ZURICH_X_MAX_POSTS_PER_RUN", "2")
	if got := c.EnvInt("X_MAX_POSTS_PER_RUN", 10); got != 2 {
		t.Errorf("channel-scoped name should be used, got %d", got)
	}

	t.Setenv("ZURICH_X_MAX_POSTS_PER_RUN", "not a number")
	if got := c.EnvInt("X_MAX_POSTS_PER_RUN", 10); got != 10 {
		t.Errorf("an unparseable value should fall back, got %d", got)
	}
}

func TestMaxAgeDaysOverrides(t *testing.T) {
	j, err := LookupJurisdiction("zurich-city")
	if err != nil {
		t.Fatal(err)
	}
	if j.MaxAgeDays != 90 {
		t.Errorf("default city age guard = %d, want 90", j.MaxAgeDays)
	}

	t.Setenv("MAX_VOTE_AGE_DAYS_ZURICH_CITY", "7")
	if j, _ := LookupJurisdiction("zurich-city"); j.MaxAgeDays != 7 {
		t.Errorf("per-jurisdiction override ignored, got %d", j.MaxAgeDays)
	}
}

func TestLookupJurisdiction_Unknown(t *testing.T) {
	if _, err := LookupJurisdiction("no-such-body"); err == nil {
		t.Error("expected an error for an unregistered jurisdiction")
	}
}

// Every body a channel posts has a source and an age guard. A body no channel
// serves may have neither: Validate refuses it a channel until it has a source.
func TestEveryPostedJurisdictionHasASource(t *testing.T) {
	for _, c := range Channels() {
		for _, key := range c.Jurisdictions {
			j, err := LookupJurisdiction(key)
			if err != nil {
				t.Fatalf("%s: %v", key, err)
			}
			if j.NewSource == nil {
				t.Errorf("%s has no source constructor", key)
			}
			// An unbounded age guard on a jurisdiction whose log may be empty
			// is how a first run posts years of history at once.
			if j.MaxAgeDays <= 0 {
				t.Errorf("%s has no age guard", key)
			}
		}
	}
}

// Nothing posts unless its switch is explicitly on.
//
// The two mistakes are not symmetric: a body that should post and does not is a
// visible gap, while a body that should not post and does has already published
// to a public account. So an absent or unreadable variable has to mean off.
func TestNoJurisdictionPostsByDefault(t *testing.T) {
	for _, key := range JurisdictionKeys() {
		j, err := LookupJurisdiction(key)
		if err != nil {
			t.Fatal(err)
		}
		if j.Enabled {
			t.Errorf("%s is enabled without its variable being set", key)
		}
	}

	// An unset variable arrives from the workflow as an empty string, and a
	// misspelled value is not a licence to publish.
	for _, raw := range []string{"", "yes please", "TRUE-ish", "0.5"} {
		t.Setenv("JURISDICTION_ZURICH_CITY_ENABLED", raw)
		if j, _ := LookupJurisdiction("zurich-city"); j.Enabled {
			t.Errorf("%q enabled the city; only an explicit true should", raw)
		}
	}

	// Values Go's parser accepts as true all work, so the variable can be
	// written the way each platform spells it.
	for _, raw := range []string{"true", "TRUE", "True", "1", "t"} {
		t.Setenv("JURISDICTION_ZURICH_CITY_ENABLED", raw)
		if j, _ := LookupJurisdiction("zurich-city"); !j.Enabled {
			t.Errorf("%q should have enabled the city", raw)
		}
	}
}

// Each body is switched independently — that is what makes it usable to pause
// one chamber whose source has published something wrong.
func TestJurisdictionsSwitchIndependently(t *testing.T) {
	t.Setenv("JURISDICTION_ZURICH_CITY_ENABLED", "true")
	t.Setenv("JURISDICTION_ZURICH_CANTON_ENABLED", "false")

	if j, _ := LookupJurisdiction("zurich-city"); !j.Enabled {
		t.Error("the city should be enabled")
	}
	if j, _ := LookupJurisdiction(ZurichCantonKey); j.Enabled {
		t.Error("the canton should be disabled")
	}

	t.Setenv("JURISDICTION_ZURICH_CITY_ENABLED", "false")
	t.Setenv("JURISDICTION_ZURICH_CANTON_ENABLED", "true")

	if j, _ := LookupJurisdiction("zurich-city"); j.Enabled {
		t.Error("the city should be pausable on its own")
	}
	if j, _ := LookupJurisdiction(ZurichCantonKey); !j.Enabled {
		t.Error("pausing the city must not affect the canton")
	}
}

func TestCantonLinks(t *testing.T) {
	t.Run("SessionURL narrows the sitting list to the day", func(t *testing.T) {
		// 16:39 local: the sitting is the calendar day the vote was taken on,
		// not a UTC day that a late-afternoon vote would fall off the end of.
		date := time.Date(2026, 8, 31, 16, 39, 38, 0, time.FixedZone("CEST", 2*60*60))
		got := cantonLinks{}.SessionURL(date)

		u, err := url.Parse(got)
		if err != nil {
			t.Fatalf("SessionURL() = %q, which does not parse: %v", got, err)
		}
		if u.Host != "www.kantonsrat.zh.ch" || u.Path != "/ratsbetrieb/sitzungenundprotokolle/" {
			t.Errorf("SessionURL() = %q, want the canton's own sitting list", got)
		}
		if s, e := u.Query().Get("startDate"), u.Query().Get("endDate"); s != "2026-08-31" || e != "2026-08-31" {
			t.Errorf("SessionURL() range = %s..%s, want both 2026-08-31", s, e)
		}
	})

	t.Run("ItemURL widens a vote link to its agenda item", func(t *testing.T) {
		got := cantonLinks{}.ItemURL("https://zh.recapp.ch/shareparl?agendaItemUid=item&segmentUid=seg")
		if got != "https://zh.recapp.ch/shareparl?agendaItemUid=item" {
			t.Errorf("ItemURL() = %q", got)
		}
	})
}

func TestParseDate(t *testing.T) {
	got, err := ParseDate("2026-10-17")
	if err != nil {
		t.Fatalf("ParseDate: %v", err)
	}
	if want := time.Date(2026, 10, 17, 0, 0, 0, 0, time.UTC); !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}

	// A start date that reads as "no limit" when missing or mistyped would
	// silently re-open the backlog it exists to close.
	for _, bad := range []string{"", "17.10.2026", "2026-13-01", "tomorrow"} {
		if _, err := ParseDate(bad); err == nil {
			t.Errorf("ParseDate(%q) succeeded, want an error", bad)
		}
	}
}

// A body without a vote source is a contacts file, not something to post:
// asking it for votes is an error, never a nil function call.
func TestSourceOfABodyWithoutVotes(t *testing.T) {
	j, err := LookupJurisdiction(StaenderatKey)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := j.Source(); err == nil {
		t.Error("the Ständerat has no vote source, want an error")
	}
}
