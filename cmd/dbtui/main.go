package main

import (
	"context"
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/otaviosoaresp/dbtui/internal/db"
	"github.com/otaviosoaresp/dbtui/internal/ui"
	"github.com/spf13/pflag"
)

func main() {
	dsn := pflag.String("dsn", "", "PostgreSQL connection string (e.g. postgres://user:pass@localhost:5432/dbname)")
	pflag.Parse()

	if *dsn == "" {
		*dsn = os.Getenv("DATABASE_URL")
	}

	if *dsn == "" {
		fmt.Fprintln(os.Stderr, "Error: connection string required")
		fmt.Fprintln(os.Stderr, "Usage: dbtui --dsn \"postgres://user:pass@localhost:5432/dbname\"")
		fmt.Fprintln(os.Stderr, "   or: DATABASE_URL=... dbtui")
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cfg := db.DefaultConnConfig(*dsn)
	pool, err := db.Connect(ctx, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer pool.Close()

	app := ui.NewApp(pool)
	p := tea.NewProgram(app, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
