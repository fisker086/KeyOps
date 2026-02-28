package temporal

import (
	"log"
	"os"

	"github.com/fisker086/keyops/internal/aiassistant"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

// StartWorker 在后台启动 Temporal Worker，注册「巡检+发报告」工作流与 Activity；需已配置 TEMPORAL_HOST、TEMPORAL_TASK_QUEUE
func StartWorker(h *aiassistant.Handler) {
	host := os.Getenv("TEMPORAL_HOST")
	taskQueue := os.Getenv("TEMPORAL_TASK_QUEUE")
	if host == "" || taskQueue == "" || h == nil {
		return
	}
	c, err := client.Dial(client.Options{HostPort: host})
	if err != nil {
		log.Printf("[AI Assistant Temporal] Worker dial failed: %v", err)
		return
	}
	defer c.Close()
	w := worker.New(c, taskQueue, worker.Options{})
	w.RegisterWorkflow(RunInspectionAndSendWorkflow)
	w.RegisterActivity(&Activities{Handler: h})
	if err := w.Run(worker.InterruptCh()); err != nil {
		log.Printf("[AI Assistant Temporal] Worker run error: %v", err)
	}
}

// RunWorkerInBackground 在 goroutine 中启动 Worker（供主程序调用）
func RunWorkerInBackground(h *aiassistant.Handler) {
	go StartWorker(h)
}
