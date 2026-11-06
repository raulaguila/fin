package input

// Application holds the inbound ports of the HTTP process.
type Application struct {
	Health     HealthChecker
	Authorizer Authorizer
	Resources  ResourceCatalog
	Roles      RoleService
	Auth       AuthService
	Account    AccountService
	Products   ProductService
}
