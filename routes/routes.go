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

			periods := api.Group("/periods")
			{
				periods.POST("", controllers.CreatePeriod)
				periods.GET("", controllers.GetPeriods)
				periods.GET("/insight", controllers.GetPeriodInsight)
				periods.GET("/:id", controllers.GetPeriod)
				periods.PUT("/:id", controllers.UpdatePeriod)
			}

			conflicts := api.Group("/conflicts")
			{
				conflicts.POST("", controllers.CreateConflict)
				conflicts.GET("", controllers.GetConflicts)
				conflicts.GET("/:id", controllers.GetConflict)
				conflicts.PUT("/:id", controllers.UpdateConflict)
				conflicts.PUT("/:id/resolve", controllers.ResolveConflict)
				conflicts.POST("/:id/notes", controllers.AddConflictNote)
			}

			rules := api.Group("/rules")
			{
				rules.POST("", controllers.CreatePartnerRule)
				rules.GET("", controllers.GetPartnerRules)
				rules.GET("/:id", controllers.GetPartnerRule)
				rules.PUT("/:id", controllers.UpdatePartnerRule)
				rules.PUT("/:id/status", controllers.SetPartnerRuleStatus)
				rules.DELETE("/:id", controllers.DeletePartnerRule)
			}

			memories := api.Group("/memories")
			{
				memories.POST("", controllers.CreateMemory)
				memories.GET("", controllers.GetMemories)
				memories.GET("/:id", controllers.GetMemory)
				memories.PUT("/:id", controllers.UpdateMemory)
				memories.DELETE("/:id", controllers.DeleteMemory)
			}

			partners := api.Group("/partners")
			{
				partners.GET("/search", controllers.SearchPartner)
				partners.POST("/invite", controllers.InvitePartner)
				partners.GET("/requests", controllers.GetPartnerRequests)
				partners.GET("/sent", controllers.GetSentPartnerRequests)
				partners.GET("/active", controllers.GetActivePartner)
				partners.GET("/active/moods", controllers.GetPartnerMoods)
				partners.GET("/active/periods", controllers.GetPartnerPeriods)
				partners.GET("/active/periods/today", controllers.GetPartnerPeriodToday)
				partners.PUT("/:id/accept", controllers.AcceptPartner)
				partners.PUT("/:id/reject", controllers.RejectPartner)
				partners.PUT("/:id/disable", controllers.DisablePartner)
			}
		}
	}

	return r
}
