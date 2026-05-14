package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/codehia/goflash/internal/store"
)

type cardsLoadedMsg struct {
	cards []store.Card
	err   error
}

func InitCardList(m RootModel) tea.Cmd {
	return func() tea.Msg {
		cards, err := store.GetCardsForTopic(m.db, m.selectedTopicID)
		if err != nil {
			return cardsLoadedMsg{err: err}
		}
		return cardsLoadedMsg{cards: cards}
	}
}

func updateCardList(msg tea.Msg, m RootModel) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case cardsLoadedMsg:
		m.cards = msg.cards
		return m, nil

	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.cards)-1 {
				m.cursor++
			}

		case "enter", "space":
			id := m.cards[m.cursor].ID
			return m, func() tea.Msg {
				return CardSelectedMsg{cardID: *id}
			}
		}

	}
	return m, nil
}

func cardListView(m RootModel) string {
	var s strings.Builder
	s.WriteString("Pick the card you want to attempt: \n\n")
	for i, card := range m.cards {
		cursor := " "
		if m.cursor == i {
			cursor = ">"
		}
		fmt.Fprintf(&s, "%s %s\n", cursor, card.Question)
	}
	s.WriteString("\nPress q to quit.\n")
	return s.String()
}
