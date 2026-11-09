package config

import (
	"fmt"
	"os"
	"time"
)

// Channel is one set of platform credentials — in practice one account per
// platform — serving one or more jurisdictions.
//
// The distinction between a channel and a jurisdiction is what makes the post
// budget correct. Rate limits and audience fatigue are properties of an
// account, so the per-run allowance belongs to the channel; dedup and contacts
// are properties of a body, so they belong to the jurisdiction. Conflating the
// two is how two jurisdictions end up each spending a full allowance on the
// same account.
type Channel struct {
	// Key names the channel and prefixes its environment variables.
	Key string

	// Jurisdictions are posted to this channel's accounts, in this order.
	// Order breaks ties when several are equally old.
	Jurisdictions []string
}

// channels is the configured set.
var channels = []Channel{
	{
		Key: "zurich",
		// "Zürich Ratsinfo" is geographically accurate for both bodies and the
		// audiences overlap almost entirely, so the canton posts to the
		// existing accounts rather than needing a new credential set and a
		// second X Premium subscription.
		Jurisdictions: []string{"zurich-city", ZurichCantonKey},
	},
	{
		Key: "chparl",
		// Bundesbern on an account named for Zürich would misstate whose
		// votes these are, so the Nationalrat has accounts of its own
		// (@chparlinfo) and its own workflow, bot-chparl.yml.
		Jurisdictions: []string{NationalratKey},
	},
}

// Channels returns the configured channels in order.
func Channels() []Channel {
	out := make([]Channel, len(channels))
	copy(out, channels)
	return out
}

// DefaultChannelKey is the channel the local tools use unless told otherwise.
const DefaultChannelKey = "zurich"

// LookupChannel returns a configured channel by key. Local tools use it so they
// resolve credentials the same way the scheduled run does, rather than reading
// a second, unprefixed set of names.
func LookupChannel(key string) (Channel, error) {
	for _, c := range channels {
		if c.Key == key {
			return c, nil
		}
	}
	return Channel{}, fmt.Errorf("unknown channel %q", key)
}

// ChannelKeys lists every configured channel key, in order.
func ChannelKeys() []string {
	keys := make([]string, 0, len(channels))
	for _, c := range channels {
		keys = append(keys, c.Key)
	}
	return keys
}

// Env reads a channel-scoped environment variable: Env("X_API_KEY") on the
// "zurich" channel reads ZURICH_X_API_KEY.
//
// Every name is prefixed, with no fallback to an unprefixed one. A channel
// missing a secret must come up empty rather than silently borrowing another
// channel's credentials and posting to the wrong account.
func (c Channel) Env(name string) string {
	return os.Getenv(c.EnvPrefix() + name)
}

// EnvPrefix is the prefix Env puts in front of every name, "ZURICH_" for the
// "zurich" channel. Tools use it to name the variable they wanted in an error.
func (c Channel) EnvPrefix() string {
	return envKey(c.Key) + "_"
}

// EnvInt is Env for integer settings, returning fallback when unset or unparseable.
func (c Channel) EnvInt(name string, fallback int) int {
	if v, ok := envInt(envKey(c.Key) + "_" + name); ok {
		return v
	}
	return fallback
}

// ResolveJurisdictions looks up every jurisdiction on this channel, in order.
func (c Channel) ResolveJurisdictions() ([]Jurisdiction, error) {
	out := make([]Jurisdiction, 0, len(c.Jurisdictions))
	for _, key := range c.Jurisdictions {
		j, err := LookupJurisdiction(key)
		if err != nil {
			return nil, fmt.Errorf("channel %q: %w", c.Key, err)
		}
		out = append(out, j)
	}
	return out, nil
}

// EnabledJurisdictions is ResolveJurisdictions filtered to those cleared to
// post. The scheduled run uses this; dry-run tools use ResolveJurisdictions,
// because previewing a body is what you do before enabling it.
func (c Channel) EnabledJurisdictions() ([]Jurisdiction, error) {
	all, err := c.ResolveJurisdictions()
	if err != nil {
		return nil, err
	}
	out := make([]Jurisdiction, 0, len(all))
	for _, j := range all {
		if j.Enabled {
			out = append(out, j)
		}
	}
	return out, nil
}

// Validate checks the configuration is internally consistent. It runs at
// startup so a misconfiguration fails loudly rather than posting somewhere
// unexpected.
func Validate() error {
	seenChannel := make(map[string]bool)
	seenJurisdiction := make(map[string]string)

	for _, c := range channels {
		if seenChannel[c.Key] {
			return fmt.Errorf("duplicate channel %q", c.Key)
		}
		seenChannel[c.Key] = true

		if len(c.Jurisdictions) == 0 {
			return fmt.Errorf("channel %q serves no jurisdictions", c.Key)
		}

		for _, key := range c.Jurisdictions {
			if other, dup := seenJurisdiction[key]; dup {
				// Two channels posting the same jurisdiction would share one
				// vote log, so whichever ran first would suppress the other.
				return fmt.Errorf("jurisdiction %q is served by both channel %q and channel %q", key, other, c.Key)
			}
			seenJurisdiction[key] = c.Key
			j, err := LookupJurisdiction(key)
			if err != nil {
				return fmt.Errorf("channel %q: %w", c.Key, err)
			}
			if _, err := j.Source(); err != nil {
				return fmt.Errorf("channel %q: %w", c.Key, err)
			}
		}
	}
	return nil
}

// ParseDate reads a calendar date written YYYY-MM-DD, as used by settings like
// FB_POST_FROM. The result is midnight UTC of that date, meant to be compared by
// calendar day. An empty string is an error: a setting that must be set should
// not read as "no limit".
func ParseDate(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, fmt.Errorf("not set")
	}
	d, err := time.Parse("2006-01-02", s)
	if err != nil {
		return time.Time{}, fmt.Errorf("%q is not a YYYY-MM-DD date", s)
	}
	return d, nil
}
