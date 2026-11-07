// Package golden holds a cross-platform snapshot test over every fixture.
//
// It exists to make refactors that are supposed to be behaviour-preserving
// provable: testdata/golden.txt contains the exact text every platform
// formatter produces for every fixture, and testdata/nationalrat_<lang>.txt
// the same for the Nationalrat in each language it is posted in. Any diff is a behaviour change and must
// be justified, not accepted.
//
// Generated images are checked separately, by properties rather than by bytes —
// see TestGeneratedImages. JPEG bytes are not portable: golang.org/x/image
// rasterises glyphs through an AMD64-specific code path, so the same card
// encodes to different bytes on arm64 and amd64. A byte hash here would pass
// locally and fail in CI, which is worse than no check at all.
//
// Regenerate with:
//
//	go test ./pkg/voteposting/golden -update
package golden

import (
	"bytes"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/siiitschiii/parlinfo/pkg/contacts"
	"github.com/siiitschiii/parlinfo/pkg/imagegen"
	"github.com/siiitschiii/parlinfo/pkg/voteposting/platforms/bluesky"
	"github.com/siiitschiii/parlinfo/pkg/voteposting/platforms/facebook"
	"github.com/siiitschiii/parlinfo/pkg/voteposting/platforms/instagram"
	"github.com/siiitschiii/parlinfo/pkg/voteposting/platforms/x"
	"github.com/siiitschiii/parlinfo/pkg/voteposting/testfixtures"
	"github.com/siiitschiii/parlinfo/pkg/votes"
)

var update = flag.Bool("update", false, "rewrite testdata/golden.txt from current output")

const goldenPath = "testdata/golden.txt"

// xCharLimits covers both the free-tier and Premium post limits, because the
// packing logic in buildReplyPosts behaves differently at each.
var xCharLimits = []int{x.DefaultMaxChars, 2000}

func TestGoldenOutput(t *testing.T) {
	got := renderGolden(t, fixtureNames(), func(name string) []votes.Vote { return fixtureByName(t, name) })
	compareGolden(t, goldenPath, got)
}

// TestGoldenOutputByLanguage pins the Nationalrat's posts in each language it
// is posted in, one file per language, so a change to one language's wording
// shows up as a diff in that file and no other.
//
// German is among them although golden.txt is German too: golden.txt is
// shaped after the Zürich bodies, and only these fixtures show the three
// languages saying the same thing side by side.
func TestGoldenOutputByLanguage(t *testing.T) {
	for _, l := range []votes.Language{votes.German, votes.French, votes.Italian} {
		t.Run(string(l), func(t *testing.T) {
			fixtures := testfixtures.NationalratFixtures(l)
			got := renderGolden(t, testfixtures.NationalratFixtureNames, func(name string) []votes.Vote {
				group, ok := fixtures[name]
				if !ok {
					t.Fatalf("unknown fixture %q", name)
				}
				return group
			})
			compareGolden(t, filepath.Join("testdata", "nationalrat_"+string(l)+".txt"), got)
		})
	}
}

