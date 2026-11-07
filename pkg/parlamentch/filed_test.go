package parlamentch

import (
	"strings"
	"testing"

	"github.com/siiitschiii/parlinfo/pkg/votes"
)

// Meanings as the service has served them, typos included. decision is
// whether Ja carried; want is what that did to the business.
func TestOutcomeOf(t *testing.T) {
	tests := []struct {
		name, typ, yes, no, decision, want string
	}{
		{"motion adopted", typeMotion, "Annahme der Motion", "Ablehnung der Motion", "angenommen", outcomeAdopted},
		{"motion rejected", typeMotion, "Adopter la motion", "Rejeter la motion (selon proposition du CF)", "abgelehnt", outcomeFailed},
		// Ja is the majority's proposal to reject: Ja carrying means the
		// motion failed, and Nein carrying means it passed.
		{"Ja proposed rejecting, carried", typeMotion,
			"Antrag der Mehrheit und des Bundesrates (Ablehnung der Motion)", "Antrag der Minderheit Bregy (Annahme der Motion)", "angenommen", outcomeFailed},
		{"Ja proposed rejecting, failed", typeMotion,
			"Antrag der Mehrheit und des Bundesrates (Ablehnung der Motion)", "Antrag der Minderheit Bregy (Annahme der Motion)", "abgelehnt", outcomeAdopted},
		{"typo in the meaning", typeMotion, "Annahme der Motion", "Ablehung der Motion", "angenommen", outcomeAdopted},
		{"postulate", typePostulat, "Antrag der Kommission (Annahme des Postulates)", "Antrag des Bundesrates (Ablehnung des Postulates)", "angenommen", outcomeAdopted},
		// 25.474: the majority proposed no leave, Ja carried 128:62.
		{"initiative given no leave", typeParlIv,
			"Proposition de la majorité (ne pas donner suite)", "Proposition de la minorité (donner suite)", "angenommen", outcomeNoLeave},
		{"initiative given leave", typeParlIv,
			"Antrag der Mehrheit (keine Folge geben)", "Antrag der Minderheit Hurni (Folge geben)", "abgelehnt", outcomeLeave},
		// 24.323: the majority proposed leave, Ja carried 119:72.
		{"Standesinitiative given leave", typeStandesIv,
			"Proposition de la majorité (donner suite)", "Proposition de la minorité Amoos (ne pas donner suite)", "angenommen", outcomeLeave},

		// Not the decision on the business, or not readable: no verdict.
		{"tie", typeMotion, "Annahme der Motion", "Ablehnung der Motion", "", ""},
		{"no meanings", typeMotion, "", "", "angenommen", ""},
		{"Nein unreadable", typeMotion, "Proposition de la majorité (aprouver la motion)", "Proposition de la minorité Sommaruga Carlo (rejeter)", "angenommen", ""},
		{"a motion turned postulate", typeMotion, "Annahme des Postulates", "Ablehnung des Postulats", "angenommen", ""},
		{"partial adoption", typeMotion,
			"Antrag der Mehrheit (Annahme der Ziffer 1, 3 und 4 und Abänderung der Ziffer 2)", "Antrag der Minderheit Page (Ablehnung der Motion)", "angenommen", ""},
		{"amended", typeMotion,
			"Proposition de la majorité (adopter la motion modifiée)", "Proposition de la minorité Källin et du Conseil fédéral (rejeter la motion)", "angenommen", outcomeAdopted},
		{"original against amended", typeMotion,
			"Proposition de la majorité (adopter la motion dans sa version initiale)", "Proposition de la minorité (adopter la motion modifiée)", "angenommen", ""},
		{"not the decision", typeMotion, "Proposition de la commission (classer)", "Proposition Gugger (ne pas classer)", "angenommen", ""},
		{"write-off", typeParlIv, "Antrag der Kommission (abschreiben)", "Antrag Aeschi Thomas (nicht abschreiben)", "angenommen", ""},
		{"deadline", typeParlIv, "Antrag der Mehrheit (Frist um 2 Jahre verlängern)", "Antrag der Minderheit de Courten (abschreiben)", "angenommen", ""},
		{"a proposer named like a guard word", typeParlIv,
			"Proposition de la majorité (donner suite)", "Proposition de la minorité Pointet (ne pas donner suite)", "angenommen", outcomeLeave},
		{"a point of a motion", typeMotion,
			"Proposition de la commission (adopter le point 2 de la motion)", "Proposition du Conseil fédéral (rejeter la motion)", "angenommen", ""},
		{"both sides the same", typeParlIv, "Antrag der Mehrheit (Folge geben)", "Antrag der Minderheit (Folge geben)", "angenommen", ""},
	}
	for _, tt := range tests {
		if got := outcomeOf(tt.typ, tt.yes, tt.no, tt.decision); got != tt.want {
			t.Errorf("%s: got %q, want %q", tt.name, got, tt.want)
		}
	}
}

