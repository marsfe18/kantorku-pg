package items

import (
	"database/sql"
	"errors"
	"time"

	"kantorku/internal/database"
	"kantorku/internal/models"
)

// ── Query helpers ─────────────────────────────────────────────────────────────

func scanItem(rows *sql.Rows) (*models.Item, error) {
	item := &models.Item{}
	err := rows.Scan(
		&item.ID, &item.Title, &item.Description,
		&item.Stock, &item.ImageURL, &item.IsDeleted,
		&item.CreatedBy, &item.CreatedAt, &item.UpdatedAt,
	)
	return item, err
}

func scanItemRow(row *sql.Row) (*models.Item, error) {
	item := &models.Item{}
	err := row.Scan(
		&item.ID, &item.Title, &item.Description,
		&item.Stock, &item.ImageURL, &item.IsDeleted,
		&item.CreatedBy, &item.CreatedAt, &item.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return item, err
}

// ── Read ──────────────────────────────────────────────────────────────────────

// GetAll returns all items (termasuk deleted) untuk supervisor
func GetAll() ([]*models.Item, error) {
	rows, err := database.DB.Query(
		`SELECT id, title, description, stock, image_url, is_deleted, created_by, created_at, updated_at
		 FROM items ORDER BY is_deleted ASC, created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*models.Item
	for rows.Next() {
		item, err := scanItem(rows)
		if err != nil {
			continue
		}
		items = append(items, item)
	}
	return items, nil
}

// GetActive returns only active (non-deleted) items — untuk pegawai
func GetActive() ([]*models.Item, error) {
	rows, err := database.DB.Query(
		`SELECT id, title, description, stock, image_url, is_deleted, created_by, created_at, updated_at
		 FROM items WHERE is_deleted = FALSE ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*models.Item
	for rows.Next() {
		item, err := scanItem(rows)
		if err != nil {
			continue
		}
		items = append(items, item)
	}
	return items, nil
}

// GetByID returns single item by id
func GetByID(id string) (*models.Item, error) {
	row := database.DB.QueryRow(
		`SELECT id, title, description, stock, image_url, is_deleted, created_by, created_at, updated_at
		 FROM items WHERE id = $1`, id,
	)
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
	Title       string
	Description string
	Stock       int
	ImageURL    string
	ActorID     string
	ActorName   string
}

func Create(input CreateItemInput) (*models.Item, error) {
	if input.Title == "" {
		return nil, errors.New("nama barang wajib diisi")
	}
	if input.Stock < 0 {
		return nil, errors.New("stok tidak boleh negatif")
	}

	now := time.Now()
	var id string
	err := database.DB.QueryRow(
		`INSERT INTO items (title, description, stock, image_url, is_deleted, created_by, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, FALSE, $5, $6, $7) RETURNING id`,
		input.Title, input.Description, input.Stock, input.ImageURL, input.ActorID, now, now,
	).Scan(&id)
	if err != nil {
		return nil, err
	}

	// Catat history
	if input.Stock > 0 {
		recordHistory(models.ItemHistory{
			ItemID:      id,
			ItemTitle:   input.Title,
			ChangeType:  "add",
			ChangeQty:   input.Stock,
			StockBefore: 0,
			StockAfter:  input.Stock,
			Reason:      "Barang baru ditambahkan",
			ActorID:     input.ActorID,
			ActorName:   input.ActorName,
		})
	}

	return GetByID(id)
}

// ── Update ────────────────────────────────────────────────────────────────────

type UpdateItemInput struct {
	Title       string
	Description string
	Stock       int
	ImageURL    string // kosong = tidak ubah gambar
	ActorID     string
	ActorName   string
}

func Update(id string, input UpdateItemInput) (*models.Item, error) {
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
	if input.Stock < 0 {
		return nil, errors.New("stok tidak boleh negatif")
	}

	imageURL := existing.ImageURL
	if input.ImageURL != "" {
		imageURL = input.ImageURL
	}

	now := time.Now()
	_, err = database.DB.Exec(
		`UPDATE items SET title=$1, description=$2, stock=$3, image_url=$4, updated_at=$5 WHERE id=$6`,
		input.Title, input.Description, input.Stock, imageURL, now, id,
	)
	if err != nil {
		return nil, err
	}

	// Catat history perubahan stok
	if existing.Stock != input.Stock {
		changeType := "add"
		diff := input.Stock - existing.Stock
		if diff < 0 {
			changeType = "reduce"
			diff = -diff
		}
		reason := "Stok diperbarui oleh supervisor"
		recordHistory(models.ItemHistory{
			ItemID:      id,
			ItemTitle:   input.Title,
			ChangeType:  changeType,
			ChangeQty:   diff,
			StockBefore: existing.Stock,
			StockAfter:  input.Stock,
			Reason:      reason,
			ActorID:     input.ActorID,
			ActorName:   input.ActorName,
		})
	}

	return GetByID(id)
}

// ── Soft Delete ───────────────────────────────────────────────────────────────

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
		ChangeType:  "delete",
		ChangeQty:   0,
		StockBefore: item.Stock,
		StockAfter:  item.Stock,
		Reason:      "Barang dihapus (soft delete)",
		ActorID:     actorID,
		ActorName:   actorName,
	})
	return nil
}

// ── History ───────────────────────────────────────────────────────────────────

func recordHistory(h models.ItemHistory) {
	database.DB.Exec(
		`INSERT INTO item_history (item_id, item_title, change_type, change_qty, stock_before, stock_after, reason, actor_id, actor_name, created_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		h.ItemID, h.ItemTitle, h.ChangeType, h.ChangeQty,
		h.StockBefore, h.StockAfter, h.Reason, h.ActorID, h.ActorName, time.Now(),
	)
}

func GetHistory(itemID string) ([]*models.ItemHistory, error) {
	rows, err := database.DB.Query(
		`SELECT id, item_id, item_title, change_type, change_qty, stock_before, stock_after, reason, actor_id, actor_name, created_at
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
			&h.ID, &h.ItemID, &h.ItemTitle, &h.ChangeType, &h.ChangeQty,
			&h.StockBefore, &h.StockAfter, &h.Reason, &h.ActorID, &h.ActorName, &h.CreatedAt,
		); err == nil {
			hist = append(hist, h)
		}
	}
	return hist, nil
}
