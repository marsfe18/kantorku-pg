package auth

import (
	"database/sql"
	"errors"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"kantorku/internal/database"
	"kantorku/internal/models"
)

var jwtSecret = []byte(getEnvOrDefault("JWT_SECRET", "kantorku-secret-key-change-in-production"))

func getEnvOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// Claims for JWT
type Claims struct {
	UserID   string   `json:"user_id"`
	Username string   `json:"username"`
	Roles    []string `json:"roles"`
	jwt.RegisteredClaims
}

type RegisterInput struct {
	Username string
	Email    string
	Password string
	FullName string
	Teams    []string
}

type LoginInput struct {
	Identifier string // email or username
	Password   string
}

// scanUser scans a single user row (without roles/teams)
func scanUser(row *sql.Row) (*models.User, error) {
	u := &models.User{}
	err := row.Scan(
		&u.ID, &u.Username, &u.Email, &u.Password,
		&u.FullName, &u.IsApproved, &u.IsActive,
		&u.CreatedAt, &u.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return u, err
}

// loadUserRelations fetches roles and teams for a user
func loadUserRelations(db *sql.DB, u *models.User) error {
	// Load roles
	rows, err := db.Query(`SELECT role FROM user_roles WHERE user_id = $1`, u.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var r string
		if err := rows.Scan(&r); err == nil {
			u.Roles = append(u.Roles, r)
		}
	}

	// Load teams
	trows, err := db.Query(`SELECT team FROM user_teams WHERE user_id = $1`, u.ID)
	if err != nil {
		return err
	}
	defer trows.Close()
	for trows.Next() {
		var t string
		if err := trows.Scan(&t); err == nil {
			u.Teams = append(u.Teams, t)
		}
	}

	if u.Roles == nil {
		u.Roles = []string{}
	}
	if u.Teams == nil {
		u.Teams = []string{}
	}
	return nil
}

// Register creates a new unapproved user account
func Register(input RegisterInput) (*models.User, error) {
	db := database.DB

	// Check username uniqueness
	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM users WHERE username = $1`, input.Username).Scan(&count)
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, errors.New("username sudah digunakan")
	}

	// Check email uniqueness
	err = db.QueryRow(`SELECT COUNT(*) FROM users WHERE email = $1`, input.Email).Scan(&count)
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, errors.New("email sudah digunakan")
	}

	// Hash password
	hashed, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	// Insert user
	var userID string
	now := time.Now()
	err = db.QueryRow(
		`INSERT INTO users (username, email, password, full_name, is_approved, is_active, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, FALSE, TRUE, $5, $6)
		 RETURNING id`,
		input.Username, input.Email, string(hashed), input.FullName, now, now,
	).Scan(&userID)
	if err != nil {
		return nil, err
	}

	// Insert default role: pegawai
	_, err = db.Exec(`INSERT INTO user_roles (user_id, role) VALUES ($1, $2)`, userID, models.RolePegawai)
	if err != nil {
		return nil, err
	}

	// Insert teams if provided
	for _, team := range input.Teams {
		db.Exec(`INSERT INTO user_teams (user_id, team) VALUES ($1, $2) ON CONFLICT DO NOTHING`, userID, team)
	}

	return GetUserByID(userID)
}

// Login authenticates user by email or username
func Login(input LoginInput) (*models.User, string, error) {
	db := database.DB

	row := db.QueryRow(
		`SELECT id, username, email, password, full_name, is_approved, is_active, created_at, updated_at
		 FROM users WHERE email = $1 OR username = $1`,
		input.Identifier,
	)

	user, err := scanUser(row)
	if err != nil {
		return nil, "", err
	}
	if user == nil {
		return nil, "", errors.New("akun tidak ditemukan")
	}

	if err := loadUserRelations(db, user); err != nil {
		return nil, "", err
	}

	if !user.IsActive {
		return nil, "", errors.New("akun dinonaktifkan")
	}
	if !user.IsApproved {
		return nil, "", errors.New("akun belum disetujui admin")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(input.Password)); err != nil {
		return nil, "", errors.New("password salah")
	}

	token, err := generateToken(user)
	if err != nil {
		return nil, "", err
	}

	return user, token, nil
}

// GetUserByID fetches a user by ID from PostgreSQL
func GetUserByID(id string) (*models.User, error) {
	db := database.DB
	row := db.QueryRow(
		`SELECT id, username, email, password, full_name, is_approved, is_active, created_at, updated_at
		 FROM users WHERE id = $1`, id,
	)
	user, err := scanUser(row)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, errors.New("user tidak ditemukan")
	}
	if err := loadUserRelations(db, user); err != nil {
		return nil, err
	}
	return user, nil
}

// GetAllUsers returns all users with their roles and teams
func GetAllUsers() ([]*models.User, error) {
	db := database.DB
	rows, err := db.Query(
		`SELECT id, username, email, password, full_name, is_approved, is_active, created_at, updated_at
		 FROM users ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*models.User
	for rows.Next() {
		u := &models.User{}
		if err := rows.Scan(
			&u.ID, &u.Username, &u.Email, &u.Password,
			&u.FullName, &u.IsApproved, &u.IsActive,
			&u.CreatedAt, &u.UpdatedAt,
		); err != nil {
			continue
		}
		if err := loadUserRelations(db, u); err != nil {
			continue
		}
		users = append(users, u)
	}
	return users, nil
}

// GetPendingUsers returns users awaiting approval
func GetPendingUsers() ([]*models.User, error) {
	db := database.DB
	rows, err := db.Query(
		`SELECT id, username, email, password, full_name, is_approved, is_active, created_at, updated_at
		 FROM users WHERE is_approved = FALSE AND is_active = TRUE ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*models.User
	for rows.Next() {
		u := &models.User{}
		if err := rows.Scan(
			&u.ID, &u.Username, &u.Email, &u.Password,
			&u.FullName, &u.IsApproved, &u.IsActive,
			&u.CreatedAt, &u.UpdatedAt,
		); err != nil {
			continue
		}
		if err := loadUserRelations(db, u); err != nil {
			continue
		}
		users = append(users, u)
	}
	return users, nil
}

// ApproveUser sets is_approved = true
func ApproveUser(userID string) error {
	_, err := database.DB.Exec(
		`UPDATE users SET is_approved = TRUE, updated_at = $1 WHERE id = $2`,
		time.Now(), userID,
	)
	return err
}

func generateToken(user *models.User) (string, error) {
	claims := &Claims{
		UserID:   user.ID,
		Username: user.Username,
		Roles:    user.Roles,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

// ParseToken validates and parses a JWT string
func ParseToken(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		return jwtSecret, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("token tidak valid")
	}
	return claims, nil
}

// ── Admin Operations ──────────────────────────────────────────────────────────

// CreateUserInput untuk admin membuat akun langsung
type CreateUserInput struct {
	Username string
	Email    string
	Password string
	FullName string
	Roles    []string
	Teams    []string
}

// AdminCreateUser membuat akun yang langsung approved
func AdminCreateUser(input CreateUserInput) (*models.User, error) {
	db := database.DB

	// Validasi unik
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM users WHERE username = $1`, input.Username).Scan(&count); err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, errors.New("username sudah digunakan")
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM users WHERE email = $1`, input.Email).Scan(&count); err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, errors.New("email sudah digunakan")
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	var userID string
	err = db.QueryRow(
		`INSERT INTO users (username, email, password, full_name, is_approved, is_active, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, TRUE, TRUE, $5, $6) RETURNING id`,
		input.Username, input.Email, string(hashed), input.FullName, now, now,
	).Scan(&userID)
	if err != nil {
		return nil, err
	}

	// Pastikan pegawai selalu ada
	roles := input.Roles
	hasPegawai := false
	for _, r := range roles {
		if r == models.RolePegawai {
			hasPegawai = true
			break
		}
	}
	if !hasPegawai {
		roles = append(roles, models.RolePegawai)
	}

	for _, role := range roles {
		db.Exec(`INSERT INTO user_roles (user_id, role) VALUES ($1, $2) ON CONFLICT DO NOTHING`, userID, role)
	}
	for _, team := range input.Teams {
		db.Exec(`INSERT INTO user_teams (user_id, team) VALUES ($1, $2) ON CONFLICT DO NOTHING`, userID, team)
	}

	return GetUserByID(userID)
}

// ChangePassword mengganti password user (oleh admin)
func ChangePassword(userID, newPassword string) error {
	hashed, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = database.DB.Exec(
		`UPDATE users SET password = $1, updated_at = $2 WHERE id = $3`,
		string(hashed), time.Now(), userID,
	)
	return err
}

// SetUserActive mengaktifkan/menonaktifkan akun
func SetUserActive(userID string, active bool) error {
	_, err := database.DB.Exec(
		`UPDATE users SET is_active = $1, updated_at = $2 WHERE id = $3`,
		active, time.Now(), userID,
	)
	return err
}

// AddRole menambahkan role ke user
func AddRole(userID, role string) error {
	_, err := database.DB.Exec(
		`INSERT INTO user_roles (user_id, role) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		userID, role,
	)
	return err
}

// RemoveRole menghapus role dari user (tidak boleh hapus pegawai)
func RemoveRole(userID, role string) error {
	if role == models.RolePegawai {
		return errors.New("role pegawai tidak dapat dihapus")
	}
	_, err := database.DB.Exec(
		`DELETE FROM user_roles WHERE user_id = $1 AND role = $2`,
		userID, role,
	)
	return err
}

// AddTeam menambahkan tim ke user
func AddTeam(userID, team string) error {
	_, err := database.DB.Exec(
		`INSERT INTO user_teams (user_id, team) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		userID, team,
	)
	return err
}

// RemoveTeam menghapus tim dari user
func RemoveTeam(userID, team string) error {
	_, err := database.DB.Exec(
		`DELETE FROM user_teams WHERE user_id = $1 AND team = $2`,
		userID, team,
	)
	return err
}
