package mapper

import "github.com/raulaguila/authenticity/internal/core/domain"

// PaginationDTO is the pagination block of every paged list.
type PaginationDTO struct {
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	TotalItems int64 `json:"total_items"`
	TotalPages int64 `json:"total_pages"`
}

// Pagination describes the page answered out of total rows.
func Pagination(page domain.Page, total int64) PaginationDTO {
	limit := int64(page.Limit)
	return PaginationDTO{
		Page:       page.Number,
		Limit:      page.Limit,
		TotalItems: total,
		TotalPages: (total + limit - 1) / limit,
	}
}

// PreferencesDTO is shared by the user object and the preferences routes.
type PreferencesDTO struct {
	Theme    string `json:"theme" example:"system"`
	Language string `json:"language" example:"pt-BR"`
}

// Preferences converts the domain value.
func Preferences(p domain.Preferences) PreferencesDTO {
	return PreferencesDTO{Theme: p.Theme, Language: p.Language}
}
