package composition

import (
	"github.com/raulaguila/authenticity/internal/core/port/input"
)

// Application holds inbound ports wired for the HTTP/API process.
type Application struct {
	Health     input.HealthChecker
	Authorizer input.Authorizer
	Auth       input.AuthService
	Account    input.AccountService
	Products   input.ProductService
}
