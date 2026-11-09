package parlamentch

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/siiitschiii/parlinfo/pkg/votes"
)

// Business types whose plenary vote is posted as it stands: a demand put to the
// chamber by a member, a Fraktion, a committee or a canton, decided in one
// vote. See WithFiledBusiness.
const (
	typeMotion     = "Motion"
	typePostulat   = "Postulat"
	typeParlIv     = "Parlamentarische Initiative"
	typeStandesIv  = "Standesinitiative"
	outcomeAdopted = "angenommen"
	outcomeFailed  = "abgelehnt"
	outcomeLeave   = "Folge gegeben"
	outcomeNoLeave = "keine Folge gegeben"
)

var filedTypes = map[string]bool{typeMotion: true, typePostulat: true, typeParlIv: true, typeStandesIv: true}

// filedKinds are the filed types by the service's BusinessType code, which is
// the same in every language's feed where the type's name is not: the French
// feed calls a parliamentary initiative "Initiative parlementaire". The German
// name is what the rest of this file reads.
var filedKinds = map[int]string{3: typeStandesIv, 4: typeParlIv, 5: typeMotion, 6: typePostulat}

// filedNumberBands selects the votes on filed business by business number, on
// the server: the Vote entity carries no business type to filter on, but the
// number does by convention. Of the 2,968 votes with no Subject from December
// 2019 to November 2026, every business numbered YY.3xx was a Standesinitiative,
// YY.4xx to YY.9xx a parliamentary initiative, and YY.3xxx to YY.4xxx a motion,
// postulate or interpellation. GroupByAffair checks the type the service states
// once a vote is enriched, so a number outside the convention costs a request
// and is dropped rather than posted.
const filedNumberBands = "((BusinessNumber mod 10000) ge 300 and (BusinessNumber mod 10000) lt 1000" +
	" or (BusinessNumber mod 10000) ge 3000 and (BusinessNumber mod 10000) lt 5000)"

// WithFiledBusiness widens the votes fetched beyond WithSubjects to the
// chamber's vote on a motion, a postulate, a parliamentary initiative or a
// Standesinitiative.
//
// Those votes carry no Subject: a business of this kind is decided in one
// vote, so there is no question within it to name. They are the votes where
// the bot can show who demanded what: in the year to November 2026, 288
// motions, 68 postulates and 46 parliamentary initiatives, 203 of them filed by
// a member with a verified account, against 11 tagged final votes.
func (c *Client) WithFiledBusiness() *Client {
	c.filedBusiness = true
	return c
}

// filedKind returns the German type of a vote WithFiledBusiness admitted — no
// Subject, on business of one of the filed types — or "" for any other vote.
// The type is read from the business's type code where it was fetched, and
// otherwise from its name, which is the German one in the German feed.
func (c *Client) filedKind(v votes.Vote) string {
	if strings.TrimSpace(v.Subtitle) != "" {
		return ""
	}
	if id, err := strconv.ParseInt(v.Affair.ID, 10, 64); err == nil {
		if code, ok := c.businessCodes[id]; ok {
			return filedKinds[code]
		}
	}
	if filedTypes[v.Affair.Type] {
		return v.Affair.Type
	}
	return ""
}

// side is the position a vote's Ja or Nein stands for, as one of its meanings
// states it.
type side int

const (
	sideUnknown side = iota
	sideFor          // adopt the motion, give the initiative leave
	sideAgainst      // reject it, give it no leave
)

var (
	// A meaning names a proposal ("Antrag der Mehrheit", "Proposition de la
	// minorité Müller Leo") and, in brackets or alone, what it does. Only the
	// what is read, in German and in French, the two languages the field has
	// been served in.
	reAgainstLeave = regexp.MustCompile(`keine folge geben|nicht folge geben|ne pas donner suite`)
	reForLeave     = regexp.MustCompile(`folge geben|donner suite`)
	reAgainst      = regexp.MustCompile(`ablehnung|ablehung|rejeter`)
	reFor          = regexp.MustCompile(`annahme|adopter`)

	// Votes on a business that do not decide it whole: writing it off,
	// extending a deadline, taking part of a motion. Each states its own
	// question, which the outcome words would misstate, so they are left
	// without one. Adopting a motion as amended does decide it, and reads as
	// adoption; amended against original version puts adoption on both sides,
	// which outcomeOf refuses.
	//
	// Each must start a word, and "point" must be one, since the proposer's
	// surname shares the text: François Pointet sits in the chamber.
	reNotTheDecision = regexp.MustCompile(`(?:^|[^\p{L}])(?:abschreib|classe|frist|délai|prolong|ziffer|chiffre|punkt|points?(?:[^\p{L}]|$)|vorlage|projet|entwurf|rückweis|renvoi)`)
)

