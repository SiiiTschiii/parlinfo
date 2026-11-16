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

// GroupView is a Group with its heading resolved for the page's language.
type GroupView struct {
	Heading string
	Lang    string // posting language of the linked accounts, "" when not tied to one
	Links   []Link
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

	// Landing page
	Cards []BrandCard

	// Brand page
	Brand  *Brand
	Bodies string
	Note   string
	Groups []GroupView

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
			p.Groups = groupViews(b, lang)
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
		Lang:      lang,
		T:         Texts[lang],
		Root:      root,
		Canonical: absURL(lang, path),
		RepoURL:   repoURL,
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

// groupViews orders a brand's groups for a page in lang: the accounts posting
// in the reader's language first, the rest in their listed order, and the
// links without a language last.
func groupViews(b *Brand, lang string) []GroupView {
	t := Texts[lang]
	var own, others, rest []GroupView
	for _, g := range b.Groups {
		switch g.Lang {
		case "":
			rest = append(rest, GroupView{Heading: t.OtherLinks, Links: g.Links})
		case lang:
			own = append(own, GroupView{Heading: t.Accounts + " · " + Texts[g.Lang].LangName, Lang: g.Lang, Links: g.Links})
		default:
			others = append(others, GroupView{Heading: t.Accounts + " · " + Texts[g.Lang].LangName, Lang: g.Lang, Links: g.Links})
		}
	}
	return append(append(own, others...), rest...)
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
