package store

import (
	"database/sql"

	"github.com/codehia/goflash/internal/db/model"
	"github.com/codehia/goflash/internal/db/table"
	"github.com/go-jet/jet/v2/sqlite"
)

type Card struct {
	// Add feedback and user answer here and in the db
	// change the QualityScore to int from int32 in the db
	ID       *string
	Question string
	Answer   string
}

func GetCardsForTopic(db *sql.DB, tagId *string) ([]Card, error) {
	var modelCards []model.Cards
	stmt := sqlite.SELECT(
		table.Cards.ID,
		table.Cards.Question,
		table.Cards.Answer,
	).FROM(
		table.Cards.INNER_JOIN(table.CardTags, table.Cards.ID.EQ(table.CardTags.CardID)),
	).WHERE(table.CardTags.TagID.EQ(sqlite.String(*tagId)))

	err := stmt.Query(db, &modelCards)
	if err != nil {
		return nil, err
	}

	cards := make([]Card, len(modelCards))
	for i, mc := range modelCards {
		cards[i] = Card{ID: mc.ID, Question: mc.Question, Answer: mc.Answer}
	}
	return cards, nil
}