// meaningSide reads one meaning. The business's own noun must appear in a
// motion's or postulate's meaning, so that "Annahme des Postulates" on a
// motion business, which happens where a motion was turned into a postulate,
// is not read as adopting the motion.
func meaningSide(meaning, businessType string) side {
	s := strings.ToLower(strings.TrimSpace(meaning))
	if s == "" || reNotTheDecision.MatchString(s) {
		return sideUnknown
	}
	switch businessType {
	case typeParlIv, typeStandesIv:
		switch {
		case reAgainstLeave.MatchString(s):
			return sideAgainst
		case reForLeave.MatchString(s):
			return sideFor
		}
	case typeMotion, typePostulat:
		noun := "motion"
		if businessType == typePostulat {
			noun = "postulat"
		}
		if !strings.Contains(s, noun) {
			return sideUnknown
		}
		against, for_ := reAgainst.MatchString(s), reFor.MatchString(s)
		switch {
		case against && !for_:
			return sideAgainst
		case for_ && !against:
			return sideFor
		}
	}
	return sideUnknown
}

// jaPhrases say what a Ja supported, by what it did and to which kind of
// business, in each post language.
var jaPhrases = map[votes.Language]map[string][2]string{ // type → {for, against}
	votes.German: {
		typeMotion:    {"Annahme der Motion", "Ablehnung der Motion"},
		typePostulat:  {"Annahme des Postulats", "Ablehnung des Postulats"},
		typeParlIv:    {"Folge geben", "keine Folge geben"},
		typeStandesIv: {"Folge geben", "keine Folge geben"},
	},
	votes.French: {
		typeMotion:    {"adopter la motion", "rejeter la motion"},
		typePostulat:  {"adopter le postulat", "rejeter le postulat"},
		typeParlIv:    {"donner suite", "ne pas donner suite"},
		typeStandesIv: {"donner suite", "ne pas donner suite"},
	},
	votes.Italian: {
		typeMotion:    {"accogliere la mozione", "respingere la mozione"},
		typePostulat:  {"accogliere il postulato", "respingere il postulato"},
		typeParlIv:    {"dare seguito", "non dare seguito"},
		typeStandesIv: {"dare seguito", "non dare seguito"},
	},
}

// jaPhrase says in l what a Ja supported, read from its meaning: "Annahme der
// Motion", "keine Folge geben". Empty where the meaning cannot be read.
func jaPhrase(l votes.Language, businessType, meaningYes string) string {
	phrases := jaPhrases[l.OrGerman()][businessType]
	switch meaningSide(meaningYes, businessType) {
	case sideFor:
		return phrases[0]
	case sideAgainst:
		return phrases[1]
	}
	return ""
}

// outcomeOf returns what a vote did to its business, or "" when the meanings
// do not say so unambiguously: both Ja and Nein must be read, and must stand
// for opposite sides. decision is whether Ja carried, as decisionFor states it.
//
// Ja is whatever proposal was put first, not the business itself: on 25.474
// the majority proposed to give Fabian Molina's initiative no leave, Ja
// carried, and the initiative failed. Reading the side Ja stood for is what
// keeps that from being posted as "✅ Angenommen".
//
// Of the votes from December 2019 to November 2026 this reads 92% on motions,
// 94% on postulates, 87% on parliamentary initiatives and 90% on
// Standesinitiativen. The rest, partial adoptions, write-offs, deadline
// extensions and meanings too garbled to read, are posted without a verdict, as the Kantonsrat's are when its source states none.
func outcomeOf(businessType, meaningYes, meaningNo, decision string) string {
	yes, no := meaningSide(meaningYes, businessType), meaningSide(meaningNo, businessType)
	if yes == sideUnknown || no == sideUnknown || yes == no {
		return ""
	}
	won := yes
	switch decision {
	case "angenommen":
	case "abgelehnt":
		won = no
	default: // a tie: nothing carried
		return ""
	}
	leave := businessType == typeParlIv || businessType == typeStandesIv
	switch {
	case won == sideFor && leave:
		return outcomeLeave
	case won == sideAgainst && leave:
		return outcomeNoLeave
	case won == sideFor:
		return outcomeAdopted
	default:
		return outcomeFailed
	}
}
