package middleware

import (
	"net/http"
	"pawtner-api/config"
	"pawtner-api/helpers"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			helpers.Error(c, http.StatusUnauthorized, "Authorization header is required")
			c.Abort()
			return
		}

		// Format header: "Bearer <token>"
		tokenParts := strings.Split(authHeader, " ")
		if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
			helpers.Error(c, http.StatusUnauthorized, "Invalid authorization format. Use Bearer <token>")
			c.Abort()
			return
		}

		tokenString := tokenParts[1]

		// Parse dan Validasi Token menggunakan Secret Key terpusat
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			// Pastikan metode signing sesuai (HMAC)
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, http.ErrAbortHandler
			}
			return config.GetJWTSecretKey(), nil
		})

		if err != nil || !token.Valid {
			helpers.Error(c, http.StatusUnauthorized, "Invalid or expired access token")
			c.Abort()
			return
		}

		// Ambil klaim (claims) dan simpan di context Gin agar bisa dipakai di controller
		if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
			c.Set("currentUserId", claims["id"])
			c.Set("currentUserEmail", claims["email"])
		}

		c.Next()
	}
}
