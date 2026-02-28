package main

import (
	_ "github.com/fisker086/keyops/docs" // swagger docs
	"github.com/fisker086/keyops/internal/app"
)

// @title           KeyOps API
// @version         2.0
// @description     KeyOps 基础设施管理平台 API 文档
// @termsOfService  http://swagger.io/terms/

// @contact.name   API Support
// @contact.email  support@keyops.com

// @license.name  Apache 2.0
// @license.url   http://www.apache.org/licenses/LICENSE-2.0.html

// @host      localhost:8080
// @BasePath  /api

// @securityDefinitions.apikey Bearer
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and JWT token.

func main() {
	// Initialize application
	application, err := app.Initialize("")
	if err != nil {
		panic(err)
	}

	// Start server
	app.StartServer(
		application.Config,
		application.Handlers,
		application.Services,
		application.Repos,
		application.BackgroundServices,
		application.SSHServer,
		application.UnifiedAuditor,
	)
}
