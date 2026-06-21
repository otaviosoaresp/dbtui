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

var version = "dev"

func main() {
	dsn := pflag.String("dsn", "", "PostgreSQL connection string (e.g. postgres://user:pass@localhost:5432/dbname)")
	showVersion := pflag.Bool("version", false, "Print version and exit")
	pflag.Parse()

	if *showVersion {
		fmt.Printf("dbtui %s\n", version)
		return
	}

	if *dsn == "" {
		*dsn = os.Getenv("DATABASE_URL")
	}

	if *dsn != "" {
		startWithDSN(*dsn)
		return
	}

	startWithForm()
}

func startWithDSN(dsn string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cfg := db.DefaultConnConfig(dsn)
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

func startWithForm() {
	root := ui.NewRoot()
	p := tea.NewProgram(root, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
