// Command generate_search_urls prints per-politician search links for the
// manual part of curating contacts.yaml: finding and verifying handles.
//
//	go run ./cmd/generate_search_urls                              # zurich-city
//	go run ./cmd/generate_search_urls -jurisdiction zurich-canton
//	go run ./cmd/generate_search_urls -jurisdiction zurich-canton -platform instagram
//
// Party, Fraktion and the member's page on the parliament's own site come from
// the body's live roster rather than from contacts.yaml, which stores none of
// them. They are what turns "Anna Müller" into a search a human can actually
// settle: the mapping is worth having only if every handle in it belongs to the
// person the post will name.
package main

import (
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"

	"github.com/siiitschiii/parlinfo/pkg/config"
	"github.com/siiitschiii/parlinfo/pkg/contacts"
	"github.com/siiitschiii/parlinfo/pkg/votes"
	"gopkg.in/yaml.v3"
)

type (
	Contact      = contacts.Contact
	ContactsFile = contacts.ContactMapping
)

// platform names a search this tool can emit, in the order it prints them.
type platform struct {
	name string
	// searchURL builds the search link, or returns "" for a platform that has
	// no linkable search.
	searchURL func(query string) string
	// key is the platform key in the contacts schema.
	key string
}

var platforms = []platform{
	{
		name:      "X/Twitter",
		key:       "x",
		searchURL: func(q string) string { return "https://x.com/search?q=" + q + "&src=typed_query&f=user" },
	},
	{
		// Instagram refuses search links opened from another site.
		name:      "Instagram",
		key:       "instagram",
		searchURL: func(string) string { return "" },
	},
	{
		name:      "Facebook",
		key:       "facebook",
		searchURL: func(q string) string { return "https://www.facebook.com/search/top?q=" + q },
	},
	{
		name:      "LinkedIn",
		key:       "linkedin",
		searchURL: func(q string) string { return "https://www.linkedin.com/search/results/all/?keywords=" + q },
	},
	{
		name:      "TikTok",
		key:       "tiktok",
		searchURL: func(q string) string { return "https://www.tiktok.com/search?q=" + q },
	},
	{
		name:      "Bluesky",
		key:       "bluesky",
		searchURL: func(q string) string { return "https://bsky.app/search?q=" + q },
	},
}

func main() {
	jurisdiction := flag.String("jurisdiction", "zurich-city",
		"jurisdiction to curate ("+strings.Join(config.JurisdictionKeys(), ", ")+")")
	only := flag.String("platform", "", "print searches for one platform only, e.g. instagram")
	flag.Parse()

	j, err := config.LookupJurisdiction(*jurisdiction)
	if err != nil {
		log.Fatalf("❌ %v", err)
	}

	wanted, err := selectPlatforms(*only)
	if err != nil {
		log.Fatalf("❌ %v", err)
	}

	path := contacts.PathFor(j.ContactsKey())
	data, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("❌ reading %s: %v", path, err)
	}

	var file ContactsFile
	if err := yaml.Unmarshal(data, &file); err != nil {
		log.Fatalf("❌ parsing %s: %v", path, err)
	}

	roster := fetchRoster(j)

	fmt.Printf("# Social media search URLs — %s\n\n", j.Name)
	fmt.Printf("Verify before adding: a handle that belongs to someone else puts a real\n")
	fmt.Printf("person's account next to a vote they did not cast.\n\n")
	fmt.Println("**Note:** Instagram does not accept search links from another site. Search the name in the app or on instagram.com.")
	fmt.Println()

	missing := 0
	for _, c := range file.Contacts {
		gaps := gapsFor(&c, wanted)
		if len(gaps) == 0 {
			continue
		}
		missing++
		printContact(c, roster[c.Name], gaps, j)
	}

	fmt.Printf("\n---\n")
	fmt.Printf("%d of %d contacts still missing an account on %s\n", missing, len(file.Contacts), platformNames(wanted))
	if len(roster) == 0 {
		fmt.Println("\n⚠️  No live roster: party and Fraktion are missing from the searches above.")
	}
}

