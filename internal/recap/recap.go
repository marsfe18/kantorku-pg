package recap

import (
	"database/sql"
	"kantorku/internal/database"
)

// ── Rekap Permintaan per Tim ──────────────────────────────────────────────────
// Rekap permintaan yang sudah di-acc, dikelompokkan per tim

type RequestRecapRow struct {
	ItemCode  string
	ItemTitle string
	ItemUnit  string
	Team      string
	Total     int // total qty yang di-acc
}

type RequestRecapParams struct {
	Year  int
	Month int // 0 = semua bulan (tahunan)
}

// GetRequestRecap — rekap permintaan yang di-acc per tim per bulan/tahun
func GetRequestRecap(p RequestRecapParams) ([]*RequestRecapRow, error) {
	db := database.DB

	var rows *sql.Rows
	var err error

	if p.Month > 0 {
		// Bulanan
		rows, err = db.Query(`
			SELECT
				r.item_code,
				r.item_title,
				r.item_unit,
				rt.team,
				SUM(r.quantity) AS total
			FROM requests r
			JOIN request_teams rt ON rt.request_id = r.id
			WHERE r.status = 'approved'
			  AND EXTRACT(YEAR  FROM r.approved_at) = $1
			  AND EXTRACT(MONTH FROM r.approved_at) = $2
			GROUP BY r.item_code, r.item_title, r.item_unit, rt.team
			ORDER BY rt.team, r.item_code
		`, p.Year, p.Month)
	} else {
		// Tahunan
		rows, err = db.Query(`
			SELECT
				r.item_code,
				r.item_title,
				r.item_unit,
				rt.team,
				SUM(r.quantity) AS total
			FROM requests r
			JOIN request_teams rt ON rt.request_id = r.id
			WHERE r.status = 'approved'
			  AND EXTRACT(YEAR FROM r.approved_at) = $1
			GROUP BY r.item_code, r.item_title, r.item_unit, rt.team
			ORDER BY rt.team, r.item_code
		`, p.Year)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*RequestRecapRow
	for rows.Next() {
		row := &RequestRecapRow{}
		if err := rows.Scan(&row.ItemCode, &row.ItemTitle, &row.ItemUnit, &row.Team, &row.Total); err == nil {
			result = append(result, row)
		}
	}
	return result, nil
}

// ── Rekap Pengeluaran / Persediaan Barang per Tim ─────────────────────────────
// Rekap stok awal, masuk, keluar, stok akhir per bulan per barang per tim

type StockRecapRow struct {
	ItemCode    string
	ItemTitle   string
	ItemUnit    string
	Team        string
	StockAwal   int // stok di awal bulan
	StockAkhir  int // stok di akhir bulan
	TotalMasuk  int // penambahan stok (supervisor add)
	TotalKeluar int // permintaan yang di-acc (keluar dari gudang)
}

// GetStockRecap — rekap persediaan barang per tim
// Stok awal = stok sebelum perubahan pertama di bulan tsb
// Stok akhir = stok setelah perubahan terakhir di bulan tsb
func GetStockRecap(p RequestRecapParams) ([]*StockRecapRow, error) {
	db := database.DB

	// Langkah 1: ambil semua item yang punya aktivitas di bulan tsb per tim
	var rows *sql.Rows
	var err error

	if p.Month > 0 {
		rows, err = db.Query(`
			SELECT DISTINCT
				ih.item_id,
				ih.item_code,
				ih.item_title,
				ih.item_unit,
				rt.team
			FROM item_history ih
			JOIN requests r ON (
				ih.change_type = 'reduce'
				AND ih.reason LIKE '%' || r.user_name || '%'
				AND EXTRACT(YEAR  FROM ih.created_at) = $1
				AND EXTRACT(MONTH FROM ih.created_at) = $2
			)
			JOIN request_teams rt ON rt.request_id = r.id
			UNION
			SELECT DISTINCT
				ih.item_id,
				ih.item_code,
				ih.item_title,
				ih.item_unit,
				'all' as team
			FROM item_history ih
			WHERE ih.change_type IN ('add','stock_add','reduce')
			  AND EXTRACT(YEAR  FROM ih.created_at) = $1
			  AND EXTRACT(MONTH FROM ih.created_at) = $2
		`, p.Year, p.Month)
	} else {
		rows, err = db.Query(`
			SELECT DISTINCT
				ih.item_id,
				ih.item_code,
				ih.item_title,
				ih.item_unit,
				'all' as team
			FROM item_history ih
			WHERE ih.change_type IN ('add','stock_add','reduce')
			  AND EXTRACT(YEAR FROM ih.created_at) = $1
		`, p.Year)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Ambil dulu list item aktif dengan tim dari permintaan
	return getStockRecapByApprovedRequests(db, p)
}

// getStockRecapByApprovedRequests — pendekatan yang lebih akurat:
// hitung masuk/keluar dari item_history + request per tim
func getStockRecapByApprovedRequests(db *sql.DB, p RequestRecapParams) ([]*StockRecapRow, error) {
	// Query rekap keluar per tim dari approved requests
	var outRows *sql.Rows
	var err error

	if p.Month > 0 {
		outRows, err = db.Query(`
			SELECT
				r.item_id,
				r.item_code,
				r.item_title,
				r.item_unit,
				rt.team,
				SUM(r.quantity) AS keluar
			FROM requests r
			JOIN request_teams rt ON rt.request_id = r.id
			WHERE r.status = 'approved'
			  AND EXTRACT(YEAR  FROM r.approved_at) = $1
			  AND EXTRACT(MONTH FROM r.approved_at) = $2
			GROUP BY r.item_id, r.item_code, r.item_title, r.item_unit, rt.team
			ORDER BY rt.team, r.item_code
		`, p.Year, p.Month)
	} else {
		outRows, err = db.Query(`
			SELECT
				r.item_id,
				r.item_code,
				r.item_title,
				r.item_unit,
				rt.team,
				SUM(r.quantity) AS keluar
			FROM requests r
			JOIN request_teams rt ON rt.request_id = r.id
			WHERE r.status = 'approved'
			  AND EXTRACT(YEAR FROM r.approved_at) = $1
			GROUP BY r.item_id, r.item_code, r.item_title, r.item_unit, rt.team
			ORDER BY rt.team, r.item_code
		`, p.Year)
	}
	if err != nil {
		return nil, err
	}
	defer outRows.Close()

	type itemTeamKey struct {
		itemID string
		team   string
	}
	rowMap := map[itemTeamKey]*StockRecapRow{}

	for outRows.Next() {
		var itemID, itemCode, itemTitle, itemUnit, team string
		var keluar int
		if err := outRows.Scan(&itemID, &itemCode, &itemTitle, &itemUnit, &team, &keluar); err != nil {
			continue
		}
		key := itemTeamKey{itemID, team}
		rowMap[key] = &StockRecapRow{
			ItemCode:    itemCode,
			ItemTitle:   itemTitle,
			ItemUnit:    itemUnit,
			Team:        team,
			TotalKeluar: keluar,
		}
	}

	// Query masuk (stock_add) dari item_history — tidak per tim, share ke semua tim item itu
	var inRows *sql.Rows
	if p.Month > 0 {
		inRows, err = db.Query(`
			SELECT item_id, SUM(change_qty) AS masuk
			FROM item_history
			WHERE change_type = 'stock_add'
			  AND EXTRACT(YEAR  FROM created_at) = $1
			  AND EXTRACT(MONTH FROM created_at) = $2
			GROUP BY item_id
		`, p.Year, p.Month)
	} else {
		inRows, err = db.Query(`
			SELECT item_id, SUM(change_qty) AS masuk
			FROM item_history
			WHERE change_type = 'stock_add'
			  AND EXTRACT(YEAR FROM created_at) = $1
			GROUP BY item_id
		`, p.Year)
	}
	if err != nil {
		return nil, err
	}
	defer inRows.Close()

	inMap := map[string]int{}
	for inRows.Next() {
		var itemID string
		var masuk int
		if inRows.Scan(&itemID, &masuk) == nil {
			inMap[itemID] = masuk
		}
	}

	// Query stok awal & akhir bulan dari item_history per item
	var stockRows *sql.Rows
	if p.Month > 0 {
		stockRows, err = db.Query(`
			SELECT DISTINCT ON (item_id)
				item_id,
				stock_before AS stok_awal
			FROM item_history
			WHERE EXTRACT(YEAR  FROM created_at) = $1
			  AND EXTRACT(MONTH FROM created_at) = $2
			ORDER BY item_id, created_at ASC
		`, p.Year, p.Month)
	} else {
		stockRows, err = db.Query(`
			SELECT DISTINCT ON (item_id)
				item_id,
				stock_before AS stok_awal
			FROM item_history
			WHERE EXTRACT(YEAR FROM created_at) = $1
			ORDER BY item_id, created_at ASC
		`, p.Year)
	}
	if err != nil {
		return nil, err
	}
	defer stockRows.Close()

	stockAwalMap := map[string]int{}
	for stockRows.Next() {
		var itemID string
		var stokAwal int
		if stockRows.Scan(&itemID, &stokAwal) == nil {
			stockAwalMap[itemID] = stokAwal
		}
	}

	// Stok akhir = ambil stock_after dari history terakhir bulan itu
	var stockAkhirRows *sql.Rows
	if p.Month > 0 {
		stockAkhirRows, err = db.Query(`
			SELECT DISTINCT ON (item_id)
				item_id,
				stock_after AS stok_akhir
			FROM item_history
			WHERE EXTRACT(YEAR  FROM created_at) = $1
			  AND EXTRACT(MONTH FROM created_at) = $2
			ORDER BY item_id, created_at DESC
		`, p.Year, p.Month)
	} else {
		stockAkhirRows, err = db.Query(`
			SELECT DISTINCT ON (item_id)
				item_id,
				stock_after AS stok_akhir
			FROM item_history
			WHERE EXTRACT(YEAR FROM created_at) = $1
			ORDER BY item_id, created_at DESC
		`, p.Year)
	}
	if err != nil {
		return nil, err
	}
	defer stockAkhirRows.Close()

	stockAkhirMap := map[string]int{}
	for stockAkhirRows.Next() {
		var itemID string
		var stokAkhir int
		if stockAkhirRows.Scan(&itemID, &stokAkhir) == nil {
			stockAkhirMap[itemID] = stokAkhir
		}
	}

	// Gabungkan semua ke result
	var result []*StockRecapRow
	for key, row := range rowMap {
		row.TotalMasuk = inMap[key.itemID]
		row.StockAwal = stockAwalMap[key.itemID]
		row.StockAkhir = stockAkhirMap[key.itemID]
		result = append(result, row)
	}

	return result, nil
}

// ── Available years ───────────────────────────────────────────────────────────

func GetAvailableYears() []int {
	rows, err := database.DB.Query(
		`SELECT DISTINCT EXTRACT(YEAR FROM approved_at)::INT FROM requests WHERE status='approved' AND approved_at IS NOT NULL ORDER BY 1 DESC`)
	if err != nil {
		return []int{}
	}
	defer rows.Close()
	var years []int
	for rows.Next() {
		var y int
		if rows.Scan(&y) == nil {
			years = append(years, y)
		}
	}
	if len(years) == 0 {
		// default tahun ini
		years = append(years, 2025)
	}
	return years
}
