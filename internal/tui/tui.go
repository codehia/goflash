package tui

import (
	// "fmt"
	// "os"
	//
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/codehia/goflash/internal/types"
)

type Model struct {
	Choices []types.Topic
	Cursor  int
}

func InitialModel(topics []types.Topic) Model {
	return Model{Choices: topics}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit

		case "up", "k":
			if m.Cursor > 0 {
				m.Cursor--
			}
		case "down", "j":
			if m.Cursor < len(m.Choices)-1 {
				m.Cursor++
			}
		case "enter", "space":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m Model) View() tea.View {
	var s strings.Builder
	s.WriteString("Select the topic you want to work on: \n\n")

	for i, choice := range m.Choices {
		cursor := " "
		if m.Cursor == i {
			cursor = ">"
		}

		fmt.Fprintf(&s, "%s %s\n", cursor, choice.Name)

		// cursor := " "
		// if m.Cursor == i {
		// 	cursor = ">"
		// }
		//
		// checked := " "

		// if _, ok := m.Selected[*i]; ok {
		// 	checked = "x"
		// }
	}
	fmt.Fprintf(&s, "\nPress q to quit.\n")

	return tea.NewView(s.String())
}
