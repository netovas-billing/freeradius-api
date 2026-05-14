package middleware

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"time"

	"github.com/gofiber/fiber/v2"

	"freeradius-api/config"
	"freeradius-api/database"
	"freeradius-api/models"
)

// Scope constants
const (
	ScopeRead  = "read"
	ScopeWrite = "write"
	ScopeAdmin = "admin"

	BootstrapKeyName = "bootstrap"
)

func HashKey(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// APIKey — middleware autentikasi. Tiga strategi cocok:
//  1. Match dengan env API_KEY → granted "admin" sebagai bootstrap (nama = "bootstrap").
//  2. Match SHA-256(key) dengan baris di tabel api_keys yang enabled=true.
//  3. Lainnya → 401.
//
// Attach `api_key_name` dan `scope` ke Locals untuk middleware berikutnya.
func APIKey(c *fiber.Ctx) error {
	key := c.Get("X-API-Key")
	if key == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "missing X-API-Key"})
	}

	// Bootstrap: env API_KEY → admin
	if subtle.ConstantTimeCompare([]byte(key), []byte(config.APIKey)) == 1 {
		c.Locals("api_key_name", BootstrapKeyName)
		c.Locals("scope", ScopeAdmin)
		return c.Next()
	}

	// DB lookup by hash
	hash := HashKey(key)
	var row models.ApiKey
	if err := database.DB.Where("key_hash = ? AND enabled = ?", hash, true).First(&row).Error; err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid API key"})
	}

	// fire-and-forget update last_used_at
	now := time.Now()
	go func(id uint) {
		database.DB.Model(&models.ApiKey{}).Where("id = ?", id).Update("last_used_at", now)
	}(row.ID)

	c.Locals("api_key_name", row.Name)
	c.Locals("scope", row.Scope)
	return c.Next()
}

// ScopeByMethod — read: GET/HEAD; write: semua method; admin: semua.
// Diterapkan setelah APIKey. Endpoint admin-only (keys, audit) pakai
// RequireAdmin sebagai tambahan.
func ScopeByMethod(c *fiber.Ctx) error {
	scope, _ := c.Locals("scope").(string)
	if scope == ScopeAdmin {
		return c.Next()
	}
	switch c.Method() {
	case fiber.MethodGet, fiber.MethodHead, fiber.MethodOptions:
		if scope == ScopeRead || scope == ScopeWrite {
			return c.Next()
		}
	default:
		if scope == ScopeWrite {
			return c.Next()
		}
	}
	return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
		"error": "insufficient scope",
		"scope": scope,
	})
}

// RequireAdmin — hanya scope=admin.
func RequireAdmin(c *fiber.Ctx) error {
	if s, _ := c.Locals("scope").(string); s == ScopeAdmin {
		return c.Next()
	}
	return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "admin scope required"})
}
