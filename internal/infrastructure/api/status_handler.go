package api

import (
	"github.com/gofiber/fiber/v2"
	"github.com/sulphurog/sulphurog/internal/usecase"
)

type StatusHandler struct {
	manageUC *usecase.ManageGroupsUseCase
}

func NewStatusHandler(manageUC *usecase.ManageGroupsUseCase) *StatusHandler {
	return &StatusHandler{manageUC: manageUC}
}

func (h *StatusHandler) RegisterRoutes(app *fiber.App) {
	app.Get("/api/health", h.Health)
	app.Get("/api/status", h.Status)
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
		"groups_total": len(groups),
		"groups_active": active,
	})
}
