package site

import (
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

// notInReadme are the links the README's platform table does not carry: the
// LinkedIn Pages exist but the bot does not post there yet.
var notInReadme = map[string]bool{
	"https://www.linkedin.com/company/zueriratsinfo": true,
	"https://www.linkedin.com/company/chparlinfo":    true,
}

// The site and the README list the same accounts. A new account added to one
// and not the other, or a URL mistyped here, fails this.
func TestBrandsMatchReadmePlatformTable(t *testing.T) {
	readme, err := os.ReadFile("../README.md")
	if err != nil {
		t.Fatal(err)
	}
	_, table, found := strings.Cut(string(readme), "## Supported Platforms")
	if !found {
		t.Fatal("README.md has no \"Supported Platforms\" section")
	}
	table, _, _ = strings.Cut(table, "\n## ")

	inReadme := map[string]bool{}
	for _, m := range regexp.MustCompile(`\]\((https://[^)]+)\)`).FindAllStringSubmatch(table, -1) {
		// The politician-mapping footnote under the table links into the repo.
		if !strings.HasPrefix(m[1], "https://github.com/") {
			inReadme[m[1]] = true
		}
	}
	if len(inReadme) == 0 {
		t.Fatal("no account links found in the README's platform table")
	}

	onSite := map[string]bool{}
	for _, b := range Brands {
		for _, a := range b.Accounts {
			onSite[a.URL] = true
			if !inReadme[a.URL] && !notInReadme[a.URL] {
				t.Errorf("%s %s: %s is not in the README's platform table", b.Name, a.Platform, a.URL)
			}
		}
	}
	for u := range inReadme {
		if !onSite[u] {
			t.Errorf("%s is in the README's platform table but on no brand page", u)
		}
	}
}

// Every language has every string, for every brand.
func TestTextsComplete(t *testing.T) {
	for _, lang := range Langs {
		text, ok := Texts[lang]
		if !ok {
			t.Errorf("no texts for %q", lang)
			continue
		}
		v := reflect.ValueOf(text)
		for i := 0; i < v.NumField(); i++ {
			if v.Field(i).Kind() == reflect.String && v.Field(i).String() == "" {
				t.Errorf("%s: %s is empty", lang, v.Type().Field(i).Name)
			}
		}
		for i, p := range text.Pillars {
			if p.Title == "" || p.Text == "" {
				t.Errorf("%s: pillar %d is incomplete", lang, i)
			}
		}
		for _, b := range Brands {
			if text.BrandBodies[b.Key] == "" {
				t.Errorf("%s: no BrandBodies for %q", lang, b.Key)
			}
			if text.BrandLead[b.Key] == "" {
				t.Errorf("%s: no BrandLead for %q", lang, b.Key)
			}
			// The note says which languages a brand posts in; for a brand
			// with one, the page says nothing about language at all.
			multi := len(b.Langs()) > 1
			if multi && text.BrandNote[b.Key] == "" {
				t.Errorf("%s: no BrandNote for %q, which posts in several languages", lang, b.Key)
			}
			if !multi && text.BrandNote[b.Key] != "" {
				t.Errorf("%s: BrandNote for %q, which posts in one language", lang, b.Key)
			}
		}
	}
}

func TestBuild(t *testing.T) {
	dir := t.TempDir()
	if err := Build(dir); err != nil {
		t.Fatal(err)
	}

	var files []string
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			rel, _ := filepath.Rel(dir, p)
			files = append(files, filepath.ToSlash(rel))
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	built := map[string]bool{}
	for _, f := range files {
		built[f] = true
		// The bot's directory on the same branch; publish.sh refuses a build
		// that has it, and this says so before a workflow run does.
		if f == "ig-images" || strings.HasPrefix(f, "ig-images/") {
			t.Errorf("build wrote %s: ig-images/ belongs to the bot", f)
		}
	}

	want := []string{"index.html", "404.html", "CNAME", ".nojekyll"}
	for _, lang := range Langs {
		want = append(want, lang+"/index.html")
		for _, b := range Brands {
			want = append(want, lang+"/"+b.Key+"/index.html")
		}
	}
	for _, f := range want {
		if !built[f] {
			t.Errorf("build did not write %s", f)
		}
	}

	// GitHub Pages reads the domain from this file; anything but the bare
	// host name on one line unsets the custom domain.
	if cname, _ := os.ReadFile(filepath.Join(dir, "CNAME")); string(cname) != Domain+"\n" {
		t.Errorf("CNAME = %q, want %q", cname, Domain+"\n")
	}

	// The contact address is assembled in the browser; no page may carry it
	// whole, or harvesters scanning the HTML would find it.
	email := contactUser + "@" + contactDomain
	for _, f := range files {
		if html, _ := os.ReadFile(filepath.Join(dir, filepath.FromSlash(f))); strings.Contains(string(html), email) {
			t.Errorf("%s contains the contact address %s in plain text", f, email)
		}
	}

	// Every link within the site leads to a file that was built. Pages are
	// linked relatively, so this is what catches a wrong "../" depth.
	ref := regexp.MustCompile(`(?:href|src)="([^"]+)"`)
	for _, f := range files {
		if !strings.HasSuffix(f, ".html") {
			continue
		}
		html, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(f)))
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range ref.FindAllStringSubmatch(string(html), -1) {
			target := m[1]
			switch {
			case strings.HasPrefix(target, "https://"+Domain+"/"):
				target = strings.TrimPrefix(target, "https://"+Domain+"/")
			case strings.Contains(target, "://"):
				continue // another site
			case strings.HasPrefix(target, "/"):
				target = strings.TrimPrefix(target, "/")
			default:
				target = path.Join(path.Dir(f), target)
				if strings.HasSuffix(m[1], "/") {
					target += "/"
				}
			}
			if target == "" || strings.HasSuffix(target, "/") {
				target += "index.html"
			}
			if !built[target] {
				t.Errorf("%s links to %s, which resolves to %s: not built", f, m[1], target)
			}
		}
	}
}

