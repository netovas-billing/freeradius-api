package handlers

// GET /api/v1/radius/health — apakah FreeRADIUS-nya SENDIRI menjawab.
//
// KENAPA ADA (insiden 17 Nov 2026, pindah RADIUS LintasHome PALI): ERP
// memverifikasi isi server baru lewat API ini, API sehat, ERP menyatakan
// "Siap dipasang" — padahal FreeRADIUS di sebelahnya restart terus karena
// "Access denied" ke MariaDB. Router dialihkan, semua login "radius timeout",
// dan Accounting-Stop terkirim ke server mati sehingga server lama mengira
// sesi masih hidup (pelanggan ditolak Simultaneous-Use). /health milik API
// hanya mem-ping MariaDB; ia tak pernah tahu FreeRADIUS hidup atau tidak.
//
// Cara memeriksa: kirim Access-Request sungguhan untuk User-Name tetap dengan
// sandi acak (sengaja salah). Jawaban APA PUN (Accept/Reject/Challenge) berarti
// proses FreeRADIUS hidup DAN semua modulnya — termasuk sql — berhasil dimuat
// (FreeRADIUS menolak start bila sql gagal konek DB; itulah yang terjadi di
// Pali). Tidak ada yang ditulis ke DB: di stack docker paket ini masuk ke
// virtual server khusus netovas_health (freeradius/overlay/sites-available/
// netovas-health) yang tak punya post-auth, jadi radpostauth tidak tersentuh.
//
// Kontrak (dipatok — ERP bergantung padanya):
//
//	200 {"ok":true,"jawaban":"Access-Reject","rtt_ms":12,"alamat":"..."}
//	503 {"ok":false,"galat":"<pesan>","alamat":"..."}
//	501 {"ok":false,"galat":"pemeriksaan kesehatan FreeRADIUS belum dikonfigurasi (RADIUS_HEALTH_SECRET)"}

import (
	"context"
	"crypto/hmac"
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"layeh.com/radius"
	"layeh.com/radius/rfc2865"
	"layeh.com/radius/rfc2869"

	"freeradius-api/config"
)

// PenggunaKesehatanRadius — User-Name tetap paket pemeriksa. Tak pernah ada di
// radcheck; kalau suatu hari ada pun, sandinya acak per permintaan.
const PenggunaKesehatanRadius = "__netovas_health__"

// penandaSQLGagal — awalan Reply-Message yang dipasang virtual server
// netovas_health saat modul sql gagal di tengah jalan (DB mati SESUDAH
// FreeRADIUS start). FreeRADIUS tetap menjawab, tetapi login pelanggan pasti
// gagal — jadi itu dilaporkan 503, bukan "hidup".
const penandaSQLGagal = "netovas-health: sql gagal"

// Batas waktu pemeriksaan: 3 detik total, 2 percobaan (kirim ulang sesudah
// 1,5 detik). Variabel, bukan konstanta, supaya tes tidak menunggu 3 detik.
var (
	batasWaktuKesehatanRadius = 3 * time.Second
	jedaUlangKesehatanRadius  = 1500 * time.Millisecond
)

const galatKesehatanBelumDikonfigurasi = "pemeriksaan kesehatan FreeRADIUS belum dikonfigurasi (RADIUS_HEALTH_SECRET)"

// hasilPeriksaRadius — jawaban FreeRADIUS yang sah.
type hasilPeriksaRadius struct {
	Jawaban      string
	ReplyMessage string
	RTT          time.Duration
}

// galatPeriksaRadius — galat yang sudah berupa pesan Indonesia untuk operator.
type galatPeriksaRadius struct{ pesan string }

func (g *galatPeriksaRadius) Error() string { return g.pesan }

func detik(d time.Duration) string {
	s := fmt.Sprintf("%.1f", d.Seconds())
	s = strings.TrimSuffix(s, ".0")
	return strings.Replace(s, ".", ",", 1) + " detik"
}

