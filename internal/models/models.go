package models

import "time"

const (
	RolePegawai    = "pegawai"
	RoleSupervisor = "supervisor"
	RoleAdmin      = "admin"
)

const (
	TimProduksi   = "produksi"
	TimDistribusi = "distribusi"
	TimIPDS       = "ipds"
	TimSosial     = "sosial"
	TimNeraca     = "neraca"
	TimUmum       = "umum"
)

type User struct {
	ID         string    `db:"id"          json:"id"`
	Username   string    `db:"username"    json:"username"`
	Email      string    `db:"email"       json:"email"`
	Password   string    `db:"password"    json:"-"`
	FullName   string    `db:"full_name"   json:"full_name"`
	Roles      []string  `db:"-"           json:"roles"`
	Teams      []string  `db:"-"           json:"teams"`
	IsApproved bool      `db:"is_approved" json:"is_approved"`
	IsActive   bool      `db:"is_active"   json:"is_active"`
	CreatedAt  time.Time `db:"created_at"  json:"created_at"`
	UpdatedAt  time.Time `db:"updated_at"  json:"updated_at"`
}

func (u *User) HasRole(role string) bool {
	for _, r := range u.Roles {
		if r == role {
			return true
		}
	}
	return false
}

func (u *User) HasTeam(team string) bool {
	for _, t := range u.Teams {
		if t == team {
			return true
		}
	}
	return false
}

// Item — ditambah ItemCode, Unit, InitialStock
type Item struct {
	ID           string    `db:"id"            json:"id"`
	ItemCode     string    `db:"item_code"     json:"item_code"`     // kode barang
	Title        string    `db:"title"         json:"title"`
	Description  string    `db:"description"   json:"description"`
	Unit         string    `db:"unit"          json:"unit"`          // satuan: pcs, rim, box, dll
	InitialStock int       `db:"initial_stock" json:"initial_stock"` // stok awal (bisa diedit)
	Stock        int       `db:"stock"         json:"stock"`         // stok saat ini
	ImageURL     string    `db:"image_url"     json:"image_url"`
	IsDeleted    bool      `db:"is_deleted"    json:"is_deleted"`
	CreatedBy    string    `db:"created_by"    json:"created_by"`
	CreatedAt    time.Time `db:"created_at"    json:"created_at"`
	UpdatedAt    time.Time `db:"updated_at"    json:"updated_at"`
}

type Request struct {
	ID         string     `db:"id"          json:"id"`
	UserID     string     `db:"user_id"     json:"user_id"`
	UserName   string     `db:"user_name"   json:"user_name"`
	UserTeams  []string   `db:"-"           json:"user_teams"`
	ItemID     string     `db:"item_id"     json:"item_id"`
	ItemTitle  string     `db:"item_title"  json:"item_title"`
	ItemCode   string     `db:"item_code"   json:"item_code"`
	ItemUnit   string     `db:"item_unit"   json:"item_unit"`
	Quantity   int        `db:"quantity"    json:"quantity"`
	Status     string     `db:"status"      json:"status"`
	Notes      string     `db:"notes"       json:"notes"`
	ApprovedBy string     `db:"approved_by" json:"approved_by"`
	ApprovedAt *time.Time `db:"approved_at" json:"approved_at"`
	CreatedAt  time.Time  `db:"created_at"  json:"created_at"`
	UpdatedAt  time.Time  `db:"updated_at"  json:"updated_at"`
}

type ItemHistory struct {
	ID          string    `db:"id"           json:"id"`
	ItemID      string    `db:"item_id"      json:"item_id"`
	ItemTitle   string    `db:"item_title"   json:"item_title"`
	ItemCode    string    `db:"item_code"    json:"item_code"`
	ItemUnit    string    `db:"item_unit"    json:"item_unit"`
	ChangeType  string    `db:"change_type"  json:"change_type"` // add, reduce, delete, stock_add
	ChangeQty   int       `db:"change_qty"   json:"change_qty"`
	StockBefore int       `db:"stock_before" json:"stock_before"`
	StockAfter  int       `db:"stock_after"  json:"stock_after"`
	Reason      string    `db:"reason"       json:"reason"`
	ActorID     string    `db:"actor_id"     json:"actor_id"`
	ActorName   string    `db:"actor_name"   json:"actor_name"`
	CreatedAt   time.Time `db:"created_at"   json:"created_at"`
}
