package tui

import (
	"database/sql"
	"image/color"

	ta "charm.land/bubbles/v2/textarea"
	list "charm.land/bubbles/v2/list"
	"github.com/codehia/goflash/internal/store"
	"github.com/codehia/goflash/internal/types"
)

// ── Screen ───────────────────────────────────────────────────────────

type TopicSummary = types.TopicSummary

type Screen int

const (
	ScreenTopicList Screen = iota
	ScreenCardQuestion
	ScreenCardAttempt
	ScreenEvalResult
	ScreenDone
)

// ── RootModel ────────────────────────────────────────────────────────

type RootModel struct {
	db *sql.DB

	currentScreen Screen
	termWidth     int
	termHeight    int
	ready         bool

	// topic list
	topics          []types.TopicSummary
	topicList       list.Model
	selectedTopicID *string
	topicName       string
	// cards
	cards     []store.Card
	cardIndex int
	// attempt
	textarea   ta.Model
	userAnswer string
	// eval
	evalResult types.EvalResult
	// session
	sessionScores []int
}

// ── Msg types ────────────────────────────────────────────────────────

type topicsLoadedMsg struct {
	topics []types.TopicSummary
	err    error
}

type cardsLoadedMsg struct {
	cards []store.Card
	err   error
}

type TopicSelectedMsg struct {
	topicID   string
	topicName string
}

type CardSelectedMsg struct {
	cardID string
}

type EvalResultMsg struct {
	result types.EvalResult
	err    error
}

type scheduleUpdatedMsg struct {
	err error
}

// ── CardParams ───────────────────────────────────────────────────────

type CardParams struct {
	BorderColor color.Color
	BgColor     color.Color
	Margins     []int
	Padding     []int
}
