package zurichapi

import (
	"encoding/xml"
	"testing"
)

// geschaeftXML is two real hits from /api/geschaeft/searchdetails, trimmed of
// fields the model ignores: a Motion filed by a member, with the nil-marked
// AnzahlMitunterzeichnende most Vorstösse carry, and a Postulat filed by a
// whole Fraktion. Names keep the non-breaking space PARIS puts between first
// and last name.
const geschaeftXML = `<?xml version="1.0" encoding="utf-8"?><SearchDetailResponse numHits="2" xmlns="http://www.cmiag.ch/cdws/searchDetailResponse" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
<Hit Guid="9be11ce93061407f8b0f2d6038b11e0e"><Geschaeft OBJ_GUID="9be11ce93061407f8b0f2d6038b11e0e" xmlns="http://www.cmiag.ch/cdws/Geschaeft"><GRNr>2026/541</GRNr><Titel>Ausscheidung einer Freihaltezone auf dem Schlachthofareal</Titel><Geschaeftsart>Motion</Geschaeftsart><Geschaeftsstatus>InBearbeitung</Geschaeftsstatus><Dringlich xsi:nil="false">false</Dringlich><Beginn><Start xsi:nil="false">2026-09-30T00:00:00.000</Start><Text>30.09.2026</Text></Beginn><PendentBei>Stadtrat</PendentBei><Ablaufschritte><Aufgabe OBJ_GUID="3457461930a7485fb1685fe1fd4bb1df"><AblaufschrittName>Eingang, Frist 6 Monate</AblaufschrittName><Datum><Start xsi:nil="false">2026-09-30T00:00:00.000</Start><Text>30.09.2026</Text></Datum><FristBis><Start xsi:nil="false">2027-03-30T00:00:00.000</Start><Text>30.03.2027</Text></FristBis></Aufgabe></Ablaufschritte><Erstunterzeichner><KontaktGremium OBJ_GUID="4e3a94fca0a944579778489fcb088278"><Name>Brigitte` + " " + `Fürer</Name><Partei>Grüne</Partei><IstGremium xsi:nil="false">false</IstGremium></KontaktGremium></Erstunterzeichner><Mitunterzeichner><KontaktGremium OBJ_GUID="c8223578a62d4cadac429d5f22429c27"><Name>Tanja` + " " + `Maag</Name><Partei>AL</Partei><IstGremium xsi:nil="false">false</IstGremium></KontaktGremium><KontaktGremium OBJ_GUID="967c0420fe744c048d5472f51a601167"><Name>Pascal` + " " + `Lamprecht</Name><Partei>SP</Partei><IstGremium xsi:nil="false">false</IstGremium></KontaktGremium></Mitunterzeichner><AnzahlMitunterzeichnende xsi:nil="true"/></Geschaeft></Hit>
<Hit Guid="5c0e03b744554a06a45319afaa626f20"><Geschaeft OBJ_GUID="5c0e03b744554a06a45319afaa626f20" xmlns="http://www.cmiag.ch/cdws/Geschaeft"><GRNr>2026/109</GRNr><Titel>Sofortmassnahmen</Titel><Geschaeftsart>Postulat</Geschaeftsart><Dringlich xsi:nil="false">true</Dringlich><Beginn><Start xsi:nil="false">2026-03-11T00:00:00.000</Start><Text>11.03.2026</Text></Beginn><Erstunterzeichner><KontaktGremium OBJ_GUID="b415a00ff14141f7ae632fe01f06b4c4"><Name>Grüne</Name><Partei/><IstGremium xsi:nil="false">true</IstGremium></KontaktGremium></Erstunterzeichner><Mitunterzeichner/><AnzahlMitunterzeichnende xsi:nil="false">11</AnzahlMitunterzeichnende></Geschaeft></Hit>
</SearchDetailResponse>`

func TestGeschaeftDecodesVorstossFields(t *testing.T) {
	var resp GeschaeftSearchResponse
	if err := xml.Unmarshal([]byte(geschaeftXML), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Hits) != 2 {
		t.Fatalf("got %d hits, want 2", len(resp.Hits))
	}

	motion := resp.Hits[0].Geschaeft
	if motion.OBJGUID != "9be11ce93061407f8b0f2d6038b11e0e" {
		t.Errorf("OBJGUID = %q", motion.OBJGUID)
	}
	if motion.Dringlich {
		t.Error("Dringlich decoded true from false")
	}
	if motion.Beginn.Text != "30.09.2026" || motion.PendentBei != "Stadtrat" {
		t.Errorf("Beginn/PendentBei = %q/%q", motion.Beginn.Text, motion.PendentBei)
	}
	if len(motion.Erstunterzeichner) != 1 || motion.Erstunterzeichner[0].Partei != "Grüne" || motion.Erstunterzeichner[0].IstGremium {
		t.Errorf("Erstunterzeichner = %+v", motion.Erstunterzeichner)
	}
	if len(motion.Mitunterzeichner) != 2 || motion.Mitunterzeichner[1].OBJGUID != "967c0420fe744c048d5472f51a601167" {
		t.Errorf("Mitunterzeichner = %+v", motion.Mitunterzeichner)
	}
	if n := motion.AnzahlMitunterzeichnende; n != nil && *n != 0 {
		t.Errorf("nil-marked AnzahlMitunterzeichnende decoded as %d", *n)
	}
	if len(motion.Ablaufschritte) != 1 || motion.Ablaufschritte[0].FristBis.Text != "30.03.2027" {
		t.Errorf("Ablaufschritte = %+v", motion.Ablaufschritte)
	}

	postulat := resp.Hits[1].Geschaeft
	if !postulat.Dringlich {
		t.Error("Dringlich decoded false from true")
	}
	if len(postulat.Erstunterzeichner) != 1 || !postulat.Erstunterzeichner[0].IstGremium {
		t.Errorf("Fraktion signer not marked IstGremium: %+v", postulat.Erstunterzeichner)
	}
	if len(postulat.Mitunterzeichner) != 0 {
		t.Errorf("empty Mitunterzeichner decoded as %+v", postulat.Mitunterzeichner)
	}
	if n := postulat.AnzahlMitunterzeichnende; n == nil || *n != 11 {
		t.Errorf("AnzahlMitunterzeichnende = %v, want 11", n)
	}
}
