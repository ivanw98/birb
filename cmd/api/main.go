// Command api is the birb HTTP backend; it wires config, database, repositories, services, and handlers into a server with graceful shutdown.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver
	"github.com/jmoiron/sqlx"

	"github.com/ivanw98/birb/internal/auth"
	"github.com/ivanw98/birb/internal/config"
	"github.com/ivanw98/birb/internal/handler"
	"github.com/ivanw98/birb/internal/httpapi"
	"github.com/ivanw98/birb/internal/service"
	"github.com/ivanw98/birb/internal/store"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	var err error
	if wantsMigrate(os.Args[1:]) {
		err = runMigrate(ctx, log)
	} else {
		err = run(ctx, log)
	}
	if err != nil {
		log.Error("fatal", "error", err)
		os.Exit(1)
	}
}

// wantsMigrate reports whether "migrate" appears anywhere in args, not just first: Fly prefixes the
// image ENTRYPOINT to release_command, so the binary path can arrive twice.
func wantsMigrate(args []string) bool {
	for _, a := range args {
		if a == "migrate" {
			return true
		}
	}
	return false
}

func run(ctx context.Context, log *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	db, err := sqlx.ConnectContext(ctx, "pgx", cfg.DatabaseURL)
	if err != nil {
		return err
	}
	//nolint:errcheck // best effort
	defer db.Close()
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	verifier, err := auth.NewJWKSVerifier(ctx, cfg.JWKSURL, cfg.JWTIssuer, cfg.JWTAudience)
	if err != nil {
		return err
	}

	repos := store.New(db)
	sightingSvc := service.NewSightings(repos.Sightings, repos.Birds, log)
	birdSvc := service.NewBirds(repos.Birds)
	accountSvc := service.NewAccount(repos.Users)
	groupSvc := service.NewGroups(repos.Groups, service.NewJoinLimiter(0, 0))
	feedSvc := service.NewFeed(repos.Feed, repos.Groups)
	h := handler.New(sightingSvc, birdSvc, accountSvc, groupSvc, feedSvc, log)
	authn := auth.NewAuthenticator(verifier, repos.Users, log)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           httpapi.NewRouter(h, authn, cfg.StaticDir),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		log.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}
