package main

import (
	"encoding/json"
	"html/template"
	"log"
	"os"
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

	r.SetFuncMap(template.FuncMap{
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
			}
			return team
		},
		"add": func(a, b int) int { return a + b },
		"sub": func(a, b int) int { return a - b },
		"json": func(v interface{}) template.JS {
			b, _ := json.Marshal(v)
			return template.JS(b)
		},
	})

	r.LoadHTMLGlob("web/templates/**/*.html")
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
		supervisor.GET("/dashboard", func(c *gin.Context) {
			c.HTML(200, "coming_soon.html", gin.H{"title": "Supervisor Dashboard"})
		})
	}

	// Pegawai
	pegawai := r.Group("/pegawai", middleware.AuthRequired())
	{
		pegawai.GET("/dashboard", func(c *gin.Context) {
			c.HTML(200, "coming_soon.html", gin.H{"title": "Pegawai Dashboard"})
		})
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
