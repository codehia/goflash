package tui

/*
topic_list.go — Screen 1: topic selection.

Header: label + subtitle
Body:   scrollable topic rows
Footer: action bar (enter / q)
*/

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/codehia/goflash/internal/store"
	"github.com/codehia/goflash/internal/types"
)

type topicsLoadedMsg struct {
	topics []types.TopicSummary
	err    error
}

func initTopicList(m RootModel) tea.Cmd {
	return func() tea.Msg {
		summaries, err := store.GetTopicSummaries(m.db)
		if err != nil {
			return topicsLoadedMsg{err: err}
		}
		return topicsLoadedMsg{topics: summaries}
	}
}

func updateTopicList(msg tea.Msg, m RootModel) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case topicsLoadedMsg:
		m.topics = msg.topics
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
			if m.cursor < len(m.topics)-1 {
				m.cursor++
			}
		case "enter":
			if len(m.topics) == 0 {
				return m, nil
			}
			topic := m.topics[m.cursor]
			return m, func() tea.Msg {
				return TopicSelectedMsg{topicID: *topic.ID, topicName: topic.Name}
			}
		}
	}
	return m, nil
}

func topicListHeader(m RootModel) string {
	totalCards := 0
	for _, t := range m.topics {
		totalCards += t.CardCount
	}
	label := hintStyle.Render("Choose a topic to work on")
	subtitle := mutedStyle.Render(fmt.Sprintf("%d cards across %d topics · use ", totalCards, len(m.topics))) +
		purpleStyle.Render("↑↓ to move") +
		mutedStyle.Render(". enter to select")
	return "\n " + label + "\n " + subtitle
}

func topicListBody(m RootModel) string {
	if len(m.topics) == 0 {
		return "\n " + mutedStyle.Render("Loading topics...")
	}

	const maxVisible = 4
	start, end := visibleWindow(m.cursor, len(m.topics), maxVisible)

	var s strings.Builder
	s.WriteString("\n")
	for i := start; i < end; i++ {
		s.WriteString(renderTopicRow(m.topics[i], m.cursor == i) + "\n")
	}
	s.WriteString("\n " + faintStyle.Render(fmt.Sprintf("%d / %d topics", end-start, len(m.topics))))
	return s.String()
}

func topicListFooter(m RootModel) string {
	selectedName := ""
	if m.cursor < len(m.topics) {
		selectedName = strings.ToLower(m.topics[m.cursor].Name)
	}
	return "\n " + actionBar("enter", "start "+selectedName, "q", "quit")
}
