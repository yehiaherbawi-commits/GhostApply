package main

import (
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"runtime"

	tea "github.com/charmbracelet/bubbletea"
)

type model struct {
	db     *sql.DB
	apps   []Application
	cursor int // Tracks which job is highlighted
	errMsg string
}

func initialModel(db *sql.DB) model {
	apps, _ := GetApplications(db)
	return model{
		db:   db,
		apps: apps,
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "up", "k":
			m.errMsg = ""
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			m.errMsg = ""
			if m.cursor < len(m.apps)-1 {
				m.cursor++
			}
		case "o":
			// Open the PDF for the selected company
			if len(m.apps) > 0 {
				selected := m.apps[m.cursor]
				filename := fmt.Sprintf("Resume_%s.pdf", selected.Company)
				if _, err := os.Stat(filename); os.IsNotExist(err) {
					m.errMsg = fmt.Sprintf("File not found: %s", filename)
				} else {
					m.errMsg = ""
					openFile(filename)
				}
			}
		}
	}
	return m, nil
}

func (m model) View() string {
	s := "\n  🤖 CAREER-OPS TERMINAL DASHBOARD \n\n"
	s += "  ID    | COMPANY         | ROLE                      | SCORE | STATUS\n"
	s += "  --------------------------------------------------------------------------------\n"

	for i, app := range m.apps {
		cursor := " "
		if m.cursor == i {
			cursor = ">" // Highlight the selected row
		}

		s += fmt.Sprintf("%s %-5d | %-15s | %-25s | %-5.1f | %-10s\n",
			cursor, app.ID, app.Company, app.Role, app.Score, app.Status)
	}

	s += "\n\n  [↑/↓] Move  •  [o] Open Resume PDF  •  [q] Quit\n"
	if m.errMsg != "" {
		s += fmt.Sprintf("\n  ⚠️  %s\n", m.errMsg)
	}
	return s
}

// openFile uses the OS-specific command to open the PDF
func openFile(path string) {
	var err error
	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", path).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", path).Start()
	case "darwin":
		err = exec.Command("open", path).Start()
	}
	if err != nil {
		fmt.Printf("Could not open file: %v\n", err)
	}
}
