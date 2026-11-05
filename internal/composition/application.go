package composition

import (
	"github.com/raulaguila/authenticity/internal/core/port/input"
	"github.com/raulaguila/authenticity/internal/core/port/output"
)

// Application holds inbound ports wired for the HTTP/API process.
type Application struct {
	Health     input.HealthChecker
	Authorizer input.Authorizer
	Auth       input.AuthService
	Products   input.ProductService
	Tokens     output.TokenIssuer
	Sessions   output.SessionRepository
	Rates      output.RateCounter // nil keeps rate limit counters in process memory
}
