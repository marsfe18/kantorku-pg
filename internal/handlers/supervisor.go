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
	"github.com/xuri/excelize/v2"
	"kantorku/internal/auth"
	"kantorku/internal/database"
	"kantorku/internal/items"
	"kantorku/internal/middleware"
	"kantorku/internal/models"
	"kantorku/internal/recap"
	"kantorku/internal/requests"
)

// ── Dashboard ─────────────────────────────────────────────────────────────────

func SupervisorDashboard(c *gin.Context) {
	claims := c.MustGet(middleware.UserClaimsKey).(*auth.Claims)
	user, err := auth.GetUserByID(claims.UserID)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{"message": "Gagal memuat data"})
		return
	}
	allItems, _ := items.GetAll()
	activeCount, deletedCount, lowStockCount := 0, 0, 0
	var lowItems []*models.Item
	for _, it := range allItems {
		if it.IsDeleted {
			deletedCount++
		} else {
			activeCount++
			if it.Stock <= 5 {
				lowStockCount++
				if len(lowItems) < 5 {
					lowItems = append(lowItems, it)
				}
			}
		}
	}
	stats, _ := requests.GetStats()
	c.HTML(http.StatusOK, "supervisor_dashboard.html", gin.H{
		"title": "Dashboard Supervisor - KantorKu", "user": user, "claims": claims,
		"activeCount": activeCount, "deletedCount": deletedCount,
		"lowStockCount": lowStockCount, "lowItems": lowItems, "reqStats": stats,
	})
}

// ── Items ─────────────────────────────────────────────────────────────────────

func SupervisorItems(c *gin.Context) {
	claims := c.MustGet(middleware.UserClaimsKey).(*auth.Claims)
	user, _ := auth.GetUserByID(claims.UserID)
	allItems, _ := items.GetAll()
	c.HTML(http.StatusOK, "supervisor_items.html", gin.H{
		"title": "Kelola Barang - KantorKu", "user": user, "claims": claims, "items": allItems,
	})
}

func SupervisorGetItem(c *gin.Context) {
	item, err := items.GetByID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, item)
}

