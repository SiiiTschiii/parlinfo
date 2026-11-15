// linkedin_digest prints a draft of the weekly digest for a channel's LinkedIn
// Page: the votes the bot posted in the last days, one line each, ready to
// paste. It posts nothing.
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
	"time"

	"github.com/siiitschiii/parlinfo/pkg/config"
	"github.com/siiitschiii/parlinfo/pkg/contacts"
	"github.com/siiitschiii/parlinfo/pkg/votelog"
	"github.com/siiitschiii/parlinfo/pkg/votes"
)

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
	days := flag.Int("days", 7, "how many days back to look for posted votes")
	fetchLimit := flag.Int("fetch", 200, "number of recent votes to fetch from each source")
	// The logs live on a state branch, not in main, so a checkout has none
	// unless they were copied in. Reading them from the ref saves the copying
	// and cannot be stale.
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

	until := time.Now()
	since := until.AddDate(0, 0, -*days)

	var sections []section
	for _, j := range jurisdictions {
		logs, err := loadLogs(j.Key, *stateRef)
		if err != nil {
			log.Fatalf("%s: %v", j.Key, err)
		}
		ids := postedSince(logs, since)
		if len(ids) == 0 {
			continue
		}

		source, err := j.Source()
		if err != nil {
			log.Fatal(err)
		}
		groups, err := fetchPosted(source, ids, *fetchLimit)
		if err != nil {
			log.Fatalf("%s: %v", j.Key, err)
		}
		if lost := missing(ids, groups); len(lost) > 0 {
			log.Printf("%s: %d posted vote(s) not among the latest %d from the source, left out; raise -fetch to include them: %v",
				j.Key, len(lost), *fetchLimit, lost)
		}
		sections = append(sections, section{Name: j.Name, Groups: groups})
	}

	mapper, err := contacts.LoadContactFiles(contacts.PathsFor(config.ContactsKeys(jurisdictions)...)...)
	if err != nil {
		log.Printf("Warning: could not load contacts, the tagging list is empty: %v", err)
		mapper = nil
	}

	draft := render(sections, since, until, mapper)
	if draft == "" {
		log.Fatalf("No votes posted for channel %q in the last %d days. Are the vote logs on disk? See -state-ref.", channel.Key, *days)
	}
	fmt.Print(draft)
}

// fetchPosted fetches a source's recent votes and groups the ones in ids.
func fetchPosted(source votes.Source, ids map[string]bool, limit int) ([][]votes.Vote, error) {
	fetched, err := source.FetchRecent(limit)
	if err != nil {
		return nil, err
	}
	var posted []votes.Vote
	for _, v := range fetched {
		if ids[v.SourceID] {
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
	return keepPosted(groups, ids), nil
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
