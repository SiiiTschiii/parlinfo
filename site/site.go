// Package site builds parlinfo.ch: a static link hub with a landing page and
// one page per brand, in German, French and Italian.
//
// The layout is /<lang>/ for the landing page and /<lang>/<brand>/ for a
// brand, with the bare domain forwarding to a language. Everything a page
// links to on the site is relative, so the output works from any base path:
// the domain, a github.io project path, or a local file server.
//
// Build never writes under ig-images/. The site shares the gh-pages branch
// with the carousel images the bot hosts there for Instagram
// (pkg/igapi/hosting.go), and publish.sh leaves that directory alone.
package site

import (
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

//go:embed templates/*.html
var templateFS embed.FS

// all: because .nojekyll starts with a dot, which embed otherwise skips.
//
//go:embed all:static
var staticFS embed.FS

// LangLink is one entry of the language switcher.
type LangLink struct {
	Code    string
	Name    string
	Href    string // relative to the page
	Current bool
}

// AltLink is one hreflang alternate, absolute as search engines want them.
type AltLink struct {
	Lang string
	URL  string
}

// BrandCard is a brand as the landing page shows it.
type BrandCard struct {
	Brand
	Bodies string
	Href   string
}

// PlatformView is one row of a brand page: a platform and its accounts.
type PlatformView struct {
	Platform string
	Handle   string        // the account shown under the name: the reader's language if there is one
	URL      string        // set when the row is a single link
	Chips    []AccountChip // set when the row links to one account per language
}

// AccountChip is one language's account within a platform row.
type AccountChip struct {
	Lang     string
	LangName string
	Handle   string
	URL      string
	Current  bool // posts in the page's language
}

// Page is what a template is given.
type Page struct {
	Lang       string
	T          Text
	Root       string // relative path to the site root, "" or "../"...
	Title      string
	Canonical  string
	Alternates []AltLink
	Switcher   []LangLink
	RepoURL    string
	SupportURL string

	// Landing page
	Cards []BrandCard

	// Brand page
	Brand     *Brand
	Bodies    string
	Note      string
	Platforms []PlatformView

	// Root and 404 page
	Langs       []LangLink
	DefaultLang string
}

// Build writes the whole site into dir, which is created if missing.
func Build(dir string) error {
	if err := copyStatic(dir); err != nil {
		return err
	}

	// The bare domain: forwards to the visitor's language.
	if err := render(dir, "index.html", "root.html", entryPage("")); err != nil {
		return err
	}
	// GitHub Pages serves /404.html for any missing path, at that path, so
	// unlike every other page its links cannot be relative.
	if err := render(dir, "404.html", "404.html", entryPage("/")); err != nil {
		return err
	}

	for _, lang := range Langs {
		t := Texts[lang]

		home := newPage(lang, "")
		home.Title = t.Title
		for _, b := range Brands {
			home.Cards = append(home.Cards, BrandCard{Brand: b, Bodies: t.BrandBodies[b.Key], Href: b.Key + "/"})
		}
		if err := render(dir, lang+"/index.html", "home.html", home); err != nil {
			return err
		}

		for i := range Brands {
			b := &Brands[i]
			p := newPage(lang, b.Key+"/")
			p.Title = b.Name + " – parlinfo"
			p.Brand = b
			p.Bodies = t.BrandBodies[b.Key]
			p.Note = t.BrandNote[b.Key]
			p.Platforms = platformViews(b, lang)
			if err := render(dir, lang+"/"+b.Key+"/index.html", "brand.html", p); err != nil {
				return err
			}
		}
	}
	return nil
}

// newPage fills in what every language page shares. path is the page's
// directory below its language, "" or "zh/".
func newPage(lang, path string) Page {
	root := "../" + strings.Repeat("../", strings.Count(path, "/"))
	p := Page{
		Lang:       lang,
		T:          Texts[lang],
		Root:       root,
		Canonical:  absURL(lang, path),
		RepoURL:    repoURL,
		SupportURL: supportURL,
	}
	for _, l := range Langs {
		p.Switcher = append(p.Switcher, LangLink{Code: l, Name: Texts[l].LangName, Href: root + l + "/" + path, Current: l == lang})
		p.Alternates = append(p.Alternates, AltLink{Lang: l, URL: absURL(l, path)})
	}
	p.Alternates = append(p.Alternates, AltLink{Lang: "x-default", URL: absURL(DefaultLang, path)})
	return p
}

// entryPage is the data of the two pages outside a language: the root, which
// forwards, and 404. root is what their links start with.
func entryPage(root string) Page {
	p := Page{Lang: DefaultLang, T: Texts[DefaultLang], Root: root, DefaultLang: DefaultLang, Title: "parlinfo", RepoURL: repoURL}
	for _, l := range Langs {
		p.Langs = append(p.Langs, LangLink{Code: l, Name: Texts[l].LangName, Href: root + l + "/"})
	}
	return p
}

// platformViews gives a brand page one row per platform, in the order the
// platforms first appear in b.Accounts. A brand that posts in one language
// gets plain links; one that posts in several gets, per platform, a chip for
// each language it has an account in, so the reader sees at a glance which
// languages exist where instead of reading the same list three times.
func platformViews(b *Brand, lang string) []PlatformView {
	multi := len(b.Langs()) > 1
	var out []PlatformView
	index := map[string]int{}
	for _, a := range b.Accounts {
		key := a.Platform
		if !multi || a.Lang == "" {
			key = a.URL // every account its own row
		}
		i, seen := index[key]
		if !seen {
			i = len(out)
			index[key] = i
			out = append(out, PlatformView{Platform: a.Platform, Handle: a.Handle, URL: a.URL})
		}
		v := &out[i]
		if !multi || a.Lang == "" {
			continue
		}
		if a.Lang == lang {
			v.Handle = a.Handle
		}
		v.URL = ""
		v.Chips = append(v.Chips, AccountChip{Lang: a.Lang, LangName: Texts[a.Lang].LangName, Handle: a.Handle, URL: a.URL, Current: a.Lang == lang})
	}
	return out
}

func absURL(lang, path string) string {
	return "https://" + Domain + "/" + lang + "/" + path
}

func render(dir, name, tmpl string, data Page) error {
	t, err := template.ParseFS(templateFS, "templates/layout.html", "templates/"+tmpl)
	if err != nil {
		return fmt.Errorf("parsing %s: %w", tmpl, err)
	}
	out := filepath.Join(dir, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return err
	}
	f, err := os.Create(out)
	if err != nil {
		return err
	}
	if err := t.ExecuteTemplate(f, "layout", data); err != nil {
		_ = f.Close()
		return fmt.Errorf("rendering %s: %w", name, err)
	}
	return f.Close()
}

// copyStatic writes static/ into dir as it is: CNAME, .nojekyll and assets/.
func copyStatic(dir string) error {
	return fs.WalkDir(staticFS, "static", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel := strings.TrimPrefix(strings.TrimPrefix(path, "static"), "/")
		out := filepath.Join(dir, filepath.FromSlash(rel))
		if d.IsDir() {
			return os.MkdirAll(out, 0o755)
		}
		data, err := staticFS.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(out, data, 0o644)
	})
}
