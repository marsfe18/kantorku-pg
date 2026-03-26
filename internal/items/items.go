package items

import (
	"database/sql"
	"errors"
	"time"

	"kantorku/internal/database"
	"kantorku/internal/models"
)

// ── Scan helpers ──────────────────────────────────────────────────────────────

func scanItem(rows *sql.Rows) (*models.Item, error) {
	item := &models.Item{}
	err := rows.Scan(
		&item.ID, &item.ItemCode, &item.Title, &item.Description,
		&item.Unit, &item.InitialStock, &item.Stock,
		&item.ImageURL, &item.IsDeleted, &item.CreatedBy,
		&item.CreatedAt, &item.UpdatedAt,
	)
	return item, err
}

func scanItemRow(row *sql.Row) (*models.Item, error) {
	item := &models.Item{}
	err := row.Scan(
		&item.ID, &item.ItemCode, &item.Title, &item.Description,
		&item.Unit, &item.InitialStock, &item.Stock,
		&item.ImageURL, &item.IsDeleted, &item.CreatedBy,
		&item.CreatedAt, &item.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return item, err
}

const selectCols = `id, item_code, title, description, unit, initial_stock, stock, image_url, is_deleted, created_by, created_at, updated_at`

// ── Read ──────────────────────────────────────────────────────────────────────

func GetAll() ([]*models.Item, error) {
	rows, err := database.DB.Query(
		`SELECT ` + selectCols + ` FROM items ORDER BY is_deleted ASC, created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []*models.Item
	for rows.Next() {
		item, err := scanItem(rows)
		if err != nil {
			continue
		}
		list = append(list, item)
	}
	return list, nil
}

func GetActive() ([]*models.Item, error) {
	rows, err := database.DB.Query(
		`SELECT ` + selectCols + ` FROM items WHERE is_deleted = FALSE AND stock > 0 ORDER BY title ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []*models.Item
	for rows.Next() {
		item, err := scanItem(rows)
		if err != nil {
			continue
		}
		list = append(list, item)
	}
	return list, nil
}

func GetByID(id string) (*models.Item, error) {
	row := database.DB.QueryRow(
		`SELECT `+selectCols+` FROM items WHERE id = $1`, id)
	item, err := scanItemRow(row)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, errors.New("barang tidak ditemukan")
	}
	return item, nil
}

// ── Create ────────────────────────────────────────────────────────────────────

type CreateItemInput struct {
	ItemCode    string
	Title       string
	Description string
	Unit        string
	Stock       int // stok awal
	ImageURL    string
	ActorID     string
	ActorName   string
}

func Create(input CreateItemInput) (*models.Item, error) {
	if input.Title == "" {
		return nil, errors.New("nama barang wajib diisi")
	}
	if input.ItemCode == "" {
		return nil, errors.New("kode barang wajib diisi")
	}
	if input.Unit == "" {
		input.Unit = "pcs"
	}
	if input.Stock < 0 {
		return nil, errors.New("stok tidak boleh negatif")
	}

	// Cek kode unik
	var count int
	database.DB.QueryRow(`SELECT COUNT(*) FROM items WHERE item_code = $1`, input.ItemCode).Scan(&count)
	if count > 0 {
		return nil, errors.New("kode barang sudah digunakan")
	}

	now := time.Now()
	var id string
	err := database.DB.QueryRow(
		`INSERT INTO items (item_code, title, description, unit, initial_stock, stock, image_url, is_deleted, created_by, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,FALSE,$8,$9,$10) RETURNING id`,
		input.ItemCode, input.Title, input.Description, input.Unit,
		input.Stock, input.Stock, input.ImageURL, input.ActorID, now, now,
	).Scan(&id)
	if err != nil {
		return nil, err
	}

	if input.Stock > 0 {
		recordHistory(models.ItemHistory{
			ItemID:      id,
			ItemTitle:   input.Title,
			ItemCode:    input.ItemCode,
			ItemUnit:    input.Unit,
			ChangeType:  "add",
			ChangeQty:   input.Stock,
			StockBefore: 0,
			StockAfter:  input.Stock,
			Reason:      "Stok awal barang baru",
			ActorID:     input.ActorID,
			ActorName:   input.ActorName,
		})
	}
	return GetByID(id)
}

// ── UpdateInfo — edit info barang (bukan stok) ────────────────────────────────

type UpdateInfoInput struct {
	ItemCode    string
	Title       string
	Description string
	Unit        string
	InitialStock int  // edit stok awal, bukan stok saat ini
	ImageURL    string
	ActorID     string
	ActorName   string
}

func UpdateInfo(id string, input UpdateInfoInput) (*models.Item, error) {
	existing, err := GetByID(id)
	if err != nil {
		return nil, err
	}
	if existing.IsDeleted {
		return nil, errors.New("barang sudah dihapus")
	}
	if input.Title == "" {
		return nil, errors.New("nama barang wajib diisi")
	}
	if input.ItemCode == "" {
		return nil, errors.New("kode barang wajib diisi")
	}
	if input.Unit == "" {
		input.Unit = "pcs"
	}
	if input.InitialStock < 0 {
		return nil, errors.New("stok awal tidak boleh negatif")
	}

	// Cek kode unik (selain diri sendiri)
	var count int
	database.DB.QueryRow(`SELECT COUNT(*) FROM items WHERE item_code = $1 AND id != $2`, input.ItemCode, id).Scan(&count)
	if count > 0 {
		return nil, errors.New("kode barang sudah digunakan")
	}

	imageURL := existing.ImageURL
	if input.ImageURL != "" {
		imageURL = input.ImageURL
	}

	now := time.Now()

	// Hitung penyesuaian stok saat ini jika initial_stock berubah
	// Logika: stok_saat_ini = stok_saat_ini + (new_initial - old_initial)
	newStock := existing.Stock + (input.InitialStock - existing.InitialStock)
	if newStock < 0 {
		newStock = 0
	}

	_, err = database.DB.Exec(
		`UPDATE items SET item_code=$1, title=$2, description=$3, unit=$4, initial_stock=$5, stock=$6, image_url=$7, updated_at=$8 WHERE id=$9`,
		input.ItemCode, input.Title, input.Description, input.Unit,
		input.InitialStock, newStock, imageURL, now, id,
	)
	if err != nil {
		return nil, err
	}

	// Catat history jika initial_stock berubah
	if existing.InitialStock != input.InitialStock {
		diff := input.InitialStock - existing.InitialStock
		ctype := "add"
		if diff < 0 {
			ctype = "reduce"
			diff = -diff
		}
		recordHistory(models.ItemHistory{
			ItemID:      id,
			ItemTitle:   input.Title,
			ItemCode:    input.ItemCode,
			ItemUnit:    input.Unit,
			ChangeType:  ctype,
			ChangeQty:   diff,
			StockBefore: existing.Stock,
			StockAfter:  newStock,
			Reason:      "Koreksi stok awal oleh supervisor",
			ActorID:     input.ActorID,
			ActorName:   input.ActorName,
		})
	}

	return GetByID(id)
}

// ── AddStock — supervisor menambahkan stok ────────────────────────────────────

func AddStock(id string, qty int, reason, actorID, actorName string) (*models.Item, error) {
	if qty <= 0 {
		return nil, errors.New("jumlah penambahan harus lebih dari 0")
	}
	item, err := GetByID(id)
	if err != nil {
		return nil, err
	}
	if item.IsDeleted {
		return nil, errors.New("barang sudah dihapus")
	}

	newStock := item.Stock + qty
	now := time.Now()
	_, err = database.DB.Exec(
		`UPDATE items SET stock=$1, updated_at=$2 WHERE id=$3`,
		newStock, now, id,
	)
	if err != nil {
		return nil, err
	}

	if reason == "" {
		reason = "Penambahan stok oleh supervisor"
	}
	recordHistory(models.ItemHistory{
		ItemID:      id,
		ItemTitle:   item.Title,
		ItemCode:    item.ItemCode,
		ItemUnit:    item.Unit,
		ChangeType:  "stock_add",
		ChangeQty:   qty,
		StockBefore: item.Stock,
		StockAfter:  newStock,
		Reason:      reason,
		ActorID:     actorID,
		ActorName:   actorName,
	})
	return GetByID(id)
}

// ── SoftDelete ────────────────────────────────────────────────────────────────

func SoftDelete(id, actorID, actorName string) error {
	item, err := GetByID(id)
	if err != nil {
		return err
	}
	if item.IsDeleted {
		return errors.New("barang sudah dihapus")
	}
	_, err = database.DB.Exec(
		`UPDATE items SET is_deleted=TRUE, updated_at=$1 WHERE id=$2`,
		time.Now(), id,
	)
	if err != nil {
		return err
	}
	recordHistory(models.ItemHistory{
		ItemID:      id,
		ItemTitle:   item.Title,
		ItemCode:    item.ItemCode,
		ItemUnit:    item.Unit,
		ChangeType:  "delete",
		ChangeQty:   0,
		StockBefore: item.Stock,
		StockAfter:  item.Stock,
		Reason:      "Barang dinonaktifkan",
		ActorID:     actorID,
		ActorName:   actorName,
	})
	return nil
}

// ── History ───────────────────────────────────────────────────────────────────

func recordHistory(h models.ItemHistory) {
	database.DB.Exec(
		`INSERT INTO item_history (item_id, item_title, item_code, item_unit, change_type, change_qty, stock_before, stock_after, reason, actor_id, actor_name, created_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		h.ItemID, h.ItemTitle, h.ItemCode, h.ItemUnit,
		h.ChangeType, h.ChangeQty, h.StockBefore, h.StockAfter,
		h.Reason, h.ActorID, h.ActorName, time.Now(),
	)
}

func GetHistory(itemID string) ([]*models.ItemHistory, error) {
	rows, err := database.DB.Query(
		`SELECT id, item_id, item_title, item_code, item_unit, change_type, change_qty, stock_before, stock_after, reason, actor_id, actor_name, created_at
		 FROM item_history WHERE item_id = $1 ORDER BY created_at DESC`,
		itemID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var hist []*models.ItemHistory
	for rows.Next() {
		h := &models.ItemHistory{}
		if err := rows.Scan(
			&h.ID, &h.ItemID, &h.ItemTitle, &h.ItemCode, &h.ItemUnit,
			&h.ChangeType, &h.ChangeQty, &h.StockBefore, &h.StockAfter,
			&h.Reason, &h.ActorID, &h.ActorName, &h.CreatedAt,
		); err == nil {
			hist = append(hist, h)
		}
	}
	return hist, nil
}
