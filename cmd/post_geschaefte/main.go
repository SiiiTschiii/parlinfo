// Command post_geschaefte announces newly filed Motionen and Postulate of the
// Gemeinderat Zürich on Bluesky, X and Instagram — at filing, before any vote.
//
// It runs after the vote bot on the same schedule and shares its channel
// credentials, its contacts for tagging and its log format, but keeps its own
// logs (data/zurich-city/posted_geschaefte_<platform>.json) and its own per-run
// budget, so announcements can never crowd out vote results.
//
// A platform whose log does not exist yet is seeded rather than posted to: every
// Vorstoss currently inside the age window is recorded as announced and nothing
// goes out. That makes the first run after deployment silent instead of a burst
// of weeks-old "new" Vorstösse, and makes a lost log fail quiet rather than loud
// — skipping a week of announcements is recoverable, repeating them is not.
//
//	go run ./cmd/post_geschaefte -dry-run                # preview what would post
//	go run ./cmd/post_geschaefte -fixtures -image-dir /tmp/cards  # preview the fixtures, write the cards
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/siiitschiii/parlinfo/pkg/config"
	"github.com/siiitschiii/parlinfo/pkg/contacts"
	"github.com/siiitschiii/parlinfo/pkg/geschaeftposting"
	gbluesky "github.com/siiitschiii/parlinfo/pkg/geschaeftposting/platforms/bluesky"
	ginstagram "github.com/siiitschiii/parlinfo/pkg/geschaeftposting/platforms/instagram"
	gx "github.com/siiitschiii/parlinfo/pkg/geschaeftposting/platforms/x"
	"github.com/siiitschiii/parlinfo/pkg/geschaeftposting/testfixtures"
	"github.com/siiitschiii/parlinfo/pkg/igapi"
	"github.com/siiitschiii/parlinfo/pkg/votelog"
	"github.com/siiitschiii/parlinfo/pkg/voteposting/platforms"
	"github.com/siiitschiii/parlinfo/pkg/voteposting/platforms/bluesky"
	"github.com/siiitschiii/parlinfo/pkg/voteposting/platforms/instagram"
	"github.com/siiitschiii/parlinfo/pkg/voteposting/platforms/x"
	"github.com/siiitschiii/parlinfo/pkg/zurichapi"
)

const (
	// defaultMaxAgeDays bounds what counts as newly filed. Vorstösse are filed
	// at the weekly Wednesday sitting, so two weeks rides out a skipped run or
	// a sitting-free week without reaching back to anything stale.
	defaultMaxAgeDays = 14
	// fetchLimit comfortably exceeds the ~25 Geschäfte of every type a sitting
	// produces, times two weeks.
	fetchLimit = 300
)

// target is one platform: how to render for it, how to post to it, and which
// log records what it has had.
type target struct {
	name     string
	log      votelog.Platform
	format   geschaeftposting.Formatter
	poster   geschaeftposting.Poster
	imageDir string // dry run only: where to write Instagram cards
}

func main() {
	dryRun := flag.Bool("dry-run", false, "print what would be posted; post nothing and record nothing")
	useFixtures := flag.Bool("fixtures", false, "format the test fixtures instead of fetching (implies -dry-run, ignores logs and age)")
	platformFlag := flag.String("platform", "all", "x, bluesky, instagram, or all")
	imageDir := flag.String("image-dir", "", "dry run only: write each Instagram card here as <GRNr>.jpg")
	flag.Parse()
	if *useFixtures {
		*dryRun = true
	}

	jurisdiction, err := config.LookupJurisdiction(zurichapi.JurisdictionKey)
	if err != nil {
		log.Fatal(err)
	}
	// The vote bot's per-body switch stops this too: a chamber paused because
	// its source is publishing something wrong is paused for every post kind.
	if !jurisdiction.Enabled && !*dryRun {
		log.Fatalf("Jurisdiction %s is not enabled (JURISDICTION_ZURICH_CITY_ENABLED); nothing to do.", jurisdiction.Key)
	}
	channel, err := channelFor(jurisdiction.Key)
	if err != nil {
		log.Fatal(err)
	}

	mapper, err := contacts.LoadContactFiles(contacts.PathFor(jurisdiction.Key))
	if err != nil {
		log.Printf("Warning: could not load contacts for tagging: %v", err)
		mapper = nil
	}

	targets := buildTargets(channel, mapper, *platformFlag, *dryRun, *imageDir)
	if len(targets) == 0 {
		log.Fatalf("No platform configured for channel %q (platform=%s).", channel.Key, *platformFlag)
	}

	var geschaefte []zurichapi.Geschaeft
	cutoff := time.Now().AddDate(0, 0, -getEnvInt("MAX_GESCHAEFT_AGE_DAYS", defaultMaxAgeDays))
	if *useFixtures {
		for _, g := range testfixtures.Supported() {
			geschaefte = append(geschaefte, g)
		}
		cutoff = time.Time{}
		// A preview of the fixtures is a preview of all of them: the live
		// budgets would show the same oldest few every time.
		for i := range targets {
			targets[i].poster = dryRunPoster{targets[i].poster.Name(), len(geschaefte)}
		}
	} else {
		geschaefte, err = zurichapi.NewClient().FetchGeschaefteSince(cutoff, fetchLimit)
		if err != nil {
			log.Fatalf("Error fetching Geschäfte: %v", err)
		}
		fmt.Printf("Fetched %d Geschäft(e) filed since %s\n", len(geschaefte), cutoff.Format("02.01.2006"))
	}

	failed := false
	for _, t := range targets {
		if err := run(t, jurisdiction.Key, geschaefte, cutoff, *dryRun, *useFixtures); err != nil {
			log.Printf("❌ %s: %v", t.name, err)
			failed = true
		}
	}
	if failed {
		os.Exit(1)
	}
}

