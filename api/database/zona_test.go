package database

import (
	"testing"
	"time"
)

// Staging 26 Okt 2026: appliance Docker (MariaDB ber-TZ UTC) dibaca sebagai
// WIB sehingga setiap sesi mundur 7 jam. Zona kini dideteksi dari MariaDB.
func TestZonaDariSelisih(t *testing.T) {
	cases := []struct {
		detik int
		mau   string
	}{
		{0, "UTC"},
		{2, "UTC"}, // dua pembacaan jam berselisih detik
		{25200, "Asia/Jakarta"},
		{25199, "Asia/Jakarta"},
		{28800, "Asia/Makassar"},
		{32400, "Asia/Jayapura"},
		{3600, "Etc/GMT-1"},
		{-18000, "Etc/GMT+5"},
	}
	for _, c := range cases {
		got, err := ZonaDariSelisih(c.detik)
		if err != nil || got != c.mau {
			t.Errorf("selisih %d: dapat %q, %v; mau %q", c.detik, got, err, c.mau)
			continue
		}
		if _, err := time.LoadLocation(got); err != nil {
			t.Errorf("zona %q tidak bisa dimuat: %v", got, err)
		}
	}
}

func TestZonaDariSelisih_BukanJamBulatDitolak(t *testing.T) {
	if _, err := ZonaDariSelisih(5*3600 + 30*60); err == nil {
		t.Fatal("selisih 5 jam 30 menit harus ditolak, bukan ditebak")
	}
}
