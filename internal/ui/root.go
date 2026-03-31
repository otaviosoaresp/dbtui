package ui

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/otaviosoaresp/dbtui/internal/db"
)

type rootState int

const (
	stateConnectionList rootState = iota
	stateApp
)

type Root struct {
	state    rootState
	connList ConnectionList
	app      App
	pool     *pgxpool.Pool
	width    int
	height   int
}

func NewRoot() Root {
	return Root{
		state:    stateConnectionList,
		connList: NewConnectionList(),
	}
}

func (r Root) Init() tea.Cmd {
	return nil
}

func (r Root) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		r.width = msg.Width
		r.height = msg.Height
		r.connList.SetSize(msg.Width, msg.Height)
		if r.state == stateApp {
			return r.updateApp(msg)
		}
		return r, nil

	case ReadyToStartMsg:
		r.pool = msg.Pool
		return r.transitionToApp()

	case connectDSNMsg:
		return r, r.connectCmd(msg.DSN)

	case poolReadyMsg:
		r.pool = msg.pool
		return r.transitionToApp()

	case ConnectErrorMsg:
		r.state = stateConnectionList
		var cmd tea.Cmd
		r.connList, cmd = r.connList.Update(msg)
		return r, cmd

	case SwitchConnectionMsg:
		if r.pool != nil {
			r.pool.Close()
			r.pool = nil
		}
		r.state = stateConnectionList
		r.connList = NewConnectionList()
		r.connList.SetSize(r.width, r.height)
		return r, nil
	}

	if r.state == stateApp {
		return r.updateApp(msg)
	}

	var cmd tea.Cmd
	r.connList, cmd = r.connList.Update(msg)
	return r, cmd
}

type poolReadyMsg struct {
	pool *pgxpool.Pool
}

func (r Root) connectCmd(dsn string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		cfg := db.DefaultConnConfig(dsn)
		pool, err := db.Connect(ctx, cfg)
		if err != nil {
			return ConnectErrorMsg{Err: err}
		}
		return poolReadyMsg{pool: pool}
	}
}

func (r *Root) transitionToApp() (tea.Model, tea.Cmd) {
	r.state = stateApp
	r.app = NewApp(r.pool)
	r.app.width = r.width
	r.app.height = r.height
	r.app.updateLayout()
	initCmd := r.app.Init()
	return r, initCmd
}

func (r Root) updateApp(msg tea.Msg) (tea.Model, tea.Cmd) {
	model, cmd := r.app.Update(msg)
	if app, ok := model.(App); ok {
		r.app = app
	}
	return r, cmd
}

func (r Root) View() string {
	if r.state == stateApp {
		return r.app.View()
	}
	return r.connList.View()
}
