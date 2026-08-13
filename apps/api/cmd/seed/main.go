package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/identity"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/config"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/database"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/seed"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		slog.Error("seed failed", "error", err)
		os.Exit(1)
	}
}

func run(arguments []string) error {
	flags := flag.NewFlagSet("seed", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	profileName := flags.String("profile", "demo", "synthetic dataset profile: demo, small, or benchmark")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected arguments: %v", flags.Args())
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}
	if cfg.DatabaseAdminURL == "" {
		return fmt.Errorf("DATABASE_ADMIN_URL is required for deterministic bulk seeding")
	}
	if strings.EqualFold(strings.TrimSpace(cfg.Environment), "production") {
		return fmt.Errorf("synthetic seed profiles are disabled in production")
	}
	profile, err := seed.ParseProfile(*profileName)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	pool, err := database.Open(ctx, cfg.DatabaseAdminURL, 1, "crm-seed-"+profile.Name)
	if err != nil {
		return err
	}
	defer pool.Close()

	result, err := seed.Run(ctx, pool, identity.NewPasswordHasher(1), profile, seed.Options{
		Environment: cfg.Environment,
		Email:       cfg.DemoEmail,
		Password:    cfg.DemoPassword,
		Progress:    os.Stdout,
	})
	if err != nil {
		return err
	}
	slog.Info("seed complete",
		"profile", result.Profile,
		"workspace_id", result.WorkspaceID,
		"contacts", result.Counts.Contacts,
		"companies", result.Counts.Companies,
		"deals", result.Counts.Deals,
		"activities", result.Counts.Activities,
		"already_applied", result.AlreadyApplied,
		"dataset_hash", result.DatasetHash,
	)
	return nil
}
