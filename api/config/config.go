package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

var (
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	DBTimeZone string
	APIKey     string

	// Port — port HTTP yang didengarkan. WAJIB dari env: satu VM menjalankan
	// BANYAK instance freeradius-api, satu proses per mitra, masing-masing di
	// portnya sendiri (skema ERP: base 20000 + k*1000). Sebelumnya nilainya
	// di-hardcode ":8000" di main.go, yang membuat model itu mustahil.
	Port string

	BasicAuthUser     string
	BasicAuthPassword string

	RateLimitMax           int
	RateLimitWindowSeconds int

	WebhookPollSeconds int

	// RadiusHealthAddr & RadiusHealthSecret — tujuan dan secret Access-Request
	// pemeriksa GET /api/v1/radius/health (handlers/radius_health.go).
	//
	// API sehat TIDAK berarti FreeRADIUS sehat (insiden Pali 17 Nov 2026: api
	// menjawab, FreeRADIUS restart terus karena "Access denied" ke MariaDB, ERP
	// menyatakan server baru "Siap dipasang", router dialihkan, semua login
	// "radius timeout"). Endpoint ini menanyai FreeRADIUS-nya sendiri.
	//
	// Stack docker: setup.sh membangkitkan secret di .env dan menanamnya ke
	// raddb/sites-available/netovas-health, yang mendengarkan port INTERNAL
	// 18190 (tak diterbitkan ke host) — karena itu bawaan alamatnya
	// freeradius:18190, bukan 1812. freeradius-manager mengisinya
	// 127.0.0.1:<port auth instance>. Secret KOSONG = pemeriksaan belum
	// dikonfigurasi (501), bukan "sehat".
	RadiusHealthAddr   string
	RadiusHealthSecret string
)

func Load() {
	_ = godotenv.Load()

	DBHost = getenv("DB_HOST", "db")
	DBPort = getenv("DB_PORT", "3306")
	DBUser = getenv("DB_USER", "radius")
	DBPassword = getenv("DB_PASSWORD", "radiuspass_ganti")
	DBName = getenv("DB_NAME", "radius")

	// DBTimeZone — zona yang dipakai menafsirkan kolom waktu radacct.
	//
	// Kolom radacct NAIF (tanpa offset) dan FreeRADIUS menulisnya dalam waktu
	// lokal WIB. Sebelumnya DSN memakai loc=Local, sehingga penafsirannya ikut
	// TZ SISTEM: benar di VM ber-TZ Asia/Jakarta, tapi bergeser 7 jam di VM
	// ber-TZ UTC (bawaan Debian bersih) — dan pergeseran itu langsung masuk ke
	// angka tagihan tanpa satu pun galat.
	//
	// KOSONG (bawaan) = DIDETEKSI dari MariaDB saat start (database.deteksiZona):
	// VM WIB maupun container UTC terbaca benar tanpa diatur. Isi hanya untuk
	// menimpa hasil deteksi.
	DBTimeZone = getenv("DB_TIMEZONE", "")

	Port = getenv("PORT", "8000")

	// SWAGGER_USERNAME/SWAGGER_PASSWORD diterima sebagai ALIAS.
	//
	// Itu nama yang ditulis provisioner (freeradius-manager menaruhnya di .env
	// setiap instance; warisan aplikasi Python). Tanpa alias ini, instance Go
	// membalas 401 untuk SETIAP panggilan ERP — backend hanya mengirim
	// Authorization: Basic, dan cabang Basic di middleware dilewati kalau kedua
	// variabel BASIC_AUTH_* kosong. Menerima keduanya berarti satu berkas .env
	// jalan untuk kedua runtime.
	BasicAuthUser = getenvAlias("BASIC_AUTH_USER", "SWAGGER_USERNAME", "")
	BasicAuthPassword = getenvAlias("BASIC_AUTH_PASSWORD", "SWAGGER_PASSWORD", "")

	RateLimitMax = getenvInt("RATE_LIMIT_MAX", 120)
	RateLimitWindowSeconds = getenvInt("RATE_LIMIT_WINDOW_SECONDS", 60)
	WebhookPollSeconds = getenvInt("WEBHOOK_POLL_SECONDS", 5)

	RadiusHealthAddr = getenv("RADIUS_HEALTH_ADDR", "freeradius:18190")
	RadiusHealthSecret = getenv("RADIUS_HEALTH_SECRET", "")

	muatAPIKey()

}

// contohAPIKey — nilai contoh di .env.example. TIDAK BOLEH jadi kunci yang sah.
const contohAPIKey = "change-me-to-a-long-random-string"

// muatAPIKey — GAGAL-TERTUTUP.
//
// Sebelumnya APIKey jatuh ke contohAPIKey saat env kosong, dan middleware
// memberi scope ADMIN kepada siapa pun yang mengirim header X-API-Key berisi
// string itu (auth.go: cocok -> ScopeAdmin). String itu ada di .env.example dan
// di repo publik, sementara instance ini di-DSTNAT ke PORT PUBLIK — jadi
// instance yang .env-nya tanpa API_KEY berarti kendali admin penuh atas RADIUS
// dari internet: buat/hapus user, baca secret NAS, tendang siapa saja.
//
// Dengan dikosongkan, cabang X-API-Key tak pernah bisa cocok (pembandingnya
// beda panjang, dan header kosong tak masuk cabang itu), sehingga satu-satunya
// jalan masuk tinggal Basic auth atau kunci di tabel api_keys.
func muatAPIKey() {
	APIKey = getenv("API_KEY", "")
	if APIKey == "" || APIKey == contohAPIKey {
		APIKey = ""
		log.Println("API_KEY tidak diset (atau masih nilai contoh) — autentikasi X-API-Key DIMATIKAN. " +
			"Pakai Basic auth, atau set API_KEY ke string acak.")
	}
}

func getenvAlias(key, alias, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	if v := os.Getenv(alias); v != "" {
		return v
	}
	return fallback
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getenvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
