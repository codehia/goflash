package tui

import (
	"fmt"
	"io"
	"strings"

	list "charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/codehia/goflash/internal/store"
)

// ── topicItem ─────────────────────────────────────────────────────────

type topicItem struct{ TopicSummary }

func (t topicItem) FilterValue() string { return t.Name }

// ── topicDelegate — renders each row as a mini renderCard ─────────────

type topicDelegate struct{ rowW int }

func (d topicDelegate) Height() int                             { return 5 }
func (d topicDelegate) Spacing() int                           { return 0 }
func (d topicDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d topicDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	t, ok := item.(topicItem)
	if !ok {
		return
	}
	selected := index == m.Index()
	bc := colorBorder
	if selected {
		bc = colorFlamingo
	}
	innerW := d.rowW - roundedBorderH
	fmt.Fprint(w, renderCard(d.rowW, bc, topicRowHeader(t.TopicSummary, selected), topicRowBody(t.TopicSummary, innerW), lipgloss.NewStyle()))
}

// ── row helpers ───────────────────────────────────────────────────────

func topicRowHeader(topic TopicSummary, selected bool) lipgloss.Style {
	cursor := faintStyle.Render("  ")
	if selected {
		cursor = purpleStyle.Render("▶ ")
	}
	pill := purplePill(fmt.Sprintf("%d cards", topic.CardCount))
	return styledBox(CardParams{BgColor: colorBase}).SetString(cursor + boldStyle.Render(topic.Name) + "  " + pill)
}

func topicRowBody(topic TopicSummary, innerW int) lipgloss.Style {
	if len(topic.Subtopics) == 0 {
		return styledBox(CardParams{BgColor: colorBase}).SetString(" ")
	}
	var kept []string
	usedW := 0
	for i, n := range topic.Subtopics {
		p := tealPill(n)
		pw := lipgloss.Width(p)
		sep := 0
		if len(kept) > 0 {
			sep = 1
		}
		remaining := len(topic.Subtopics) - i
		more := hintStyle.Render(fmt.Sprintf("+%d more", remaining))
		moreW := 1 + lipgloss.Width(more)
		fits := usedW+sep+pw <= innerW
		fitsWithMore := usedW+sep+pw+moreW <= innerW
		if !fits || (i < len(topic.Subtopics)-1 && !fitsWithMore) {
			kept = append(kept, more)
			break
		}
		kept = append(kept, p)
		usedW += sep + pw
	}
	return styledBox(CardParams{BgColor: colorBase}).SetString(strings.Join(kept, " "))
}

// ── list factory ──────────────────────────────────────────────────────

func newTopicList(topics []TopicSummary, w, h int) list.Model {
	items := make([]list.Item, len(topics))
	for i, t := range topics {
		items[i] = topicItem{t}
	}
	l := list.New(items, topicDelegate{rowW: w}, w, h)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	return l
}

// ── screen functions ──────────────────────────────────────────────────

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
		innerW := cardWidth - roundedBorderH
		m.topicList = newTopicList(msg.topics, innerW, listBodyH)
		return m, nil

	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "enter":
			item, ok := m.topicList.SelectedItem().(topicItem)
			if !ok {
				return m, nil
			}
			return m, func() tea.Msg {
				return TopicSelectedMsg{topicID: *item.ID, topicName: item.Name}
			}
		}
	}

	if len(m.topics) == 0 {
		return m, nil
	}
	var cmd tea.Cmd
	m.topicList, cmd = m.topicList.Update(msg)
	return m, cmd
}

func topicListHeader(m RootModel) lipgloss.Style {
	totalCards := 0
	for _, t := range m.topics {
		totalCards += t.CardCount
	}
	label := hintStyle.Render("Choose a topic to work on")
	subtitle := mutedStyle.Render(fmt.Sprintf("%d cards across %d topics · use ", totalCards, len(m.topics))) +
		purpleStyle.Render("↑↓ to move") + mutedStyle.Render(". enter to select")
	return styledBox(CardParams{BgColor: colorBase}).SetString("\n " + label + "\n " + subtitle)
}

func topicListBody(m RootModel) lipgloss.Style {
	if len(m.topics) == 0 {
		return styledBox(CardParams{BgColor: colorBase}).SetString(mutedStyle.Render("Loading topics..."))
	}
	return styledBox(CardParams{BgColor: colorBase}).SetString(m.topicList.View())
}

func topicListFooter(m RootModel) lipgloss.Style {
	selectedName := ""
	if item, ok := m.topicList.SelectedItem().(topicItem); ok {
		selectedName = strings.ToLower(item.Name)
	}
	return styledBox(CardParams{BgColor: colorBase}).SetString("\n " + actionBar("enter", "start "+selectedName, "q", "quit"))
}
