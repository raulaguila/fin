package product

import (
	"context"
	"math/big"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/raulaguila/authenticity/internal/core/domain"
	"github.com/raulaguila/authenticity/internal/core/port/input"
	"github.com/raulaguila/authenticity/internal/core/port/output"
	"github.com/raulaguila/authenticity/pkg/apperror"
	"github.com/raulaguila/authenticity/pkg/validate"
)

const (
	optionsCap     = 1000
	maxSearchBytes = 120
)

var sortable = []string{"name", "price", "created_at", "updated_at"}

// Fits NUMERIC(12,2). Strict on purpose: floats accept NaN, Inf and hex.
var priceFormat = regexp.MustCompile(`^[0-9]{1,10}(\.[0-9]{1,2})?$`)

type Service struct {
	Repo output.ProductRepository
}

var _ input.ProductService = (*Service)(nil)

func New(repo output.ProductRepository) *Service {
	return &Service{Repo: repo}
}

func (s *Service) List(ctx context.Context, filter domain.ProductFilter) ([]domain.Product, int64, error) {
	filter.Search = strings.TrimSpace(filter.Search)
	if len(filter.Search) > maxSearchBytes || !utf8.ValidString(filter.Search) || strings.ContainsFunc(filter.Search, unicode.IsControl) {
		return nil, 0, apperror.New(apperror.CodeInvalidInput, "invalid search").WithField("search")
	}
	if filter.Sort == "" {
		filter.Sort = "created_at"
	}
	if !slices.Contains(sortable, filter.Sort) {
		return nil, 0, apperror.New(apperror.CodeInvalidInput, "sort must be one of "+strings.Join(sortable, ", ")).WithField("sort")
	}
	if filter.Order == "" {
		filter.Order = "desc"
	}
	if filter.Order != "asc" && filter.Order != "desc" {
		return nil, 0, apperror.New(apperror.CodeInvalidInput, "order must be asc or desc").WithField("order")
	}
	return s.Repo.List(ctx, filter)
}

func (s *Service) Options(ctx context.Context) ([]domain.Product, error) {
	return s.Repo.ListOptions(ctx, optionsCap)
}

func (s *Service) Get(ctx context.Context, id string) (*domain.Product, error) {
	if err := validate.ID(id); err != nil {
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
	if err := validate.ID(cmd.ID); err != nil {
		return nil, err
	}
	patch := domain.ProductPatch{Enabled: cmd.Enabled}
	if cmd.Name != nil {
		name, err := normalizeName(*cmd.Name)
		if err != nil {
			return nil, err
		}
		patch.Name = &name
	}
	if cmd.Price != nil {
		price, err := normalizePrice(*cmd.Price)
		if err != nil {
			return nil, err
		}
		patch.Price = &price
	}
	return s.Repo.Update(ctx, cmd.ID, patch)
}

func (s *Service) Delete(ctx context.Context, id string) error {
	if err := validate.ID(id); err != nil {
		return err
	}
	return s.Repo.Delete(ctx, id)
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
