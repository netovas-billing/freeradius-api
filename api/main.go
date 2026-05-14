package main

import (
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	fiberSwagger "github.com/gofiber/swagger"

	_ "freeradius-api/docs"

	"freeradius-api/config"
	"freeradius-api/database"
	"freeradius-api/handlers"
	"freeradius-api/middleware"
	"freeradius-api/webhooks"
)

// @title FreeRADIUS API
// @version 0.3.0
// @description REST API untuk FreeRADIUS dengan multi-key auth, scope, audit log, rate limit,
// @description dashboard stats, RFC 5176 disconnect, IP pool, dan webhook event dispatch.
// @BasePath /
//
// @tag.name users
// @tag.description User & password & per-user attribute CRUD
// @tag.name groups
// @tag.description Group attributes & membership
// @tag.name nas
// @tag.description NAS clients (DB-backed alternative to clients.conf)
// @tag.name accounting
// @tag.description Session history & traffic aggregation
// @tag.name sessions
// @tag.description Live session control (Disconnect / CoA)
// @tag.name postauth
// @tag.description Post-authentication audit log
// @tag.name stats
// @tag.description Dashboard counters & top-N
// @tag.name ippool
// @tag.description radippool / sqlippool inspection
// @tag.name webhooks
// @tag.description Event subscriptions (admin only)
// @tag.name keys
// @tag.description API key management (admin only)
// @tag.name audit
// @tag.description API access audit log (admin only)
// @tag.name meta
// @tag.description Healthcheck & root
//
// @tag.name v1-users
// @tag.description Legacy /api/v1/users — radcheck row-level CRUD (nasvpntest-api compat)
// @tag.name v1-radusergroup
// @tag.description Legacy /api/v1/user-group/ — radusergroup row CRUD
// @tag.name v1-radgroupreply
// @tag.description Legacy /api/v1/group/ — radgroupreply row CRUD
// @tag.name v1-radgroupcheck
// @tag.description Legacy /api/v1/group-check/ — radgroupcheck row CRUD
// @tag.name v1-radreply
// @tag.description Legacy /api/v1/reply/ — radreply row CRUD
// @tag.name v1-nas
// @tag.description Legacy /api/v1/nas/ — NAS row CRUD (secret exposed)
// @tag.name v1-radacct
// @tag.description Legacy /api/v1/radacct/ — accounting read + user status
// @tag.name v1-disconnect
// @tag.description Legacy /api/v1/disconnect — kick user by username
//
// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name X-API-Key
func main() {
	config.Load()
	database.Init()

	poller := webhooks.NewPoller(config.WebhookPollSeconds)
	poller.Start()

	app := fiber.New(fiber.Config{
		AppName:               "freeradius-api",
		DisableStartupMessage: false,
	})
	app.Use(recover.New())
	app.Use(logger.New())

	app.Get("/", handlers.Root)
	app.Get("/health", handlers.Health)
	app.Get("/docs/*", fiberSwagger.HandlerDefault)

	api := app.Group("/api",
		middleware.APIKey,
		middleware.RateLimit(),
		middleware.ScopeByMethod,
		middleware.Audit,
	)

	// Modern endpoints
	handlers.RegisterUsers(api)
	handlers.RegisterBulk(api)
	handlers.RegisterGroups(api)
	handlers.RegisterNAS(api)
	handlers.RegisterAccounting(api)
	handlers.RegisterPostAuth(api)
	handlers.RegisterSessions(api)
	handlers.RegisterStats(api)
	handlers.RegisterIPPool(api)

	// Legacy v1 endpoints (nasvpntest-api compat)
	handlers.RegisterV1(api)

	adminOnly := api.Group("", middleware.RequireAdmin)
	handlers.RegisterKeys(adminOnly)
	handlers.RegisterAudit(adminOnly)
	handlers.RegisterWebhooks(adminOnly)

	log.Fatal(app.Listen(":8000"))
}
