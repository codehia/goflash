package cmd

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/codehia/goflash/internal/db/model"
	"github.com/codehia/goflash/internal/db/table"
	"github.com/codehia/goflash/internal/store"
	"github.com/codehia/goflash/internal/types"
	"github.com/codehia/goflash/internal/utils"
	"github.com/go-jet/jet/v2/qrm"
	"github.com/go-jet/jet/v2/sqlite"
)

func readJsonFile(path string) ([]types.Response, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("readJsonFile: read %q: %w", path, err)
	}
	var responses []types.Response
	err = json.Unmarshal(data, &responses)
	if err != nil {
		return nil, fmt.Errorf("readJsonFile: unmarshal: %w", err)
	}
	return responses, nil
}

func getTagRecord(tagName string, db *sql.DB) (model.Tags, error) {
	var foundTag model.Tags
	selectStmt := table.Tags.SELECT(table.Tags.AllColumns).WHERE(table.Tags.Name.EQ(sqlite.String(tagName)))

	err := selectStmt.Query(db, &foundTag)
	if err != nil {
		return foundTag, err
	}
	return foundTag, nil
}

func saveTagsToDB(nodes []types.Node, db *sql.DB, parentID *string) {
	for _, node := range nodes {
		foundTag, err := getTagRecord(node.Name, db)

		var tagID string
		if errors.Is(err, qrm.ErrNoRows) {
			tag := model.Tags{Name: node.Name, ParentID: parentID}
			stmt := table.Tags.INSERT(table.Tags.Name, table.Tags.ParentID).MODEL(tag).RETURNING(table.Tags.AllColumns)
			var inserted model.Tags
			err := stmt.Query(db, &inserted)
			if err != nil {
				log.Fatalf("saveTagsToDB: insert tag %q: %v", node.Name, err)
			}
			tagID = *inserted.ID
		} else if err != nil {
			log.Fatalf("saveTagsToDB: query tag %q: %v", node.Name, err)
		} else {
			tagID = *foundTag.ID
			if !utils.StringPtrEqual(foundTag.ParentID, parentID) {
				_, err := table.Tags.UPDATE(table.Tags.ParentID).SET(parentID).WHERE(table.Tags.ID.EQ(sqlite.String(*foundTag.ID))).Exec(db)
				if err != nil {
					log.Fatalf("saveTagsToDB: update parent for tag %q: %v", node.Name, err)
				}
			}
		}
		saveTagsToDB(node.Children, db, &tagID)
	}
}

func saveCardsToDB(outputFilePath string, db *sql.DB) {
	responseData, err := readJsonFile(outputFilePath)
	if err != nil {
		log.Fatal(err)
	}

	tagCache := map[string]model.Tags{}

	for _, response := range responseData {
		cards := response.Cards
		seen := map[string]bool{}
		var responseTags []string
		for _, tag := range append([]string{response.Tag}, response.TagPath...) {
			if !seen[tag] {
				seen[tag] = true
				responseTags = append(responseTags, tag)
			}
		}

		var responseTagRecords []model.Tags
		for _, tag := range responseTags {
			if cached, ok := tagCache[tag]; ok {
				responseTagRecords = append(responseTagRecords, cached)
				continue
			}
			foundTag, err := getTagRecord(tag, db)
			if err != nil && !errors.Is(err, qrm.ErrNoRows) {
				log.Fatalf("saveCardsToDB: query tag %q: %v", tag, err)
			} else if errors.Is(err, qrm.ErrNoRows) {
				log.Fatalf("saveCardsToDB: tag %q not found in DB, run saveTagsToDB first", tag)
			}
			tagCache[tag] = foundTag
			responseTagRecords = append(responseTagRecords, foundTag)
		}

		for _, card := range cards {
			tx, err := db.Begin()
			if err != nil {
				log.Fatalf("saveCardsToDB: begin tx: %v", err)
			}
			var existing model.Cards
			err = table.Cards.SELECT(table.Cards.AllColumns).
				WHERE(table.Cards.Question.EQ(sqlite.String(card.Question))).
				Query(tx, &existing)

			var cardID string
			if errors.Is(err, qrm.ErrNoRows) {
				now := time.Now().UTC()
				cardRecord := model.Cards{
					Question:  card.Question,
					Answer:    card.Answer,
					Examples:  card.Examples,
					Tradeoffs: card.TradeOffs,
					CardType:  card.CardType,
					DueDate:   &now,
				}
				var insertedCard model.Cards
				if err := table.Cards.INSERT(table.Cards.Question, table.Cards.Answer, table.Cards.Examples,
					table.Cards.Tradeoffs,
					table.Cards.CardType, table.Cards.DueDate).MODEL(cardRecord).RETURNING(table.Cards.AllColumns).
					Query(tx, &insertedCard); err != nil {
					tx.Rollback() //nolint:errcheck
					log.Fatalf("saveCardsToDB: insert card %q: %v", card.Question, err)
				}
				cardID = *insertedCard.ID
			} else if err != nil {
				tx.Rollback() //nolint:errcheck
				log.Fatalf("saveCardsToDB: query card: %v", err)
			} else {
				cardID = *existing.ID
			}

			var missingCardTags []model.CardTags
			for _, tag := range responseTagRecords {
				var existingCardTag model.CardTags
				err := table.CardTags.SELECT(table.CardTags.AllColumns).
					WHERE(table.CardTags.CardID.EQ(sqlite.String(cardID)).
						AND(table.CardTags.TagID.EQ(sqlite.String(*tag.ID)))).
					Query(tx, &existingCardTag)

				if errors.Is(err, qrm.ErrNoRows) {
					missingCardTags = append(missingCardTags, model.CardTags{CardID: cardID, TagID: *tag.ID})
				} else if err != nil {
					tx.Rollback() //nolint:errcheck
					log.Fatalf("saveCardsToDB: query card_tag: %v", err)
				}
			}

			if len(missingCardTags) > 0 {
				if _, err := table.CardTags.INSERT(table.CardTags.CardID, table.CardTags.TagID).
					MODELS(missingCardTags).
					Exec(tx); err != nil {
					tx.Rollback() //nolint:errcheck
					log.Fatalf("saveCardsToDB: batch insert card_tags: %v", err)
				}
			}

			if err := tx.Commit(); err != nil {
				tx.Rollback() //nolint:errcheck
				log.Fatalf("saveCardsToDB: commit tx: %v", err)
			}
		}
	}
}

func StoreCards() {
	db, err := store.Open()
	if err != nil {
		log.Fatalf("failed to open store: %v", err)
	}
	defer db.Close() //nolint:errcheck

	root, err := types.LoadNode("system-design-hierarchy.json")
	if err != nil {
		log.Fatalf("failed to read hierarchy: %v", err)
	}
	saveTagsToDB(root.Children, db, nil)
	saveCardsToDB("output.json", db)
}
