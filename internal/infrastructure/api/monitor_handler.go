package api

import (
	"context"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
)

type MonitorController struct {
	cancel   context.CancelFunc
	running  bool
	mu       sync.RWMutex
	lastRun  time.Time
	filesProcessed int
}

func NewMonitorController() *MonitorController {
	return &MonitorController{}
}

func (mc *MonitorController) RegisterRoutes(app *fiber.App) {
	api := app.Group("/api/monitor")
	api.Post("/start", mc.Start)
	api.Post("/pause", mc.Pause)
	api.Get("/status", mc.Status)
}

func (mc *MonitorController) Start(c *fiber.Ctx) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	if mc.running {
		return c.JSON(fiber.Map{
			"status":  "already running",
			"running": true,
		})
	}

	ctx, cancel := context.WithCancel(context.Background())
	_ = ctx
	mc.cancel = cancel
	mc.running = true

	// O monitor real vai ser injetado aqui
	// Por agora, so registra o estado
	mc.lastRun = time.Now()

	return c.JSON(fiber.Map{
		"status":  "started",
		"running": true,
	})
}

func (mc *MonitorController) Pause(c *fiber.Ctx) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	if !mc.running {
		return c.JSON(fiber.Map{
			"status":  "already paused",
			"running": false,
		})
	}

	if mc.cancel != nil {
		mc.cancel()
		mc.cancel = nil
	}
	mc.running = false

	return c.JSON(fiber.Map{
		"status":  "paused",
		"running": false,
	})
}

func (mc *MonitorController) Status(c *fiber.Ctx) error {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	return c.JSON(fiber.Map{
		"running":          mc.running,
		"last_run":         mc.lastRun,
		"files_processed":  mc.filesProcessed,
	})
}

func (mc *MonitorController) IsRunning() bool {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	return mc.running
}

func (mc *MonitorController) SetRunning(running bool) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.running = running
}

func (mc *MonitorController) IncrementProcessed() {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.filesProcessed++
}