// A page in one language offers the same page in the others, not their
// landing pages: the switcher keeps the reader on the brand.
func TestLanguageSwitcherKeepsThePage(t *testing.T) {
	p := newPage("fr", "ch/")
	got := map[string]string{}
	for _, l := range p.Switcher {
		got[l.Code] = l.Href
	}
	want := map[string]string{"de": "../../de/ch/", "fr": "../../fr/ch/", "it": "../../it/ch/"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("switcher = %v, want %v", got, want)
	}
}

func brand(t *testing.T, key string) *Brand {
	t.Helper()
	for i := range Brands {
		if Brands[i].Key == key {
			return &Brands[i]
		}
	}
	t.Fatalf("no brand %q", key)
	return nil
}

// A multilingual brand gets one row per platform with a chip per language,
// the handle shown being the reader's; a platform without one is a plain link.
func TestPlatformViewsMultilingual(t *testing.T) {
	rows := platformViews(brand(t, "ch"), "fr")
	var platforms []string
	for _, r := range rows {
		platforms = append(platforms, r.Platform)
	}
	if want := []string{"Instagram", "Facebook", "X", "Bluesky", "LinkedIn"}; !reflect.DeepEqual(platforms, want) {
		t.Fatalf("platforms = %q, want %q", platforms, want)
	}

	ig := rows[0]
	if ig.Handle != "@chparlinfo_fr" {
		t.Errorf("Instagram row: handle %q, want the French handle", ig.Handle)
	}
	for _, c := range ig.Chips {
		if c.Lang == "fr" && ig.URL != c.URL {
			t.Errorf("Instagram row URL = %q, want the French account %q", ig.URL, c.URL)
		}
	}
	var langs []string
	for _, c := range ig.Chips {
		langs = append(langs, c.Lang)
		if c.Current != (c.Lang == "fr") {
			t.Errorf("Instagram %s chip: Current = %v", c.Lang, c.Current)
		}
	}
	if want := []string{"de", "fr", "it"}; !reflect.DeepEqual(langs, want) {
		t.Errorf("Instagram chips = %q, want %q", langs, want)
	}

	// X has a German account only: the chip says so, and the handle is it.
	// The row still leads somewhere: to that German account.
	if x := rows[2]; len(x.Chips) != 1 || x.Chips[0].Lang != "de" || x.Handle != "@chparlinfo" || x.URL != x.Chips[0].URL {
		t.Errorf("X row = %+v, want one German chip", x)
	}
	if li := rows[4]; li.URL == "" || len(li.Chips) != 0 {
		t.Errorf("LinkedIn row = %+v, want a plain link", li)
	}
}

// A brand posting in one language gets plain links, one per account.
func TestPlatformViewsSingleLanguage(t *testing.T) {
	b := brand(t, "zh")
	rows := platformViews(b, "it")
	if len(rows) != len(b.Accounts) {
		t.Fatalf("%d rows for %d accounts", len(rows), len(b.Accounts))
	}
	for _, r := range rows {
		if r.URL == "" || len(r.Chips) != 0 {
			t.Errorf("%s row = %+v, want a plain link", r.Platform, r)
		}
	}
}
