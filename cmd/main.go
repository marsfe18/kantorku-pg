package main

import (
	"encoding/json"
	"html/template"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"kantorku/internal/database"
	"kantorku/internal/handlers"
	"kantorku/internal/middleware"
	"kantorku/internal/models"
)

func main() {
	database.InitDB()

	r := gin.Default()

	// ── Template FuncMap ────────────────────────────────────────────────────
	funcMap := template.FuncMap{
		"hasRole": func(roles []string, role string) bool {
			for _, r := range roles {
				if r == role {
					return true
				}
			}
			return false
		},
		"hasItem": func(items []string, item string) bool {
			for _, i := range items {
				if i == item {
					return true
				}
			}
			return false
		},
		"joinStrings": strings.Join,
		"formatDate": func(t time.Time) string {
			return t.Format("02 Jan 2006")
		},
		"formatDateTime": func(t time.Time) string {
			return t.Format("02 Jan 2006, 15:04")
		},
		"roleLabel": func(role string) string {
			switch role {
			case models.RoleAdmin:
				return "Admin"
			case models.RoleSupervisor:
				return "Supervisor"
			case models.RolePegawai:
				return "Pegawai"
			}
			return role
		},
		"teamLabel": func(team string) string {
			switch team {
			case models.TimProduksi:
				return "Produksi"
			case models.TimDistribusi:
				return "Distribusi"
			case models.TimIPDS:
				return "IPDS"
			case models.TimSosial:
				return "Sosial"
			case models.TimNeraca:
				return "Neraca"
			case models.TimUmum:
				return "Umum"
			}
			return team
		},
		"add": func(a, b int) int { return a + b },
		"sub": func(a, b int) int { return a - b },
		"json": func(v interface{}) template.JS {
			b, _ := json.Marshal(v)
			return template.JS(b)
		},
		"strSlice": func(items ...string) []string { return items },
	}
 
	// ── Load semua template sekaligus (termasuk partial sidebar) ───────────
	//
	// Pola folder:
	//   web/templates/sidebar/sidebar_dinamis.html   ← partial (define "sidebar")
	//   web/templates/auth/login.html
	//   web/templates/admin/admin_dashboard.html
	//   web/templates/supervisor/supervisor_dashboard.html
	//   web/templates/pegawai/pegawai_dashboard.html
	//   web/templates/error.html
	//   web/templates/coming_soon.html
	//
	templ := template.New("").Funcs(funcMap)
 
	patterns := []string{
		"web/templates/sidebar/*.html",    // ← sidebar partial dimuat pertama
		"web/templates/auth/*.html",
		"web/templates/admin/*.html",
		"web/templates/supervisor/*.html",
		"web/templates/pegawai/*.html",
		"web/templates/*.html",            // error.html, coming_soon.html, dll
	}
 
	for _, pattern := range patterns {
		files, err := filepath.Glob(pattern)
		if err != nil {
			log.Fatalf("Glob error untuk pattern %q: %v", pattern, err)
		}
		if len(files) == 0 {
			log.Printf("⚠️  Tidak ada file ditemukan untuk pattern: %s", pattern)
			continue
		}
		if _, err := templ.ParseFiles(files...); err != nil {
			log.Fatalf("Gagal parse template %q: %v", pattern, err)
		}
	}
 
	r.SetHTMLTemplate(templ)
	r.Static("/static", "./web/static")

	// Public
	r.GET("/", func(c *gin.Context) { c.Redirect(302, "/login") })
	r.GET("/login", handlers.ShowLogin)
	r.POST("/login", handlers.HandleLogin)
	r.GET("/register", handlers.ShowRegister)
	r.POST("/register", handlers.HandleRegister)
	r.POST("/logout", handlers.HandleLogout)

	// Admin
	admin := r.Group("/admin", middleware.AuthRequired(), middleware.RequireRole(models.RoleAdmin))
	{
		admin.GET("/dashboard", handlers.AdminDashboard)
		admin.GET("/users", handlers.AdminUsers)
		admin.POST("/users", handlers.AdminCreateUser)
		admin.POST("/users/:id/approve", handlers.AdminApproveUser)
		admin.POST("/users/:id/password", handlers.AdminChangePassword)
		admin.POST("/users/:id/toggle-active", handlers.AdminToggleActive)
		admin.POST("/users/:id/roles/add", handlers.AdminAddRole)
		admin.POST("/users/:id/roles/remove", handlers.AdminRemoveRole)
		admin.POST("/users/:id/teams/add", handlers.AdminAddTeam)
		admin.POST("/users/:id/teams/remove", handlers.AdminRemoveTeam)
	}

	// Supervisor
	supervisor := r.Group("/supervisor", middleware.AuthRequired(), middleware.RequireRole(models.RoleSupervisor))
	{
		supervisor.GET("/dashboard", handlers.SupervisorDashboard)
		supervisor.GET("/items", handlers.SupervisorItems)
		supervisor.POST("/items", handlers.SupervisorCreateItem)
		supervisor.GET("/items/:id", handlers.SupervisorGetItem)
		supervisor.POST("/items/:id/update", handlers.SupervisorUpdateItem)
		supervisor.POST("/items/:id/add-stock", handlers.SupervisorAddStock)
		supervisor.POST("/items/:id/delete", handlers.SupervisorDeleteItem)
		supervisor.GET("/requests", handlers.SupervisorRequests)
		supervisor.POST("/requests/:id/approve", handlers.SupervisorApproveRequest)
		supervisor.POST("/requests/:id/reject", handlers.SupervisorRejectRequest)
		supervisor.GET("/recap", handlers.SupervisorRecap)
		supervisor.GET("/recap/export", handlers.SupervisorExportRecap)
		supervisor.GET("/history", handlers.SupervisorItemHistory)
	}

	// Pegawai
	pegawai := r.Group("/pegawai", middleware.AuthRequired())
	{
		pegawai.GET("/dashboard", handlers.PegawaiDashboard)
		pegawai.GET("/items", handlers.PegawaiItems)
		pegawai.POST("/requests", handlers.PegawaiCreateRequest)
		pegawai.GET("/requests", handlers.PegawaiRequests)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("🚀 KantorKu server running on :%s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
