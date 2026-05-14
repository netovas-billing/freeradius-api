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
	APIKey     string

	BasicAuthUser     string
	BasicAuthPassword string

	RateLimitMax           int
	RateLimitWindowSeconds int

	WebhookPollSeconds int
)

func Load() {
	_ = godotenv.Load()

	DBHost = getenv("DB_HOST", "db")
	DBPort = getenv("DB_PORT", "3306")
	DBUser = getenv("DB_USER", "radius")
	DBPassword = getenv("DB_PASSWORD", "radiuspass_ganti")
	DBName = getenv("DB_NAME", "radius")
	APIKey = getenv("API_KEY", "change-me-to-a-long-random-string")

	BasicAuthUser = getenv("BASIC_AUTH_USER", "")
	BasicAuthPassword = getenv("BASIC_AUTH_PASSWORD", "")

	RateLimitMax = getenvInt("RATE_LIMIT_MAX", 120)
	RateLimitWindowSeconds = getenvInt("RATE_LIMIT_WINDOW_SECONDS", 60)
	WebhookPollSeconds = getenvInt("WEBHOOK_POLL_SECONDS", 5)

	if APIKey == "change-me-to-a-long-random-string" {
		log.Println("WARNING: API_KEY masih default — set env API_KEY ke random string.")
	}
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