// renderGolden renders every named fixture on every platform, in order.
func renderGolden(t *testing.T, names []string, fixture func(string) []votes.Vote) string {
	t.Helper()
	mapper, err := contacts.LoadContacts(filepath.Join("..", "..", "..", "data", "contacts_test.yaml"))
	if err != nil {
		t.Fatalf("loading test contacts: %v", err)
	}

	var sb strings.Builder
	for _, name := range names {
		group := fixture(name)
		sb.WriteString(fmt.Sprintf("################ FIXTURE %s ################\n\n", name))

		for _, limit := range xCharLimits {
			sb.WriteString(fmt.Sprintf("---- X (limit %d) ----\n", limit))
			for i, post := range x.FormatVoteThread(group, mapper, limit) {
				sb.WriteString(fmt.Sprintf("[post %d]\n%s\n\n", i, post.Text))
			}
		}

		sb.WriteString("---- Bluesky ----\n")
		for i, post := range bluesky.FormatVoteThread(group, mapper) {
			sb.WriteString(fmt.Sprintf("[post %d]\n%s\n", i, post.Text))
			for _, f := range post.Facets {
				sb.WriteString(fmt.Sprintf("[facet] %+v\n", f))
			}
			for _, m := range post.Mentions {
				sb.WriteString(fmt.Sprintf("[mention] %s @ %d-%d\n", m.Handle, m.ByteStart, m.ByteEnd))
			}
			sb.WriteString("\n")
		}

		sb.WriteString("---- Instagram ----\n")
		content, err := instagram.FormatCarouselWithContacts(group, mapper)
		if err != nil {
			t.Fatalf("formatting Instagram content for %s: %v", name, err)
		}
		sb.WriteString(fmt.Sprintf("[images] %d\n", len(content.Images)))
		sb.WriteString(fmt.Sprintf("[caption]\n%s\n\n", content.Caption))

		sb.WriteString("---- Facebook ----\n")
		fbContent, err := facebook.FormatPost(group)
		if err != nil {
			t.Fatalf("formatting Facebook content for %s: %v", name, err)
		}
		sb.WriteString(fmt.Sprintf("[images] %d\n", len(fbContent.Images)))
		sb.WriteString(fmt.Sprintf("[message]\n%s\n", fbContent.Message))
		for i, caption := range fbContent.Captions {
			if caption != "" {
				sb.WriteString(fmt.Sprintf("[caption of image %d]\n%s\n", i+1, caption))
			}
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

// compareGolden checks got against the file at path, or rewrites the file
// under -update.
func compareGolden(t *testing.T, path, got string) {
	t.Helper()
	if *update {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("creating testdata dir: %v", err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatalf("writing golden file: %v", err)
		}
		t.Logf("wrote %s (%d bytes)", path, len(got))
		return
	}

	wantBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading golden file (run with -update to create it): %v", err)
	}

	if want := string(wantBytes); got != want {
		t.Errorf("output differs from %s: %s", path, firstDiff(want, got))
	}
}

func fixtureNames() []string {
	return append(append([]string{}, testfixtures.FixtureNames...), "instagram-long-multi-vote-truncation")
}

func fixtureByName(t *testing.T, name string) []votes.Vote {
	t.Helper()
	if name == "instagram-long-multi-vote-truncation" {
		return testfixtures.InstagramLongMultiVoteTruncation()
	}
	group, ok := testfixtures.AllFixtures()[name]
	if !ok {
		t.Fatalf("unknown fixture %q", name)
	}
	return group
}

// firstDiff reports the line number and surrounding context of the first
// difference, which is far more useful than dumping two multi-KB blobs.
func firstDiff(want, got string) string {
	wantLines := strings.Split(want, "\n")
	gotLines := strings.Split(got, "\n")
	for i := 0; i < len(wantLines) || i < len(gotLines); i++ {
		w, g := "<missing>", "<missing>"
		if i < len(wantLines) {
			w = wantLines[i]
		}
		if i < len(gotLines) {
			g = gotLines[i]
		}
		if w != g {
			return fmt.Sprintf("first difference at line %d\n  want: %q\n  got:  %q", i+1, w, g)
		}
	}
	return "no line difference (trailing whitespace?)"
}

// TestGeneratedImages checks the carousel images by the properties that must
// hold everywhere, rather than by a byte hash that only holds on one
// architecture.
//
// The background is a flat fill with no anti-aliasing, so its colour is
// bit-exact on every platform — which makes it a real assertion about
// SelectColor. The body caption is checked as a fact ("something was drawn
// there"), not a value, so glyph-rasterisation differences cannot break it.
func TestGeneratedImages(t *testing.T) {
	groups := map[string][]votes.Vote{}
	var names []string
	for _, name := range fixtureNames() {
		groups[name] = fixtureByName(t, name)
		names = append(names, name)
	}
	for _, l := range []votes.Language{votes.German, votes.French, votes.Italian} {
		for name, group := range testfixtures.NationalratFixtures(l) {
			key := name + "-" + string(l)
			groups[key] = group
			names = append(names, key)
		}
	}
	sort.Strings(names)

	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			group := groups[name]

			images, err := imagegen.GenerateCarousel(group)
			if err != nil {
				t.Fatalf("GenerateCarousel: %v", err)
			}
			if len(images) == 0 {
				t.Fatal("no images generated")
			}
			// Instagram caps a carousel at 10; the formatter trims, but the
			// generator should not be producing wildly more than that.
			if len(images) > 11 {
				t.Errorf("generated %d images", len(images))
			}

			wantBG := imagegen.SelectColor(group[0].Jurisdiction, group[0].Affair.Number)

			for i, data := range images {
				img, err := jpeg.Decode(bytes.NewReader(data))
				if err != nil {
					t.Fatalf("image %d does not decode: %v", i, err)
				}

				b := img.Bounds()
				if b.Dx() != 1080 || b.Dy() != 1350 {
					t.Errorf("image %d is %dx%d, want 1080x1350", i, b.Dx(), b.Dy())
				}

				// Sample a corner the layout never draws into.
				if got := img.At(b.Max.X-4, b.Max.Y-4); !closeTo(got, wantBG) {
					t.Errorf("image %d background = %v, want %v (SelectColor must key on jurisdiction and affair)",
						i, got, wantBG)
				}

				if !hasInk(img, image.Rect(50, 50, 500, 120), wantBG) {
					t.Errorf("image %d has nothing drawn in the body-caption area; "+
						"posts from two chambers share an account and must say which is which", i)
				}
			}
		})
	}
}

