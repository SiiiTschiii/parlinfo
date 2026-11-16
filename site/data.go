package site

// Domain is where the site is served. It is also the content of the CNAME
// file, which is what tells GitHub Pages to serve the branch there.
const Domain = "parlinfo.ch"

// DefaultLang is the language the bare domain falls back to.
const DefaultLang = "de"

// Langs are the interface languages, in the order the switcher shows them.
// Each is a top-level directory of the site.
var Langs = []string{"de", "fr", "it"}

// repoURL is the one link every page carries, in the footer.
const repoURL = "https://github.com/SiiiTschiii/parlinfo"

// supportURL is where readers can support the project.
const supportURL = "https://buymeacoffee.com/zuerichratsinfo"

// Account is one account on one platform.
type Account struct {
	Platform string // "Instagram": a proper name, the same in every language
	Lang     string // the language it posts in, "" when not tied to one
	Handle   string // what the account is called there
	URL      string
}

// Brand is one family of accounts, and one directory under each language:
// /de/zh/, /fr/ch/. The vote archive of a brand will live below it.
type Brand struct {
	Key      string // path segment and the code in the mark
	Name     string // the same in every language
	Mark     string // file under assets/
	Accounts []Account
}

// Langs are the languages the brand posts in, in the order of Langs.
func (b *Brand) Langs() []string {
	var out []string
	for _, l := range Langs {
		for _, a := range b.Accounts {
			if a.Lang == l {
				out = append(out, l)
				break
			}
		}
	}
	return out
}

// Brands lists every account the bot posts to. The URLs are those of the
// "Supported Platforms" table in README.md, which site_test.go holds this
// list against, plus the two LinkedIn Pages. A brand page shows one row per
// platform, in the order a platform first appears here.
var Brands = []Brand{
	{
		Key:  "zh",
		Name: "Züri Ratsinfo",
		Mark: "parlinfo-zh-mark.svg",
		Accounts: []Account{
			{"Instagram", "de", "@zueriratsinfo", "https://www.instagram.com/zueriratsinfo"},
			{"Facebook", "de", "ZueriRatsinfo", "https://www.facebook.com/people/ZueriRatsinfo/1063164230212573/"},
			{"X", "de", "@zuerichratsinfo", "https://x.com/zuerichratsinfo"},
			{"Bluesky", "de", "@zuerichratsinfo.bsky.social", "https://bsky.app/profile/zuerichratsinfo.bsky.social"},
			{"LinkedIn", "de", "Züri Ratsinfo", "https://www.linkedin.com/company/zueriratsinfo"},
		},
	},
	{
		Key:  "ch",
		Name: "CH Parl Info",
		Mark: "parlinfo-ch-mark.svg",
		Accounts: []Account{
			{"Instagram", "de", "@chparlinfo", "https://www.instagram.com/chparlinfo"},
			{"Instagram", "fr", "@chparlinfo_fr", "https://www.instagram.com/chparlinfo_fr"},
			{"Instagram", "it", "@chparlinfo_it", "https://www.instagram.com/chparlinfo_it"},
			{"Facebook", "de", "CH Parl Info", "https://www.facebook.com/profile.php?id=61595771216866"},
			{"Facebook", "fr", "CHparlinfo - Français", "https://www.facebook.com/share/1Jcks2Kb85/"},
			{"Facebook", "it", "CHparlinfo - Italiano", "https://www.facebook.com/share/19ubfSSmN2/"},
			{"X", "de", "@chparlinfo", "https://x.com/chparlinfo"},
			{"Bluesky", "de", "@chparlinfo.bsky.social", "https://bsky.app/profile/chparlinfo.bsky.social"},
			{"Bluesky", "fr", "@chparlinfo-fr.bsky.social", "https://bsky.app/profile/chparlinfo-fr.bsky.social"},
			{"Bluesky", "it", "@chparlinfo-it.bsky.social", "https://bsky.app/profile/chparlinfo-it.bsky.social"},
			{"LinkedIn", "", "CH Parl Info", "https://www.linkedin.com/company/chparlinfo"},
		},
	},
}

// Pillar is one of the two things the project is for.
type Pillar struct {
	Title string
	Text  string
}

// Text is the interface in one language. Vote content is not here: it exists
// only in the languages a body is posted in. site_test.go fails on an empty
// field, so a string added here cannot be forgotten in one language.
type Text struct {
	LangName    string // the language's own name, for the switcher and language chips
	Title       string // <title> of the landing page
	Description string // meta description
	Tagline     string
	Intro       string            // why the project exists, leading into the pillars
	Pillars     [2]Pillar         // participation and accountability, as in README.md
	Choose      string            // heading above the brand cards
	BrandBodies map[string]string // by Brand.Key: the parliaments a brand covers
	BrandNote   map[string]string // by Brand.Key: only for brands posting in several languages
	Accounts    string            // heading of a brand page's list
	Support     string            // heading of the support box
	SupportText string
	SupportCTA  string // the button
	Overview    string // link back to the landing page
	Source      string // footer, before the repository link
	SwitchLabel string // aria-label of the language switcher
}

