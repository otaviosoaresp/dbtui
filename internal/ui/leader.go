package ui

import (
	tea "github.com/charmbracelet/bubbletea"
)

type LeaderActionResult struct {
	Cmd       tea.Cmd
	StatusMsg string
}

type LeaderAction func(*App) LeaderActionResult
type LeaderDigitAction func(*App, int) LeaderActionResult

type LeaderNode struct {
	Key      string
	Desc     string
	Group    string
	Children []*LeaderNode
	Action   LeaderAction
	Digit    LeaderDigitAction
	DigitDesc string
}

func (n *LeaderNode) findChild(key string) *LeaderNode {
	for _, c := range n.Children {
		if c.Key == key {
			return c
		}
	}
	return nil
}

func buildLeaderRoot() *LeaderNode {
	bufferNode := &LeaderNode{
		Key:  "b",
		Desc: "+buffer",
		Children: []*LeaderNode{
			{
				Key: "b", Desc: "list (picker)", Group: "view",
				Action: func(a *App) LeaderActionResult {
					a.bufferPicker.Show(a.buffers, a.activeBuffer, a.width, a.height)
					return LeaderActionResult{}
				},
			},
			{
				Key: "d", Desc: "delete active", Group: "manage",
				Action: func(a *App) LeaderActionResult {
					if len(a.buffers) <= 1 {
						return LeaderActionResult{StatusMsg: "Cannot close last buffer"}
					}
					closed := a.buffers[a.activeBuffer].Name
					a.closeActiveBuffer()
					return LeaderActionResult{StatusMsg: "Closed: " + closed}
				},
			},
			{
				Key: "n", Desc: "next", Group: "nav",
				Action: func(a *App) LeaderActionResult {
					if len(a.buffers) > 1 {
						a.activeBuffer = (a.activeBuffer + 1) % len(a.buffers)
						a.updateLayout()
						return LeaderActionResult{StatusMsg: bufferStatus(a)}
					}
					return LeaderActionResult{}
				},
			},
			{
				Key: "p", Desc: "previous", Group: "nav",
				Action: func(a *App) LeaderActionResult {
					if len(a.buffers) > 1 {
						a.activeBuffer = (a.activeBuffer - 1 + len(a.buffers)) % len(a.buffers)
						a.updateLayout()
						return LeaderActionResult{StatusMsg: bufferStatus(a)}
					}
					return LeaderActionResult{}
				},
			},
		},
		DigitDesc: "jump to N",
		Digit: func(a *App, n int) LeaderActionResult {
			idx := n - 1
			if idx < 0 || idx >= len(a.buffers) {
				return LeaderActionResult{StatusMsg: "No buffer at " + itoa(n)}
			}
			a.activeBuffer = idx
			a.updateLayout()
			return LeaderActionResult{StatusMsg: bufferStatus(a)}
		},
	}

	return &LeaderNode{
		Key:  "<leader>",
		Desc: "<leader>",
		Children: []*LeaderNode{
			bufferNode,
			{
				Key: "r", Desc: "refresh schema", Group: "database",
				Action: func(a *App) LeaderActionResult {
					if a.loading || a.tableList.filtering {
						return LeaderActionResult{}
					}
					a.loading = true
					return LeaderActionResult{
						Cmd:       a.refreshSchemaCmd(),
						StatusMsg: "Refreshing schema...",
					}
				},
			},
			{
				Key: "c", Desc: "switch connection", Group: "database",
				Action: func(a *App) LeaderActionResult {
					return LeaderActionResult{
						Cmd: func() tea.Msg { return SwitchConnectionMsg{} },
					}
				},
			},
			{
				Key: "q", Desc: "quit", Group: "app",
				Action: func(a *App) LeaderActionResult {
					return LeaderActionResult{Cmd: tea.Quit}
				},
			},
			{
				Key: "?", Desc: "help overlay", Group: "app",
				Action: func(a *App) LeaderActionResult {
					a.help.SetSize(a.width, a.height)
					a.help.Toggle()
					return LeaderActionResult{}
				},
			},
		},
	}
}

func bufferStatus(a *App) string {
	return "Buffer: " + a.buffers[a.activeBuffer].Name + " [" + itoa(a.activeBuffer+1) + "/" + itoa(len(a.buffers)) + "]"
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
