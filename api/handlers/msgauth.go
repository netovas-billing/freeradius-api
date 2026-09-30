package handlers

import (
	"crypto/hmac"
	"crypto/md5"

	"layeh.com/radius/rfc2865"
	"layeh.com/radius/rfc2866"

	"layeh.com/radius"
	"layeh.com/radius/rfc2869"
)

// lengkapiMessageAuthenticator menambahkan atribut 80 (RFC 3579 §3.2) ke paket
// Disconnect-Request.
//
// WAJIB, bukan pelengkap: RouterOS >= 7.17 (mitigasi Blast-RADIUS,
// CVE-2024-3596) MEMBUANG Disconnect-Request tanpa atribut ini TANPA balasan dan
// TANPA log. Gejalanya "kick tidak terjadi" — mustahil didiagnosis dari sisi mana
// pun, karena dari sini paketnya terlihat terkirim dan yang terjadi hanya
// timeout. Sebelum ini ada, seluruh jalur isolir bisa mati senyap di NAS modern.
//
// URUTANNYA TIDAK BOLEH DIBALIK:
//  1. nilai MA = 16 nol DAN Authenticator = 16 nol;
//  2. HMAC-MD5(secret) atas SELURUH paket dalam keadaan itu -> jadi nilai MA;
//  3. Request Authenticator dihitung SESUDAH MA final, sebagai
//     MD5(code|id|len || 16 nol || atribut || secret) sesuai RFC 5176 §2.3.
//     Langkah 3 dikerjakan Packet.Encode(), yang dipanggil Client.Exchange().
//
// Kalau dibalik, NAS menghitung ulang HMAC atas isi yang berbeda lalu menolak
// paketnya diam-diam. Algoritma ini SENGAJA identik dengan
// backend-super-app/internal/pkg/radiusclient/coa.go (buildDisconnectPacket),
// yang sudah terbukti melawan MikroTik produksi.
func lengkapiMessageAuthenticator(pkt *radius.Packet) error {
	// Nol dulu: New() mengisinya acak, dan nilai acak itu akan masuk ke input
	// HMAC lewat MarshalBinary() kalau tidak dibersihkan.
	pkt.Authenticator = [16]byte{}
	if err := rfc2869.MessageAuthenticator_Add(pkt, make([]byte, 16)); err != nil {
		return err
	}
	b, err := pkt.MarshalBinary()
	if err != nil {
		return err
	}
	mac := hmac.New(md5.New, pkt.Secret)
	mac.Write(b)
	pkt.Set(rfc2869.MessageAuthenticator_Type, radius.Attribute(mac.Sum(nil)))
	return nil
}

// bangunPaketDisconnect — satu-satunya tempat paket Disconnect-Request dirakit.
//
// SATU tempat, karena dua handler (v1Disconnect dan disconnectSession) pernah
// merakitnya sendiri-sendiri dan langsung berselisih: yang satu mengirim
// Calling-Station-Id, yang lain tidak, dan keduanya mengirim NAS-IP-Address
// serta Framed-IP-Address yang seharusnya tidak dikirim sama sekali. Selisih
// seperti itu tak terlihat dari tes mana pun selama perakitnya dua.
//
// Isinya sengaja MINIMAL — lihat uraian panjang di v1.go: RFC 5176 menuntut
// setiap atribut identifikasi yang dikirim COCOK dengan sesi di NAS, sehingga
// tiap atribut tambahan adalah syarat baru yang bisa gagal. Python yang
// digantikan repo ini hanya mengirim User-Name.
func bangunPaketDisconnect(secret, username, acctSessionID string) (*radius.Packet, error) {
	pkt := radius.New(radius.CodeDisconnectRequest, []byte(secret))
	if err := rfc2865.UserName_SetString(pkt, username); err != nil {
		return nil, err
	}
	// Nilai KOSONG bukan "tanpa saringan" melainkan "saring pada string
	// kosong", dan itu tak pernah cocok dengan sesi mana pun.
	if acctSessionID != "" {
		if err := rfc2866.AcctSessionID_SetString(pkt, acctSessionID); err != nil {
			return nil, err
		}
	}
	if err := lengkapiMessageAuthenticator(pkt); err != nil {
		return nil, err
	}
	return pkt, nil
}
