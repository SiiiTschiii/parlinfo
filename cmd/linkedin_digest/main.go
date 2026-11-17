// linkedin_digest prints a draft of the weekly digest for a channel's LinkedIn
// Page: the votes the bot posted in the last days, one line each, ready to
// paste, and writes the card to attach for each. It posts nothing.
//
//	go run ./cmd/linkedin_digest -state-ref origin/state-log
//	go run ./cmd/linkedin_digest -channel chparl -days 14 -state-ref origin/state-log-chparl
//
// The vote logs say which votes were posted and when, but not what they were
// about, so the votes are fetched again from the source and matched by ID.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/siiitschiii/parlinfo/pkg/config"
	"github.com/siiitschiii/parlinfo/pkg/contacts"
	"github.com/siiitschiii/parlinfo/pkg/votelog"
	"github.com/siiitschiii/parlinfo/pkg/votes"
)

// sites are the parlinfo.ch pages listing each channel's accounts, which a
// digest's first comment points to for every vote.
var sites = map[string]string{
	"zurich": "https://parlinfo.ch/de/zh/",
	"chparl": "https://parlinfo.ch/de/ch/",
}

// logPlatforms are the platforms whose logs are read. A jurisdiction with no
// log for one of them contributes nothing for it.
var logPlatforms = []votelog.Platform{
	votelog.PlatformX,
	votelog.PlatformBluesky,
	votelog.PlatformInstagram,
	votelog.PlatformFacebook,
	votelog.PlatformMastodon,
}

func main() {
	channelKey := flag.String("channel", config.DefaultChannelKey, "channel whose votes to digest")
	days := flag.Int("days", 7, "how many sitting days back to cover, counting -until")
	// A digest for an earlier week, to catch up or to post several at once.
	untilFlag := flag.String("until", "", "last sitting date covered, YYYY-MM-DD (default: today)")
	fetchLimit := flag.Int("fetch", 200, "number of recent votes to fetch from each source")
	// The logs live on a state branch, not in main, so a checkout has none
	// unless they were copied in. Reading them from the ref saves the copying
	// and cannot be stale.
	all := flag.Bool("all", false, "list every business of a sitting, not only its rejections and closest votes")
	cardsDir := flag.String("cards", "out/linkedin", "directory for each entry's first card, to attach to its post (empty: no cards)")
	openCards := flag.Bool("open", runtime.GOOS == "darwin", "open the cards directory in Finder once written (macOS)")
	stateRef := flag.String("state-ref", "", "git ref to read the vote logs from, e.g. origin/state-log (default: the logs on disk)")
	flag.Parse()

	if *days < 1 {
		log.Fatalf("-days must be at least 1, got %d", *days)
	}

	channel, err := config.LookupChannel(*channelKey)
	if err != nil {
		log.Fatalf("%v. Available: %v", err, config.ChannelKeys())
	}
	jurisdictions, err := channel.ResolveJurisdictions()
	if err != nil {
		log.Fatal(err)
	}

	last := time.Now()
	if *untilFlag != "" {
		if last, err = config.ParseDate(*untilFlag); err != nil {
			log.Fatalf("-until: %v", err)
		}
	}
	w := newWindow(last, *days)

	var sections []section
	for _, j := range jurisdictions {
		logs, err := loadLogs(j.Key, *stateRef)
		if err != nil {
			log.Fatalf("%s: %v", j.Key, err)
		}
		ids := postedIDs(logs)
		if len(ids) == 0 {
			continue
		}

		source, err := j.Source()
		if err != nil {
			log.Fatal(err)
		}
		groups, err := fetchPosted(source, j.Key, ids, w, *fetchLimit)
		if err != nil {
			log.Fatalf("%s: %v", j.Key, err)
		}
		sections = append(sections, section{Name: j.Name, Groups: groups})
	}

	mapper, err := contacts.LoadContactFiles(contacts.PathsFor(config.ContactsKeys(jurisdictions)...)...)
	if err != nil {
		log.Printf("Warning: could not load contacts, the tagging list is empty: %v", err)
		mapper = nil
	}

	draft, shortened := render(sections, mapper, options{Site: sites[channel.Key], All: *all})
	if shortened > 0 {
		log.Printf("%d title(s) cut to fit LinkedIn's %d characters; the links in the first comment have them in full", shortened, maxPostChars)
	}
	if draft == "" {
		log.Fatalf("No posted votes for channel %q from sittings %s to %s. Are the vote logs on disk? See -state-ref.", channel.Key, w.From, w.To)
	}
	fmt.Print(draft)

	if *cardsDir != "" {
		written, err := writeCards(*cardsDir, sections, *all)
		if err != nil {
			log.Fatalf("writing cards: %v", err)
		}
		log.Printf("Wrote %d card file(s): attach <date>-<body>-<n>.jpg in the order of n, with the alt text from <date>-<body>-alt.txt", len(written))
		dir, err := filepath.Abs(*cardsDir)
		if err != nil {
			dir = *cardsDir
		}
		log.Printf("Cards: %s", dir)
		// The cards are attached from a file picker, and not every terminal
		// makes a path clickable, so the folder is opened for them.
		if *openCards {
			if err := exec.Command("open", dir).Run(); err != nil {
				log.Printf("Could not open %s: %v", dir, err)
			}
		}
	}
}

// fetchPosted fetches a source's recent votes and groups the ones in ids that
// were voted in the window.
func fetchPosted(source votes.Source, key string, ids map[string]bool, w window, limit int) ([][]votes.Vote, error) {
	fetched, err := source.FetchRecent(limit)
	if err != nil {
		return nil, err
	}
	if !reachesBack(fetched, w) {
		log.Printf("%s: the latest %d votes from the source do not reach back to %s, so the window's first days may be missing; raise -fetch",
			key, limit, w.From)
	}
	accept := func(v votes.Vote) bool { return ids[v.SourceID] && w.holds(v) }
	var posted []votes.Vote
	for _, v := range fetched {
		if accept(v) {
			posted = append(posted, v)
		}
	}
	if len(posted) == 0 {
		return nil, nil
	}
	groups, err := source.GroupByAffair(posted)
	if err != nil {
		return nil, err
	}
	return keep(groups, accept), nil
}

// loadLogs reads a jurisdiction's vote log for every platform, from ref when
// one is given and from disk otherwise.
func loadLogs(jurisdiction, ref string) ([]*votelog.VoteLog, error) {
	var logs []*votelog.VoteLog
	for _, p := range logPlatforms {
		if ref == "" {
			l, err := votelog.Load(jurisdiction, p)
			if err != nil {
				return nil, err
			}
			logs = append(logs, l)
			continue
		}

		path := votelog.LogFilePath(jurisdiction, p)
		// cat-file -e tells a file the ref does not hold, which is expected
		// for a platform the jurisdiction does not post to, from a ref that
		// does not exist, which git show reports below.
		if exec.Command("git", "cat-file", "-e", ref+":"+path).Run() != nil {
			if err := exec.Command("git", "rev-parse", "--verify", "--quiet", ref+"^{commit}").Run(); err != nil {
				return nil, fmt.Errorf("git ref %q not found; fetch it first (git fetch origin)", ref)
			}
			continue
		}
		data, err := exec.Command("git", "show", ref+":"+path).Output()
		if err != nil {
			return nil, fmt.Errorf("reading %s from %s: %w", path, ref, err)
		}
		l := &votelog.VoteLog{}
		if err := json.Unmarshal(data, l); err != nil {
			return nil, fmt.Errorf("parsing %s from %s: %w", path, ref, err)
		}
		logs = append(logs, l)
	}
	return logs, nil
}
