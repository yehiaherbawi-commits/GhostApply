package main

import (
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type model struct {
	db        *sql.DB
	apps      []Application
	cursor    int // Tracks which job is highlighted
	errMsg    string
	viewingMd bool
	mdContent []string
	scrollOff int
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
		case "esc":
			if m.viewingMd {
				m.viewingMd = false
				m.mdContent = nil
			}
		case "up", "k":
			if m.viewingMd {
				if m.scrollOff > 0 {
					m.scrollOff--
				}
			} else {
				m.errMsg = ""
				if m.cursor > 0 {
					m.cursor--
				}
			}
		case "down", "j":
			if m.viewingMd {
				if m.scrollOff < len(m.mdContent)-1 {
					m.scrollOff++
				}
			} else {
				m.errMsg = ""
				if m.cursor < len(m.apps)-1 {
					m.cursor++
				}
			}
		case "enter":
			if m.viewingMd {
				m.viewingMd = false
				m.mdContent = nil
			} else {
				if len(m.apps) > 0 {
					selected := m.apps[m.cursor]
					if selected.ReportPath == "" {
						m.errMsg = "No deep evaluation report available."
					} else {
						content, err := os.ReadFile(selected.ReportPath)
						if err != nil {
							m.errMsg = fmt.Sprintf("Could not read report: %v", err)
						} else {
							m.mdContent = strings.Split(string(content), "\n")
							m.viewingMd = true
							m.scrollOff = 0
							m.errMsg = ""
						}
					}
				}
			}
		case "o":
			// Open the PDF for the selected company
			if !m.viewingMd && len(m.apps) > 0 {
				selected := m.apps[m.cursor]
				// We don't save PDF path directly, but we know the naming convention
				safeName := strings.ReplaceAll(selected.Company, " ", "_")
				filename := fmt.Sprintf("Resume_%s.pdf", safeName)
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
	if m.viewingMd {
		s := "\n  📄 DEEP EVALUATION REPORT PREVIEW\n"
		s += "  --------------------------------------------------------------------------------\n"
		displayLines := 30
		end := m.scrollOff + displayLines
		if end > len(m.mdContent) {
			end = len(m.mdContent)
		}
		for i := m.scrollOff; i < end; i++ {
			s += "  " + m.mdContent[i] + "\n"
		}
		s += "\n  --------------------------------------------------------------------------------\n"
		s += "  [↑/↓] Scroll  •  [Enter/Esc] Close Preview\n"
		return s
	}

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

	s += "\n\n  [↑/↓] Move  •  [Enter] View Report  •  [o] Open PDF  •  [q] Quit\n"
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
