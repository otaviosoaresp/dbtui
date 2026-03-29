package ui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/otaviosoaresp/dbtui/internal/config"
)

type connectionListState int

const (
	listStateSelect connectionListState = iota
	listStateForm
	listStateSavePrompt
)

type ConnectionList struct {
	connections []config.SavedConnection
	cursor      int
	state       connectionListState
	form        ConnectForm
	width       int
	height      int
	err         error
	lastConn    config.SavedConnection
	pendingPool *pgxpool.Pool
}

func NewConnectionList() ConnectionList {
	conns, _ := config.LoadConnections()

	return ConnectionList{
		connections: conns,
		form:        NewConnectForm(),
	}
}

func (cl *ConnectionList) SetSize(width, height int) {
	cl.width = width
	cl.height = height
	cl.form.SetSize(width, height)
}

func (cl ConnectionList) Update(msg tea.Msg) (ConnectionList, tea.Cmd) {
	switch cl.state {
	case listStateSelect:
		return cl.updateSelect(msg)
	case listStateForm:
		return cl.updateForm(msg)
	case listStateSavePrompt:
		return cl.updateSavePrompt(msg)
	}
	return cl, nil
}

func (cl ConnectionList) updateSelect(msg tea.Msg) (ConnectionList, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		totalItems := len(cl.connections) + 1

		switch msg.String() {
		case "j", "down":
			if cl.cursor < totalItems-1 {
				cl.cursor++
			}
		case "k", "up":
			if cl.cursor > 0 {
				cl.cursor--
			}
		case "enter":
			if cl.cursor < len(cl.connections) {
				conn := cl.connections[cl.cursor]
				cl.lastConn = conn
				return cl, cl.connectSaved(conn)
			}
			cl.state = listStateForm
			cl.form = NewConnectForm()
			cl.form.SetSize(cl.width, cl.height)
			return cl, nil
		case "d":
			if cl.cursor < len(cl.connections) {
				name := cl.connections[cl.cursor].Name
				config.DeleteConnection(name)
				cl.connections, _ = config.LoadConnections()
				if cl.cursor >= len(cl.connections) && cl.cursor > 0 {
					cl.cursor--
				}
			}
			return cl, nil
		case "q", "ctrl+c":
			return cl, tea.Quit
		}
	}
	return cl, nil
}

func (cl ConnectionList) updateForm(msg tea.Msg) (ConnectionList, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "esc" && !cl.form.connecting {
			if len(cl.connections) > 0 {
				cl.state = listStateSelect
				return cl, nil
			}
		}
	case FormConnectedMsg:
		cl.pendingPool = msg.Pool
		cl.lastConn = config.SavedConnection{
			Host:     cl.form.fields[fieldHost].Value(),
			Port:     cl.form.fields[fieldPort].Value(),
			Database: cl.form.fields[fieldDatabase].Value(),
			User:     cl.form.fields[fieldUser].Value(),
			Password: cl.form.fields[fieldPassword].Value(),
		}
		cl.state = listStateSavePrompt
		return cl, nil
	}

	var cmd tea.Cmd
	cl.form, cmd = cl.form.Update(msg)
	return cl, cmd
}

func (cl ConnectionList) updateSavePrompt(msg tea.Msg) (ConnectionList, tea.Cmd) {
	pool := cl.pendingPool
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "y", "Y":
			name := cl.lastConn.Database
			if name == "" {
				name = fmt.Sprintf("%s:%s", cl.lastConn.Host, cl.lastConn.Port)
			}
			cl.lastConn.Name = name
			config.SaveConnection(cl.lastConn)
			cl.connections, _ = config.LoadConnections()
			return cl, func() tea.Msg {
				return ReadyToStartMsg{Pool: pool}
			}
		case "n", "N", "enter":
			return cl, func() tea.Msg {
				return ReadyToStartMsg{Pool: pool}
			}
		}
	}
	return cl, nil
}

func (cl ConnectionList) connectSaved(conn config.SavedConnection) tea.Cmd {
	dsn := conn.DSN()
	return func() tea.Msg {
		return connectDSNMsg{DSN: dsn}
	}
}

type connectDSNMsg struct {
	DSN string
}

func (cl ConnectionList) View() string {
	if cl.width == 0 || cl.height == 0 {
		return ""
	}

	switch cl.state {
	case listStateForm:
		return cl.form.View()
	case listStateSavePrompt:
		return cl.renderSavePrompt()
	default:
		return cl.renderList()
	}
}

func (cl ConnectionList) renderList() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("6"))

	selectedStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("4")).
		Foreground(lipgloss.Color("15")).
		Bold(true)

	normalStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("250"))

	nameStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("15")).
		Bold(true)

	detailStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("245"))

	dimStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("245"))

	newConnStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("6"))

	var lines []string
	lines = append(lines, titleStyle.Render("  dbTUI - Connections"))
	lines = append(lines, "")

	for i, conn := range cl.connections {
		prefix := "  "
		if i == cl.cursor {
			prefix = "> "
			line := fmt.Sprintf("%s%-20s %s", prefix, conn.Name, conn.DisplayString())
			lines = append(lines, selectedStyle.Width(cl.width-8).Render(line))
		} else {
			name := nameStyle.Render(fmt.Sprintf("%-20s", conn.Name))
			detail := detailStyle.Render(conn.DisplayString())
			lines = append(lines, normalStyle.Render(prefix)+name+" "+detail)
		}
	}

	lines = append(lines, "")

	newIdx := len(cl.connections)
	if cl.cursor == newIdx {
		lines = append(lines, selectedStyle.Width(cl.width-8).Render("> + New Connection"))
	} else {
		lines = append(lines, newConnStyle.Render("  + New Connection"))
	}

	lines = append(lines, "")
	lines = append(lines, dimStyle.Render("  [Enter] Connect  [d] Delete  [q] Quit"))

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("4")).
		Padding(1, 2)

	box := boxStyle.Render(content)

	return lipgloss.Place(
		cl.width, cl.height,
		lipgloss.Center, lipgloss.Center,
		box,
	)
}

func (cl ConnectionList) renderSavePrompt() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("6"))

	connStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("15")).
		Bold(true)

	dimStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("245"))

	var lines []string
	lines = append(lines, titleStyle.Render("  Connection successful!"))
	lines = append(lines, "")
	lines = append(lines, "  "+connStyle.Render(cl.lastConn.DisplayString()))
	lines = append(lines, "")
	lines = append(lines, "  Save this connection for future use?")
	lines = append(lines, "")
	lines = append(lines, dimStyle.Render("  [y] Yes  [n] No"))

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("4")).
		Padding(1, 2)

	box := boxStyle.Render(content)

	return lipgloss.Place(
		cl.width, cl.height,
		lipgloss.Center, lipgloss.Center,
		box,
	)
}