func SupervisorCreateItem(c *gin.Context) {
	claims := c.MustGet(middleware.UserClaimsKey).(*auth.Claims)
	stock, _ := strconv.Atoi(c.PostForm("stock"))
	imageURL, err := handleImageUpload(c, "image")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	item, err := items.Create(items.CreateItemInput{
		ItemCode:    strings.TrimSpace(c.PostForm("item_code")),
		Title:       strings.TrimSpace(c.PostForm("title")),
		Description: strings.TrimSpace(c.PostForm("description")),
		Unit:        strings.TrimSpace(c.PostForm("unit")),
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

func SupervisorUpdateItem(c *gin.Context) {
	claims := c.MustGet(middleware.UserClaimsKey).(*auth.Claims)
	id := c.Param("id")
	initialStock, _ := strconv.Atoi(c.PostForm("initial_stock"))
	imageURL, err := handleImageUpload(c, "image")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	_, err = items.UpdateInfo(id, items.UpdateInfoInput{
		ItemCode:     strings.TrimSpace(c.PostForm("item_code")),
		Title:        strings.TrimSpace(c.PostForm("title")),
		Description:  strings.TrimSpace(c.PostForm("description")),
		Unit:         strings.TrimSpace(c.PostForm("unit")),
		InitialStock: initialStock,
		ImageURL:     imageURL,
		ActorID:      claims.UserID,
		ActorName:    claims.Username,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Barang berhasil diperbarui"})
}

func SupervisorAddStock(c *gin.Context) {
	claims := c.MustGet(middleware.UserClaimsKey).(*auth.Claims)
	id := c.Param("id")
	qty, err := strconv.Atoi(c.PostForm("qty"))
	if err != nil || qty <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Jumlah harus lebih dari 0"})
		return
	}
	reason := strings.TrimSpace(c.PostForm("reason"))
	_, err = items.AddStock(id, qty, reason, claims.UserID, claims.Username)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("Stok bertambah %d unit", qty)})
}

func SupervisorDeleteItem(c *gin.Context) {
	claims := c.MustGet(middleware.UserClaimsKey).(*auth.Claims)
	if err := items.SoftDelete(c.Param("id"), claims.UserID, claims.Username); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Barang berhasil dinonaktifkan"})
}

// ── Requests ──────────────────────────────────────────────────────────────────

func SupervisorRequests(c *gin.Context) {
	claims := c.MustGet(middleware.UserClaimsKey).(*auth.Claims)
	user, _ := auth.GetUserByID(claims.UserID)
	allReqs, _ := requests.GetAll()
	stats, _ := requests.GetStats()
	c.HTML(http.StatusOK, "supervisor_requests.html", gin.H{
		"title": "Kelola Permintaan - KantorKu", "user": user, "claims": claims,
		"requests": allReqs, "stats": stats,
	})
}

func SupervisorApproveRequest(c *gin.Context) {
	claims := c.MustGet(middleware.UserClaimsKey).(*auth.Claims)
	if err := requests.Approve(c.Param("id"), claims.UserID, claims.Username); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Permintaan disetujui"})
}

func SupervisorRejectRequest(c *gin.Context) {
	claims := c.MustGet(middleware.UserClaimsKey).(*auth.Claims)
	if err := requests.Reject(c.Param("id"), claims.UserID, claims.Username); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Permintaan ditolak"})
}

// ── Rekap ─────────────────────────────────────────────────────────────────────

func SupervisorRecap(c *gin.Context) {
	claims := c.MustGet(middleware.UserClaimsKey).(*auth.Claims)
	user, _ := auth.GetUserByID(claims.UserID)

	now := time.Now()
	year, _ := strconv.Atoi(c.DefaultQuery("year", strconv.Itoa(now.Year())))
	month, _ := strconv.Atoi(c.DefaultQuery("month", strconv.Itoa(int(now.Month()))))
	recapType := c.DefaultQuery("type", "request") // "request" atau "stock"

	p := recap.RequestRecapParams{Year: year, Month: month}

	var reqRecap []*recap.RequestRecapRow
	var stockRecap []*recap.StockRecapRow

	if recapType == "stock" {
		stockRecap, _ = recap.GetStockRecap(p)
	} else {
		reqRecap, _ = recap.GetRequestRecap(p)
	}

	years := recap.GetAvailableYears()
	// Pastikan tahun saat ini selalu ada
	hasCurrentYear := false
	for _, y := range years {
		if y == now.Year() {
			hasCurrentYear = true
			break
		}
	}
	if !hasCurrentYear {
		years = append([]int{now.Year()}, years...)
	}

	months := []gin.H{
		{"val": 1, "label": "Januari"}, {"val": 2, "label": "Februari"},
		{"val": 3, "label": "Maret"}, {"val": 4, "label": "April"},
		{"val": 5, "label": "Mei"}, {"val": 6, "label": "Juni"},
		{"val": 7, "label": "Juli"}, {"val": 8, "label": "Agustus"},
		{"val": 9, "label": "September"}, {"val": 10, "label": "Oktober"},
		{"val": 11, "label": "November"}, {"val": 12, "label": "Desember"},
	}

	c.HTML(http.StatusOK, "supervisor_recap.html", gin.H{
		"title": "Rekap - KantorKu", "user": user, "claims": claims,
		"recapType":       recapType,
		"year":            year,
		"month":           month,
		"years":           years,
		"months":          months,
		"reqRecapByTeam":  groupReqRecapByTeam(reqRecap),
		"stockRecapByTeam": groupStockRecapByTeam(stockRecap),
		"hasReqData":      len(reqRecap) > 0,
		"hasStockData":    len(stockRecap) > 0,
	})
}

// ── Export Excel ──────────────────────────────────────────────────────────────

func SupervisorExportRecap(c *gin.Context) {
	now := time.Now()
	year, _ := strconv.Atoi(c.DefaultQuery("year", strconv.Itoa(now.Year())))
	month, _ := strconv.Atoi(c.DefaultQuery("month", strconv.Itoa(int(now.Month()))))
	recapType := c.DefaultQuery("type", "request")

	p := recap.RequestRecapParams{Year: year, Month: month}

	monthNames := []string{"", "Januari", "Februari", "Maret", "April", "Mei", "Juni",
		"Juli", "Agustus", "September", "Oktober", "November", "Desember"}

	periodLabel := fmt.Sprintf("%d", year)
	if month > 0 && month <= 12 {
		periodLabel = fmt.Sprintf("%s %d", monthNames[month], year)
	}

	teamLabels := map[string]string{
		"produksi": "Produksi", "distribusi": "Distribusi",
		"ipds": "IPDS", "sosial": "Sosial", "neraca": "Neraca",
	}

	f := excelize.NewFile()
	defer f.Close()

	styleHeader, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF", Size: 10},
		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"03539c"}},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
		Border:    borderStyle(),
	})
	styleCell, _ := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{Vertical: "center", WrapText: true},
		Border:    borderStyle(),
	})
	styleNum, _ := f.NewStyle(&excelize.Style{
		Alignment:  &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Border:     borderStyle(),
		NumFmt:     1,
	})
	styleTitle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 12},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})

	if recapType == "request" {
		data, err := recap.GetRequestRecap(p)
		if err != nil || len(data) == 0 {
			c.JSON(http.StatusOK, gin.H{"message": "Tidak ada data untuk diekspor"})
			return
		}

		// Kelompokkan per tim
		teamOrder := []string{"produksi", "distribusi", "ipds", "sosial", "neraca"}
		teamData := map[string][]*recap.RequestRecapRow{}
		for _, row := range data {
			teamData[row.Team] = append(teamData[row.Team], row)
		}

		sheetName := "Rekap Permintaan"
		f.SetSheetName("Sheet1", sheetName)

		// Judul
		f.MergeCell(sheetName, "A1", "F1")
		f.SetCellValue(sheetName, "A1", fmt.Sprintf("REKAP PERMINTAAN BARANG - %s", strings.ToUpper(periodLabel)))
		f.SetCellStyle(sheetName, "A1", "F1", styleTitle)
		f.SetRowHeight(sheetName, 1, 24)

		row := 3
		for _, team := range teamOrder {
			rows, ok := teamData[team]
			if !ok || len(rows) == 0 {
				continue
			}

			// Header tim
			f.MergeCell(sheetName, fmt.Sprintf("A%d", row), fmt.Sprintf("E%d", row))
			f.SetCellValue(sheetName, fmt.Sprintf("A%d", row), "TIM: "+strings.ToUpper(teamLabels[team]))
			teamStyle, _ := f.NewStyle(&excelize.Style{
				Font: &excelize.Font{Bold: true, Size: 10},
				Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"e0effe"}},
				Border: borderStyle(),
			})
			f.SetCellStyle(sheetName, fmt.Sprintf("A%d", row), fmt.Sprintf("E%d", row), teamStyle)
			f.SetRowHeight(sheetName, row, 18)
			row++

			// Header kolom
			headers := []string{"No", "Kode Barang", "Nama Barang", "Satuan", "Jumlah"}
			cols := []string{"A", "B", "C", "D", "E"}
			for i, h := range headers {
				f.SetCellValue(sheetName, fmt.Sprintf("%s%d", cols[i], row), h)
				f.SetCellStyle(sheetName, fmt.Sprintf("%s%d", cols[i], row), fmt.Sprintf("%s%d", cols[i], row), styleHeader)
			}
			f.SetRowHeight(sheetName, row, 20)
			row++

			totalQty := 0
			for i, r := range rows {
				f.SetCellValue(sheetName, fmt.Sprintf("A%d", row), i+1)
				f.SetCellValue(sheetName, fmt.Sprintf("B%d", row), r.ItemCode)
				f.SetCellValue(sheetName, fmt.Sprintf("C%d", row), r.ItemTitle)
				f.SetCellValue(sheetName, fmt.Sprintf("D%d", row), r.ItemUnit)
				f.SetCellValue(sheetName, fmt.Sprintf("E%d", row), r.Total)
				f.SetCellStyle(sheetName, fmt.Sprintf("A%d", row), fmt.Sprintf("D%d", row), styleCell)
				f.SetCellStyle(sheetName, fmt.Sprintf("E%d", row), fmt.Sprintf("E%d", row), styleNum)
				totalQty += r.Total
				row++
			}

			// Total
			totalStyle, _ := f.NewStyle(&excelize.Style{
				Font:   &excelize.Font{Bold: true},
				Fill:   excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"f8fafc"}},
				Border: borderStyle(),
				Alignment: &excelize.Alignment{Horizontal: "center"},
			})
			f.MergeCell(sheetName, fmt.Sprintf("A%d", row), fmt.Sprintf("D%d", row))
			f.SetCellValue(sheetName, fmt.Sprintf("A%d", row), "TOTAL")
			f.SetCellValue(sheetName, fmt.Sprintf("E%d", row), totalQty)
			f.SetCellStyle(sheetName, fmt.Sprintf("A%d", row), fmt.Sprintf("E%d", row), totalStyle)
			row += 2
		}

		// Set lebar kolom
		f.SetColWidth(sheetName, "A", "A", 5)
		f.SetColWidth(sheetName, "B", "B", 14)
		f.SetColWidth(sheetName, "C", "C", 30)
		f.SetColWidth(sheetName, "D", "D", 10)
		f.SetColWidth(sheetName, "E", "E", 12)

	} else {
		// Rekap stok
		data, err := recap.GetStockRecap(p)
		if err != nil || len(data) == 0 {
			c.JSON(http.StatusOK, gin.H{"message": "Tidak ada data untuk diekspor"})
			return
		}

		teamOrder := []string{"produksi", "distribusi", "ipds", "sosial", "neraca"}
		teamData := map[string][]*recap.StockRecapRow{}
		for _, row := range data {
			teamData[row.Team] = append(teamData[row.Team], row)
		}

		sheetName := "Rekap Persediaan"
		f.SetSheetName("Sheet1", sheetName)

		f.MergeCell(sheetName, "A1", "H1")
		f.SetCellValue(sheetName, "A1", fmt.Sprintf("REKAP PERSEDIAAN BARANG - %s", strings.ToUpper(periodLabel)))
		f.SetCellStyle(sheetName, "A1", "H1", styleTitle)
		f.SetRowHeight(sheetName, 1, 24)

		row := 3
		for _, team := range teamOrder {
			rows, ok := teamData[team]
			if !ok || len(rows) == 0 {
				continue
			}

			f.MergeCell(sheetName, fmt.Sprintf("A%d", row), fmt.Sprintf("H%d", row))
			f.SetCellValue(sheetName, fmt.Sprintf("A%d", row), "TIM: "+strings.ToUpper(teamLabels[team]))
			teamStyle, _ := f.NewStyle(&excelize.Style{
				Font: &excelize.Font{Bold: true, Size: 10},
				Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"e0effe"}},
				Border: borderStyle(),
			})
			f.SetCellStyle(sheetName, fmt.Sprintf("A%d", row), fmt.Sprintf("H%d", row), teamStyle)
			f.SetRowHeight(sheetName, row, 18)
			row++

			headers := []string{"No", "Kode", "Nama Barang", "Satuan", "Stok Awal", "Masuk", "Keluar", "Stok Akhir"}
			cols := []string{"A", "B", "C", "D", "E", "F", "G", "H"}
			for i, h := range headers {
				f.SetCellValue(sheetName, fmt.Sprintf("%s%d", cols[i], row), h)
				f.SetCellStyle(sheetName, fmt.Sprintf("%s%d", cols[i], row), fmt.Sprintf("%s%d", cols[i], row), styleHeader)
			}
			f.SetRowHeight(sheetName, row, 20)
			row++

			for i, r := range rows {
				f.SetCellValue(sheetName, fmt.Sprintf("A%d", row), i+1)
				f.SetCellValue(sheetName, fmt.Sprintf("B%d", row), r.ItemCode)
				f.SetCellValue(sheetName, fmt.Sprintf("C%d", row), r.ItemTitle)
				f.SetCellValue(sheetName, fmt.Sprintf("D%d", row), r.ItemUnit)
				f.SetCellValue(sheetName, fmt.Sprintf("E%d", row), r.StockAwal)
				f.SetCellValue(sheetName, fmt.Sprintf("F%d", row), r.TotalMasuk)
				f.SetCellValue(sheetName, fmt.Sprintf("G%d", row), r.TotalKeluar)
				f.SetCellValue(sheetName, fmt.Sprintf("H%d", row), r.StockAkhir)
				f.SetCellStyle(sheetName, fmt.Sprintf("A%d", row), fmt.Sprintf("D%d", row), styleCell)
				f.SetCellStyle(sheetName, fmt.Sprintf("E%d", row), fmt.Sprintf("H%d", row), styleNum)
				row++
			}
			row++
		}

		f.SetColWidth(sheetName, "A", "A", 5)
		f.SetColWidth(sheetName, "B", "B", 14)
		f.SetColWidth(sheetName, "C", "C", 30)
		f.SetColWidth(sheetName, "D", "D", 10)
		f.SetColWidth(sheetName, "E", "H", 12)
	}

	// Tulis ke response
	typeLabel := "permintaan"
	if recapType == "stock" {
		typeLabel = "persediaan"
	}
	filename := fmt.Sprintf("rekap_%s_%s.xlsx", typeLabel, strings.ReplaceAll(strings.ToLower(periodLabel), " ", "_"))

	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	c.Header("Cache-Control", "no-cache")

	if err := f.Write(c.Writer); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengekspor Excel"})
	}
}

