package router

import (
	"github.com/fisker086/keyops/internal/api/handler"
	"github.com/gin-gonic/gin"
)

func registerBill(
	authenticated *gin.RouterGroup,
	billHandler *handler.BillHandler,
	expensesMapHandler *handler.ExpensesMapHandler,
	cloudAccountHandler *handler.CloudAccountHandler,
	resourcesHandler *handler.ResourcesHandler,
	billDashboardHandler *handler.BillDashboardHandler,
) {
	bill := authenticated.Group("/bill")
	{
		bill.GET("/records", billHandler.GetRecords)
		bill.GET("/summary", billHandler.GetSummary)
		bill.GET("/statistics", billHandler.GetStatistics)
		bill.GET("/trend", billHandler.GetTrend)
		bill.GET("/trend/month", billHandler.GetTrendMonth)
		bill.GET("/vm", billHandler.GetVM)

		bill.GET("/breakdown/tags", billHandler.GetBreakdownByTags)
		bill.GET("/breakdown/accounts", billHandler.GetBreakdownByAccounts)
		bill.GET("/breakdown/region", billHandler.GetBreakdownByRegion)
		bill.GET("/breakdown/service", billHandler.GetBreakdownByService)

		cloud := bill.Group("/cloud-accounts")
		{
			cloud.GET("", cloudAccountHandler.ListCloudAccounts)
			cloud.POST("", cloudAccountHandler.AddCloudAccount)
			cloud.GET("/:id", cloudAccountHandler.GetCloudAccount)
			cloud.PUT("/:id", cloudAccountHandler.UpdateCloudAccount)
			cloud.DELETE("/:id", cloudAccountHandler.DeleteCloudAccount)
			cloud.POST("/validate", cloudAccountHandler.ValidateCloudAccount)
			cloud.POST("/:id/sync", cloudAccountHandler.SyncCloudAccount)
		}

		bill.POST("/sync/:cloud_account_id", billHandler.SyncBilling)
		bill.POST("/trigger-sync", billHandler.TriggerSync)
		bill.GET("/summary-by-cloud", billHandler.GetSummaryByCloud)
		bill.GET("/pricing", billHandler.GetPricing)

		bill.GET("/region-expenses", expensesMapHandler.GetRegionExpenses)
		bill.GET("/traffic-expenses", expensesMapHandler.GetTrafficExpenses)

		resources := bill.Group("/resources")
		{
			resources.GET("/expenses-breakdown", resourcesHandler.ExpensesBreakdown)
			resources.GET("/resource-count-breakdown", resourcesHandler.ResourceCountBreakdown)
		}

		bill.GET("/dashboard", billDashboardHandler.GetDashboardData)
		bill.GET("/recommendations", billDashboardHandler.GetRecommendations)
	}
}
