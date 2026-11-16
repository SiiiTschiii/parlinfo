package site

// Domain is where the site is served. It is also the content of the CNAME
// file, which is what tells GitHub Pages to serve the branch there.
const Domain = "parlinfo.ch"

// DefaultLang is the language the bare domain falls back to.
const DefaultLang = "de"

// Langs are the interface languages, in the order the switcher shows them.
// Each is a top-level directory of the site.
var Langs = []string{"de", "fr", "it"}

// repoURL is the one link every page carries.
const repoURL = "https://github.com/SiiiTschiii/parlinfo"

// Link is one account on one platform.
type Link struct {
	Platform string // "Instagram": a proper name, the same in every language
	Handle   string // what the account is called there
	URL      string
}

// Group is a brand's accounts in one posting language. Lang is empty for
// links that are not tied to one, which are listed last under their own
// heading.
type Group struct {
	Lang  string
	Links []Link
}

// Brand is one family of accounts, and one directory under each language:
// /de/zh/, /fr/ch/. The vote archive of a brand will live below it.
type Brand struct {
	Key    string // path segment and the code in the mark
	Name   string // the same in every language
	Mark   string // file under assets/
	Groups []Group
}

// Brands lists every account the bot posts to. The URLs are those of the
// "Supported Platforms" table in README.md, which site_test.go holds this
// list against, plus the two LinkedIn Pages and the repository.
var Brands = []Brand{
	{
		Key:  "zh",
		Name: "Züri Ratsinfo",
		Mark: "parlinfo-zh-mark.svg",
		Groups: []Group{
			{Lang: "de", Links: []Link{
				{"Instagram", "@zueriratsinfo", "https://www.instagram.com/zueriratsinfo"},
				{"Facebook", "ZueriRatsinfo", "https://www.facebook.com/people/ZueriRatsinfo/1063164230212573/"},
				{"X", "@zuerichratsinfo", "https://x.com/zuerichratsinfo"},
				{"Bluesky", "@zuerichratsinfo.bsky.social", "https://bsky.app/profile/zuerichratsinfo.bsky.social"},
				{"LinkedIn", "Züri Ratsinfo", "https://www.linkedin.com/company/zueriratsinfo"},
			}},
			{Links: []Link{
				{"GitHub", "SiiiTschiii/parlinfo", repoURL},
			}},
		},
	},
	{
		Key:  "ch",
		Name: "CH Parl Info",
		Mark: "parlinfo-ch-mark.svg",
		Groups: []Group{
			{Lang: "de", Links: []Link{
				{"Instagram", "@chparlinfo", "https://www.instagram.com/chparlinfo"},
				{"Facebook", "CH Parl Info", "https://www.facebook.com/profile.php?id=61595771216866"},
				{"X", "@chparlinfo", "https://x.com/chparlinfo"},
				{"Bluesky", "@chparlinfo.bsky.social", "https://bsky.app/profile/chparlinfo.bsky.social"},
			}},
			{Lang: "fr", Links: []Link{
				{"Instagram", "@chparlinfo_fr", "https://www.instagram.com/chparlinfo_fr"},
				{"Facebook", "CHparlinfo - Français", "https://www.facebook.com/share/1Jcks2Kb85/"},
				{"Bluesky", "@chparlinfo-fr.bsky.social", "https://bsky.app/profile/chparlinfo-fr.bsky.social"},
			}},
			{Lang: "it", Links: []Link{
				{"Instagram", "@chparlinfo_it", "https://www.instagram.com/chparlinfo_it"},
				{"Facebook", "CHparlinfo - Italiano", "https://www.facebook.com/share/19ubfSSmN2/"},
				{"Bluesky", "@chparlinfo-it.bsky.social", "https://bsky.app/profile/chparlinfo-it.bsky.social"},
			}},
			{Links: []Link{
				{"LinkedIn", "CH Parl Info", "https://www.linkedin.com/company/chparlinfo"},
				{"GitHub", "SiiiTschiii/parlinfo", repoURL},
			}},
		},
	},
}

