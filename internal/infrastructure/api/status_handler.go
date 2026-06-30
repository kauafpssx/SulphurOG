package api

import (
	"github.com/gofiber/fiber/v2"
	"github.com/sulphurog/sulphurog/internal/domain"
	"github.com/sulphurog/sulphurog/internal/usecase"
)

type StatusHandler struct {
	manageUC *usecase.ManageGroupsUseCase
	tracker  domain.Tracker
}

func NewStatusHandler(manageUC *usecase.ManageGroupsUseCase, tracker domain.Tracker) *StatusHandler {
	return &StatusHandler{manageUC: manageUC, tracker: tracker}
}

func (h *StatusHandler) RegisterRoutes(app *fiber.App) {
	app.Get("/api/health", h.Health)
	app.Get("/api/status", h.Status)
	app.Get("/api/stats", h.Stats)
}

func (h *StatusHandler) Health(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{"status": "ok"})
}

func (h *StatusHandler) Status(c *fiber.Ctx) error {
	groups, err := h.manageUC.ListGroups()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	active := 0
	for _, g := range groups {
		if g.Active {
			active++
		}
	}

	return c.JSON(fiber.Map{
		"groups_total":  len(groups),
		"groups_active": active,
	})
}

func (h *StatusHandler) Stats(c *fiber.Ctx) error {
	groups, err := h.manageUC.ListGroups()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	// Group breakdown
	groupsActive := 0
	groupsDead := 0
	groupsUnauthed := 0
	for _, g := range groups {
		if g.Dead {
			groupsDead++
		} else if g.Active {
			groupsActive++
		} else {
			groupsUnauthed++
		}
	}

	// Tracker detailed stats
	detailed, err := h.tracker.GetDetailedStats()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	detailed.GroupsTotal = len(groups)
	detailed.GroupsActive = groupsActive
	detailed.GroupsDead = groupsDead
	detailed.GroupsUnauthed = groupsUnauthed

	// Predictions
	var predictions fiber.Map
	if detailed.FilesPerDay > 0 {
		avgFileSizeGB := detailed.AvgFileSizeMB / 1024
		// Assume 50GB Supabase free tier
		bucketCapacityGB := 50.0
		usedGB := float64(detailed.FinishedBytes) / 1024 / 1024 / 1024
		freeGB := bucketCapacityGB - usedGB
		daysUntilFull := 0.0
		if avgFileSizeGB > 0 && detailed.FilesPerDay > 0 {
			daysUntilFull = freeGB / (avgFileSizeGB * detailed.FilesPerDay)
		}

		predictions = fiber.Map{
			"files_per_day":    detailed.FilesPerDay,
			"files_per_month":  detailed.FilesPerDay * 30,
			"avg_file_size_mb": detailed.AvgFileSizeMB,
			"bucket_used_gb":   usedGB,
			"bucket_free_gb":   freeGB,
			"days_until_full":  daysUntilFull,
		}
	}

	return c.JSON(fiber.Map{
		"queued":       detailed.Queued,
		"downloading":  detailed.Downloading,
		"downloaded":   detailed.Downloaded,
		"uploading":    detailed.Uploading,
		"finished":     detailed.Finished,
		"failed":       detailed.Failed,
		"pending":      detailed.Pending,
		"total_bytes":  detailed.TotalBytes,
		"finished_bytes": detailed.FinishedBytes,
		"total_ulps":   detailed.TotalULPs,
		"by_extension": detailed.ByExtension,
		"groups_total":    detailed.GroupsTotal,
		"groups_active":   detailed.GroupsActive,
		"groups_dead":     detailed.GroupsDead,
		"groups_unauthed": detailed.GroupsUnauthed,
		"first_file_at": detailed.FirstFileAt,
		"last_file_at":  detailed.LastFileAt,
		"predictions":   predictions,
	})
}
