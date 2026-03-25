package handlers

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"kantorku/internal/auth"
	"kantorku/internal/items"
	"kantorku/internal/middleware"
	"kantorku/internal/models"
	"kantorku/internal/requests"
)

// ── Supervisor Dashboard ──────────────────────────────────────────────────────

func SupervisorDashboard(c *gin.Context) {
	claims := c.MustGet(middleware.UserClaimsKey).(*auth.Claims)
	user, err := auth.GetUserByID(claims.UserID)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{"message": "Gagal memuat data"})
		return
	}

	allItems, _ := items.GetAll()
	activeCount, deletedCount, lowStockCount := 0, 0, 0
	for _, it := range allItems {
		if it.IsDeleted {
			deletedCount++
		} else {
			activeCount++
			if it.Stock <= 5 {
				lowStockCount++
			}
		}
	}

	// Ambil 5 barang dengan stok terendah (aktif)
	var lowItems []*models.Item
	for _, it := range allItems {
		if !it.IsDeleted && it.Stock <= 5 {
			lowItems = append(lowItems, it)
			if len(lowItems) >= 5 {
				break
			}
		}
	}

	c.HTML(http.StatusOK, "supervisor_dashboard.html", gin.H{
		"title":         "Dashboard Supervisor - KantorKu",
		"user":          user,
		"claims":        claims,
		"activeCount":   activeCount,
		"deletedCount":  deletedCount,
		"lowStockCount": lowStockCount,
		"lowItems":      lowItems,
	})
}

// ── Item List ─────────────────────────────────────────────────────────────────

func SupervisorItems(c *gin.Context) {
	claims := c.MustGet(middleware.UserClaimsKey).(*auth.Claims)
	user, _ := auth.GetUserByID(claims.UserID)
	allItems, _ := items.GetAll()

	c.HTML(http.StatusOK, "supervisor_items.html", gin.H{
		"title":  "Kelola Barang - KantorKu",
		"user":   user,
		"claims": claims,
		"items":  allItems,
	})
}

// ── Create Item ───────────────────────────────────────────────────────────────

func SupervisorCreateItem(c *gin.Context) {
	claims := c.MustGet(middleware.UserClaimsKey).(*auth.Claims)

	title := strings.TrimSpace(c.PostForm("title"))
	description := strings.TrimSpace(c.PostForm("description"))
	stockStr := c.PostForm("stock")

	stock, err := strconv.Atoi(stockStr)
	if err != nil || stock < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Stok harus berupa angka >= 0"})
		return
	}

	// Handle image upload
	imageURL, err := handleImageUpload(c, "image")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	item, err := items.Create(items.CreateItemInput{
		Title:       title,
		Description: description,
		Stock:       stock,
		ImageURL:    imageURL,
		ActorID:     claims.UserID,
		ActorName:   claims.Username,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Barang berhasil ditambahkan", "id": item.ID})
}

// ── Update Item ───────────────────────────────────────────────────────────────

func SupervisorUpdateItem(c *gin.Context) {
	claims := c.MustGet(middleware.UserClaimsKey).(*auth.Claims)
	id := c.Param("id")

	title := strings.TrimSpace(c.PostForm("title"))
	description := strings.TrimSpace(c.PostForm("description"))
	stockStr := c.PostForm("stock")

	stock, err := strconv.Atoi(stockStr)
	if err != nil || stock < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Stok harus berupa angka >= 0"})
		return
	}

	// Handle image upload (opsional, boleh kosong)
	imageURL, err := handleImageUpload(c, "image")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	_, err = items.Update(id, items.UpdateItemInput{
		Title:       title,
		Description: description,
		Stock:       stock,
		ImageURL:    imageURL, // kosong = tidak ubah
		ActorID:     claims.UserID,
		ActorName:   claims.Username,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Barang berhasil diperbarui"})
}

// ── Soft Delete ───────────────────────────────────────────────────────────────

func SupervisorDeleteItem(c *gin.Context) {
	claims := c.MustGet(middleware.UserClaimsKey).(*auth.Claims)
	id := c.Param("id")

	if err := items.SoftDelete(id, claims.UserID, claims.Username); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Barang berhasil dihapus"})
}

// ── Get Item (JSON, untuk modal edit) ────────────────────────────────────────

func SupervisorGetItem(c *gin.Context) {
	id := c.Param("id")
	item, err := items.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, item)
}

// ── Image Upload Helper ───────────────────────────────────────────────────────

func handleImageUpload(c *gin.Context, field string) (string, error) {
	file, header, err := c.Request.FormFile(field)
	if err != nil {
		// Tidak ada file = oke, tidak wajib
		return "", nil
	}
	defer file.Close()

	// Validasi ekstensi
	ext := strings.ToLower(filepath.Ext(header.Filename))
	allowed := map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".webp": true, ".gif": true}
	if !allowed[ext] {
		return "", fmt.Errorf("format gambar tidak didukung (jpg, png, webp, gif)")
	}

	// Validasi ukuran (max 2MB)
	if header.Size > 2*1024*1024 {
		return "", fmt.Errorf("ukuran gambar maksimal 2MB")
	}

	// Buat folder uploads jika belum ada
	uploadDir := "web/static/uploads"
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		return "", fmt.Errorf("gagal membuat folder upload")
	}

	// Nama file unik: timestamp + original name
	filename := fmt.Sprintf("%d_%s", time.Now().UnixNano(), header.Filename)
	filename = sanitizeFilename(filename)
	savePath := filepath.Join(uploadDir, filename)

	if err := c.SaveUploadedFile(header, savePath); err != nil {
		return "", fmt.Errorf("gagal menyimpan gambar")
	}

	return "/static/uploads/" + filename, nil
}

func sanitizeFilename(name string) string {
	replacer := strings.NewReplacer(" ", "_", "/", "_", "\\", "_", ":", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_")
	return replacer.Replace(name)
}


// ── Supervisor: Requests Management ──────────────────────────────────────────

func SupervisorRequests(c *gin.Context) {
	claims := c.MustGet(middleware.UserClaimsKey).(*auth.Claims)
	user, _ := auth.GetUserByID(claims.UserID)

	allRequests, _ := requests.GetAll()
	stats, _ := requests.GetStats()

	c.HTML(http.StatusOK, "supervisor_requests.html", gin.H{
		"title":    "Kelola Permintaan - KantorKu",
		"user":     user,
		"claims":   claims,
		"requests": allRequests,
		"stats":    stats,
	})
}

func SupervisorApproveRequest(c *gin.Context) {
	claims := c.MustGet(middleware.UserClaimsKey).(*auth.Claims)
	id := c.Param("id")

	if err := requests.Approve(id, claims.UserID, claims.Username); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Permintaan disetujui"})
}

func SupervisorRejectRequest(c *gin.Context) {
	claims := c.MustGet(middleware.UserClaimsKey).(*auth.Claims)
	id := c.Param("id")

	if err := requests.Reject(id, claims.UserID, claims.Username); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Permintaan ditolak"})
}
