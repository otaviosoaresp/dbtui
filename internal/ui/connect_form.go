package ui

import (
	"context"
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/otaviosoaresp/dbtui/internal/db"
)

type FormConnectedMsg struct {
	Pool *pgxpool.Pool
	DSN  string
}

type ReadyToStartMsg struct {
	Pool *pgxpool.Pool
}

type ConnectErrorMsg struct {
	Err error
}

type ConnectForm struct {
	fields    []textinput.Model
	labels    []string
	focused   int
	width     int
	height    int
	err       error
	connecting bool
}

const (
	fieldHost = iota
	fieldPort
	fieldDatabase
	fieldUser
	fieldPassword
)

func NewConnectForm() ConnectForm {
	fields := make([]textinput.Model, 5)
	labels := []string{"Host", "Port", "Database", "User", "Password"}

	for i := range fields {
		fields[i] = textinput.New()
		fields[i].CharLimit = 128
		fields[i].Width = 40
	}

	fields[fieldHost].Placeholder = "localhost"
	fields[fieldHost].SetValue("localhost")
	fields[fieldHost].Focus()

	fields[fieldPort].Placeholder = "5432"
	fields[fieldPort].SetValue("5432")

	fields[fieldDatabase].Placeholder = "database name"

	fields[fieldUser].Placeholder = "postgres"
	fields[fieldUser].SetValue("postgres")

	fields[fieldPassword].Placeholder = "password"
	fields[fieldPassword].EchoMode = textinput.EchoPassword
	fields[fieldPassword].EchoCharacter = '*'

	return ConnectForm{
		fields: fields,
		labels: labels,
	}
}

func (cf *ConnectForm) SetSize(width, height int) {
	cf.width = width
	cf.height = height
}

func (cf ConnectForm) Update(msg tea.Msg) (ConnectForm, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "tab", "down":
			cf.nextField()
			return cf, nil
		case "shift+tab", "up":
			cf.prevField()
			return cf, nil
		case "enter":
			if cf.connecting {
				return cf, nil
			}
			if cf.focused == len(cf.fields)-1 || msg.String() == "enter" {
				return cf, cf.connect()
			}
			cf.nextField()
			return cf, nil
		case "ctrl+c":
			return cf, tea.Quit
		}
	case FormConnectedMsg:
		return cf, nil
	case ConnectErrorMsg:
		cf.connecting = false
		cf.err = msg.Err
		return cf, nil
	}

	var cmd tea.Cmd
	cf.fields[cf.focused], cmd = cf.fields[cf.focused].Update(msg)
	return cf, cmd
}

func (cf *ConnectForm) nextField() {
	cf.fields[cf.focused].Blur()
	cf.focused = (cf.focused + 1) % len(cf.fields)
	cf.fields[cf.focused].Focus()
}

func (cf *ConnectForm) prevField() {
	cf.fields[cf.focused].Blur()
	cf.focused = (cf.focused - 1 + len(cf.fields)) % len(cf.fields)
	cf.fields[cf.focused].Focus()
}

func (cf ConnectForm) buildDSN() string {
	host := cf.fields[fieldHost].Value()
	port := cf.fields[fieldPort].Value()
	database := cf.fields[fieldDatabase].Value()
	user := cf.fields[fieldUser].Value()
	password := cf.fields[fieldPassword].Value()

	if host == "" {
		host = "localhost"
	}
	if port == "" {
		port = "5432"
	}
	if user == "" {
		user = "postgres"
	}

	if password != "" {
		return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", user, password, host, port, database)
	}
	return fmt.Sprintf("postgres://%s@%s:%s/%s?sslmode=disable", user, host, port, database)
}

func (cf *ConnectForm) connect() tea.Cmd {
	cf.connecting = true
	cf.err = nil
	dsn := cf.buildDSN()

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		cfg := db.DefaultConnConfig(dsn)
		pool, err := db.Connect(ctx, cfg)
		if err != nil {
			return ConnectErrorMsg{Err: err}
		}
		return FormConnectedMsg{Pool: pool, DSN: dsn}
	}
}

func (cf ConnectForm) View() string {
	if cf.width == 0 || cf.height == 0 {
		return ""
	}

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("6")).
		MarginBottom(1)

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("4")).
		Bold(true).
		Width(12)

	focusedLabelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("6")).
		Bold(true).
		Width(12)

	errStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("1")).
		Bold(true)

	dimStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("245"))

	var lines []string

	lines = append(lines, titleStyle.Render("dbTUI - Connect to PostgreSQL"))
	lines = append(lines, "")

	for i, field := range cf.fields {
		ls := labelStyle
		if i == cf.focused {
			ls = focusedLabelStyle
		}
		lines = append(lines, ls.Render(cf.labels[i]+":")+field.View())
	}

	lines = append(lines, "")

	if cf.connecting {
		lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Render("Connecting..."))
	} else if cf.err != nil {
		lines = append(lines, errStyle.Render(fmt.Sprintf("Error: %v", cf.err)))
		lines = append(lines, "")
	}

	lines = append(lines, dimStyle.Render("[Tab] Next field  [Enter] Connect  [Ctrl+C] Quit"))

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("4")).
		Padding(1, 3)

	box := boxStyle.Render(content)

	return lipgloss.Place(
		cf.width, cf.height,
		lipgloss.Center, lipgloss.Center,
		box,
	)
}
