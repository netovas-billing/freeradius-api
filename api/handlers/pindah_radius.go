package handlers

// Pendukung "Pindah Server RADIUS" di ERP: filter radacct per NAS (bukti
// login NAS di server baru), paging keyset `after_id` (snapshot verifikasi
// yang tak melompati baris saat tabel berubah), dan hitung username radcheck
// yang tak dikenal ERP (pratinjau). Semua parameter baru opsional — tanpa
// parameter, perilaku endpoint lama tidak berubah.

import (
	"errors"
	"net"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"freeradius-api/apierr"
	"freeradius-api/database"
	"freeradius-api/models"
)

// filterRadacct = filter tambahan GET /api/v1/radacct/.
type filterRadacct struct {
	NASIP []string
	Sejak *time.Time
}

// bacaFilterRadacct memvalidasi `nasipaddress` (boleh berulang) dan
// `updated_since` (RFC3339). Nilai tak sah DITOLAK, bukan diabaikan: filter
// yang hilang diam-diam membuat ERP membaca sesi lama sebagai bukti login.
func bacaFilterRadacct(nasip []string, updatedSince string) (filterRadacct, error) {
	var f filterRadacct
	for _, s := range nasip {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if net.ParseIP(s) == nil {
			return filterRadacct{}, errors.New("nasipaddress tidak sah: " + s)
		}
		f.NASIP = append(f.NASIP, s)
	}
	if s := strings.TrimSpace(updatedSince); s != "" {
		t, err := time.Parse(time.RFC3339, s)
		if err != nil {
			return filterRadacct{}, errors.New("updated_since harus RFC3339: " + s)
		}
		f.Sejak = &t
	}
	return f, nil
}

// terapkan menambahkan filter ke query radacct. Sejak = sesi yang dimulai,
// diperbarui (interim), atau berhenti pada/sesudah t.
func (f filterRadacct) terapkan(q *gorm.DB) *gorm.DB {
	if len(f.NASIP) > 0 {
		q = q.Where("nasipaddress IN ?", f.NASIP)
	}
	if f.Sejak != nil {
		t := *f.Sejak
		q = q.Where("(acctstarttime >= ? OR acctupdatetime >= ? OR acctstoptime >= ?)", t, t, t)
	}
	return q
}

// bacaAfterID: "" → 0 (tanpa keyset); selain bilangan bulat ≥ 0 → galat.
func bacaAfterID(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n < 0 {
		return 0, errors.New("after_id harus bilangan bulat >= 0")
	}
	return n, nil
}

// terapkanAfterID: keyset `id > afterID`. afterID 0 tidak menambah filter
// (id auto-increment mulai dari 1, jadi hasilnya sama).
func terapkanAfterID(q *gorm.DB, afterID int64) *gorm.DB {
	if afterID > 0 {
		return q.Where("id > ?", afterID)
	}
	return q
}

// pagingKeyset = paging() + after_id. Bila after_id diisi, skip diabaikan
// (keyset dan offset tak boleh digabung — offset melompati baris).
func pagingKeyset(c *fiber.Ctx) (skip, limit int, afterID int64, err error) {
	skip, limit = paging(c)
	afterID, err = bacaAfterID(c.Query("after_id"))
	if err != nil {
		return 0, 0, 0, err
	}
	if c.Query("after_id") != "" {
		skip = 0
	}
	return skip, limit, afterID, nil
}

// selisihUsername menghitung username di server yang tidak ada di daftar
// dikenal, plus contoh terurut paling banyak maksContoh.
func selisihUsername(diServer, dikenal []string, maksContoh int) (int, []string) {
	set := make(map[string]struct{}, len(dikenal))
	for _, u := range dikenal {
		set[u] = struct{}{}
	}
	asing := make([]string, 0)
	for _, u := range diServer {
		if u == "" {
			continue
		}
		if _, ok := set[u]; !ok {
			asing = append(asing, u)
		}
	}
	sort.Strings(asing)
	contoh := asing
	if maksContoh < 0 {
		maksContoh = 0
	}
	if len(contoh) > maksContoh {
		contoh = contoh[:maksContoh]
	}
	return len(asing), append([]string{}, contoh...)
}

type tidakDikenalIn struct {
	Dikenal []string `json:"dikenal"`
	Contoh  *int     `json:"contoh"`
}

type tidakDikenalOut struct {
	Jumlah        int      `json:"jumlah"`
	Contoh        []string `json:"contoh"`
	TotalUsername int      `json:"total_username"`
}

// v1RadcheckTidakDikenal godoc
// @Summary Hitung username radcheck yang tak dikenal pemanggil
// @Description Dipakai pratinjau Pindah Server RADIUS di ERP. Body berisi semua username yang dikenal ERP; respons = jumlah username radcheck lain + contoh. Butuh scope write.
// @Tags v1-users
// @Security ApiKeyAuth
// @Security BasicAuth
// @Accept json
// @Produce json
// @Param body body tidakDikenalIn true "{dikenal: [...], contoh: 50 (maks 200)}"
// @Success 200 {object} tidakDikenalOut
// @Router /api/v1/radcheck/tidak-dikenal [post]
func v1RadcheckTidakDikenal(c *fiber.Ctx) error {
	var p tidakDikenalIn
	if err := c.BodyParser(&p); err != nil {
		return apierr.BadRequest(c, "Invalid request body: "+err.Error())
	}
	maks := 50
	if p.Contoh != nil {
		maks = *p.Contoh
	}
	if maks < 0 {
		maks = 0
	}
	if maks > 200 {
		maks = 200
	}
	var semua []string
	if err := database.DB.Model(&models.Radcheck{}).Distinct("username").Pluck("username", &semua).Error; err != nil {
		return apierr.Internal(c, err.Error())
	}
	n, contoh := selisihUsername(semua, p.Dikenal, maks)
	return c.JSON(tidakDikenalOut{Jumlah: n, Contoh: contoh, TotalUsername: len(semua)})
}
