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

func (h *GroupsHandler) RegisterRoutes(app *fiber.App) {
	api := app.Group("/api")
	api.Get("/groups", h.List)
	api.Post("/groups", h.Create)
	api.Get("/groups/:id", h.Get)
	api.Put("/groups/:id", h.Update)
	api.Delete("/groups/:id", h.Delete)
	api.Get("/groups/:id/health", h.Health)
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

type CreateGroupRequest struct {
	Identifier            string `json:"identifier"`
	Name                  string `json:"name"`
	IgnoreWithoutPassword *bool  `json:"ignore_without_password,omitempty"`
}

func (h *GroupsHandler) Create(c *fiber.Ctx) error {
	var req CreateGroupRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request body"})
	}

	ignoreWithoutPassword := false
	if req.IgnoreWithoutPassword != nil {
		ignoreWithoutPassword = *req.IgnoreWithoutPassword
	}

	group, err := h.uc.CreateGroup(req.Identifier, req.Name, ignoreWithoutPassword)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(201).JSON(group)
}

type UpdateGroupRequest struct {
	Identifier            *string `json:"identifier,omitempty"`
	Name                  *string `json:"name,omitempty"`
	Active                *bool   `json:"active,omitempty"`
	Dead                  *bool   `json:"dead,omitempty"`
	IgnoreWithoutPassword *bool   `json:"ignore_without_password,omitempty"`
}

func (h *GroupsHandler) Update(c *fiber.Ctx) error {
	id := c.Params("id")

	var req UpdateGroupRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request body"})
	}

	group, err := h.uc.UpdateGroup(id, req.Identifier, req.Name, req.Active, req.Dead, req.IgnoreWithoutPassword)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(group)
}

func (h *GroupsHandler) Delete(c *fiber.Ctx) error {
	id := c.Params("id")
	if err := h.uc.DeleteGroup(id); err != nil {
		return c.Status(404).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"deleted": true})
}

func (h *GroupsHandler) Health(c *fiber.Ctx) error {
	id := c.Params("id")
	result, err := h.uc.CheckHealth(id)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(result)
}
