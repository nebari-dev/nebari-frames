package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/nebari-dev/nebari-frames/backend/internal/auth"
	"github.com/nebari-dev/nebari-frames/backend/internal/devfixture"
	"github.com/nebari-dev/nebari-frames/backend/internal/frames"
	mcppkg "github.com/nebari-dev/nebari-frames/backend/internal/mcp"
	"github.com/nebari-dev/nebari-frames/backend/internal/seed"
	"github.com/nebari-dev/nebari-frames/backend/internal/server"
	sqlitestore "github.com/nebari-dev/nebari-frames/backend/internal/store/sqlite"
	"github.com/nebari-dev/nebari-frames/backend/internal/store/sqlite/migrations"
)

func main() {
	port := envOr("PORT", "8080")
	dbPath := envOr("DB_PATH", "nebari-frames.db")

	db, err := sqlitestore.Open(dbPath)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := migrations.Run(context.Background(), db); err != nil {
		log.Fatalf("run migrations: %v", err)
	}

	repo := sqlitestore.New(db)

	if err := seed.Run(context.Background(), repo, seed.Config{
		OrgSlug:        os.Getenv("SEED_ORG_SLUG"),
		OrgDisplayName: os.Getenv("SEED_ORG_DISPLAY_NAME"),
		AdminSub:       os.Getenv("SEED_ADMIN_SUB"),
		AdminEmail:     os.Getenv("SEED_ADMIN_EMAIL"),
	}); err != nil {
		log.Fatalf("seed: %v", err)
	}

	if os.Getenv("SEED_DEV_FIXTURE") == "true" {
		if err := devfixture.Load(context.Background(), repo, os.Getenv("SEED_ORG_SLUG")); err != nil {
			log.Fatalf("dev fixture: %v", err)
		}
		log.Println("SEED_DEV_FIXTURE=true - seeded representative local-dev fixture data")
	}

	authCfg := auth.Config{
		IssuerURL:      os.Getenv("OIDC_ISSUER_URL"),
		ClientID:       os.Getenv("OIDC_CLIENT_ID"),
		DeviceClientID: os.Getenv("OIDC_DEVICE_CLIENT_ID"),
		GroupsClaim:    envOr("OIDC_GROUPS_CLAIM", "groups"),
	}
	devMode, err := selectAuthMode(os.Getenv("FRAMES_DEV_MODE"), authCfg.IssuerURL, authCfg.ClientID)
	if err != nil {
		log.Fatalf("%v", err)
	}
	var validator auth.TokenValidator
	if devMode {
		log.Println("WARNING: FRAMES_DEV_MODE=true - authentication DISABLED; injecting fixed dev-user identity")
	} else {
		validator = auth.NewLazyValidator(context.Background(), authCfg)
		log.Printf("auth enabled (issuer: %s); validating OIDC readiness in background", authCfg.IssuerURL)
	}

	mcpCfg := mcppkg.Config{
		PublicURL: os.Getenv("FRAMES_PUBLIC_URL"),
		IssuerURL: authCfg.IssuerURL,
		Audience:  os.Getenv("OIDC_MCP_AUDIENCE"),
		DevMode:   devMode,
	}
	var mcpValidator auth.TokenValidator
	if !devMode && mcpCfg.PublicURL != "" {
		mcpAuthCfg := authCfg
		mcpAuthCfg.ClientID = mcpCfg.ResolvedAudience()
		mcpValidator = auth.NewLazyValidator(context.Background(), mcpAuthCfg)
		log.Printf("MCP endpoint enabled (resource: %s)", mcpCfg.ResolvedAudience())
	}
	// Kept as a server.Mounter (interface) so a disabled endpoint is a nil
	// interface, not a typed-nil *Component that would satisfy a != nil check.
	var mcpMounter server.Mounter
	if mcpCfg.PublicURL != "" || devMode {
		framesService := frames.NewService(repo)
		mcpMounter = mcppkg.NewComponent(mcpCfg, framesService, mcpValidator)
	}

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           server.New(repo, validator, authCfg, devMode, mcpMounter).Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	log.Printf("starting server on :%s (db: %s)", port, dbPath)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// selectAuthMode resolves the auth bootstrap decision from environment values.
// FRAMES_DEV_MODE=true (exactly) disables auth. Otherwise OIDC_ISSUER_URL and
// OIDC_CLIENT_ID are both required; a missing one is a fatal misconfiguration.
func selectAuthMode(devModeEnv, issuerURL, clientID string) (devMode bool, err error) {
	if devModeEnv == "true" {
		return true, nil
	}
	var missing []string
	if issuerURL == "" {
		missing = append(missing, "OIDC_ISSUER_URL")
	}
	if clientID == "" {
		missing = append(missing, "OIDC_CLIENT_ID")
	}
	if len(missing) > 0 {
		return false, fmt.Errorf(
			"authentication is required but OIDC configuration is incomplete: %s not set; "+
				"set OIDC_ISSUER_URL and OIDC_CLIENT_ID, or set FRAMES_DEV_MODE=true for local development without authentication",
			strings.Join(missing, " and "),
		)
	}
	return false, nil
}
