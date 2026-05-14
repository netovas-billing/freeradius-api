package handlers

import (
	"time"

	"github.com/gofiber/fiber/v2"

	"freeradius-api/database"
	"freeradius-api/models"
	"freeradius-api/schemas"
)

func RegisterStats(r fiber.Router) {
	g := r.Group("/stats")
	g.Get("/overview", statsOverview)
	g.Get("/traffic/top-users", topUsers)
	g.Get("/auth/summary", authSummary)
}

// statsOverview godoc
// @Summary Dashboard overview (counts + traffic 24h + auth 24h)
// @Tags stats
// @Security ApiKeyAuth
// @Security BasicAuth
// @Produce json
// @Success 200 {object} schemas.StatsOverview
// @Router /api/stats/overview [get]
func statsOverview(c *fiber.Ctx) error {
	var out schemas.StatsOverview

	database.DB.Model(&models.Radcheck{}).Distinct("username").Count(&out.Users)
	{
		groups := map[string]struct{}{}
		var rows []string
		database.DB.Model(&models.Radgroupcheck{}).Distinct("groupname").Pluck("groupname", &rows)
		for _, x := range rows {
			groups[x] = struct{}{}
		}
		database.DB.Model(&models.Radgroupreply{}).Distinct("groupname").Pluck("groupname", &rows)
		for _, x := range rows {
			groups[x] = struct{}{}
		}
		database.DB.Model(&models.Radusergroup{}).Distinct("groupname").Pluck("groupname", &rows)
		for _, x := range rows {
			groups[x] = struct{}{}
		}
		out.Groups = int64(len(groups))
	}
	database.DB.Model(&models.NAS{}).Count(&out.NAS)
	database.DB.Model(&models.Radacct{}).Where("acctstoptime IS NULL").Count(&out.ActiveSessions)

	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	cutoff24h := now.Add(-24 * time.Hour)

	database.DB.Model(&models.Radacct{}).
		Where("acctstarttime >= ?", startOfDay).
		Count(&out.SessionsToday)

	var traffic struct {
		Input  int64
		Output int64
	}
	database.DB.Model(&models.Radacct{}).
		Select("COALESCE(SUM(acctinputoctets),0) AS input, COALESCE(SUM(acctoutputoctets),0) AS output").
		Where("acctstarttime >= ?", cutoff24h).
		Scan(&traffic)
	out.InputBytes24h = traffic.Input
	out.OutputBytes24h = traffic.Output

	database.DB.Model(&models.Radpostauth{}).
		Where("authdate >= ? AND reply = ?", cutoff24h, "Access-Accept").
		Count(&out.AuthAccept24h)
	database.DB.Model(&models.Radpostauth{}).
		Where("authdate >= ? AND reply = ?", cutoff24h, "Access-Reject").
		Count(&out.AuthReject24h)

	return c.JSON(out)
}

// topUsers godoc
// @Summary Top users by traffic (input+output)
// @Tags stats
// @Security ApiKeyAuth
// @Security BasicAuth
// @Produce json
// @Param limit query int false "Top N (max 100)"
// @Param since_hours query int false "Window in hours (default 24)"
// @Success 200 {array} schemas.TopUser
// @Router /api/stats/traffic/top-users [get]
func topUsers(c *fiber.Ctx) error {
	limit := c.QueryInt("limit", 10)
	if limit > 100 {
		limit = 100
	}
	since := c.QueryInt("since_hours", 24)
	cutoff := time.Now().Add(-time.Duration(since) * time.Hour)

	var rows []schemas.TopUser
	database.DB.Model(&models.Radacct{}).
		Select(
			"username, "+
				"COALESCE(SUM(acctinputoctets),0) AS input_bytes, "+
				"COALESCE(SUM(acctoutputoctets),0) AS output_bytes, "+
				"COALESCE(SUM(acctinputoctets),0) + COALESCE(SUM(acctoutputoctets),0) AS total_bytes",
		).
		Where("acctstarttime >= ?", cutoff).
		Group("username").
		Order("total_bytes DESC").
		Limit(limit).
		Scan(&rows)
	return c.JSON(rows)
}

// authSummary godoc
// @Summary Auth accept/reject counts in last N hours
// @Tags stats
// @Security ApiKeyAuth
// @Security BasicAuth
// @Produce json
// @Param since_hours query int false "Window in hours (default 24)"
// @Success 200 {object} schemas.AuthSummary
// @Router /api/stats/auth/summary [get]
func authSummary(c *fiber.Ctx) error {
	since := c.QueryInt("since_hours", 24)
	cutoff := time.Now().Add(-time.Duration(since) * time.Hour)

	var accept, reject int64
	database.DB.Model(&models.Radpostauth{}).
		Where("authdate >= ? AND reply = ?", cutoff, "Access-Accept").
		Count(&accept)
	database.DB.Model(&models.Radpostauth{}).
		Where("authdate >= ? AND reply = ?", cutoff, "Access-Reject").
		Count(&reject)

	return c.JSON(schemas.AuthSummary{
		SinceHours: int64(since),
		Accept:     accept,
		Reject:     reject,
	})
}
