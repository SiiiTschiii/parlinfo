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
const supportURL = "https://buymeacoffee.com/parlinfo"

// The contact address, in two parts: the pages never carry it whole, so
// address harvesters scanning the HTML find nothing. A script in layout.html
// joins the parts into a mailto: link in the browser.
const (
	contactUser   = "chparlinfo"
	contactDomain = "proton.me"
)

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
	BrandLead   map[string]string // by Brand.Key: what the accounts post, under a brand page's title
	BrandNote   map[string]string // by Brand.Key: only for brands posting in several languages
	Accounts    string            // heading of a brand page's list
	Support     string            // heading of the support box
	SupportText string
	SupportCTA  string // the button
	Contact     string // footer link that opens the visitor's mail app
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
			{"Partizipation", "Mitverfolgen, ohne auf der Zuschauertribüne zu sitzen oder die Parlamentswebsite zu durchsuchen: Die Entscheide kommen zu den Bürgerinnen und Bürgern, nicht umgekehrt."}, //nolint:misspell // German: "Partizipation"
			{"Rechenschaft", "Worauf das Parlament seine Zeit verwendet: welche Vorstösse eingereicht werden und was aus ihnen wird. Denn mehr Vorstösse heisst noch nicht bessere Arbeit."},
		},
		Choose: "Parlamente",
		BrandBodies: map[string]string{
			"zh": "Gemeinderat Stadt Zürich und Kantonsrat Zürich",
			"ch": "Nationalrat",
		},
		BrandLead: map[string]string{
			"zh": "Abstimmungen im Gemeinderat der Stadt Zürich und im Kantonsrat Zürich – als Posts auf Social Media, mit den Politikerinnen und Politikern markiert, die den Vorstoss verantworten.",
			"ch": "Abstimmungen im Nationalrat – als Posts auf Social Media, mit den Politikerinnen und Politikern markiert, die den Vorstoss verantworten.",
		},
		BrandNote: map[string]string{
			"ch": "Die Beiträge erscheinen auf Deutsch, Französisch und Italienisch, je auf eigenen Accounts.",
		},
		Accounts:    "Accounts",
		Support:     "Projekt unterstützen",
		SupportText: "parlinfo ist unabhängig. Helfen Sie mit, dass es wächst – damit die Menschen sehen, wie ihre Parlamente in ihrem Namen arbeiten.",
		SupportCTA:  "Unterstützen",
		Contact:     "Kontakt",
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
			{"Participation", "Suivre sans s’asseoir à la tribune ni fouiller le site du parlement : les décisions viennent aux citoyennes et citoyens, et non l’inverse."},
			{"Responsabilité", "Ce à quoi le parlement consacre son temps : quelles interventions sont déposées et ce qu’il en advient. Car plus d’interventions ne veut pas dire meilleur travail."},
		},
		Choose: "Parlements",
		BrandBodies: map[string]string{
			"zh": "Gemeinderat Stadt Zürich (parlement de la ville) et Kantonsrat Zürich (parlement cantonal)",
			"ch": "Conseil national",
		},
		BrandLead: map[string]string{
			"zh": "Les votes du Gemeinderat de la ville de Zurich et du Kantonsrat zurichois – publiés sur les réseaux sociaux, en mentionnant les élu·e·s qui portent l’intervention.",
			"ch": "Les votes du Conseil national – publiés sur les réseaux sociaux, en mentionnant les élu·e·s qui portent l’intervention.",
		},
		BrandNote: map[string]string{
			"ch": "Les publications paraissent en allemand, en français et en italien, chaque langue sur ses propres comptes.",
		},
		Accounts:    "Comptes",
		Support:     "Soutenir le projet",
		SupportText: "parlinfo est indépendant. Aidez-nous à le faire grandir – pour que les gens voient comment leurs parlements travaillent en leur nom.",
		SupportCTA:  "Soutenir",
		Contact:     "Contact",
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
			{"Partecipazione", "Seguire senza sedersi in tribuna né cercare sul sito del parlamento: sono le decisioni ad arrivare alle cittadine e ai cittadini, non il contrario."},
			{"Responsabilità", "A cosa dedica il suo tempo il parlamento: quali atti parlamentari vengono presentati e che fine fanno. Perché più atti non significa lavoro migliore."},
		},
		Choose: "Parlamenti",
		BrandBodies: map[string]string{
			"zh": "Gemeinderat Stadt Zürich (parlamento cittadino) e Kantonsrat Zürich (parlamento cantonale)",
			"ch": "Consiglio nazionale",
		},
		BrandLead: map[string]string{
			"zh": "Le votazioni del Gemeinderat della città di Zurigo e del Kantonsrat zurighese – pubblicate sui social media, menzionando le e i politici che firmano l’atto parlamentare.",
			"ch": "Le votazioni del Consiglio nazionale – pubblicate sui social media, menzionando le e i politici che firmano l’atto parlamentare.",
		},
		BrandNote: map[string]string{
			"ch": "I post sono pubblicati in tedesco, francese e italiano, ogni lingua sui propri account.",
		},
		Accounts:    "Account",
		Support:     "Sostieni il progetto",
		SupportText: "parlinfo è indipendente. Aiutateci a farlo crescere – perché le persone vedano come i loro parlamenti lavorano in loro nome.",
		SupportCTA:  "Sostenere",
		Contact:     "Contatto",
		Overview:    "Panoramica",
		Source:      "Open source su",
		SwitchLabel: "Lingua",
	},
}
