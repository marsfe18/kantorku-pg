package requests

import (
	"database/sql"
	"errors"
	"time"

	"kantorku/internal/database"
	"kantorku/internal/models"
)

type CreateRequestInput struct {
	UserID    string
	UserName  string
	UserTeams []string
	ItemID    string
	Quantity  int
	Notes     string
}

type Stats struct {
	Pending, Approved, Rejected, Total int
}

func scanRequest(rows *sql.Rows) (*models.Request, error) {
	r := &models.Request{}
	err := rows.Scan(
		&r.ID, &r.UserID, &r.UserName, &r.ItemID, &r.ItemTitle, &r.ItemCode, &r.ItemUnit,
		&r.Quantity, &r.Status, &r.Notes, &r.ApprovedBy, &r.ApprovedAt, &r.CreatedAt, &r.UpdatedAt,
	)
	return r, err
}

func loadTeams(db *sql.DB, r *models.Request) {
	rows, _ := db.Query(`SELECT team FROM request_teams WHERE request_id = $1`, r.ID)
	if rows == nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var t string
		if rows.Scan(&t) == nil {
			r.UserTeams = append(r.UserTeams, t)
		}
	}
	if r.UserTeams == nil {
		r.UserTeams = []string{}
	}
}

func GetByID(id string) (*models.Request, error) {
	db := database.DB
	rows, err := db.Query(
		`SELECT id, user_id, user_name, item_id, item_title, item_code, item_unit,
		        quantity, status, notes, approved_by, approved_at, created_at, updated_at
		 FROM requests WHERE id = $1`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if rows.Next() {
		r, err := scanRequest(rows)
		if err != nil {
			return nil, err
		}
		loadTeams(db, r)
		return r, nil
	}
	return nil, errors.New("permintaan tidak ditemukan")
}

func GetByUser(userID string) ([]*models.Request, error) {
	db := database.DB
	rows, err := db.Query(
		`SELECT id, user_id, user_name, item_id, item_title, item_code, item_unit,
		        quantity, status, notes, approved_by, approved_at, created_at, updated_at
		 FROM requests WHERE user_id = $1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var reqs []*models.Request
	for rows.Next() {
		r, err := scanRequest(rows)
		if err != nil {
			continue
		}
		loadTeams(db, r)
		reqs = append(reqs, r)
	}
	return reqs, nil
}

func GetAll() ([]*models.Request, error) {
	db := database.DB
	rows, err := db.Query(
		`SELECT id, user_id, user_name, item_id, item_title, item_code, item_unit,
		        quantity, status, notes, approved_by, approved_at, created_at, updated_at
		 FROM requests
		 ORDER BY CASE status WHEN 'pending' THEN 0 WHEN 'approved' THEN 1 ELSE 2 END, created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var reqs []*models.Request
	for rows.Next() {
		r, err := scanRequest(rows)
		if err != nil {
			continue
		}
		loadTeams(db, r)
		reqs = append(reqs, r)
	}
	return reqs, nil
}

func Create(input CreateRequestInput) (*models.Request, error) {
	db := database.DB
	if input.Quantity <= 0 {
		return nil, errors.New("jumlah permintaan harus lebih dari 0")
	}
	var stock int
	var itemTitle, itemCode, itemUnit string
	var isDeleted bool
	err := db.QueryRow(
		`SELECT title, item_code, unit, stock, is_deleted FROM items WHERE id = $1`, input.ItemID,
	).Scan(&itemTitle, &itemCode, &itemUnit, &stock, &isDeleted)
	if err == sql.ErrNoRows {
		return nil, errors.New("barang tidak ditemukan")
	}
	if err != nil {
		return nil, err
	}
	if isDeleted {
		return nil, errors.New("barang sudah tidak tersedia")
	}
	if stock < input.Quantity {
		return nil, errors.New("stok tidak mencukupi")
	}

	now := time.Now()
	var reqID string
	err = db.QueryRow(
		`INSERT INTO requests (user_id, user_name, item_id, item_title, item_code, item_unit, quantity, status, notes, approved_by, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,'pending',$8,'',$9,$10) RETURNING id`,
		input.UserID, input.UserName, input.ItemID, itemTitle, itemCode, itemUnit,
		input.Quantity, input.Notes, now, now,
	).Scan(&reqID)
	if err != nil {
		return nil, err
	}
	for _, team := range input.UserTeams {
		db.Exec(`INSERT INTO request_teams (request_id, team) VALUES ($1,$2) ON CONFLICT DO NOTHING`, reqID, team)
	}
	return GetByID(reqID)
}

func Approve(requestID, actorID, actorName string) error {
	db := database.DB
	req, err := GetByID(requestID)
	if err != nil {
		return err
	}
	if req.Status != "pending" {
		return errors.New("permintaan sudah diproses")
	}
	var stock int
	var itemTitle string
	err = db.QueryRow(`SELECT stock, title FROM items WHERE id = $1 AND is_deleted = FALSE`, req.ItemID).
		Scan(&stock, &itemTitle)
	if err == sql.ErrNoRows {
		return errors.New("barang tidak tersedia lagi")
	}
	if err != nil {
		return err
	}
	if stock < req.Quantity {
		return errors.New("stok tidak mencukupi untuk disetujui")
	}

	now := time.Now()
	db.Exec(`UPDATE requests SET status='approved', approved_by=$1, approved_at=$2, updated_at=$3 WHERE id=$4`,
		actorID, now, now, requestID)

	newStock := stock - req.Quantity
	db.Exec(`UPDATE items SET stock=$1, updated_at=$2 WHERE id=$3`, newStock, now, req.ItemID)

	db.Exec(
		`INSERT INTO item_history (item_id, item_title, item_code, item_unit, change_type, change_qty, stock_before, stock_after, reason, actor_id, actor_name, created_at)
		 VALUES ($1,$2,$3,$4,'reduce',$5,$6,$7,$8,$9,$10,$11)`,
		req.ItemID, itemTitle, req.ItemCode, req.ItemUnit,
		req.Quantity, stock, newStock,
		"Permintaan disetujui untuk "+req.UserName,
		actorID, actorName, now,
	)
	return nil
}

func Reject(requestID, actorID, actorName string) error {
	db := database.DB
	req, err := GetByID(requestID)
	if err != nil {
		return err
	}
	if req.Status != "pending" {
		return errors.New("permintaan sudah diproses")
	}
	now := time.Now()
	_, err = db.Exec(`UPDATE requests SET status='rejected', approved_by=$1, approved_at=$2, updated_at=$3 WHERE id=$4`,
		actorID, now, now, requestID)
	return err
}

func GetStats() (Stats, error) {
	var s Stats
	rows, err := database.DB.Query(`SELECT status, COUNT(*) FROM requests GROUP BY status`)
	if err != nil {
		return s, err
	}
	defer rows.Close()
	for rows.Next() {
		var status string
		var count int
		if rows.Scan(&status, &count) == nil {
			switch status {
			case "pending":
				s.Pending = count
			case "approved":
				s.Approved = count
			case "rejected":
				s.Rejected = count
			}
		}
	}
	s.Total = s.Pending + s.Approved + s.Rejected
	return s, nil
}
