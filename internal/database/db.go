package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "github.com/lib/pq"
)

var DB *sql.DB

// InitDB initializes PostgreSQL connection
func InitDB() *sql.DB {
	host := getEnv("DB_HOST", "localhost")
	port := getEnv("DB_PORT", "5432")
	user := getEnv("DB_USER", "postgres")
	password := getEnv("DB_PASSWORD", "123")
	dbname := getEnv("DB_NAME", "kantorku")
	sslmode := getEnv("DB_SSLMODE", "disable")

	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		host, port, user, password, dbname, sslmode,
	)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("❌ Gagal membuka koneksi database: %v", err)
	}

	if err := db.Ping(); err != nil {
		log.Fatalf("❌ Gagal terhubung ke PostgreSQL: %v\nPastikan PostgreSQL berjalan dan konfigurasi DB_* sudah benar.", err)
	}

	DB = db
	log.Println("✅ PostgreSQL terhubung")

	// Run migrations
	if err := migrate(db); err != nil {
		log.Fatalf("❌ Gagal menjalankan migrasi: %v", err)
	}
	log.Println("✅ Migrasi selesai")

	return db
}

func migrate(db *sql.DB) error {
	queries := []string{
		// Enable uuid extension
		`CREATE EXTENSION IF NOT EXISTS "pgcrypto"`,

		// Users table
		`CREATE TABLE IF NOT EXISTS users (
			id          TEXT PRIMARY KEY DEFAULT gen_random_uuid()::TEXT,
			username    TEXT UNIQUE NOT NULL,
			email       TEXT UNIQUE NOT NULL,
			password    TEXT NOT NULL,
			full_name   TEXT NOT NULL,
			is_approved BOOLEAN NOT NULL DEFAULT FALSE,
			is_active   BOOLEAN NOT NULL DEFAULT TRUE,
			created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,

		// User roles (many-to-many via array stored as rows)
		`CREATE TABLE IF NOT EXISTS user_roles (
			user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			role    TEXT NOT NULL,
			PRIMARY KEY (user_id, role)
		)`,

		// User teams
		`CREATE TABLE IF NOT EXISTS user_teams (
			user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			team    TEXT NOT NULL,
			PRIMARY KEY (user_id, team)
		)`,

		// Items table
		`CREATE TABLE IF NOT EXISTS items (
			id          TEXT PRIMARY KEY DEFAULT gen_random_uuid()::TEXT,
			title       TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			stock       INT NOT NULL DEFAULT 0,
			image_url   TEXT NOT NULL DEFAULT '',
			is_deleted  BOOLEAN NOT NULL DEFAULT FALSE,
			created_by  TEXT NOT NULL DEFAULT '',
			created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,

		// Requests table
		`CREATE TABLE IF NOT EXISTS requests (
			id          TEXT PRIMARY KEY DEFAULT gen_random_uuid()::TEXT,
			user_id     TEXT NOT NULL REFERENCES users(id),
			user_name   TEXT NOT NULL,
			item_id     TEXT NOT NULL REFERENCES items(id),
			item_title  TEXT NOT NULL,
			quantity    INT NOT NULL,
			status      TEXT NOT NULL DEFAULT 'pending',
			notes       TEXT NOT NULL DEFAULT '',
			approved_by TEXT NOT NULL DEFAULT '',
			approved_at TIMESTAMPTZ,
			created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,

		// Request user teams snapshot (for recap)
		`CREATE TABLE IF NOT EXISTS request_teams (
			request_id TEXT NOT NULL REFERENCES requests(id) ON DELETE CASCADE,
			team       TEXT NOT NULL,
			PRIMARY KEY (request_id, team)
		)`,

		// Item history
		`CREATE TABLE IF NOT EXISTS item_history (
			id           TEXT PRIMARY KEY DEFAULT gen_random_uuid()::TEXT,
			item_id      TEXT NOT NULL,
			item_title   TEXT NOT NULL,
			change_type  TEXT NOT NULL,
			change_qty   INT NOT NULL DEFAULT 0,
			stock_before INT NOT NULL DEFAULT 0,
			stock_after  INT NOT NULL DEFAULT 0,
			reason       TEXT NOT NULL DEFAULT '',
			actor_id     TEXT NOT NULL DEFAULT '',
			actor_name   TEXT NOT NULL DEFAULT '',
			created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,

		// Index for faster queries
		`CREATE INDEX IF NOT EXISTS idx_users_email ON users(email)`,
		`CREATE INDEX IF NOT EXISTS idx_users_username ON users(username)`,
		`CREATE INDEX IF NOT EXISTS idx_users_approved ON users(is_approved, is_active)`,
		`CREATE INDEX IF NOT EXISTS idx_requests_user ON requests(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_requests_status ON requests(status)`,
		`CREATE INDEX IF NOT EXISTS idx_item_history_item ON item_history(item_id)`,
	}

	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			return fmt.Errorf("query gagal:\n%s\nerror: %w", q, err)
		}
	}
	return nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
