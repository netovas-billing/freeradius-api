package database

import (
	"fmt"
	"math"
)

// ZonaDariSelisih — nama zona IANA untuk kolom waktu radacct, dari selisih
// NOW() − UTC_TIMESTAMP() MariaDB (detik).
//
// Kolom radacct NAIF: FreeRADIUS menulisnya dalam zona SERVER MariaDB
// (NOW() / FROM_UNIXTIME). Zona itu beda antar mesin — VM RADIUS Manager
// ber-TZ WIB, appliance Docker ber-TZ UTC — dan salah tebak menggeser setiap
// sesi 7 jam tanpa galat (staging 26 Okt 2026: bukti login Pindah RADIUS ERP
// tak pernah terhitung). Jadi zonanya DITANYAKAN ke MariaDB, bukan ditebak.
//
// Selisih dibulatkan ke 15 menit (detik dari dua pembacaan jam). Zona
// Indonesia memakai nama wilayahnya; jam bulat lain memakai Etc/GMT (tandanya
// TERBALIK menurut konvensi POSIX: UTC+7 = Etc/GMT-7). Selisih bukan jam
// bulat → galat, supaya pemanggil menolak menebak.
func ZonaDariSelisih(detik int) (string, error) {
	menit := int(math.Round(float64(detik)/900)) * 15
	switch menit {
	case 0:
		return "UTC", nil
	case 7 * 60:
		return "Asia/Jakarta", nil
	case 8 * 60:
		return "Asia/Makassar", nil
	case 9 * 60:
		return "Asia/Jayapura", nil
	}
	if menit%60 != 0 || menit < -12*60 || menit > 14*60 {
		return "", fmt.Errorf("selisih zona MariaDB %d menit tidak didukung — isi DB_TIMEZONE secara eksplisit", menit)
	}
	jam := menit / 60
	if jam > 0 {
		return fmt.Sprintf("Etc/GMT-%d", jam), nil
	}
	return fmt.Sprintf("Etc/GMT+%d", -jam), nil
}
