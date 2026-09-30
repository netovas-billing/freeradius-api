package handlers

import (
	"testing"

	"layeh.com/radius"
	"layeh.com/radius/rfc2865"
	"layeh.com/radius/rfc2866"
	"layeh.com/radius/rfc2869"
)

// Paket Disconnect-Request harus MINIMAL.
//
// KEGAGALAN NYATA 30 Sep 2026. Pemutusan sesi lewat backend berhenti bekerja
// sesudah migrasi Python->Go, sementara panggilan manual ke endpoint yang sama
// berhasil. Salah satu dari dua sebabnya: versi Go menambahkan NAS-IP-Address,
// Framed-IP-Address, dan Calling-Station-Id ke paket.
//
// Kenapa itu merugikan: RFC 5176 §3 menuntut SETIAP atribut identifikasi sesi
// yang dikirim COCOK dengan sesi di NAS. Atribut tambahan bukan informasi
// bonus — ia syarat tambahan yang bisa gagal, dan tiap satu memberi NAS alasan
// baru menjawab Disconnect-NAK.
//
// Python yang digantikan repo ini hanya mengirim User-Name:
//
//	echo 'User-Name = "<u>"' | radclient <nas>:<port> disconnect <secret>
//
// dan jalur backend yang sudah terbukti melawan MikroTik produksi
// (radiusclient/coa.go) hanya User-Name + Acct-Session-Id.
func tipeAtribut(p *radius.Packet) map[radius.Type]int {
	out := map[radius.Type]int{}
	for _, a := range p.Attributes {
		out[a.Type]++
	}
	return out
}

func TestBangunPaketDisconnect_MinimalDanTanpaAtributBerbahaya(t *testing.T) {
	pkt, err := bangunPaketDisconnect("rahasia", "2609290321", "81200166")
	if err != nil {
		t.Fatalf("bangun: %v", err)
	}
	ada := tipeAtribut(pkt)

	// Yang WAJIB ada.
	for nama, tipe := range map[string]radius.Type{
		"User-Name":             rfc2865.UserName_Type,
		"Acct-Session-Id":       rfc2866.AcctSessionID_Type,
		"Message-Authenticator": rfc2869.MessageAuthenticator_Type,
	} {
		if ada[tipe] != 1 {
			t.Errorf("%s muncul %d kali, mau tepat 1", nama, ada[tipe])
		}
	}

	// Yang TIDAK BOLEH ada. Masing-masing menjelaskan kenapa.
	for nama, tipe := range map[string]radius.Type{
		// RouterOS mencocokkannya dengan identitas DIRINYA. Di topologi VPN
		// satu NAS punya alamat tunnel, LAN, dan publik sekaligus — kirim yang
		// keliru dan ia menolak, padahal sesinya ada dan secretnya benar.
		"NAS-IP-Address": rfc2865.NASIPAddress_Type,
		// Diambil dari radacct, yang bisa BASI relatif terhadap sesi hidup.
		"Framed-IP-Address": rfc2865.FramedIPAddress_Type,
		// MAC pelanggan berubah antar sesi; satu ketidakcocokan cukup.
		"Calling-Station-Id": rfc2865.CallingStationID_Type,
	} {
		if ada[tipe] != 0 {
			t.Errorf("%s ikut dikirim — itu syarat tambahan yang bisa gagal, "+
				"dan Python yang selalu berhasil tak pernah mengirimnya", nama)
		}
	}

	// Tepat tiga atribut, tidak lebih. Penjaga terhadap tambahan berikutnya
	// yang "sepertinya tidak apa-apa".
	if n := len(pkt.Attributes); n != 3 {
		t.Errorf("jumlah atribut = %d, mau 3 (User-Name, Acct-Session-Id, Message-Authenticator)", n)
	}
}

// Acct-Session-Id KOSONG harus ABSEN, bukan dikirim bernilai kosong: nilai
// kosong bukan "tanpa saringan" melainkan "saring pada string kosong", dan itu
// tak pernah cocok dengan sesi mana pun.
func TestBangunPaketDisconnect_AcctSessionIDKosongTidakDikirim(t *testing.T) {
	pkt, err := bangunPaketDisconnect("rahasia", "2609290321", "")
	if err != nil {
		t.Fatalf("bangun: %v", err)
	}
	ada := tipeAtribut(pkt)
	if ada[rfc2866.AcctSessionID_Type] != 0 {
		t.Error("Acct-Session-Id kosong tetap dikirim — tak akan pernah cocok dengan sesi mana pun")
	}
	if ada[rfc2865.UserName_Type] != 1 || ada[rfc2869.MessageAuthenticator_Type] != 1 {
		t.Error("User-Name / Message-Authenticator hilang saat tanpa Acct-Session-Id")
	}
	// Ini bentuk yang PERSIS dikirim Python: satu atribut identifikasi saja.
	if n := len(pkt.Attributes); n != 2 {
		t.Errorf("jumlah atribut = %d, mau 2", n)
	}
}

// Message-Authenticator wajib 16 byte dan bukan nol — RouterOS >= 7.17
// membuang Disconnect-Request tanpa atribut 80 yang sah, tanpa balasan.
func TestBangunPaketDisconnect_MessageAuthenticatorTerisi(t *testing.T) {
	pkt, err := bangunPaketDisconnect("rahasia", "u", "s")
	if err != nil {
		t.Fatalf("bangun: %v", err)
	}
	ma := rfc2869.MessageAuthenticator_Get(pkt)
	if len(ma) != 16 {
		t.Fatalf("panjang Message-Authenticator = %d, mau 16", len(ma))
	}
	nol := true
	for _, b := range ma {
		if b != 0 {
			nol = false
			break
		}
	}
	if nol {
		t.Error("Message-Authenticator masih 16 nol — HMAC tidak pernah dihitung")
	}
}