func borderStyle() []excelize.Border {
	return []excelize.Border{
		{Type: "left", Color: "CBD5E1", Style: 1},
		{Type: "right", Color: "CBD5E1", Style: 1},
		{Type: "top", Color: "CBD5E1", Style: 1},
		{Type: "bottom", Color: "CBD5E1", Style: 1},
	}
}

// ── Image upload helper ───────────────────────────────────────────────────────

func handleImageUpload(c *gin.Context, field string) (string, error) {
	file, header, err := c.Request.FormFile(field)
	if err != nil {
		return "", nil
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(header.Filename))
	if !map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".webp": true, ".gif": true}[ext] {
		return "", fmt.Errorf("format gambar tidak didukung (jpg, png, webp, gif)")
	}
	if header.Size > 2*1024*1024 {
		return "", fmt.Errorf("ukuran gambar maksimal 2MB")
	}

	uploadDir := "web/static/uploads"
	os.MkdirAll(uploadDir, 0755)
	filename := fmt.Sprintf("%d_%s", time.Now().UnixNano(), header.Filename)
	filename = strings.NewReplacer(" ", "_", "/", "_", "\\", "_").Replace(filename)
	savePath := filepath.Join(uploadDir, filename)
	if err := c.SaveUploadedFile(header, savePath); err != nil {
		return "", fmt.Errorf("gagal menyimpan gambar")
	}
	return "/static/uploads/" + filename, nil
}

