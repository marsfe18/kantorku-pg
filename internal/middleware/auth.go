package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"kantorku/internal/auth"
)

const UserClaimsKey = "user_claims"

// AuthRequired validates JWT from cookie
func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := c.Cookie("token")
		if err != nil || token == "" {
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}

		claims, err := auth.ParseToken(token)
		if err != nil {
			c.SetCookie("token", "", -1, "/", "", false, true)
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}

		c.Set(UserClaimsKey, claims)
		c.Next()
	}
}

// RequireRole checks if user has specific role
func RequireRole(role string) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, exists := c.Get(UserClaimsKey)
		if !exists {
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}

		userClaims := claims.(*auth.Claims)
		for _, r := range userClaims.Roles {
			if r == role {
				c.Next()
				return
			}
		}

		c.HTML(http.StatusForbidden, "error.html", gin.H{
			"message": "Anda tidak memiliki akses ke halaman ini",
		})
		c.Abort()
	}
}
