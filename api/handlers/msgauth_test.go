package handlers

import (
	"bytes"
	"crypto/hmac"
	"crypto/md5"
	"encoding/binary"
	"testing"

	"layeh.com/radius"
	"layeh.com/radius/rfc2865"
	"layeh.com/radius/rfc2866"
	"layeh.com/radius/rfc2869"
)

const rahasiaUji = "s3cr3t-nas"

func paketUji(t *testing.T) *radius.Packet {
	t.Helper()
	pkt := radius.New(radius.CodeDisconnectRequest, []byte(rahasiaUji))
	if err := rfc2865.UserName_SetString(pkt, "pelanggan01"); err != nil {
		t.Fatalf("set UserName: %v", err)
	}
	if err := rfc2866.AcctSessionID_SetString(pkt, "81f00c21"); err != nil {
		t.Fatalf("set AcctSessionID: %v", err)
	}
	return pkt
}

// Atribut 80 harus ADA dan panjangnya tepat 16 byte.
//
// Kalau ia tidak ada, RouterOS >= 7.17 membuang paketnya tanpa balasan dan tanpa
// log — dan dari sisi kita itu tak bisa dibedakan dari NAS mati.
func TestMessageAuthenticator_Ada16Byte(t *testing.T) {
	pkt := paketUji(t)
	if got := rfc2869.MessageAuthenticator_Get(pkt); len(got) != 0 {
		t.Fatalf("belum dilengkapi tapi atribut 80 sudah ada (%d byte)", len(got))
	}
	if err := lengkapiMessageAuthenticator(pkt); err != nil {
		t.Fatalf("lengkapi: %v", err)
	}
	ma := rfc2869.MessageAuthenticator_Get(pkt)
	if len(ma) != 16 {
		t.Fatalf("atribut 80 harus 16 byte, dapat %d", len(ma))
	}
	if bytes.Equal(ma, make([]byte, 16)) {
		t.Fatal("atribut 80 masih 16 nol — HMAC tidak pernah disubstitusikan")
	}
}

// HMAC-nya harus SAMA dengan perhitungan independen yang meniru
// coa.go/buildDisconnectPacket — algoritma yang sudah terbukti di produksi.
//
// Dihitung ulang dari nol di sini (bukan memanggil kode yang diuji) supaya tes
// ini benar-benar mengunci nilainya, bukan cuma mengulang implementasinya.
func TestMessageAuthenticator_CocokDenganPerhitunganIndependen(t *testing.T) {
	pkt := paketUji(t)
	if err := lengkapiMessageAuthenticator(pkt); err != nil {
		t.Fatalf("lengkapi: %v", err)
	}
	ma := rfc2869.MessageAuthenticator_Get(pkt)

	// Rakit ulang paketnya dengan tangan, persis seperti coa.go:
	// UserName, Acct-Session-Id, lalu MA bernilai 16 nol; Authenticator 16 nol.
	atr := atribut(1, []byte("pelanggan01"))
	atr = append(atr, atribut(44, []byte("81f00c21"))...)
	offsetMA := 20 + len(atr) + 2
	atr = append(atr, atribut(80, make([]byte, 16))...)

	panjang := 20 + len(atr)
	mentah := make([]byte, 0, panjang)
	mentah = append(mentah, byte(radius.CodeDisconnectRequest), pkt.Identifier)
	var lb [2]byte
	binary.BigEndian.PutUint16(lb[:], uint16(panjang))
	mentah = append(mentah, lb[:]...)
	mentah = append(mentah, make([]byte, 16)...)
	mentah = append(mentah, atr...)

	mac := hmac.New(md5.New, []byte(rahasiaUji))
	mac.Write(mentah)
	mau := mac.Sum(nil)

	if !bytes.Equal(ma, mau) {
		t.Fatalf("HMAC tidak cocok\n dapat: %x\n mau  : %x", ma, mau)
	}
	_ = offsetMA
}

// Request Authenticator dihitung SESUDAH MA final, atas atribut yang sudah
// memuat HMAC — itu tugas Encode(). Kalau urutannya terbalik, NAS menghitung
// ulang atas isi berbeda dan menolak paketnya diam-diam.
func TestEncode_AuthenticatorDihitungSesudahMAFinal(t *testing.T) {
	pkt := paketUji(t)
	if err := lengkapiMessageAuthenticator(pkt); err != nil {
		t.Fatalf("lengkapi: %v", err)
	}
	wire, err := pkt.Encode()
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	// MD5(code|id|len || 16 nol || atribut || secret) — RFC 5176 §2.3.
	h := md5.New()
	h.Write(wire[:4])
	h.Write(make([]byte, 16))
	h.Write(wire[20:])
	h.Write([]byte(rahasiaUji))
	mau := h.Sum(nil)

	if !bytes.Equal(wire[4:20], mau) {
		t.Fatalf("Request Authenticator salah\n dapat: %x\n mau  : %x", wire[4:20], mau)
	}

	// Dan HMAC di dalam wire harus masih yang final (bukan 16 nol).
	if bytes.Contains(wire[20:], make([]byte, 16)) {
		t.Fatal("masih ada 16 nol berurutan di area atribut — MA tidak tersubstitusi di wire")
	}
}

// Authenticator acak dari New() TIDAK BOLEH ikut masuk input HMAC.
// Dua paket dengan atribut sama harus menghasilkan MA yang sama, meski New()
// mengacak Authenticator tiap kali.
func TestMessageAuthenticator_TidakTerpengaruhAuthenticatorAcak(t *testing.T) {
	var pertama []byte
	for i := 0; i < 5; i++ {
		pkt := radius.New(radius.CodeDisconnectRequest, []byte(rahasiaUji))
		pkt.Identifier = 7 // identifier ikut di-HMAC, jadi disamakan
		if err := rfc2865.UserName_SetString(pkt, "pelanggan01"); err != nil {
			t.Fatal(err)
		}
		if err := lengkapiMessageAuthenticator(pkt); err != nil {
			t.Fatal(err)
		}
		ma := rfc2869.MessageAuthenticator_Get(pkt)
		if pertama == nil {
			pertama = ma
			continue
		}
		if !bytes.Equal(ma, pertama) {
			t.Fatalf("MA berubah antar-paket dengan isi sama — Authenticator acak ikut ter-HMAC\n%x vs %x", ma, pertama)
		}
	}
}

func atribut(tipe byte, nilai []byte) []byte {
	out := make([]byte, 0, 2+len(nilai))
	out = append(out, tipe, byte(2+len(nilai)))
	return append(out, nilai...)
}
