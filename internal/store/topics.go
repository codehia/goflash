package store

import (
	"database/sql"

	"github.com/codehia/goflash/internal/db/model"
	"github.com/codehia/goflash/internal/db/table"
	"github.com/go-jet/jet/v2/sqlite"
)

/*
Create a method to get all the top level topics
Return all the top level topics
*/

func GetTopLevelTopics(db *sql.DB) ([]model.Tags, error) {
	var tags []model.Tags

	stmt := sqlite.SELECT(table.Tags.Name).FROM(table.Tags).WHERE(table.Tags.ParentID.IS_NULL()).ORDER_BY(table.Tags.Name)
	err := stmt.Query(db, &tags)
	if err != nil {
		return nil, err
	}
	return tags, nil
}