// Texts holds the interface by language code.
var Texts = map[string]Text{
	"de": {
		LangName:    "Deutsch",
		Title:       "parlinfo – Parlamentsentscheide auf Social Media",
		Description: "parlinfo bringt die Entscheide Schweizer Parlamente auf Social Media: für mehr Partizipation und Rechenschaft, dort wo die Menschen sind.", //nolint:misspell // German: "Partizipation"
		Tagline:     "Parlamentsentscheide, dort wo die Menschen sind.",
		Intro:       "parlinfo bringt die Entscheide Schweizer Parlamente auf Social Media, weil eine gesunde Demokratie auf zwei Dingen beruht, die leicht aus dem Blick geraten:",
		Pillars: [2]Pillar{
			{"Partizipation", "Bürgerinnen und Bürger, die die Entscheide, die sie betreffen, mitverfolgen und mitgestalten können."}, //nolint:misspell // German: "Partizipation"
			{"Rechenschaft", "Was Politikerinnen und Politiker wirklich tun, vom Abstimmungsverhalten bis zu den Vorstössen, die sie mittragen, sichtbar gemacht."},
		},
		Choose: "Parlamente",
		BrandBodies: map[string]string{
			"zh": "Gemeinderat Stadt Zürich und Kantonsrat Zürich",
			"ch": "Nationalrat",
		},
		BrandNote: map[string]string{
			"ch": "Die Beiträge erscheinen auf Deutsch, Französisch und Italienisch, je auf eigenen Accounts.",
		},
		Accounts:    "Accounts",
		Support:     "Projekt unterstützen",
		SupportText: "parlinfo ist unabhängig und nicht gewinnorientiert.",
		SupportCTA:  "Unterstützen",
		Overview:    "Übersicht",
		Source:      "Open Source auf",
		SwitchLabel: "Sprache",
	},
	"fr": {
		LangName:    "Français",
		Title:       "parlinfo – Les décisions des parlements sur les réseaux sociaux",
		Description: "parlinfo publie les décisions des parlements suisses sur les réseaux sociaux : pour plus de participation et de responsabilité, là où les gens se trouvent.",
		Tagline:     "Les décisions des parlements, là où les gens se trouvent.",
		Intro:       "parlinfo publie les décisions des parlements suisses sur les réseaux sociaux, parce qu’une démocratie saine repose sur deux choses que l’on perd facilement de vue :",
		Pillars: [2]Pillar{
			{"Participation", "Des citoyennes et citoyens qui peuvent suivre les décisions qui les concernent, et y prendre part."},
			{"Responsabilité", "Ce que font réellement les élu·e·s, de leurs votes aux interventions qu’ils et elles soutiennent, rendu visible."},
		},
		Choose: "Parlements",
		BrandBodies: map[string]string{
			"zh": "Gemeinderat Stadt Zürich (parlement de la ville) et Kantonsrat Zürich (parlement cantonal)",
			"ch": "Conseil national",
		},
		BrandNote: map[string]string{
			"ch": "Les publications paraissent en allemand, en français et en italien, chaque langue sur ses propres comptes.",
		},
		Accounts:    "Comptes",
		Support:     "Soutenir le projet",
		SupportText: "parlinfo est indépendant et à but non lucratif.",
		SupportCTA:  "Soutenir",
		Overview:    "Aperçu",
		Source:      "Open source sur",
		SwitchLabel: "Langue",
	},
	"it": {
		LangName:    "Italiano",
		Title:       "parlinfo – Le decisioni dei parlamenti sui social media",
		Description: "parlinfo porta le decisioni dei parlamenti svizzeri sui social media: per più partecipazione e responsabilità, là dove si trovano le persone.",
		Tagline:     "Le decisioni dei parlamenti, là dove si trovano le persone.",
		Intro:       "parlinfo porta le decisioni dei parlamenti svizzeri sui social media, perché una democrazia sana poggia su due cose che si perdono facilmente di vista:",
		Pillars: [2]Pillar{
			{"Partecipazione", "Cittadine e cittadini che possono seguire le decisioni che li riguardano e prendervi parte."},
			{"Responsabilità", "Ciò che fanno davvero le e i politici, da come votano agli atti parlamentari che sostengono, reso visibile."},
		},
		Choose: "Parlamenti",
		BrandBodies: map[string]string{
			"zh": "Gemeinderat Stadt Zürich (parlamento cittadino) e Kantonsrat Zürich (parlamento cantonale)",
			"ch": "Consiglio nazionale",
		},
		BrandNote: map[string]string{
			"ch": "I post sono pubblicati in tedesco, francese e italiano, ogni lingua sui propri account.",
		},
		Accounts:    "Account",
		Support:     "Sostieni il progetto",
		SupportText: "parlinfo è indipendente e senza scopo di lucro.",
		SupportCTA:  "Sostenere",
		Overview:    "Panoramica",
		Source:      "Open source su",
		SwitchLabel: "Lingua",
	},
}
