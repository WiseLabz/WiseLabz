// Command migrate runs database migrations (up/down).
package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/WiseLabz/wiselabz/internal/config"
	"github.com/WiseLabz/wiselabz/internal/store"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: migrate <up|down>\n")
		os.Exit(1)
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	db, err := store.OpenDB(cfg.DB.Driver, cfg.DB.DSN)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close() //nolint:errcheck

	direction := os.Args[1]
	switch direction {
	case "up":
		if err := store.RunMigrations(db, cfg.DB.Driver, logger); err != nil {
			fmt.Fprintf(os.Stderr, "Migration up failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Migrations complete (up).")
	case "down":
		// TODO: implement down migrations using golang-migrate's Down()
		fmt.Println("Down migrations not yet implemented — drop and recreate manually.")
	default:
		fmt.Fprintf(os.Stderr, "Unknown direction: %s (use 'up' or 'down')\n", direction)
		os.Exit(1)
	}
}
