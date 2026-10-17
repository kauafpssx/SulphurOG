package api

import (
	"github.com/gofiber/fiber/v2"
	"github.com/sulphurog/sulphurog/internal/usecase"
)

type GroupsHandler struct {
	uc *usecase.ManageGroupsUseCase
}

func NewGroupsHandler(uc *usecase.ManageGroupsUseCase) *GroupsHandler {
	return &GroupsHandler{uc: uc}
}

// RegisterRoutes registra apenas as rotas de leitura de grupos.
// As rotas de criação/atualização/exclusão foram removidas; o gerenciamento
// de grupos agora é feito exclusivamente via linha de comando (--addgroup).
func (h *GroupsHandler) RegisterRoutes(app *fiber.App) {
	api := app.Group("/api")
	api.Get("/groups", h.List)
	api.Get("/groups/:id", h.Get)
}

func (h *GroupsHandler) List(c *fiber.Ctx) error {
	groups, err := h.uc.ListGroups()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"groups": groups})
}

func (h *GroupsHandler) Get(c *fiber.Ctx) error {
	id := c.Params("id")
	group, err := h.uc.GetGroup(id)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(group)
}