// selectPlatforms narrows the search list to one platform, or returns them all.
func selectPlatforms(only string) ([]platform, error) {
	if only == "" {
		return platforms, nil
	}
	for _, p := range platforms {
		if strings.EqualFold(only, p.name) || strings.EqualFold(only, strings.SplitN(p.name, "/", 2)[0]) {
			return []platform{p}, nil
		}
	}
	return nil, fmt.Errorf("unknown platform %q, want one of %s", only, platformNames(platforms))
}

func platformNames(ps []platform) string {
	names := make([]string, len(ps))
	for i, p := range ps {
		names[i] = p.name
	}
	return strings.Join(names, ", ")
}

// gapsFor returns the platforms this contact has no *verified* account on yet.
//
// A platform holding only unverified candidates is still a gap: nothing is
// posted until someone confirms one, and confirming is exactly the work this
// tool exists to hand over.
func gapsFor(c *Contact, wanted []platform) []platform {
	var gaps []platform
	for _, p := range wanted {
		if len(c.Verified(p.key)) == 0 {
			gaps = append(gaps, p)
		}
	}
	return gaps
}

// fetchRoster returns the body's sitting members by name, or an empty map if
// the roster cannot be reached.
//
// A missing roster costs the affiliation lines, not the run: the search URLs
// themselves only need the name, and a curator halfway through a session should
// not be stopped by a third-party outage.
func fetchRoster(j config.Jurisdiction) map[string]votes.Member {
	if j.NewMemberSource == nil {
		return nil
	}

	members, err := j.NewMemberSource().FetchMembers()
	if err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  could not fetch the %s roster: %v\n", j.Name, err)
		return nil
	}

	byName := make(map[string]votes.Member, len(members))
	for _, m := range members {
		byName[m.Name] = m
	}
	return byName
}

func printContact(contact Contact, member votes.Member, gaps []platform, j config.Jurisdiction) {
	name := contact.Name
	fmt.Printf("## %s", name)
	if affiliation := affiliationOf(member); affiliation != "" {
		fmt.Printf(" — %s", affiliation)
	}
	fmt.Println()

	if member.ProfileURL != "" {
		// The parliament's own page, first: it is the one source that cannot be
		// the wrong person, and it often links the accounts outright.
		fmt.Printf("- Profil (%s): %s\n", j.ShortName, member.ProfileURL)
	}

	query := url.QueryEscape(name)
	for _, p := range gaps {
		// Candidates already harvested for this platform come first: checking
		// one of them is cheaper than searching again, and it is the step that
		// actually turns a lead into a taggable account.
		for _, a := range contact.Accounts(p.key) {
			conf := a.Confidence
			if conf == "" {
				conf = "unrated"
			}
			fmt.Printf("- %s candidate (%s, unconfirmed): %s\n", p.name, conf, a.URL)
		}
		if u := p.searchURL(query); u != "" {
			fmt.Printf("- %s: %s\n", p.name, u)
		} else {
			fmt.Printf("- %s: search %q in the app\n", p.name, name)
		}
	}

	// The catch-all, narrowed by whatever the roster knows: "Anna Müller" alone
	// finds the wrong Anna Müller.
	web := name + " " + j.ShortName
	if member.Party != "" {
		web += " " + member.Party
	}
	fmt.Printf("- Google: https://www.google.com/search?q=%s\n\n", url.QueryEscape(web))
}

// affiliationOf renders party and Fraktion when they differ, since sitting with
// a Fraktion is not the same as belonging to its party.
func affiliationOf(m votes.Member) string {
	switch {
	case m.Party == "" && m.Fraktion == "":
		return ""
	case m.Fraktion == "" || m.Fraktion == m.Party:
		return m.Party
	case m.Party == "":
		return "Fraktion " + m.Fraktion
	default:
		return m.Party + ", Fraktion " + m.Fraktion
	}
}
