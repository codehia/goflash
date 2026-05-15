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
	ID        *string
	Question  string
	Answer    string
	Subtopics []string
}

func GetCardsForTopic(db *sql.DB, tagId *string) ([]Card, error) {
	tagTree := sqlite.CTE("tag_tree")
	tagID := sqlite.StringColumn("tag_id")

	stmt := sqlite.WITH_RECURSIVE(
		tagTree.AS(
			sqlite.SELECT(table.Tags.ID.AS("tag_id")).
				FROM(table.Tags).
				WHERE(table.Tags.ID.EQ(sqlite.String(*tagId))).
				UNION_ALL(
					sqlite.SELECT(table.Tags.ID).
						FROM(table.Tags.INNER_JOIN(tagTree, table.Tags.ParentID.EQ(tagID.From(tagTree)))),
				),
		),
	)(
		sqlite.SELECT(
			table.Cards.ID,
			table.Cards.Question,
			table.Cards.Answer,
		).DISTINCT().FROM(
			table.Cards.
				INNER_JOIN(table.CardTags, table.Cards.ID.EQ(table.CardTags.CardID)).
				INNER_JOIN(tagTree, table.CardTags.TagID.EQ(tagID.From(tagTree))),
		),
	)

	var modelCards []model.Cards
	if err := stmt.Query(db, &modelCards); err != nil {
		return nil, err
	}

	cards := make([]Card, len(modelCards))
	for i, mc := range modelCards {
		cards[i] = Card{ID: mc.ID, Question: mc.Question, Answer: mc.Answer}
	}

	if err := attachSubtopics(db, cards); err != nil {
		return nil, err
	}
	return cards, nil
}

// attachSubtopics fetches non-root tags for each card and sets Subtopics.
func attachSubtopics(db *sql.DB, cards []Card) error {
	if len(cards) == 0 {
		return nil
	}

	ids := make([]sqlite.Expression, len(cards))
	for i, c := range cards {
		ids[i] = sqlite.String(*c.ID)
	}

	var rows []struct {
		model.CardTags
		model.Tags
	}
	stmt := sqlite.SELECT(
		table.CardTags.CardID,
		table.Tags.Name,
	).FROM(
		table.CardTags.INNER_JOIN(table.Tags, table.Tags.ID.EQ(table.CardTags.TagID)),
	).WHERE(
		table.CardTags.CardID.IN(ids...).AND(table.Tags.ParentID.IS_NOT_NULL()),
	).ORDER_BY(table.Tags.Name)

	if err := stmt.Query(db, &rows); err != nil {
		return err
	}

	byID := make(map[string][]string)
	for _, r := range rows {
		byID[r.CardTags.CardID] = append(byID[r.CardTags.CardID], r.Tags.Name)
	}
	for i, c := range cards {
		cards[i].Subtopics = byID[*c.ID]
	}
	return nil
}
