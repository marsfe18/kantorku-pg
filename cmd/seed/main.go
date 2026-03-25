package main

import (
	"fmt"
	"log"

	"golang.org/x/crypto/bcrypt"
	"kantorku/internal/auth"
	"kantorku/internal/database"
	"kantorku/internal/models"
)

func main() {
	database.InitDB()
	db := database.DB

	// Check if admin already exists
	var count int
	db.QueryRow(`SELECT COUNT(*) FROM users WHERE username = 'admin'`).Scan(&count)
	if count > 0 {
		fmt.Println("⚠️  Admin sudah ada, skip seed.")
		return
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
	if err != nil {
		log.Fatal(err)
	}

	// Use Register then manually approve + add extra roles
	user, err := auth.Register(auth.RegisterInput{
		Username: "admin",
		Email:    "admin@kantorku.com",
		Password: "admin123",
		FullName: "Administrator",
	})
	if err != nil {
		// fallback: insert directly
		var id string
		err2 := db.QueryRow(
			`INSERT INTO users (username, email, password, full_name, is_approved, is_active)
			 VALUES ('admin', 'admin@kantorku.com', $1, 'Administrator', TRUE, TRUE) RETURNING id`,
			string(hashed),
		).Scan(&id)
		if err2 != nil {
			log.Fatalf("Gagal membuat admin: %v", err2)
		}
		// Add all roles
		for _, role := range []string{models.RoleAdmin, models.RoleSupervisor, models.RolePegawai} {
			db.Exec(`INSERT INTO user_roles (user_id, role) VALUES ($1, $2) ON CONFLICT DO NOTHING`, id, role)
		}
		fmt.Printf("✅ Admin dibuat (langsung)\nID: %s\n", id)
		return
	}

	// Approve & add admin/supervisor roles
	db.Exec(`UPDATE users SET is_approved = TRUE WHERE id = $1`, user.ID)
	for _, role := range []string{models.RoleAdmin, models.RoleSupervisor} {
		db.Exec(`INSERT INTO user_roles (user_id, role) VALUES ($1, $2) ON CONFLICT DO NOTHING`, user.ID, role)
	}

	fmt.Printf("✅ Admin berhasil dibuat!\nUsername : admin\nPassword : admin123\nEmail    : admin@kantorku.com\nID       : %s\n", user.ID)
}