// run announces the unposted Geschäfte on one platform.
func run(t target, jurisdiction string, gs []zurichapi.Geschaeft, cutoff time.Time, dryRun, ignoreLog bool) error {
	fmt.Printf("\n━━━ Geschäfte / %s ━━━\n", t.name)

	if ignoreLog {
		pending := geschaeftposting.Unposted(gs, votelog.NewNoOp(jurisdiction, t.log), cutoff)
		_, err := geschaeftposting.Post(pending, t.format, t.poster, nil, true)
		return err
	}

	if !votelog.Exists(votelog.KindGeschaefte, jurisdiction, t.log) {
		pending := geschaeftposting.Unposted(gs, votelog.NewEmpty(jurisdiction, t.log), cutoff)
		if dryRun {
			fmt.Printf("No Geschäft log for %s yet: a real run would seed it with %d Geschäft(e) and post nothing.\n", t.name, len(pending))
			return nil
		}
		gl, err := votelog.LoadKind(votelog.KindGeschaefte, jurisdiction, t.log)
		if err != nil {
			return err
		}
		if err := geschaeftposting.Seed(pending, gl); err != nil {
			return fmt.Errorf("seeding log: %w", err)
		}
		fmt.Printf("🌱 No Geschäft log for %s: seeded it with %d Geschäft(e), posted nothing.\n", t.name, len(pending))
		return nil
	}

	gl, err := votelog.LoadKind(votelog.KindGeschaefte, jurisdiction, t.log)
	if err != nil {
		return fmt.Errorf("loading log: %w", err)
	}
	pending := geschaeftposting.Unposted(gs, gl, cutoff)
	if len(pending) == 0 {
		fmt.Printf("No new Geschäfte for %s.\n", t.name)
		return nil
	}
	fmt.Printf("%d new Geschäft(e) for %s\n", len(pending), t.name)

	posted, err := geschaeftposting.Post(pending, t.format, t.poster, gl, dryRun)
	if err != nil {
		return fmt.Errorf("after %d posted: %w", posted, err)
	}
	if !dryRun {
		fmt.Printf("🎉 Announced %d Geschäft(e) on %s\n", posted, t.name)
	}
	return nil
}

// channelFor returns the channel whose accounts post a jurisdiction.
func channelFor(jurisdiction string) (config.Channel, error) {
	for _, c := range config.Channels() {
		if slices.Contains(c.Jurisdictions, jurisdiction) {
			return c, nil
		}
	}
	return config.Channel{}, fmt.Errorf("no channel posts jurisdiction %q", jurisdiction)
}

