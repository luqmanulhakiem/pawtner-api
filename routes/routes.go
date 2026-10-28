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

			partners := api.Group("/partners")
			{
				partners.GET("/search", controllers.SearchPartner)
				partners.POST("/invite", controllers.InvitePartner)
				partners.GET("/requests", controllers.GetPartnerRequests)
				partners.GET("/sent", controllers.GetSentPartnerRequests)
				partners.GET("/active", controllers.GetActivePartner)
				partners.PUT("/:id/accept", controllers.AcceptPartner)
				partners.PUT("/:id/reject", controllers.RejectPartner)
				partners.PUT("/:id/disable", controllers.DisablePartner)
			}
		}
	}

	return r
}
