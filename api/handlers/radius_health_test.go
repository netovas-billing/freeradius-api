package handlers

import (
	"context"
	"crypto/hmac"
	"crypto/md5"
	"encoding/json"
	"io"
	"net"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"layeh.com/radius"
	"layeh.com/radius/rfc2865"
	"layeh.com/radius/rfc2869"

	"freeradius-api/config"
)

const secretUji = "rahasia-pemeriksa"

// maValid memeriksa Message-Authenticator Access-Request seperti FreeRADIUS
// (require_message_authenticator = yes): HMAC-MD5(secret) atas paket dengan
// atribut 80 dinolkan dan Request Authenticator apa adanya.
func maValid(t *testing.T, mentah []byte, secret string) bool {
	t.Helper()
	pkt, err := radius.Parse(mentah, []byte(secret))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	ma := rfc2869.MessageAuthenticator_Get(pkt)
	if len(ma) != 16 {
		return false
	}
	salinan := append([]byte(nil), mentah...)
	// Cari atribut 80 di paket mentah lalu nolkan nilainya.
	for i := 20; i+2 <= len(salinan); {
		tipe, panjang := salinan[i], int(salinan[i+1])
		if panjang < 2 {
			t.Fatal("atribut rusak")
		}
		if tipe == byte(rfc2869.MessageAuthenticator_Type) {
			for j := i + 2; j < i+panjang; j++ {
				salinan[j] = 0
			}
		}
		i += panjang
	}
	mac := hmac.New(md5.New, []byte(secret))
	mac.Write(salinan)
	return hmac.Equal(mac.Sum(nil), ma)
}

// radiusTiruan — server UDP yang membalas sesuai fungsi jawab (nil = diam).
// Mengembalikan alamat dan kanal paket mentah yang diterima.
func radiusTiruan(t *testing.T, secret string, jawab func(req *radius.Packet) *radius.Packet) (string, <-chan []byte) {
	t.Helper()
	conn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	masuk := make(chan []byte, 8)
	go func() {
		buf := make([]byte, radius.MaxPacketLength)
		for {
			n, dari, err := conn.ReadFrom(buf)
			if err != nil {
				return
			}
			mentah := append([]byte(nil), buf[:n]...)
			select {
			case masuk <- mentah:
			default:
			}
			if jawab == nil {
				continue
			}
			req, err := radius.Parse(mentah, []byte(secret))
			if err != nil {
				continue
			}
			resp := jawab(req)
			if resp == nil {
				continue
			}
			b, err := resp.Encode()
			if err != nil {
				continue
			}
			_, _ = conn.WriteTo(b, dari)
		}
	}()
	return conn.LocalAddr().String(), masuk
}

func tolak(req *radius.Packet) *radius.Packet { return req.Response(radius.CodeAccessReject) }

func pasangConfigUji(t *testing.T, addr, secret string) {
	t.Helper()
	lamaAddr, lamaSecret := config.RadiusHealthAddr, config.RadiusHealthSecret
	lamaBatas, lamaJeda := batasWaktuKesehatanRadius, jedaUlangKesehatanRadius
	config.RadiusHealthAddr, config.RadiusHealthSecret = addr, secret
	batasWaktuKesehatanRadius, jedaUlangKesehatanRadius = 400*time.Millisecond, 200*time.Millisecond
	t.Cleanup(func() {
		config.RadiusHealthAddr, config.RadiusHealthSecret = lamaAddr, lamaSecret
		batasWaktuKesehatanRadius, jedaUlangKesehatanRadius = lamaBatas, lamaJeda
	})
}

func panggilKesehatan(t *testing.T) (int, map[string]any) {
	t.Helper()
	app := fiber.New()
	RegisterV1(app.Group("/api"))
	resp, err := app.Test(httptest.NewRequest("GET", "/api/v1/radius/health", nil), 5000)
	if err != nil {
		t.Fatalf("panggil: %v", err)
	}
	b, _ := io.ReadAll(resp.Body)
	var isi map[string]any
	if err := json.Unmarshal(b, &isi); err != nil {
		t.Fatalf("json %q: %v", b, err)
	}
	return resp.StatusCode, isi
}