// closeTo compares against the JPEG-decoded colour, allowing for the small
// error lossy encoding introduces in a flat region.
func closeTo(got color.Color, want color.RGBA) bool {
	const tolerance = 0x0800 // ~2 levels in 8-bit terms

	gr, gg, gb, _ := got.RGBA()
	wr, wg, wb, _ := color.RGBA{want.R, want.G, want.B, 0xFF}.RGBA()

	return absDiff(gr, wr) <= tolerance && absDiff(gg, wg) <= tolerance && absDiff(gb, wb) <= tolerance
}

func absDiff(a, b uint32) uint32 {
	if a > b {
		return a - b
	}
	return b - a
}

// hasInk reports whether anything was drawn in a region, by looking for pixels
// clearly lighter than the background. Text is drawn in white on a dark fill.
func hasInk(img image.Image, region image.Rectangle, bg color.RGBA) bool {
	bgR, bgG, bgB, _ := color.RGBA{bg.R, bg.G, bg.B, 0xFF}.RGBA()
	bgSum := int(bgR) + int(bgG) + int(bgB)

	for y := region.Min.Y; y < region.Max.Y; y++ {
		for x := region.Min.X; x < region.Max.X; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			if int(r)+int(g)+int(b) > bgSum+0x8000 {
				return true
			}
		}
	}
	return false
}

// TestExtremeTitleFixtureAddsUp holds the extreme-title fixture to its own
// headline and to the chamber's seat count.
//
// It is the regression case for cards that shipped a Fraktion table
// disagreeing with their totals, so it must not ship one itself. It first did:
// the rows carried 64 Nein under a 60 Nein headline, across 129 seats in a
// 125-seat chamber. That also put it the wrong side of IsBreakdownComplete,
// which would have made the posting path drop the very table the fixture
// exists to exercise — and the golden snapshot pinned the mismatch in place.
//
// Deliberately incomplete fixtures do exist, to drive that same gate: the tail
// of ten-vote-stress-test and the Kantonsrat groups. So this checks the one
// fixture whose job is a complete, self-consistent table, not all of them.
func TestExtremeTitleFixtureAddsUp(t *testing.T) {
	const seats = 125 // Gemeinderat der Stadt Zürich

	group := testfixtures.ExtremeTitleFullRoster()
	if len(group) != 1 {
		t.Fatalf("expected a single vote, got %d", len(group))
	}
	v := group[0]

	tally := map[string]int{}
	for _, mv := range v.MemberVotes {
		tally[mv.Choice]++
	}
	for _, c := range []struct {
		choice   string
		reported *int
	}{
		{"Ja", v.Yes}, {"Nein", v.No}, {"Enthaltung", v.Abstention}, {"Abwesend", v.Absent},
	} {
		want := 0
		if c.reported != nil {
			want = *c.reported
		}
		if tally[c.choice] != want {
			t.Errorf("%s: rows total %d against a headline of %d", c.choice, tally[c.choice], want)
		}
	}

	if len(v.MemberVotes) != seats {
		t.Errorf("roster holds %d members, but the Gemeinderat has %d seats", len(v.MemberVotes), seats)
	}
	if !votes.IsBreakdownComplete(v) {
		t.Error("breakdown is incomplete, so the posting path would drop the Fraktion table entirely")
	}
}
