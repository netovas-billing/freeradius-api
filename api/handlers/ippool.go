package handlers

import (
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"

	"freeradius-api/apierr"
	"freeradius-api/database"
	"freeradius-api/models"
	"freeradius-api/schemas"
)

func RegisterIPPool(r fiber.Router) {
	g := r.Group("/ippool")
	g.Get("/pools", listPools)
	g.Get("/pools/:pool", poolDetail)
	g.Get("/leases", listLeases)
	g.Delete("/leases/:id", releaseLease)
}

// listPools godoc
// @Summary List IP pools (distinct pool_name + counts)
// @Tags ippool
// @Security ApiKeyAuth
// @Security BasicAuth
// @Produce json
// @Success 200 {array} schemas.PoolSummary
// @Router /api/ippool/pools [get]
func listPools(c *fiber.Ctx) error {
	var names []string
	database.DB.Model(&models.Radippool{}).Distinct("pool_name").Order("pool_name").Pluck("pool_name", &names)

	now := time.Now()
	out := make([]schemas.PoolSummary, 0, len(names))
	for _, n := range names {
		var total, allocated int64
		database.DB.Model(&models.Radippool{}).Where("pool_name = ?", n).Count(&total)
		database.DB.Model(&models.Radippool{}).
			Where("pool_name = ? AND expiry_time > ? AND username != ''", n, now).
			Count(&allocated)
		out = append(out, schemas.PoolSummary{
			PoolName:  n,
			Total:     total,
			Allocated: allocated,
			Available: total - allocated,
		})
	}
	return c.JSON(out)
}

// poolDetail godoc
// @Summary Pool detail + entries paginated
// @Tags ippool
// @Security ApiKeyAuth
// @Security BasicAuth
// @Produce json
// @Param pool path string true "pool_name"
// @Param allocated_only query bool false "Only currently allocated (expiry > now)"
// @Param limit query int false "Limit (max 1000)"
// @Param offset query int false "Offset"
// @Success 200 {array} models.Radippool
// @Header 200 {integer} X-Total-Count "total entries in pool"
// @Router /api/ippool/pools/{pool} [get]
func poolDetail(c *fiber.Ctx) error {
	pool := c.Params("pool")
	limit := c.QueryInt("limit", 100)
	if limit > 1000 {
		limit = 1000
	}
	offset := c.QueryInt("offset", 0)

	q := database.DB.Model(&models.Radippool{}).Where("pool_name = ?", pool)
	if c.QueryBool("allocated_only", false) {
		q = q.Where("expiry_time > ? AND username != ''", time.Now())
	}

	var total int64
	q.Count(&total)
	c.Set("X-Total-Count", strconv.FormatInt(total, 10))

	var rows []models.Radippool
	q.Order("framedipaddress").Offset(offset).Limit(limit).Find(&rows)
	return c.JSON(rows)
}

// listLeases godoc
// @Summary List active leases (filter by username/pool)
// @Tags ippool
// @Security ApiKeyAuth
// @Security BasicAuth
// @Produce json
// @Param username query string false "Filter by username"
// @Param pool query string false "Filter by pool_name"
// @Param limit query int false "Limit (max 1000)"
// @Success 200 {array} models.Radippool
// @Router /api/ippool/leases [get]
func listLeases(c *fiber.Ctx) error {
	limit := c.QueryInt("limit", 100)
	if limit > 1000 {
		limit = 1000
	}
	q := database.DB.Model(&models.Radippool{}).Where("expiry_time > ? AND username != ''", time.Now())
	if v := c.Query("username"); v != "" {
		q = q.Where("username = ?", v)
	}
	if v := c.Query("pool"); v != "" {
		q = q.Where("pool_name = ?", v)
	}
	var rows []models.Radippool
	q.Order("expiry_time").Limit(limit).Find(&rows)
	return c.JSON(rows)
}

// releaseLease godoc
// @Summary Release a lease (set expiry_time to past, clear username)
// @Description Tidak menghapus row (FreeRADIUS sqlippool perlu row tetap ada),
// @Description hanya menandai lease selesai supaya bisa di-realloc.
// @Tags ippool
// @Security ApiKeyAuth
// @Security BasicAuth
// @Param id path int true "radippool.id"
// @Success 204
// @Failure 404 {object} schemas.ErrorResponse
// @Router /api/ippool/leases/{id} [delete]
func releaseLease(c *fiber.Ctx) error {
	id, _ := c.ParamsInt("id")
	var row models.Radippool
	if err := database.DB.First(&row, id).Error; err != nil {
		return apierr.NotFound(c, "Lease not found")
	}
	row.Username = ""
	row.ExpiryTime = time.Now().Add(-time.Second)
	if err := database.DB.Save(&row).Error; err != nil {
		return apierr.Internal(c, err.Error())
	}
	return c.SendStatus(204)
}