// Text is the interface in one language. Vote content is not here: it exists
// only in the languages a body is posted in. site_test.go fails on an empty
// field, so a string added here cannot be forgotten in one language.
type Text struct {
	LangName    string // the language's own name, for the switcher and group headings
	Title       string // <title> of the landing page
	Tagline     string
	Intro       string
	Choose      string            // heading above the brand cards
	BrandBodies map[string]string // by Brand.Key: the parliaments a brand covers
	BrandNote   map[string]string // by Brand.Key: which languages it posts in
	Accounts    string            // heading of a brand page's list, before the language
	OtherLinks  string            // heading of the group without a language
	Overview    string            // link back to the landing page
	Source      string            // footer, before the repository link
	SwitchLabel string            // aria-label of the language switcher
}

// Texts holds the interface by language code.
var Texts = map[string]Text{
	"de": {
		LangName: "Deutsch",
		Title:    "parlinfo – Parlamentsabstimmungen auf Social Media",
		Tagline:  "Parlamentsabstimmungen, dort wo die Menschen sind.",
		Intro:    "parlinfo bringt die Abstimmungen von Schweizer Parlamenten auf Social Media: mit dem Resultat, dem Stimmverhalten jeder Fraktion und dem Link zum offiziellen Geschäft.",
		Choose:   "Parlamente",
		BrandBodies: map[string]string{
			"zh": "Gemeinderat Stadt Zürich und Kantonsrat Zürich",
			"ch": "Nationalrat",
		},
		BrandNote: map[string]string{
			"zh": "Die Beiträge erscheinen auf Deutsch.",
			"ch": "Die Beiträge erscheinen auf Deutsch, Französisch und Italienisch, je auf eigenen Accounts.",
		},
		Accounts:    "Accounts",
		OtherLinks:  "Weitere Links",
		Overview:    "Übersicht",
		Source:      "Open Source auf",
		SwitchLabel: "Sprache",
	},
	"fr": {
		LangName: "Français",
		Title:    "parlinfo – Les votes des parlements sur les réseaux sociaux",
		Tagline:  "Les votes des parlements, là où les gens se trouvent.",
		Intro:    "parlinfo publie les votes de parlements suisses sur les réseaux sociaux : avec le résultat, le vote de chaque groupe et le lien vers l’objet officiel.",
		Choose:   "Parlements",
		BrandBodies: map[string]string{
			"zh": "Gemeinderat Stadt Zürich (parlement de la ville) et Kantonsrat Zürich (parlement cantonal)",
			"ch": "Conseil national",
		},
		BrandNote: map[string]string{
			"zh": "Les publications paraissent en allemand.",
			"ch": "Les publications paraissent en allemand, en français et en italien, chaque langue sur ses propres comptes.",
		},
		Accounts:    "Comptes",
		OtherLinks:  "Autres liens",
		Overview:    "Aperçu",
		Source:      "Open source sur",
		SwitchLabel: "Langue",
	},
	"it": {
		LangName: "Italiano",
		Title:    "parlinfo – Le votazioni dei parlamenti sui social media",
		Tagline:  "Le votazioni dei parlamenti, là dove si trovano le persone.",
		Intro:    "parlinfo porta le votazioni dei parlamenti svizzeri sui social media: con il risultato, il voto di ogni gruppo e il link all’oggetto ufficiale.",
		Choose:   "Parlamenti",
		BrandBodies: map[string]string{
			"zh": "Gemeinderat Stadt Zürich (parlamento cittadino) e Kantonsrat Zürich (parlamento cantonale)",
			"ch": "Consiglio nazionale",
		},
		BrandNote: map[string]string{
			"zh": "I post sono pubblicati in tedesco.",
			"ch": "I post sono pubblicati in tedesco, francese e italiano, ogni lingua sui propri account.",
		},
		Accounts:    "Account",
		OtherLinks:  "Altri link",
		Overview:    "Panoramica",
		Source:      "Open source su",
		SwitchLabel: "Lingua",
	},
}
