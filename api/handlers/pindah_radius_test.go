package handlers

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestBacaFilterRadacct_DuaIPDanSejak(t *testing.T) {
	f, err := bacaFilterRadacct([]string{"10.0.0.2", " 103.1.1.1 "}, "2026-10-23T10:00:00+07:00")
	if err != nil || len(f.NASIP) != 2 || f.NASIP[1] != "103.1.1.1" || f.Sejak == nil {
		t.Fatalf("%+v %v", f, err)
	}
}

func TestBacaFilterRadacct_TanpaParameterPerilakuLama(t *testing.T) {
	f, err := bacaFilterRadacct(nil, "")
	if err != nil || len(f.NASIP) != 0 || f.Sejak != nil {
		t.Fatalf("%+v %v", f, err)
	}
}

func TestBacaFilterRadacct_IPTakSahDitolak(t *testing.T) {
	if _, err := bacaFilterRadacct([]string{"bukan-ip"}, ""); err == nil {
		t.Fatal("harus galat")
	}
}

func TestBacaFilterRadacct_SejakTakSahDitolakBukanDiabaikan(t *testing.T) {
	if _, err := bacaFilterRadacct(nil, "kemarin"); err == nil {
		t.Fatal("filter waktu hilang diam-diam = bukti palsu di ERP")
	}
}

func TestBacaAfterID_NegatifDanHurufDitolak(t *testing.T) {
	for _, s := range []string{"-1", "x"} {
		if _, err := bacaAfterID(s); err == nil {
			t.Errorf("%q harus galat", s)
		}
	}
	if n, err := bacaAfterID(""); n != 0 || err != nil {
		t.Fatal(n, err)
	}
	if n, err := bacaAfterID("42"); n != 42 || err != nil {
		t.Fatal(n, err)
	}
}

func TestSelisihUsername_HanyaYangTakDikenalDanContohDibatasi(t *testing.T) {
	n, c := selisihUsername([]string{"a", "b", "c", "d"}, []string{"b", "d"}, 1)
	if n != 2 || len(c) != 1 || (c[0] != "a" && c[0] != "c") {
		t.Fatalf("%d %v", n, c)
	}
}

// Parameter tak sah ditolak 400 SEBELUM menyentuh DB (tanpa DB di tes ini).
func TestV1List_ParameterTakSah400(t *testing.T) {
	app := fiber.New()
	RegisterV1(app.Group("/api"))
	for _, u := range []string{
		"/api/v1/radacct/?updated_since=kemarin",
		"/api/v1/radacct/?nasipaddress=10.0.0.1&nasipaddress=bukan-ip",
		"/api/v1/users?after_id=-1",
		"/api/v1/reply/?after_id=x",
		"/api/v1/user-group/?after_id=x",
		"/api/v1/group/?after_id=x",
		"/api/v1/nas/?after_id=x",
	} {
		resp, err := app.Test(httptest.NewRequest("GET", u, nil))
		if err != nil || resp.StatusCode != 400 {
			t.Errorf("%s: status %v err %v", u, resp.StatusCode, err)
		}
	}
}
