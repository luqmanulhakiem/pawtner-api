package routes

import (
	"pawtner-api/controllers"
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
			moods := api.Group("/daily-moods")
			{
				moods.POST("", controllers.CreateDailyMood)
				moods.GET("", controllers.GetDailyMoods)
				moods.GET("/:id", controllers.GetDailyMood)
				moods.PUT("/:id", controllers.UpdateDailyMood)
			}
		}
	}

	return r
}
