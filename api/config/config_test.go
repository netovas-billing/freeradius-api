package config

import (
	"os"
	"testing"
)

func bersihkanEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"API_KEY", "BASIC_AUTH_USER", "BASIC_AUTH_PASSWORD",
		"SWAGGER_USERNAME", "SWAGGER_PASSWORD", "PORT", "DB_TIMEZONE",
	} {
		t.Setenv(k, "")
		_ = os.Unsetenv(k)
	}
}

// GAGAL-TERTUTUP. Nilai contoh di .env.example tidak boleh pernah jadi kunci
// yang sah: middleware memberi scope ADMIN kalau header X-API-Key cocok, string
// itu ada di repo, dan instance ini di-DSTNAT ke PORT PUBLIK. Kalau ia berlaku,
// siapa pun di internet bisa membuat/menghapus user RADIUS dan membaca secret NAS.
func TestAPIKey_GagalTertutup(t *testing.T) {
	for _, nilai := range []string{"", contohAPIKey} {
		bersihkanEnv(t)
		if nilai != "" {
			t.Setenv("API_KEY", nilai)
		}
		Load()
		if APIKey != "" {
			t.Fatalf("API_KEY=%q menghasilkan kunci sah %q — cabang X-API-Key harus MATI", nilai, APIKey)
		}
	}
}

// Kunci sungguhan tetap dipakai.
func TestAPIKey_NilaiSungguhanDipakai(t *testing.T) {
	bersihkanEnv(t)
	t.Setenv("API_KEY", "kunci-acak-yang-panjang-sekali-123")
	Load()
	if APIKey != "kunci-acak-yang-panjang-sekali-123" {
		t.Fatalf("kunci sungguhan tidak dipakai: %q", APIKey)
	}
}

// Provisioner menulis SWAGGER_USERNAME/SWAGGER_PASSWORD (warisan aplikasi
// Python). Tanpa alias ini, instance Go membalas 401 untuk SETIAP panggilan ERP
// — backend hanya mengirim Authorization: Basic, dan cabang Basic dilewati kalau
// kedua BASIC_AUTH_* kosong.
func TestBasicAuth_AliasSwagger(t *testing.T) {
	bersihkanEnv(t)
	t.Setenv("SWAGGER_USERNAME", "admin")
	t.Setenv("SWAGGER_PASSWORD", "rahasia123")
	Load()
	if BasicAuthUser != "admin" || BasicAuthPassword != "rahasia123" {
		t.Fatalf("alias SWAGGER_* tidak dipakai: user=%q pass=%q", BasicAuthUser, BasicAuthPassword)
	}
}

// Nama baku MENANG atas alias, supaya .env yang eksplisit tidak ditimpa warisan.
func TestBasicAuth_NamaBakuMenang(t *testing.T) {
	bersihkanEnv(t)
	t.Setenv("BASIC_AUTH_USER", "baku")
	t.Setenv("BASIC_AUTH_PASSWORD", "baku-pass")
	t.Setenv("SWAGGER_USERNAME", "warisan")
	t.Setenv("SWAGGER_PASSWORD", "warisan-pass")
	Load()
	if BasicAuthUser != "baku" || BasicAuthPassword != "baku-pass" {
		t.Fatalf("alias menimpa nama baku: user=%q pass=%q", BasicAuthUser, BasicAuthPassword)
	}
}

// Port WAJIB bisa dari env: satu VM menjalankan banyak instance, satu proses per
// mitra di portnya sendiri. Kalau ini balik ter-hardcode, instance kedua dan
// seterusnya gagal bind dan model per-instance-nya runtuh.
func TestPort_DariEnv(t *testing.T) {
	bersihkanEnv(t)
	t.Setenv("PORT", "20100")
	Load()
	if Port != "20100" {
		t.Fatalf("PORT tidak dibaca dari env: %q", Port)
	}
	bersihkanEnv(t)
	Load()
	if Port != "8000" {
		t.Fatalf("bawaan port seharusnya 8000, dapat %q", Port)
	}
}

// Zona waktu DB tidak boleh ikut TZ mesin: kolom radacct naif (WIB), dan
// pergeseran 7 jam masuk langsung ke angka tagihan tanpa galat apa pun.
func TestDBTimeZone_BawaanJakartaBukanLocal(t *testing.T) {
	bersihkanEnv(t)
	Load()
	if DBTimeZone != "Asia/Jakarta" {
		t.Fatalf("bawaan DB_TIMEZONE harus Asia/Jakarta (eksplisit), dapat %q", DBTimeZone)
	}
	bersihkanEnv(t)
	t.Setenv("DB_TIMEZONE", "UTC")
	Load()
	if DBTimeZone != "UTC" {
		t.Fatalf("DB_TIMEZONE tak bisa ditimpa: %q", DBTimeZone)
	}
}
