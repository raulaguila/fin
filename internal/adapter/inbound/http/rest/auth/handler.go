package auth

import (
	"github.com/gofiber/fiber/v3"

	"github.com/raulaguila/authenticity/internal/adapter/inbound/http/rest/mapper"
	"github.com/raulaguila/authenticity/internal/adapter/inbound/http/rest/middleware"
	"github.com/raulaguila/authenticity/internal/core/port/input"
	"github.com/raulaguila/authenticity/pkg/apperror"
)

type Handler struct {
	svc input.AuthService
}

func NewHandler(svc input.AuthService) *Handler {
	return &Handler{svc: svc}
}

type registerDTO struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type activateDTO struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

type loginDTO struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type refreshDTO struct {
	RefreshToken string `json:"refresh_token"`
}

type resetRequestDTO struct {
	Email string `json:"email"`
}

type resetConfirmDTO struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

type tokenPairDTO struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

func (h *Handler) Register(c fiber.Ctx) error {
	var body registerDTO
	if err := c.Bind().Body(&body); err != nil {
		return mapper.WriteError(c, apperror.New(apperror.CodeInvalidInput, "invalid body"))
	}
	res, err := h.svc.Register(c.Context(), input.RegisterCommand{Name: body.Name, Email: body.Email})
	if err != nil {
		return mapper.WriteError(c, err)
	}
	out := fiber.Map{
		"user_id": res.UserID,
		"status":  res.Status,
	}
	if res.ManualDelivery {
		out["activation_token"] = res.ActivationToken
	}
	return c.Status(fiber.StatusCreated).JSON(out)
}

func (h *Handler) Activate(c fiber.Ctx) error {
	var body activateDTO
	if err := c.Bind().Body(&body); err != nil {
		return mapper.WriteError(c, apperror.New(apperror.CodeInvalidInput, "invalid body"))
	}
	if err := h.svc.Activate(c.Context(), input.ActivateCommand{Token: body.Token, Password: body.Password}); err != nil {
		return mapper.WriteError(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) Login(c fiber.Ctx) error {
	var body loginDTO
	if err := c.Bind().Body(&body); err != nil {
		return mapper.WriteError(c, apperror.New(apperror.CodeInvalidInput, "invalid body"))
	}
	ip := c.IP()
	ua := string(c.Request().Header.UserAgent())
	pair, err := h.svc.Login(c.Context(), input.LoginCommand{
		Email:     body.Email,
		Password:  body.Password,
		IPAddress: &ip,
		UserAgent: &ua,
	})
	if err != nil {
		return mapper.WriteError(c, err)
	}
	return c.JSON(tokenPairDTO{
		AccessToken:  pair.AccessToken,
		RefreshToken: pair.RefreshToken,
		ExpiresIn:    pair.ExpiresIn,
		TokenType:    pair.TokenType,
	})
}

func (h *Handler) Refresh(c fiber.Ctx) error {
	var body refreshDTO
	if err := c.Bind().Body(&body); err != nil {
		return mapper.WriteError(c, apperror.New(apperror.CodeInvalidInput, "invalid body"))
	}
	pair, err := h.svc.Refresh(c.Context(), input.RefreshCommand{RefreshToken: body.RefreshToken})
	if err != nil {
		return mapper.WriteError(c, err)
	}
	return c.JSON(tokenPairDTO{
		AccessToken:  pair.AccessToken,
		RefreshToken: pair.RefreshToken,
		ExpiresIn:    pair.ExpiresIn,
		TokenType:    pair.TokenType,
	})
}

func (h *Handler) Logout(c fiber.Ctx) error {
	token := middleware.AccessToken(c)
	if token == "" {
		return mapper.WriteError(c, apperror.New(apperror.CodeUnauthorized, "missing bearer token"))
	}
	if err := h.svc.Logout(c.Context(), input.LogoutCommand{AccessToken: token}); err != nil {
		return mapper.WriteError(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) RequestPasswordReset(c fiber.Ctx) error {
	var body resetRequestDTO
	if err := c.Bind().Body(&body); err != nil {
		return mapper.WriteError(c, apperror.New(apperror.CodeInvalidInput, "invalid body"))
	}
	res, err := h.svc.RequestPasswordReset(c.Context(), input.RequestPasswordResetCommand{Email: body.Email})
	if err != nil {
		return mapper.WriteError(c, err)
	}
	out := fiber.Map{"ok": true}
	if res.ManualDelivery {
		out["reset_token"] = res.ResetToken
	}
	return c.JSON(out)
}

func (h *Handler) ConfirmPasswordReset(c fiber.Ctx) error {
	var body resetConfirmDTO
	if err := c.Bind().Body(&body); err != nil {
		return mapper.WriteError(c, apperror.New(apperror.CodeInvalidInput, "invalid body"))
	}
	if err := h.svc.ConfirmPasswordReset(c.Context(), input.ConfirmPasswordResetCommand{
		Token: body.Token, Password: body.Password,
	}); err != nil {
		return mapper.WriteError(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}
