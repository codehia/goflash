package tui

import (
	"database/sql"

	ta "charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// Card dimensions — fixed, never resize with terminal.
const (
	cardWidth  = 80
	cardHeight = 40
	listBodyH  = cardHeight - 16 // outer border(2) + header(4) + footer(3) + dividers(2) + buffer(5)
)

func NewRootModel(db *sql.DB) RootModel {
	return RootModel{db: db, currentScreen: ScreenTopicList}
}

func (m RootModel) Init() tea.Cmd {
	return tea.Batch(initTopicList(m), func() tea.Msg { return tea.RequestWindowSize() })
}

func (m RootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.termWidth = msg.Width
		m.termHeight = msg.Height
		m.ready = true
		if len(m.topics) > 0 {
			m.topicList.SetSize(cardWidth-roundedBorderH, listBodyH)
		}
		return m, nil

	case TopicSelectedMsg:
		m.selectedTopicID = &msg.topicID
		m.topicName = msg.topicName
		m.cursor = 0
		m.cardIndex = 0
		m.sessionScores = nil
		m.currentScreen = ScreenCardQuestion
		return m, InitCardList(m)

	case cardsLoadedMsg:
		m.cards = msg.cards
		return m, nil

	case CardSelectedMsg:
		m.textarea = buildTextarea()
		m.currentScreen = ScreenCardAttempt
		return m, nil

	case EvalResultMsg:
		m.evalResult = msg.result
		m.sessionScores = append(m.sessionScores, msg.result.Score)
		m.currentScreen = ScreenEvalResult
		return m, nil
	}

	switch m.currentScreen {
	case ScreenTopicList:
		return updateTopicList(msg, m)
	case ScreenCardQuestion:
		return updateCardQuestion(msg, m)
	case ScreenCardAttempt:
		return updateCardAttempt(msg, m)
	case ScreenEvalResult:
		return updateEvalResult(msg, m)
	case ScreenDone:
		return updateDone(msg, m)
	default:
		return m, nil
	}
}

func (m RootModel) View() tea.View {
	if !m.ready {
		return tea.NewView("")
	}

	if m.termWidth < cardWidth || m.termHeight < cardHeight {
		return tea.NewView(renderTooSmall(m.termWidth, m.termHeight))
	}

	out := centerCard(m.termWidth, m.termHeight,
		renderCard(cardWidth, colorFlamingo,
			screenHeader(m),
			screenBody(m),
			screenFooter(m)))
	return tea.NewView(out)
}

func screenHeader(m RootModel) lipgloss.Style {
	switch m.currentScreen {
	case ScreenTopicList:
		return topicListHeader(m)
	// case ScreenCardQuestion:
	// 	return cardQuestionHeader(m)
	// case ScreenCardAttempt:
	// 	return cardAttemptHeader(m)
	// case ScreenEvalResult:
	// 	return evalResultHeader(m)
	// case ScreenDone:
	// 	return doneHeader(m)
	default:
		return lipgloss.NewStyle()
	}
}

func screenBody(m RootModel) lipgloss.Style {
	switch m.currentScreen {
	case ScreenTopicList:
		return topicListBody(m)
	// case ScreenCardQuestion:
	// 	return cardQuestionBody(m)
	// case ScreenCardAttempt:
	// 	return cardAttemptBody(m)
	// case ScreenEvalResult:
	// 	return evalResultBody(m)
	// case ScreenDone:
	// 	return doneBody(m)
	default:
		return lipgloss.NewStyle()
	}
}

func screenFooter(m RootModel) lipgloss.Style {
	switch m.currentScreen {
	case ScreenTopicList:
		return topicListFooter(m)
	// case ScreenCardQuestion:
	// 	return cardQuestionFooter(m)
	// case ScreenCardAttempt:
	// 	return cardAttemptFooter(m)
	// case ScreenEvalResult:
	// 	return evalResultFooter(m)
	// case ScreenDone:
	// 	return doneFooter(m)
	default:
		return lipgloss.NewStyle()
	}
}

func buildTextarea() ta.Model {
	t := ta.New()
	t.Placeholder = "Answer to the selected card ...."
	t.DynamicHeight = true
	t.MinHeight = 4
	t.MaxHeight = 15
	t.SetWidth(contentWidth - 6)
	t.SetVirtualCursor(false)
	t.ShowLineNumbers = false
	t.Prompt = ""

	t.Focus()
	return t
}