// FreeRADIUS menolak (jawaban normal untuk pengguna pemeriksa) → 200 ok.
func TestRadiusHealth_RejectBerarti200(t *testing.T) {
	addr, masuk := radiusTiruan(t, secretUji, tolak)
	pasangConfigUji(t, addr, secretUji)

	status, isi := panggilKesehatan(t)
	if status != 200 || isi["ok"] != true || isi["jawaban"] != "Access-Reject" || isi["alamat"] != addr {
		t.Fatalf("status %d isi %v", status, isi)
	}
	if _, ada := isi["rtt_ms"].(float64); !ada {
		t.Fatalf("rtt_ms tidak ada: %v", isi)
	}

	// Paket yang dikirim: User-Name tetap, sandi ada, Message-Authenticator
	// SAH — FreeRADIUS dengan require_message_authenticator = yes membuang
	// paket ber-MA salah tanpa jawaban, dan itu akan terbaca "mati".
	mentah := <-masuk
	pkt, err := radius.Parse(mentah, []byte(secretUji))
	if err != nil {
		t.Fatal(err)
	}
	if pkt.Code != radius.CodeAccessRequest {
		t.Fatalf("kode %v", pkt.Code)
	}
	if u := rfc2865.UserName_GetString(pkt); u != PenggunaKesehatanRadius {
		t.Fatalf("User-Name %q", u)
	}
	if rfc2865.UserPassword_GetString(pkt) == "" {
		t.Fatal("User-Password kosong")
	}
	if !maValid(t, mentah, secretUji) {
		t.Fatal("Message-Authenticator tidak sah")
	}
}

// Accept dan Challenge juga bukti hidup.
func TestRadiusHealth_AcceptDanChallengeJuga200(t *testing.T) {
	for _, kode := range []radius.Code{radius.CodeAccessAccept, radius.CodeAccessChallenge} {
		kode := kode
		addr, _ := radiusTiruan(t, secretUji, func(r *radius.Packet) *radius.Packet { return r.Response(kode) })
		pasangConfigUji(t, addr, secretUji)
		status, isi := panggilKesehatan(t)
		if status != 200 || isi["jawaban"] != kode.String() {
			t.Fatalf("%v: status %d isi %v", kode, status, isi)
		}
	}
}

// FreeRADIUS diam (mati / restart terus / paket dibuang) → 503 dengan pesan
// yang menunjuk tempat mencari sebabnya, dan pengiriman diulang.
func TestRadiusHealth_DiamBerarti503(t *testing.T) {
	addr, masuk := radiusTiruan(t, secretUji, nil)
	pasangConfigUji(t, addr, secretUji)

	status, isi := panggilKesehatan(t)
	if status != 503 || isi["ok"] != false || isi["alamat"] != addr {
		t.Fatalf("status %d isi %v", status, isi)
	}
	galat, _ := isi["galat"].(string)
	if !strings.Contains(galat, "tidak menjawab") || !strings.Contains(galat, "docker compose ps") {
		t.Fatalf("galat kurang jelas: %q", galat)
	}
	if n := len(masuk); n < 2 {
		t.Fatalf("paket terkirim %d kali, mau >= 2 (percobaan ulang)", n)
	}
}

// Secret berbeda di FreeRADIUS → jawabannya tak sah → 503 yang MENYEBUT
// secret, bukan "tidak menjawab".
func TestRadiusHealth_SecretBedaBerarti503Spesifik(t *testing.T) {
	addr, _ := radiusTiruan(t, "secret-lain", tolak)
	pasangConfigUji(t, addr, secretUji)
	status, isi := panggilKesehatan(t)
	galat, _ := isi["galat"].(string)
	if status != 503 || !strings.Contains(galat, "secret lain") {
		t.Fatalf("status %d isi %v", status, isi)
	}
}

