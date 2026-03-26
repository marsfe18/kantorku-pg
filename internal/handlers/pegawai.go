// internal/handlers/pegawai_handler.go
package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"kantorku/internal/auth"
	"kantorku/internal/items"
	"kantorku/internal/middleware"
	"kantorku/internal/requests"
)

// ── Pegawai Dashboard ─────────────────────────────────────────────────────────

func PegawaiDashboard(c *gin.Context) {
	claims := c.MustGet(middleware.UserClaimsKey).(*auth.Claims)
	user, err := auth.GetUserByID(claims.UserID)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{"message": "Gagal memuat data"})
		return
	}

	myRequests, _ := requests.GetByUser(claims.UserID)

	pendingCount, approvedCount, rejectedCount := 0, 0, 0
	for _, r := range myRequests {
		switch r.Status {
		case "pending":
			pendingCount++
		case "approved":
			approvedCount++
		case "rejected":
			rejectedCount++
		}
	}

	recent := myRequests
	if len(recent) > 5 {
		recent = recent[:5]
	}

	c.HTML(http.StatusOK, "pegawai_dashboard.html", MergeH(gin.H{
		"title":          "Dashboard - KantorKu",
		"user":           user,
		"claims":         claims,
		"activePage":     "pegawai_dashboard", // ← highlight sidebar
		"pendingCount":   pendingCount,
		"approvedCount":  approvedCount,
		"rejectedCount":  rejectedCount,
		"totalRequests":  len(myRequests),
		"recentRequests": recent,
	}, SidebarData(claims)))
}

// ── Pegawai: Lihat Barang ─────────────────────────────────────────────────────

func PegawaiItems(c *gin.Context) {
	claims := c.MustGet(middleware.UserClaimsKey).(*auth.Claims)
	user, _ := auth.GetUserByID(claims.UserID)
	activeItems, _ := items.GetActive()

	c.HTML(http.StatusOK, "pegawai_items.html", MergeH(gin.H{
		"title":      "Barang Tersedia - KantorKu",
		"user":       user,
		"claims":     claims,
		"activePage": "pegawai_items", // ← highlight sidebar
		"items":      activeItems,
	}, SidebarData(claims)))
}

// ── Pegawai: Submit Request ───────────────────────────────────────────────────

func PegawaiCreateRequest(c *gin.Context) {
	claims := c.MustGet(middleware.UserClaimsKey).(*auth.Claims)

	itemID := strings.TrimSpace(c.PostForm("item_id"))
	quantityStr := c.PostForm("quantity")
	notes := strings.TrimSpace(c.PostForm("notes"))

	quantity, err := strconv.Atoi(quantityStr)
	if err != nil || quantity <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Jumlah harus berupa angka lebih dari 0"})
		return
	}

	user, err := auth.GetUserByID(claims.UserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memuat data user"})
		return
	}

	_, err = requests.Create(requests.CreateRequestInput{
		UserID:    claims.UserID,
		UserName:  user.FullName,
		UserTeams: user.Teams,
		ItemID:    itemID,
		Quantity:  quantity,
		Notes:     notes,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Permintaan berhasil diajukan"})
}

// ── Pegawai: History Permintaan ───────────────────────────────────────────────

func PegawaiRequests(c *gin.Context) {
	claims := c.MustGet(middleware.UserClaimsKey).(*auth.Claims)
	user, _ := auth.GetUserByID(claims.UserID)
	myRequests, _ := requests.GetByUser(claims.UserID)

	c.HTML(http.StatusOK, "pegawai_requests.html", MergeH(gin.H{
		"title":      "Riwayat Permintaan - KantorKu",
		"user":       user,
		"claims":     claims,
		"activePage": "pegawai_requests", // ← highlight sidebar
		"requests":   myRequests,
	}, SidebarData(claims)))
}