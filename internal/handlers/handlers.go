// internal/handlers/auth_handler.go
package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"kantorku/internal/auth"
	"kantorku/internal/middleware"
	"kantorku/internal/models"
)

// ── Auth ──────────────────────────────────────────────────────────────────────

func ShowLogin(c *gin.Context) {
	if token, err := c.Cookie("token"); err == nil && token != "" {
		if claims, err := auth.ParseToken(token); err == nil {
			c.Redirect(http.StatusFound, getDashboardURL(claims.Roles))
			return
		}
	}
	c.HTML(http.StatusOK, "login.html", gin.H{"title": "Masuk - KantorKu"})
}

func HandleLogin(c *gin.Context) {
	identifier := strings.TrimSpace(c.PostForm("identifier"))
	password := c.PostForm("password")
	if identifier == "" || password == "" {
		c.HTML(http.StatusBadRequest, "login.html", gin.H{
			"title": "Masuk - KantorKu",
			"error": "Email/username dan password wajib diisi",
		})
		return
	}
	_, token, err := auth.Login(auth.LoginInput{Identifier: identifier, Password: password})
	if err != nil {
		c.HTML(http.StatusUnauthorized, "login.html", gin.H{
			"title": "Masuk - KantorKu",
			"error": err.Error(),
		})
		return
	}
	claims, _ := auth.ParseToken(token)
	c.SetCookie("token", token, 86400, "/", "", false, true)
	c.Redirect(http.StatusFound, getDashboardURL(claims.Roles))
}

func ShowRegister(c *gin.Context) {
	c.HTML(http.StatusOK, "register.html", gin.H{"title": "Daftar Akun - KantorKu"})
}

func HandleRegister(c *gin.Context) {
	username := strings.TrimSpace(c.PostForm("username"))
	email := strings.TrimSpace(c.PostForm("email"))
	password := c.PostForm("password")
	confirmPassword := c.PostForm("confirm_password")
	fullName := strings.TrimSpace(c.PostForm("full_name"))
	teams := c.PostFormArray("teams")

	validTeams := map[string]bool{
		models.TimProduksi: true, models.TimDistribusi: true,
		models.TimIPDS: true, models.TimSosial: true,
		models.TimNeraca: true, models.TimUmum: true,
	}
	var cleanTeams []string
	for _, t := range teams {
		if validTeams[t] {
			cleanTeams = append(cleanTeams, t)
		}
	}

	var errs []string
	if len(username) < 3 {
		errs = append(errs, "Username minimal 3 karakter")
	}
	if email == "" {
		errs = append(errs, "Email wajib diisi")
	}
	if fullName == "" {
		errs = append(errs, "Nama lengkap wajib diisi")
	}
	if len(password) < 6 {
		errs = append(errs, "Password minimal 6 karakter")
	}
	if password != confirmPassword {
		errs = append(errs, "Konfirmasi password tidak cocok")
	}

	formData := gin.H{"username": username, "email": email, "full_name": fullName, "teams": teams}
	if len(errs) > 0 {
		c.HTML(http.StatusBadRequest, "register.html", gin.H{
			"title": "Daftar Akun - KantorKu", "errors": errs, "form": formData,
		})
		return
	}

	_, err := auth.Register(auth.RegisterInput{
		Username: username, Email: email, Password: password,
		FullName: fullName, Teams: cleanTeams,
	})
	if err != nil {
		c.HTML(http.StatusBadRequest, "register.html", gin.H{
			"title": "Daftar Akun - KantorKu", "errors": []string{err.Error()}, "form": formData,
		})
		return
	}
	c.HTML(http.StatusOK, "register.html", gin.H{
		"title":   "Daftar Akun - KantorKu",
		"success": "Pendaftaran berhasil! Tunggu persetujuan dari admin.",
	})
}

func HandleLogout(c *gin.Context) {
	c.SetCookie("token", "", -1, "/", "", false, true)
	c.Redirect(http.StatusFound, "/login")
}

// ── Admin Dashboard ───────────────────────────────────────────────────────────

func AdminDashboard(c *gin.Context) {
	claims := c.MustGet(middleware.UserClaimsKey).(*auth.Claims)
	user, err := auth.GetUserByID(claims.UserID)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{"message": "Gagal memuat data"})
		return
	}

	allUsers, _ := auth.GetAllUsers()
	pendingUsers, _ := auth.GetPendingUsers()

	adminCount, supvCount, pegawaiCount := 0, 0, 0
	for _, u := range allUsers {
		if u.HasRole(models.RoleAdmin) {
			adminCount++
		}
		if u.HasRole(models.RoleSupervisor) {
			supvCount++
		}
		if u.HasRole(models.RolePegawai) {
			pegawaiCount++
		}
	}

	c.HTML(http.StatusOK, "admin_dashboard.html", MergeH(gin.H{
		"title":        "Dashboard Admin - KantorKu",
		"user":         user,
		"claims":       claims,
		"activePage":   "admin_dashboard", // ← untuk highlight sidebar
		"totalUsers":   len(allUsers),
		"totalPending": len(pendingUsers),
		"adminCount":   adminCount,
		"supvCount":    supvCount,
		"pegawaiCount": pegawaiCount,
		"pendingUsers": pendingUsers,
	}, SidebarData(claims)))
}