// ── Item History ──────────────────────────────────────────────────────────────

func SupervisorItemHistory(c *gin.Context) {
	claims := c.MustGet(middleware.UserClaimsKey).(*auth.Claims)
	user, _ := auth.GetUserByID(claims.UserID)

	// Ambil semua history dari semua barang, dengan info barang
	rows, err := database.DB.Query(
		`SELECT ih.id, ih.item_id, ih.item_title, ih.item_code, ih.item_unit,
		        ih.change_type, ih.change_qty, ih.stock_before, ih.stock_after,
		        ih.reason, ih.actor_id, ih.actor_name, ih.created_at
		 FROM item_history ih
		 ORDER BY ih.created_at DESC
		 LIMIT 500`,
	)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{"message": "Gagal memuat history"})
		return
	}
	defer rows.Close()

	type HistoryRow struct {
		models.ItemHistory
	}

	var history []models.ItemHistory
	for rows.Next() {
		h := models.ItemHistory{}
		if err := rows.Scan(
			&h.ID, &h.ItemID, &h.ItemTitle, &h.ItemCode, &h.ItemUnit,
			&h.ChangeType, &h.ChangeQty, &h.StockBefore, &h.StockAfter,
			&h.Reason, &h.ActorID, &h.ActorName, &h.CreatedAt,
		); err == nil {
			history = append(history, h)
		}
	}

	// Ambil list barang untuk filter dropdown
	allItems, _ := items.GetAll()

	// Filter by item_id jika ada query param
	filterItemID := c.Query("item_id")

	c.HTML(http.StatusOK, "supervisor_history.html", gin.H{
		"title":        "History Stok - KantorKu",
		"user":         user,
		"claims":       claims,
		"history":      history,
		"allItems":     allItems,
		"filterItemID": filterItemID,
	})
}
