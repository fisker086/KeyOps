package router

import (
	"github.com/fisker086/keyops/internal/api/handler"
	"github.com/gin-gonic/gin"
)

func registerBill(
	authenticated *gin.RouterGroup,
	billHandler *handler.BillHandler,
	cloudAccountHandler *handler.CloudAccountHandler,
	resourcesHandler *handler.ResourcesHandler,
	billDashboardHandler *handler.BillDashboardHandler,
) {
	bill := authenticated.Group("/bill")
	{
		cloud := bill.Group("/cloud-accounts")
		{
			cloud.GET("", cloudAccountHandler.ListCloudAccounts)
			cloud.POST("", cloudAccountHandler.AddCloudAccount)
			cloud.GET("/:id", cloudAccountHandler.GetCloudAccount)
			cloud.PUT("/:id", cloudAccountHandler.UpdateCloudAccount)
			cloud.DELETE("/:id", cloudAccountHandler.DeleteCloudAccount)
			cloud.POST("/validate", cloudAccountHandler.ValidateCloudAccount)
			cloud.POST("/:id/sync", cloudAccountHandler.SyncCloudAccount)
			cloud.POST("/:id/sync/cancel", cloudAccountHandler.CancelSync)
		}

		bill.POST("/sync/:cloud_account_id", billHandler.SyncBilling)
		bill.POST("/trigger-sync", billHandler.TriggerSync)
		bill.POST("/rebuild-aggregates", billHandler.RebuildAggregates)
		bill.GET("/summary-by-cloud", billHandler.GetSummaryByCloud)
		bill.GET("/pricing", billHandler.GetPricing)

		resources := bill.Group("/resources")
		{
			resources.GET("/expenses-breakdown", resourcesHandler.ExpensesBreakdown)
		}

		bill.GET("/dashboard/full", billDashboardHandler.GetDashboardFull)
		bill.GET("/recommendations", billDashboardHandler.GetRecommendations)
		bill.GET("/insights", billDashboardHandler.GetInsights)
	}
}
