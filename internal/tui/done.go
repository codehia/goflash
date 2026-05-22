package tui

/*
done.go — Screen 5: session complete.

Header: empty
Body:   ✓ + topic name + subtitle + stat boxes (cards seen, avg score, to revisit)
Footer: action bar (enter / q)
*/

import (
	"fmt"
	"image/color"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

func updateDone(msg tea.Msg, m RootModel) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "enter":
			m.currentScreen = ScreenTopicList
			m.cursor = 0
			m.cards = nil
			m.cardIndex = 0
			m.sessionScores = nil
			return m, nil
		}
	}
	return m, nil
}

func doneHeader(_ RootModel) string { return "" }

func doneBody(m RootModel) string {
	seen := len(m.cards)
	revisit := 0
	total := 0
	for _, s := range m.sessionScores {
		total += s
		if s < 4 {
			revisit++
		}
	}
	avg := 0.0
	if len(m.sessionScores) > 0 {
		avg = float64(total) / float64(len(m.sessionScores))
	}

	check := colorGreen
	checkMark := lipgloss.NewStyle().Foreground(check).Bold(true).Render("✓")
	title := boldStyle.Render(m.topicName + " complete")
	subtitle := mutedStyle.Render(fmt.Sprintf("You went through all %d cards.", seen))

	statBox := func(value, label string, color color.Color) string {
		val := lipgloss.NewStyle().Foreground(color).Bold(true).Render(value)
		lbl := faintStyle.Render(strings.ToUpper(label))
		inner := lipgloss.NewStyle().Width(14).Align(lipgloss.Center).Render(val + "\n" + lbl)
		return borderedBox(colorBorder).Render(inner)
	}

	boxes := lipgloss.JoinHorizontal(lipgloss.Top,
		statBox(fmt.Sprintf("%d", seen), "cards seen", colorFlamingo),
		statBox(fmt.Sprintf("%.1f", avg), "avg score", colorSapphire),
		statBox(fmt.Sprintf("%d", revisit), "to revisit", colorAmber),
	)
	boxesRow := lipgloss.NewStyle().Width(cardInnerW).Align(lipgloss.Center).Render(boxes)

	content := lipgloss.JoinVertical(lipgloss.Center,
		checkMark,
		title,
		subtitle,
		"",
		boxesRow,
	)
	return lipgloss.Place(cardInnerW, cardInnerH, lipgloss.Center, lipgloss.Center, content)
}

func doneFooter(_ RootModel) string {
	return "\n " + actionBar("enter", "pick another topic", "q", "quit")
}
