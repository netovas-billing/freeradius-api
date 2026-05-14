package middleware

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"freeradius-api/config"
	"freeradius-api/database"
	"freeradius-api/models"
)

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

// APIKey — dual-auth middleware. Urutan strategi:
//  1. Header X-API-Key match env API_KEY → admin (bootstrap).
//  2. Header X-API-Key match SHA-256 di tabel api_keys → scope row.
//  3. Header Authorization: Basic <base64(user:pass)> match env
//     BASIC_AUTH_USER + BASIC_AUTH_PASSWORD → admin (basic-auth bootstrap).
//  4. Lainnya → 401, WWW-Authenticate: Basic supaya browser munculkan dialog.
func APIKey(c *fiber.Ctx) error {
	// Strategy 1+2: X-API-Key
	if key := c.Get("X-API-Key"); key != "" {
		if subtle.ConstantTimeCompare([]byte(key), []byte(config.APIKey)) == 1 {
			c.Locals("api_key_name", BootstrapKeyName)
			c.Locals("scope", ScopeAdmin)
			return c.Next()
		}
		hash := HashKey(key)
		var row models.ApiKey
		if err := database.DB.Where("key_hash = ? AND enabled = ?", hash, true).First(&row).Error; err == nil {
			now := time.Now()
			go func(id uint) {
				database.DB.Model(&models.ApiKey{}).Where("id = ?", id).Update("last_used_at", now)
			}(row.ID)
			c.Locals("api_key_name", row.Name)
			c.Locals("scope", row.Scope)
			return c.Next()
		}
		return unauthorized(c, "invalid API key")
	}

	// Strategy 3: HTTP Basic Auth
	if config.BasicAuthUser != "" && config.BasicAuthPassword != "" {
		if hdr := c.Get("Authorization"); strings.HasPrefix(hdr, "Basic ") {
			user, pass, ok := decodeBasic(strings.TrimPrefix(hdr, "Basic "))
			if ok &&
				subtle.ConstantTimeCompare([]byte(user), []byte(config.BasicAuthUser)) == 1 &&
				subtle.ConstantTimeCompare([]byte(pass), []byte(config.BasicAuthPassword)) == 1 {
				c.Locals("api_key_name", "basic:"+user)
				c.Locals("scope", ScopeAdmin)
				return c.Next()
			}
			return unauthorized(c, "invalid Basic credentials")
		}
	}

	return unauthorized(c, "missing X-API-Key or Authorization header")
}

func decodeBasic(b64 string) (user, pass string, ok bool) {
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return "", "", false
	}
	parts := strings.SplitN(string(raw), ":", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func unauthorized(c *fiber.Ctx, msg string) error {
	if config.BasicAuthUser != "" {
		c.Set("WWW-Authenticate", `Basic realm="freeradius-api"`)
	}
	return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": msg})
}

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

func RequireAdmin(c *fiber.Ctx) error {
	if s, _ := c.Locals("scope").(string); s == ScopeAdmin {
		return c.Next()
	}
	return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "admin scope required"})
}