// buildTargets constructs each requested platform from the channel's
// credentials. Budgets are separate from the vote bot's
// (<CHANNEL>_GESCHAEFTE_<PLATFORM>_MAX_POSTS_PER_RUN) so the two can be tuned
// apart; a dry run needs no credentials and gets stand-in posters.
func buildTargets(channel config.Channel, mapper *contacts.Mapper, which string, dryRun bool, imageDir string) []target {
	want := func(p string) bool { return which == "all" || which == p }
	var targets []target

	if want("x") {
		budget := channel.EnvInt("GESCHAEFTE_X_MAX_POSTS_PER_RUN", 3)
		maxChars := channel.EnvInt("X_MAX_CHARS", x.DefaultMaxChars)
		t := target{name: "X/Twitter", log: votelog.PlatformX, format: gx.Formatter(mapper, maxChars)}
		key, secret, token, tokenSecret := channel.Env("X_API_KEY"), channel.Env("X_API_SECRET"), channel.Env("X_ACCESS_TOKEN"), channel.Env("X_ACCESS_SECRET")
		switch {
		case dryRun:
			t.poster = dryRunPoster{"X", budget}
		case key != "" && secret != "" && token != "" && tokenSecret != "":
			xp := x.NewXPlatform(key, secret, token, tokenSecret, mapper, budget)
			xp.SetMaxChars(maxChars)
			t.poster = xp
		default:
			log.Printf("⚠️  X/Twitter not configured for channel %q, skipping", channel.Key)
		}
		if t.poster != nil {
			targets = append(targets, t)
		}
	}

	if want("bluesky") {
		budget := channel.EnvInt("GESCHAEFTE_BLUESKY_MAX_POSTS_PER_RUN", 3)
		t := target{name: "Bluesky", log: votelog.PlatformBluesky, format: gbluesky.Formatter(mapper)}
		handle, password := channel.Env("BLUESKY_HANDLE"), channel.Env("BLUESKY_PASSWORD")
		switch {
		case dryRun:
			t.poster = dryRunPoster{"Bluesky", budget}
		case handle != "" && password != "":
			t.poster = bluesky.NewBlueskyPlatform(handle, password, budget, mapper)
		default:
			log.Printf("⚠️  Bluesky not configured for channel %q, skipping", channel.Key)
		}
		if t.poster != nil {
			targets = append(targets, t)
		}
	}

	if want("instagram") {
		// Lower than the text platforms: every post publishes an image through
		// GitHub Pages and counts against Instagram's daily publishing cap,
		// which the vote carousels share.
		budget := channel.EnvInt("GESCHAEFTE_IG_MAX_POSTS_PER_RUN", 2)
		t := target{name: "Instagram", log: votelog.PlatformInstagram, format: ginstagram.Formatter(mapper), imageDir: imageDir}
		userID, accessToken := channel.Env("IG_USER_ID"), channel.Env("IG_ACCESS_TOKEN")
		hostRepo, hostToken := channel.Env("IMAGE_HOST_REPO"), channel.Env("IMAGE_HOST_TOKEN")
		switch {
		case dryRun:
			t.poster = dryRunPoster{"Instagram", budget}
			if imageDir != "" {
				t.format = writingCards(t.format, imageDir)
			}
		case userID != "" && accessToken != "" && hostRepo != "" && hostToken != "":
			owner, name, err := igapi.ParseRepo(hostRepo)
			if err != nil {
				log.Fatalf("Channel %q: %sIMAGE_HOST_REPO: %v", channel.Key, channel.EnvPrefix(), err)
			}
			t.poster = instagram.NewInstagramPlatformWithCredentials(userID, accessToken, hostToken, owner, name, budget)
		default:
			log.Printf("⚠️  Instagram not configured for channel %q, skipping", channel.Key)
		}
		if t.poster != nil {
			targets = append(targets, t)
		}
	}

	return targets
}

// dryRunPoster stands in for a platform in a dry run. geschaeftposting.Post
// never calls Post when dry, so only the budget and the name matter.
type dryRunPoster struct {
	name   string
	budget int
}

func (p dryRunPoster) Post(platforms.Content) (votelog.PostRef, bool, error) {
	return votelog.PostRef{}, false, fmt.Errorf("dry-run poster asked to post")
}
func (p dryRunPoster) MaxPostsPerRun() int { return p.budget }
func (p dryRunPoster) Name() string        { return p.name }

// writingCards wraps the Instagram formatter to also save each card, so a dry
// run can be checked by eye.
func writingCards(format geschaeftposting.Formatter, dir string) geschaeftposting.Formatter {
	return func(g zurichapi.Geschaeft) (platforms.Content, error) {
		content, err := format(g)
		if err != nil {
			return nil, err
		}
		if ig, ok := content.(*instagram.InstagramContent); ok && len(ig.Images) > 0 {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return nil, err
			}
			path := filepath.Join(dir, strings.ReplaceAll(g.GRNr, "/", "_")+".jpg")
			if err := os.WriteFile(path, ig.Images[0], 0o644); err != nil {
				return nil, err
			}
			fmt.Printf("🖼️  Wrote %s\n", path)
		}
		return content, nil
	}
}

// getEnvInt reads an integer from the environment, falling back on absence or
// a value that does not parse.
func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
