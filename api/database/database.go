package database

import (
	"fmt"
	"log"
	"net/url"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"freeradius-api/config"
	"freeradius-api/models"
)

var DB *gorm.DB

func Init() {
	dsn := fmt.Sprintf(
		// loc EKSPLISIT, bukan Local: kolom waktu radacct naif (WIB) dan
		// loc=Local membuat penafsirannya ikut TZ sistem — bergeser 7 jam di VM
		// ber-TZ UTC, langsung masuk ke angka tagihan tanpa galat apa pun.
		"%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=%s",
		config.DBUser, config.DBPassword, config.DBHost, config.DBPort, config.DBName,
		url.QueryEscape(config.DBTimeZone),
	)

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Warn),
	})
	if err != nil {
		log.Fatalf("failed to connect to db: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("failed to get sql.DB handle: %v", err)
	}
	sqlDB.SetMaxOpenConns(20)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(time.Hour)

	DB = db

	// AutoMigrate hanya tabel milik API ini (api_keys, api_audit_log).
	// Tabel FreeRADIUS tidak di-migrate — biarkan dikelola oleh schema.sql
	// supaya struktur tetap sinkron dengan FreeRADIUS upstream.
	if err := db.AutoMigrate(
		&models.ApiKey{},
		&models.ApiAuditLog{},
		&models.Webhook{},
		&models.WebhookDelivery{},
	); err != nil {
		// Pesan yang bisa DITINDAK, bukan sekadar "automigrate failed".
		//
		// Sebab paling sering sejauh ini: user basis data instance hanya punya
		// SELECT/INSERT/UPDATE/DELETE. Tabel-tabel di atas milik API ini sendiri
		// dan dibuat saat start, jadi tanpa CREATE prosesnya mati di sini — dan
		// yang terlihat operator cuma "API service gagal start!" tanpa petunjuk
		// apa pun. Aplikasi Python tak pernah butuh hak itu karena ia tak pernah
		// membuat tabel, jadi instance lama tidak memperlihatkan gejala ini.
		log.Fatalf("automigrate gagal: %v\n"+
			"  Tabel yang dibuat API ini: api_keys, api_audit_log, webhooks, webhook_deliveries.\n"+
			"  Kalau galatnya soal izin, user DB %q kurang hak. Perbaiki sebagai root MariaDB:\n"+
			"    GRANT SELECT,INSERT,UPDATE,DELETE,CREATE,ALTER,INDEX,REFERENCES ON `%s`.* TO '%s'@'localhost';\n"+
			"    FLUSH PRIVILEGES;",
			err, config.DBUser, config.DBName, config.DBUser)
	}
}
