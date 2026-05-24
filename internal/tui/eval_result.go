package tui

/*
eval_result.go — Screen 4: AI feedback + reference answer.

Header: progress bar + topic/subtopic pills
Body:   score box (score + bar + feedback) + reference answer box
Footer: action bar (n / q)
*/

import (
	"fmt"
	"image/color"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/codehia/goflash/internal/scheduler"
	"github.com/codehia/goflash/internal/store"
)

func updateEvalResult(msg tea.Msg, m RootModel) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "n":
			m.cardIndex++
			if m.cardIndex >= len(m.cards) {
				m.currentScreen = ScreenDone
				return m, nil
			}
			m.userAnswer = ""
			m.textarea = buildTextarea()
			m.currentScreen = ScreenCardQuestion
			return m, nil
		}
	}
	return m, nil
}

func evalResultHeader(m RootModel) lipgloss.Style {
	return cardQuestionHeader(m)
}

func evalResultBody(m RootModel) lipgloss.Style {
	r := m.evalResult
	scoreColor := scoreAccentColor(r.Score)
	innerW := contentWidth - roundedBorderH // text area inside inner renderCard border

	scoreAccent := lipgloss.NewStyle().Foreground(scoreColor).Bold(true)

	// AI FEEDBACK header: label left, score right; stars right-aligned below
	contentW := innerW - 2 // -2 for Padding[]int{0,1}: 1 char each side
	scoreNum := scoreAccent.Render(fmt.Sprintf("%d / 5", r.Score))
	label := lipgloss.NewStyle().Foreground(colorFlamingo).Bold(true).Render("AI FEEDBACK")
	gap := contentW - lipgloss.Width(label) - lipgloss.Width(scoreNum)
	scoreLine := label + strings.Repeat(" ", max(gap, 1)) + scoreNum
	stars := scoreAccent.Render(strings.Repeat("★", r.Score)) +
		faintStyle.Render(strings.Repeat("☆", 5-r.Score))
	starsLine := strings.Repeat(" ", max(contentW-lipgloss.Width(stars), 0)) + stars
	feedbackHeader := styledBox(CardParams{BgColor: colorBase, Padding: []int{1, 1}}).SetString(scoreLine + "\n" + starsLine)

	// AI FEEDBACK body: feedback text
	feedback := lipgloss.Wrap(r.Feedback, contentW, "")
	feedbackBody := styledBox(CardParams{BgColor: colorBase, Padding: []int{0, 1}}).SetString(mutedStyle.Render(feedback))

	feedbackCard := renderCard(contentWidth, colorFlamingo, feedbackHeader, feedbackBody, lipgloss.NewStyle())
	feedbackCentered := lipgloss.Place(cardInnerW, lipgloss.Height(feedbackCard), lipgloss.Center, lipgloss.Top, feedbackCard)

	// REFERENCE ANSWER header: label only
	refLabel := lipgloss.NewStyle().Foreground(colorSapphire).Bold(true).Render("REFERENCE ANSWER")
	refHeader := styledBox(CardParams{BgColor: colorBase, Padding: []int{1, 1}}).SetString(refLabel)

	// REFERENCE ANSWER body: answer text
	answer := lipgloss.Wrap(m.cards[m.cardIndex].Answer, contentW, "")
	refBody := styledBox(CardParams{BgColor: colorBase, Padding: []int{0, 1}}).SetString(mutedStyle.Render(answer))

	refCard := renderCard(contentWidth, colorSapphire, refHeader, refBody, lipgloss.NewStyle())
	refCentered := lipgloss.Place(cardInnerW, lipgloss.Height(refCard), lipgloss.Center, lipgloss.Top, refCard)

	return styledBox(CardParams{BgColor: colorBase}).SetString("\n" + feedbackCentered + "\n\n" + refCentered)
}

func evalResultFooter(_ RootModel) lipgloss.Style {
	return styledBox(CardParams{BgColor: colorBase, Padding: []int{1, 1}}).SetString(actionBar("n", "next card", "q", "quit"))
}

func saveAndSchedule(m RootModel) tea.Cmd {
	return func() tea.Msg {
		card := m.cards[m.cardIndex]
		score := m.evalResult.Score

		if _, err := store.SaveAttempt(m.db, *card.ID, score); err != nil {
			return scheduleUpdatedMsg{err: err}
		}

		streaks, err := store.GetStreakLength(m.db, *card.ID)
		if err != nil {
			return scheduleUpdatedMsg{err: err}
		}

		sched := scheduler.GetCardSchedule(score, streaks[*card.ID], card.IntervalDays, card.EaseFactor)
		dueDate := time.Now().Add(time.Duration(sched.IntervalDays) * 24 * time.Hour)

		if err := store.UpdateSchedule(m.db, *card.ID, sched.EaseFactor, sched.IntervalDays, dueDate); err != nil {
			return scheduleUpdatedMsg{err: err}
		}

		return scheduleUpdatedMsg{}
	}
}

// scoreAccentColor returns a color matching the score (0–5).
func scoreAccentColor(score int) color.Color {
	switch {
	case score >= 4:
		return colorGreen
	case score >= 2:
		return colorAmber
	default:
		return colorRed
	}
}
