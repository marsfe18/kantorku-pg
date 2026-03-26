package handlers
 
import (
	"kantorku/internal/auth"
	"kantorku/internal/requests"
 
	"github.com/gin-gonic/gin"
)
 
// SidebarData mengisi data yang dibutuhkan sidebar dinamis.
// Panggil di setiap handler dan gabungkan hasilnya ke gin.H utama.
//
// Contoh penggunaan:
//
//	data := MergeH(gin.H{
//	    "title":      "Judul Halaman",
//	    "activePage": "nama_page",
//	    ...
//	}, SidebarData(claims))
//	c.HTML(http.StatusOK, "nama_template.html", data)
func SidebarData(claims *auth.Claims) gin.H {
	data := gin.H{}
 
	// Tampilkan badge jumlah permintaan pending di menu supervisor
	for _, role := range claims.Roles {
		if role == "supervisor" {
			if stats, err := requests.GetStats(); err == nil {
				data["sidebarPendingCount"] = stats.Pending
			}
			break
		}
	}
 
	return data
}
 
// MergeH menggabungkan dua gin.H menjadi satu.
// Key dari `extra` akan menimpa key yang sama di `base`.
func MergeH(base gin.H, extra gin.H) gin.H {
	for k, v := range extra {
		base[k] = v
	}
	return base
}
 