// ── Admin Users ───────────────────────────────────────────────────────────────

func AdminUsers(c *gin.Context) {
	claims := c.MustGet(middleware.UserClaimsKey).(*auth.Claims)
	user, _ := auth.GetUserByID(claims.UserID)
	allUsers, _ := auth.GetAllUsers()

	c.HTML(http.StatusOK, "admin_users.html", MergeH(gin.H{
		"title":      "Kelola Pengguna - KantorKu",
		"user":       user,
		"claims":     claims,
		"activePage": "admin_users", // ← untuk highlight sidebar
		"users":      allUsers,
		"allRoles":   []string{models.RoleAdmin, models.RoleSupervisor, models.RolePegawai},
		"allTeams":   []string{models.TimProduksi, models.TimDistribusi, models.TimIPDS, models.TimSosial, models.TimNeraca, models.TimUmum},
	}, SidebarData(claims)))
}

func AdminApproveUser(c *gin.Context) {
	userID := c.Param("id")
	if err := auth.ApproveUser(userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyetujui akun"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Akun berhasil disetujui"})
}

func AdminCreateUser(c *gin.Context) {
	username := strings.TrimSpace(c.PostForm("username"))
	email := strings.TrimSpace(c.PostForm("email"))
	password := c.PostForm("password")
	fullName := strings.TrimSpace(c.PostForm("full_name"))
	roles := c.PostFormArray("roles")
	teams := c.PostFormArray("teams")

	if username == "" || email == "" || password == "" || fullName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Semua field wajib diisi"})
		return
	}
	if len(password) < 6 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Password minimal 6 karakter"})
		return
	}

	_, err := auth.AdminCreateUser(auth.CreateUserInput{
		Username: username,
		Email:    email,
		Password: password,
		FullName: fullName,
		Roles:    roles,
		Teams:    teams,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Akun berhasil dibuat"})
}

func AdminChangePassword(c *gin.Context) {
	userID := c.Param("id")
	newPassword := c.PostForm("password")
	if len(newPassword) < 6 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Password minimal 6 karakter"})
		return
	}
	if err := auth.ChangePassword(userID, newPassword); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengubah password"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Password berhasil diubah"})
}

func AdminToggleActive(c *gin.Context) {
	userID := c.Param("id")
	user, err := auth.GetUserByID(userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User tidak ditemukan"})
		return
	}
	newActive := !user.IsActive
	if err := auth.SetUserActive(userID, newActive); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengubah status akun"})
		return
	}
	msg := "Akun dinonaktifkan"
	if newActive {
		msg = "Akun diaktifkan"
	}
	c.JSON(http.StatusOK, gin.H{"message": msg, "is_active": newActive})
}

func AdminAddRole(c *gin.Context) {
	userID := c.Param("id")
	role := c.PostForm("role")
	validRoles := map[string]bool{
		models.RoleAdmin: true, models.RoleSupervisor: true, models.RolePegawai: true,
	}
	if !validRoles[role] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Role tidak valid"})
		return
	}
	if err := auth.AddRole(userID, role); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menambahkan role"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Role berhasil ditambahkan"})
}

func AdminRemoveRole(c *gin.Context) {
	userID := c.Param("id")
	role := c.PostForm("role")
	if err := auth.RemoveRole(userID, role); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Role berhasil dihapus"})
}

func AdminAddTeam(c *gin.Context) {
	userID := c.Param("id")
	team := c.PostForm("team")
	validTeams := map[string]bool{
		models.TimProduksi: true, models.TimDistribusi: true, models.TimIPDS: true,
		models.TimSosial: true, models.TimNeraca: true, models.TimUmum: true,
	}
	if !validTeams[team] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Tim tidak valid"})
		return
	}
	if err := auth.AddTeam(userID, team); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menambahkan tim"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Tim berhasil ditambahkan"})
}

func AdminRemoveTeam(c *gin.Context) {
	userID := c.Param("id")
	team := c.PostForm("team")
	if err := auth.RemoveTeam(userID, team); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menghapus tim"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Tim berhasil dihapus"})
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func getDashboardURL(roles []string) string {
	for _, r := range roles {
		if r == models.RoleAdmin {
			return "/admin/dashboard"
		}
	}
	for _, r := range roles {
		if r == models.RoleSupervisor {
			return "/supervisor/dashboard"
		}
	}
	return "/pegawai/dashboard"
}