// FreeRADIUS hidup tapi sql gagal (DB mati sesudah start) → 503.
func TestRadiusHealth_SQLGagalBerarti503(t *testing.T) {
	addr, _ := radiusTiruan(t, secretUji, func(r *radius.Packet) *radius.Packet {
		resp := r.Response(radius.CodeAccessReject)
		_ = rfc2865.ReplyMessage_SetString(resp, penandaSQLGagal)
		return resp
	})
	pasangConfigUji(t, addr, secretUji)
	status, isi := panggilKesehatan(t)
	galat, _ := isi["galat"].(string)
	if status != 503 || !strings.Contains(galat, "database") {
		t.Fatalf("status %d isi %v", status, isi)
	}
}

// Tanpa secret → 501, TANPA mengirim paket apa pun.
func TestRadiusHealth_TanpaSecretBerarti501(t *testing.T) {
	addr, masuk := radiusTiruan(t, secretUji, tolak)
	pasangConfigUji(t, addr, "")
	status, isi := panggilKesehatan(t)
	if status != 501 || isi["ok"] != false || isi["galat"] != galatKesehatanBelumDikonfigurasi {
		t.Fatalf("status %d isi %v", status, isi)
	}
	if _, ada := isi["alamat"]; ada {
		t.Fatalf("501 tidak membawa alamat: %v", isi)
	}
	select {
	case <-masuk:
		t.Fatal("paket terkirim padahal secret kosong")
	case <-time.After(50 * time.Millisecond):
	}
}

// Nama host yang tak bisa di-resolve (container freeradius mati → nama hilang
// dari DNS docker) → 503, bukan panik/500.
func TestRadiusHealth_HostTakDikenal503(t *testing.T) {
	_, err := periksaRadius(context.Background(), "tidak-ada.invalid:1812", secretUji, 400*time.Millisecond, 200*time.Millisecond)
	if err == nil {
		t.Fatal("harus galat")
	}
	if !strings.Contains(err.Error(), "tidak-ada.invalid:1812") {
		t.Fatalf("galat tak menyebut alamat: %v", err)
	}
}

// Port UDP tertutup (FreeRADIUS belum di-restart sesudah setup.sh menambah
// socket pemeriksa) → pesan "socket pemeriksa belum aktif" + cara restart,
// BUKAN "tidak menjawab dalam 3 detik" yang menyuruh mencari masalah lain.
// Temuan peninjau API-1, 17 Nov 2026.
func TestRadiusHealth_PortTertutupMenyuruhRestart(t *testing.T) {
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := pc.LocalAddr().String()
	pc.Close() // port kini tertutup → ICMP port-unreachable

	_, err = periksaRadius(context.Background(), addr, secretUji, 2*time.Second, 500*time.Millisecond)
	if err == nil {
		t.Fatal("harus galat")
	}
	if !strings.Contains(err.Error(), "socket pemeriksa belum aktif") {
		t.Fatalf("pesan tak menyebut socket belum aktif: %v", err)
	}
	// 127.0.0.1 = instance freeradius-manager, bukan docker.
	if !strings.Contains(err.Error(), "aktifkan-kesehatan") || strings.Contains(err.Error(), "docker compose") {
		t.Fatalf("petunjuk restart salah untuk alamat loopback: %v", err)
	}
}

func TestPetunjukRestart_DockerLawanManager(t *testing.T) {
	for addr, mau := range map[string]string{
		"freeradius:18190": "docker compose restart freeradius",
		"172.20.0.3:18190": "docker compose restart freeradius",
		"127.0.0.1:20000":  "aktifkan-kesehatan",
		"localhost:20000":  "aktifkan-kesehatan",
		"[::1]:20000":      "aktifkan-kesehatan",
	} {
		if got := petunjukRestart(addr); !strings.Contains(got, mau) {
			t.Errorf("petunjukRestart(%q) = %q, mau memuat %q", addr, got, mau)
		}
	}
}
