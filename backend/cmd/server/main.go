package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	_ "modernc.org/sqlite"

	"github.com/nebari-dev/nebari-frames/backend/internal/auth"
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
	}); err != nil {
		log.Fatalf("seed: %v", err)
	}

	authCfg := auth.Config{
		IssuerURL:   os.Getenv("OIDC_ISSUER_URL"),
		ClientID:    os.Getenv("OIDC_CLIENT_ID"),
		GroupsClaim: envOr("OIDC_GROUPS_CLAIM", "groups"),
	}
	var validator auth.TokenValidator
	if authCfg.IssuerURL != "" {
		v, err := auth.NewValidator(context.Background(), authCfg)
		if err != nil {
			log.Fatalf("init auth: %v", err)
		}
		validator = v
		log.Printf("auth enabled (issuer: %s)", authCfg.IssuerURL)
	} else {
		log.Println("WARNING: running in dev mode with authentication disabled")
	}

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           server.New(repo, validator).Handler(),
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