// lengkapiMessageAuthenticatorAkses menambahkan atribut 80 ke Access-Request
// (RFC 3579 §3.2). FreeRADIUS >= 3.2.5 (mitigasi Blast-RADIUS) boleh menuntutnya
// per client — client netovas_health memakai require_message_authenticator =
// yes — dan paket tanpanya DIBUANG tanpa jawaban, yang dari sini tampak
// persis seperti FreeRADIUS mati.
//
// Beda dengan Disconnect-Request (msgauth.go): di Access-Request Authenticator
// adalah Request Authenticator ACAK yang dikirim apa adanya, jadi HMAC dihitung
// atas paket dengan Authenticator itu (BUKAN 16 nol) dan MA = 16 nol. Sandi
// (User-Password) sudah dienkripsi memakai Authenticator yang sama, jadi
// atribut itu harus dipasang SEBELUM fungsi ini dipanggil.
func lengkapiMessageAuthenticatorAkses(pkt *radius.Packet) error {
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

// bangunPaketKesehatan merakit Access-Request pemeriksa: User-Name tetap, sandi
// acak, Message-Authenticator.
func bangunPaketKesehatan(secret string) (*radius.Packet, error) {
	pkt := radius.New(radius.CodeAccessRequest, []byte(secret))
	if err := rfc2865.UserName_SetString(pkt, PenggunaKesehatanRadius); err != nil {
		return nil, err
	}
	acak := make([]byte, 16)
	if _, err := rand.Read(acak); err != nil {
		return nil, err
	}
	if err := rfc2865.UserPassword_SetString(pkt, hex.EncodeToString(acak)); err != nil {
		return nil, err
	}
	if err := lengkapiMessageAuthenticatorAkses(pkt); err != nil {
		return nil, err
	}
	return pkt, nil
}

// periksaRadius mengirim Access-Request pemeriksa ke addr dan menunggu jawaban
// paling lama batas, mengirim ulang tiap jeda.
func periksaRadius(ctx context.Context, addr, secret string, batas, jeda time.Duration) (hasilPeriksaRadius, error) {
	pkt, err := bangunPaketKesehatan(secret)
	if err != nil {
		return hasilPeriksaRadius{}, &galatPeriksaRadius{"gagal merakit paket pemeriksa: " + err.Error()}
	}

	ctx, cancel := context.WithTimeout(ctx, batas)
	defer cancel()

	// MaxPacketErrors 1: jawaban yang Response Authenticator-nya tak cocok
	// berarti FreeRADIUS memakai secret lain — itu galat konfigurasi yang harus
	// disebut, bukan ditunggu sampai batas waktu lalu dilaporkan "mati".
	klien := radius.Client{Retry: jeda, MaxPacketErrors: 1}
	mulai := time.Now()
	jwb, err := klien.Exchange(ctx, pkt, addr)
	rtt := time.Since(mulai)
	if err != nil {
		var takSah *radius.NonAuthenticResponseError
		switch {
		case errors.As(err, &takSah):
			return hasilPeriksaRadius{}, &galatPeriksaRadius{fmt.Sprintf(
				"FreeRADIUS di %s menjawab, tetapi dengan secret lain — RADIUS_HEALTH_SECRET di .env tidak sama dengan raddb; jalankan ulang setup.sh lalu 'docker compose up -d'",
				addr)}
		case errors.Is(err, context.DeadlineExceeded):
			return hasilPeriksaRadius{}, &galatPeriksaRadius{fmt.Sprintf(
				"FreeRADIUS tidak menjawab di %s dalam %s — cek 'docker compose ps' dan log /var/log/freeradius/radius.log (FreeRADIUS restart terus? jalankan 'docker compose run --rm --entrypoint freeradius freeradius -X'); bila FreeRADIUS menyala, secret pemeriksa mungkin tak cocok — jalankan ulang setup.sh",
				addr, detik(batas))}
		default:
			var dns *net.DNSError
			if errors.As(err, &dns) {
				return hasilPeriksaRadius{}, &galatPeriksaRadius{fmt.Sprintf(
					"alamat FreeRADIUS %s tidak bisa di-resolve (%v) — container freeradius kemungkinan mati atau restart terus; cek 'docker compose ps' dan 'docker compose run --rm --entrypoint freeradius freeradius -X'",
					addr, err)}
			}
			return hasilPeriksaRadius{}, &galatPeriksaRadius{fmt.Sprintf(
				"gagal menghubungi FreeRADIUS di %s: %v — cek 'docker compose ps' dan log /var/log/freeradius/radius.log",
				addr, err)}
		}
	}

	switch jwb.Code {
	case radius.CodeAccessAccept, radius.CodeAccessReject, radius.CodeAccessChallenge:
	default:
		return hasilPeriksaRadius{}, &galatPeriksaRadius{fmt.Sprintf(
			"FreeRADIUS di %s menjawab dengan kode tak terduga %s", addr, jwb.Code)}
	}
	return hasilPeriksaRadius{
		Jawaban:      jwb.Code.String(),
		ReplyMessage: rfc2865.ReplyMessage_GetString(jwb),
		RTT:          rtt,
	}, nil
}

// v1RadiusHealth godoc
// @Summary Kesehatan FreeRADIUS (bukan API)
// @Description Mengirim Access-Request pemeriksa (User-Name __netovas_health__, sandi acak) ke RADIUS_HEALTH_ADDR. Jawaban apa pun = FreeRADIUS hidup dan modulnya (termasuk sql) termuat → 200. Tak ada jawaban dalam 3 detik (2 percobaan) atau galat jaringan → 503. RADIUS_HEALTH_SECRET kosong → 501. Tidak menulis apa pun ke DB.
// @Tags meta
// @Security ApiKeyAuth
// @Security BasicAuth
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 501 {object} map[string]interface{}
// @Failure 503 {object} map[string]interface{}
// @Router /api/v1/radius/health [get]
func v1RadiusHealth(c *fiber.Ctx) error {
	addr := config.RadiusHealthAddr
	secret := config.RadiusHealthSecret
	if secret == "" {
		return c.Status(fiber.StatusNotImplemented).JSON(fiber.Map{
			"ok":    false,
			"galat": galatKesehatanBelumDikonfigurasi,
		})
	}
	hasil, err := periksaRadius(c.UserContext(), addr, secret, batasWaktuKesehatanRadius, jedaUlangKesehatanRadius)
	if err == nil && strings.HasPrefix(hasil.ReplyMessage, penandaSQLGagal) {
		err = &galatPeriksaRadius{fmt.Sprintf(
			"FreeRADIUS di %s menyala tetapi tidak bisa membaca database (modul sql gagal) — login pelanggan akan ditolak; cek 'docker compose ps db' dan log /var/log/freeradius/radius.log",
			addr)}
	}
	if err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"ok":     false,
			"galat":  err.Error(),
			"alamat": addr,
		})
	}
	return c.JSON(fiber.Map{
		"ok":      true,
		"jawaban": hasil.Jawaban,
		"rtt_ms":  hasil.RTT.Milliseconds(),
		"alamat":  addr,
	})
}