// A filed vote states what it did to the business and drops the meaning of
// Ja; one whose meanings cannot be read keeps the meaning and loses the
// verdict, since its Ja/Nein result would say nothing a reader can trust.
func TestLabel_FiledVote(t *testing.T) {
	c := New(testJurisdiction)
	read := votes.Vote{SourceID: "1", Decision: "angenommen", Affair: votes.Affair{Type: typeParlIv}}
	unread := votes.Vote{SourceID: "2", Decision: "angenommen", Affair: votes.Affair{Type: typeMotion}}
	plain := votes.Vote{SourceID: "4", Decision: "angenommen", Affair: votes.Affair{Type: typeParlIv}}
	c.labels["4"] = voteLabel{meaningYes: "Annahme der Vorlage", meaningNo: "Ablehnung der Vorlage"}
	bill := votes.Vote{SourceID: "3", Decision: "angenommen", Subtitle: "Gesamtabstimmung", Affair: votes.Affair{Type: typeMotion}}
	c.labels["1"] = voteLabel{meaningYes: "Proposition de la majorité (ne pas donner suite)", meaningNo: "Proposition de la minorité (donner suite)"}
	c.labels["2"] = voteLabel{meaningYes: "Proposition de la majorité (aprouver la motion)", meaningNo: "Proposition de la minorité (rejeter)"}
	c.labels["3"] = voteLabel{meaningYes: "Adopter le projet"}
	group := []votes.Vote{read, unread, bill, plain}
	c.label(group)

	if group[0].Outcome != outcomeNoLeave || group[0].Subtitle != "" || group[0].Decision != "angenommen" {
		t.Errorf("read: Outcome %q, Subtitle %q, Decision %q", group[0].Outcome, group[0].Subtitle, group[0].Decision)
	}
	if group[1].Outcome != "" || group[1].Decision != "" || !strings.Contains(group[1].Subtitle, "(Ja: Proposition de la majorité (aprouver la motion))") {
		t.Errorf("unread: Outcome %q, Subtitle %q, Decision %q", group[1].Outcome, group[1].Subtitle, group[1].Decision)
	}
	// No verdict, so even a plain meaning is kept: it alone says what Ja was.
	if group[3].Decision != "" || group[3].Subtitle != "(Ja: Annahme der Vorlage)" {
		t.Errorf("plain: Subtitle %q, Decision %q", group[3].Subtitle, group[3].Decision)
	}
	// A vote with a Subject is a bill's, labelled as before.
	if group[2].Outcome != "" || group[2].Subtitle != "Gesamtabstimmung" || group[2].Decision != "angenommen" {
		t.Errorf("bill: Outcome %q, Subtitle %q, Decision %q", group[2].Outcome, group[2].Subtitle, group[2].Decision)
	}
}

func TestVoteFilter_FiledBusiness(t *testing.T) {
	c := New(testJurisdiction).WithSubjects("Schlussabstimmung").WithFiledBusiness()
	f := c.voteFilter()
	if !strings.Contains(f, "startswith(tolower(Subject),'schlussabstimmung') eq true or (Subject eq null and ((BusinessNumber mod 10000) ge 300") {
		t.Errorf("filter %q does not add the filed business beside the subjects", f)
	}
	if strings.Contains(New(testJurisdiction).WithSubjects("Schlussabstimmung").voteFilter(), "Subject eq null") {
		t.Error("filed business selected without WithFiledBusiness")
	}
}

func TestFilingCanton(t *testing.T) {
	n := func(v int64) *int64 { return &v }
	for _, tt := range []struct {
		roles []businessRoleDTO
		want  string
	}{
		{[]businessRoleDTO{{Role: roleAuthor, CantonNumber: n(17)}}, "St. Gallen"},
		{[]businessRoleDTO{{Role: roleAuthor, CantonNumber: n(27)}}, "Tessin"},
		{[]businessRoleDTO{{Role: roleAuthor, CantonNumber: n(15)}}, "Appenzell Ausserrhoden"},
		{[]businessRoleDTO{{Role: roleCoSigned, CantonNumber: n(1)}}, ""},
		{[]businessRoleDTO{{Role: roleAuthor, MemberCouncilNumber: n(214)}}, ""},
	} {
		if got := filingCanton(tt.roles); got != tt.want {
			t.Errorf("filingCanton(%v) = %q, want %q", tt.roles[0], got, tt.want)
		}
	}
}

// A vote without a Subject is kept only on the four filed types. One whose
// business type the service would not give fails the run rather than being
// dropped, since a dropped vote is never asked about again.
func TestGroupByAffair_FiledVoteType(t *testing.T) {
	c, _ := newTestClient(t)
	c.WithFiledBusiness()

	// 25.059 is the Federal Council's business: no Subject, not filed.
	gov := votes.Vote{SourceID: "38953", Affair: votes.Affair{ID: "20250059", Number: "25.059"}}
	groups, err := c.GroupByAffair([]votes.Vote{gov})
	if err != nil || len(groups) != 0 {
		t.Errorf("government business without a Subject: groups %v, err %v; want it dropped", groups, err)
	}

	// The test server knows no business 26.4038, as when /Business fails.
	unknown := votes.Vote{SourceID: "38953", Affair: votes.Affair{ID: "20264038", Number: "26.4038"}}
	if _, err := c.GroupByAffair([]votes.Vote{unknown}); err == nil || !strings.Contains(err.Error(), "26.4038") {
		t.Errorf("unknown type: err %v, want one naming 26.4038", err)
	}
}
