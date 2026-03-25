# KantorKu — Sistem Pengelolaan Barang Kantor

Aplikasi web internal untuk pengelolaan barang kantor. Dibangun dengan **Go Gin**, **Go Templ**, **HTMX**, **Tailwind CSS**, dan **PostgreSQL**.

---

## Tech Stack

| Komponen | Teknologi                        |
|----------|----------------------------------|
| Backend  | Go 1.22 + Gin                    |
| Template | HTML/Go template + HTMX 2.x      |
| Styling  | Tailwind CSS (via CDN)           |
| Database | PostgreSQL                        |
| Auth     | JWT (golang-jwt/jwt v5)          |
| Password | bcrypt                           |
| Driver   | lib/pq                           |

---

## Prasyarat

- Go 1.22+
- PostgreSQL 14+ (lokal atau remote)

---

## Setup PostgreSQL

### 1. Buat Database

```sql
CREATE DATABASE kantorku;
```

> Tabel dan index dibuat otomatis saat aplikasi pertama kali dijalankan (auto-migrate).

### 2. Konfigurasi Environment

Buat file `.env` atau set environment variables:

```env
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=passwordmu
DB_NAME=kantorku
DB_SSLMODE=disable

JWT_SECRET=ganti-dengan-secret-kuat-dan-random
PORT=8080
```

> Untuk load `.env` otomatis, tambahkan package `godotenv` ke `main.go`.

---

## Instalasi & Menjalankan

### 1. Install Dependencies

```bash
go mod tidy
```

### 2. Jalankan Aplikasi

```bash
go run cmd/main.go
```

> Tabel akan dibuat otomatis di PostgreSQL saat startup.

### 3. Buat Akun Admin Pertama

```bash
go run cmd/seed/main.go
```

Output:
```
✅ Admin berhasil dibuat!
Username : admin
Password : admin123
Email    : admin@kantorku.com
```

> Ganti password setelah login pertama!

### 4. Buka Browser

```
http://localhost:8080
```

---

## Struktur Database

```
users           — Data akun pengguna
user_roles      — Role per user (pegawai, supervisor, admin)
user_teams      — Tim per user (produksi, distribusi, ipds, sosial, neraca)
items           — Barang kantor
requests        — Permintaan barang dari pegawai
request_teams   — Snapshot tim user saat request
item_history    — Riwayat perubahan stok
```

---

## Struktur Direktori

```
kantorku/
├── cmd/
│   ├── main.go              # Entry point
│   └── seed/
│       └── main.go          # Seed admin pertama
├── internal/
│   ├── auth/
│   │   └── auth.go          # Login, register, JWT
│   ├── database/
│   │   └── db.go            # Koneksi & migrasi PostgreSQL
│   ├── handlers/
│   │   └── handlers.go      # HTTP handlers
│   ├── middleware/
│   │   └── auth.go          # JWT middleware, role check
│   └── models/
│       └── models.go        # Struct: User, Item, Request, dll
├── web/
│   ├── static/
│   └── templates/
├── go.mod
└── README.md
```

---

## Role & Akses

| Halaman                   | Admin | Supervisor | Pegawai |
|---------------------------|:-----:|:----------:|:-------:|
| Dashboard Admin           |  ✅   |            |         |
| Kelola Pengguna           |  ✅   |            |         |
| Setujui Pendaftaran       |  ✅   |            |         |
| CRUD Barang               |       |    ✅      |         |
| Acc Permintaan            |       |    ✅      |         |
| Rekap Bulanan/Tahunan     |       |    ✅      |         |
| Request Barang            |       |            |   ✅    |
| History Permintaan        |       |            |   ✅    |

---

## Troubleshooting

**Q: `failed to connect to PostgreSQL`**
A: Pastikan PostgreSQL berjalan dan variabel `DB_*` sudah benar.

**Q: `database "kantorku" does not exist`**
A: Buat dulu: `CREATE DATABASE kantorku;` di psql.

**Q: Login gagal "akun belum disetujui admin"**
A: Jalankan `go run cmd/seed/main.go` untuk membuat admin, atau approve manual via halaman admin.
