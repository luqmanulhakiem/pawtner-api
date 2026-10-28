package routes

import (
	"net/http"
	"pawtner-api/controllers"
	"pawtner-api/helpers"
	"pawtner-api/middleware"

	"github.com/gin-gonic/gin"
)

func SetupRouter() *gin.Engine {
	r := gin.Default()

	api := r.Group("/api")
	{
		auth := api.Group("/auth")
		{
			auth.POST("/login", controllers.Login)
			auth.POST("/register", controllers.Register)
		}

		api.Use(middleware.AuthMiddleware())
		{
			// Contoh endpoint yang butuh token
			api.GET("/profile", func(c *gin.Context) {
				// Mengambil data user ID yang disimpan dari middleware sebelumnya
				userID, _ := c.Get("currentUserId")
				email, _ := c.Get("currentUserEmail")

				helpers.Success(c, http.StatusOK, "Access granted to protected route", gin.H{
					"user_id": userID,
					"email":   email,
				})
			})
		}
	}

	return r
}
