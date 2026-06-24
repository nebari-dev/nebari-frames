package auth

// Config holds OIDC authentication configuration.
type Config struct {
	IssuerURL      string // OIDC discovery URL, e.g. https://keycloak.example.com/realms/myrealm
	ClientID       string // expected audience claim in the JWT
	DeviceClientID string // public client ID for CLI device flow auth (optional)
	AdminGroup     string // OIDC group name for admin role checks
	GroupsClaim    string // JWT claim name for group membership; must be set by caller (main.go defaults to "groups")
}
