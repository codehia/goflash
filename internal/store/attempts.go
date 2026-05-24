package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/codehia/goflash/internal/db/model"
	"github.com/codehia/goflash/internal/db/table"
	"github.com/go-jet/jet/v2/sqlite"
)

type Attempt struct {
	// Add feedback and user answer here and in the db
	// change the QualityScore to int from int32 in the db
	ID           string
	CardID       string
	QualityScore int
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// tag := model.Tags{Name: node.Name, ParentID: parentID}
// stmt := table.Tags.INSERT(table.Tags.Name, table.Tags.ParentID).MODEL(tag).RETURNING(table.Tags.AllColumns)
// var inserted model.Tags
// err := stmt.Query(db, &inserted)
// if err != nil {
// 	log.Fatalf("saveTagsToDB: insert tag %q: %v", node.Name, err)
// }
// tagID = *inserted.ID

func SaveAttempt(db *sql.DB, cardID string, score int) (*model.CardAttempts, error) {
	attempt := model.CardAttempts{CardID: cardID, QualityScore: int32(score)}
	stmt := table.CardAttempts.INSERT(table.CardAttempts.CardID, table.CardAttempts.QualityScore).MODEL(attempt).RETURNING(table.CardAttempts.AllColumns)
	var inserted model.CardAttempts
	err := stmt.Query(db, &inserted)
	if err != nil {
		return nil, err
	}
	return &inserted, nil
}

// GetStreakLength returns the current correct-answer streak for every card.
// Result: map[cardID] → n (number of consecutive correct attempts from most recent).
func GetStreakLength(db *sql.DB, cardID string) (map[string]int, error) {
	// ordered CTE columns
	ordCardID := sqlite.StringColumn("card_id")
	ordQScore := sqlite.IntegerColumn("quality_score")
	ordRn := sqlite.IntegerColumn("rn")
	ordered := sqlite.CTE("ordered", ordCardID, ordQScore, ordRn)

	// first_fail CTE columns
	ffCardID := sqlite.StringColumn("card_id")
	ffN := sqlite.IntegerColumn("n")
	firstFail := sqlite.CTE("first_fail", ffCardID, ffN)

	stmt := sqlite.WITH(
		ordered.AS(
			sqlite.SELECT(
				table.CardAttempts.CardID,
				table.CardAttempts.QualityScore,
				sqlite.ROW_NUMBER().OVER(
					sqlite.PARTITION_BY(table.CardAttempts.CardID).
						ORDER_BY(table.CardAttempts.CreatedAt.DESC()),
				).AS("rn"),
			).FROM(table.CardAttempts),
		),
		firstFail.AS(
			sqlite.SELECT(
				ordCardID,
				sqlite.MIN(ordRn.SUB(sqlite.Int(1))).AS("n"),
			).FROM(ordered).
				WHERE(ordQScore.LT(sqlite.Int(3))).
				GROUP_BY(ordCardID),
		),
	)(
		sqlite.SELECT(
			ordCardID,
			sqlite.COALESCE(ffN, sqlite.MAX(ordRn)).AS("n"),
		).FROM(
			ordered.LEFT_JOIN(firstFail, ordCardID.EQ(ffCardID)),
		).WHERE(ordCardID.EQ(sqlite.String(cardID))).GROUP_BY(ordCardID),
	)

	var rows []struct {
		CardID string `db:"card_id"`
		N      int    `db:"n"`
	}
	if err := stmt.Query(db, &rows); err != nil {
		return nil, fmt.Errorf("GetConsecutiveN: %w", err)
	}

	result := make(map[string]int, len(rows))
	for _, r := range rows {
		result[r.CardID] = r.N
	}
	return result, nil
}
