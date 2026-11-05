package product

import (
	"strconv"

	"github.com/gofiber/fiber/v3"

	"github.com/raulaguila/authenticity/internal/adapter/inbound/http/rest/mapper"
	"github.com/raulaguila/authenticity/internal/core/domain"
	"github.com/raulaguila/authenticity/internal/core/port/input"
	"github.com/raulaguila/authenticity/pkg/apperror"
	"github.com/raulaguila/authenticity/pkg/pointer"
)

type Handler struct {
	svc input.ProductService
}

func NewHandler(svc input.ProductService) *Handler {
	return &Handler{svc: svc}
}

type productDTO struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Price   string `json:"price"`
	Enabled bool   `json:"enabled"`
}

type createDTO struct {
	Name    string `json:"name"`
	Price   any    `json:"price"`
	Enabled *bool  `json:"enabled"`
}

type updateDTO struct {
	Name    *string `json:"name"`
	Price   any     `json:"price"`
	Enabled *bool   `json:"enabled"`
}

func toDTO(p domain.Product) productDTO {
	return productDTO{ID: p.ID, Name: p.Name, Price: p.Price, Enabled: p.Enabled}
}

func priceString(v any) (string, error) {
	switch t := v.(type) {
	case nil:
		return "", apperror.New(apperror.CodeInvalidInput, "price required").WithField("price")
	case string:
		return t, nil
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64), nil
	default:
		return "", apperror.New(apperror.CodeInvalidInput, "price must be a number").WithField("price")
	}
}

func (h *Handler) List(c fiber.Ctx) error {
	limit, _ := strconv.Atoi(c.Query("limit", "100"))
	items, err := h.svc.List(c.Context(), limit)
	if err != nil {
		return mapper.WriteError(c, err)
	}
	out := make([]productDTO, 0, len(items))
	for _, p := range items {
		out = append(out, toDTO(p))
	}
	return c.JSON(fiber.Map{"items": out})
}

func (h *Handler) Get(c fiber.Ctx) error {
	p, err := h.svc.Get(c.Context(), c.Params("id"))
	if err != nil {
		return mapper.WriteError(c, err)
	}
	return c.JSON(toDTO(*p))
}

func (h *Handler) Create(c fiber.Ctx) error {
	var body createDTO
	if err := c.Bind().Body(&body); err != nil {
		return mapper.WriteError(c, apperror.New(apperror.CodeInvalidInput, "invalid body"))
	}
	price, err := priceString(body.Price)
	if err != nil {
		return mapper.WriteError(c, err)
	}
	p, err := h.svc.Create(c.Context(), input.CreateProductCommand{
		Name: body.Name, Price: price, Enabled: body.Enabled,
	})
	if err != nil {
		return mapper.WriteError(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(toDTO(*p))
}

func (h *Handler) Update(c fiber.Ctx) error {
	var body updateDTO
	if err := c.Bind().Body(&body); err != nil {
		return mapper.WriteError(c, apperror.New(apperror.CodeInvalidInput, "invalid body"))
	}
	cmd := input.UpdateProductCommand{ID: c.Params("id"), Name: body.Name, Enabled: body.Enabled}
	if body.Price != nil {
		price, err := priceString(body.Price)
		if err != nil {
			return mapper.WriteError(c, err)
		}
		cmd.Price = pointer.Of(price)
	}
	p, err := h.svc.Update(c.Context(), cmd)
	if err != nil {
		return mapper.WriteError(c, err)
	}
	return c.JSON(toDTO(*p))
}

func (h *Handler) Delete(c fiber.Ctx) error {
	if err := h.svc.Delete(c.Context(), c.Params("id")); err != nil {
		return mapper.WriteError(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}
