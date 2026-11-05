package product

import (
	"context"
	"math/big"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/raulaguila/authenticity/internal/core/domain"
	"github.com/raulaguila/authenticity/internal/core/port/input"
	"github.com/raulaguila/authenticity/internal/core/port/output"
	"github.com/raulaguila/authenticity/pkg/apperror"
)

const listCap = 100

// Fits NUMERIC(12,2). Strict on purpose: floats accept NaN, Inf and hex.
var priceFormat = regexp.MustCompile(`^[0-9]{1,10}(\.[0-9]{1,2})?$`)

type Service struct {
	Repo output.ProductRepository
}

var _ input.ProductService = (*Service)(nil)

func New(repo output.ProductRepository) *Service {
	return &Service{Repo: repo}
}

func (s *Service) List(ctx context.Context, limit int) ([]domain.Product, error) {
	if limit <= 0 || limit > listCap {
		limit = listCap
	}
	return s.Repo.List(ctx, limit)
}

func (s *Service) Get(ctx context.Context, id string) (*domain.Product, error) {
	if err := validateID(id); err != nil {
		return nil, err
	}
	return s.Repo.FindByID(ctx, id)
}

func (s *Service) Create(ctx context.Context, cmd input.CreateProductCommand) (*domain.Product, error) {
	name, price, err := validateNamePrice(cmd.Name, cmd.Price)
	if err != nil {
		return nil, err
	}
	enabled := true
	if cmd.Enabled != nil {
		enabled = *cmd.Enabled
	}
	p := &domain.Product{Name: name, Price: price, Enabled: enabled}
	if err := s.Repo.Create(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

func (s *Service) Update(ctx context.Context, cmd input.UpdateProductCommand) (*domain.Product, error) {
	if err := validateID(cmd.ID); err != nil {
		return nil, err
	}
	p, err := s.Repo.FindByID(ctx, cmd.ID)
	if err != nil {
		return nil, err
	}
	if cmd.Name != nil {
		name, err := normalizeName(*cmd.Name)
		if err != nil {
			return nil, err
		}
		p.Name = name
	}
	if cmd.Price != nil {
		price, err := normalizePrice(*cmd.Price)
		if err != nil {
			return nil, err
		}
		p.Price = price
	}
	if cmd.Enabled != nil {
		p.Enabled = *cmd.Enabled
	}
	if err := s.Repo.Update(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

func (s *Service) Delete(ctx context.Context, id string) error {
	if err := validateID(id); err != nil {
		return err
	}
	return s.Repo.Delete(ctx, id)
}

func validateID(id string) error {
	if uuid.Validate(id) != nil {
		return apperror.New(apperror.CodeInvalidInput, "invalid id").WithField("id")
	}
	return nil
}

func validateNamePrice(name, price string) (string, string, error) {
	name, err := normalizeName(name)
	if err != nil {
		return "", "", err
	}
	p, err := normalizePrice(price)
	if err != nil {
		return "", "", err
	}
	return name, p, nil
}

func normalizeName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if n := utf8.RuneCountInString(name); n < 1 || n > 120 || strings.ContainsFunc(name, unicode.IsControl) {
		return "", apperror.New(apperror.CodeInvalidInput, "name must be 1-120 characters").WithField("name")
	}
	return name, nil
}

func normalizePrice(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if !priceFormat.MatchString(raw) {
		return "", apperror.New(apperror.CodeInvalidInput, "price must be a decimal with up to 10 digits and 2 decimals").WithField("price")
	}
	price, _ := new(big.Rat).SetString(raw) // cannot fail: priceFormat matched
	return price.FloatString(2), nil
}
