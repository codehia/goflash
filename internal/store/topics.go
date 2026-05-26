package store

import (
	"database/sql"
	"fmt"

	"github.com/codehia/goflash/internal/db/model"
	"github.com/codehia/goflash/internal/db/table"
	"github.com/codehia/goflash/internal/types"
	"github.com/go-jet/jet/v2/sqlite"
)

func GetTopLevelTopics(db *sql.DB) ([]model.Tags, error) {
	var tags []model.Tags

	stmt := sqlite.SELECT(table.Tags.ID, table.Tags.Name).FROM(table.Tags).WHERE(table.Tags.ParentID.IS_NULL()).ORDER_BY(table.Tags.Name)
	err := stmt.Query(db, &tags)
	if err != nil {
		return nil, err
	}
	return tags, nil
}

// GetTopicSummaries returns all top-level topics with card counts and subtopic
// names. Uses three queries then assembles in Go:
//  1. Top-level tags (jet) — parent_id IS NULL
//  2. Card counts per top-level topic (jet WITH_RECURSIVE) — recursive CTE walks the tag
//     tree so cards tagged at any depth are counted toward the root
//  3. Direct subtopics (jet) — tags with parent_id matching a top-level tag
func GetTopicSummaries(db *sql.DB) ([]types.TopicSummary, error) {
	// 1. Top-level tags
	topLevel, err := GetTopLevelTopics(db)
	if err != nil {
		return nil, fmt.Errorf("GetTopicSummaries: top-level tags: %w", err)
	}

	// 2. Card counts per root topic via recursive CTE.
	// The CTE walks from each root tag down through all descendants,
	// then counts distinct cards tagged at any level in the tree.
	cardCounts, err := getCardCountsByRoot(db)
	if err != nil {
		return nil, fmt.Errorf("GetTopicSummaries: card counts: %w", err)
	}

	// 3. All non-root tags (subtopics), grouped by parent in Go.
	subtopicMap, err := getSubtopicsByParent(db)
	if err != nil {
		return nil, fmt.Errorf("GetTopicSummaries: subtopics: %w", err)
	}

	// Assemble results
	summaries := make([]types.TopicSummary, len(topLevel))
	for i, t := range topLevel {
		id := ""
		if t.ID != nil {
			id = *t.ID
		}
		summaries[i] = types.TopicSummary{
			ID:        t.ID,
			Name:      t.Name,
			CardCount: cardCounts[id],
			Subtopics: subtopicMap[id],
		}
	}
	return summaries, nil
}

// getCardCountsByRoot returns a map of root_tag_id → card count.
// Uses a recursive CTE to walk from each root tag through all descendants,
// then counts distinct cards tagged anywhere in the subtree.
func getCardCountsByRoot(db *sql.DB) (map[string]int, error) {
	topicTree := sqlite.CTE("topic_tree")
	rootID := sqlite.StringColumn("root_id")
	tagID := sqlite.StringColumn("tag_id")

	stmt := sqlite.WITH_RECURSIVE(
		topicTree.AS(
			sqlite.SELECT(
				table.Tags.ID.AS("root_id"),
				table.Tags.ID.AS("tag_id"),
			).FROM(table.Tags).
				WHERE(table.Tags.ParentID.IS_NULL()).
				UNION_ALL(
					sqlite.SELECT(
						rootID.From(topicTree),
						table.Tags.ID,
					).FROM(
						table.Tags.INNER_JOIN(
							topicTree,
							table.Tags.ParentID.EQ(tagID.From(topicTree)),
						),
					),
				),
		),
	)(
		sqlite.SELECT(
			rootID.From(topicTree),
			sqlite.COUNT(sqlite.DISTINCT(table.CardTags.CardID)).AS("count"),
		).FROM(
			topicTree.INNER_JOIN(
				table.CardTags,
				table.CardTags.TagID.EQ(tagID.From(topicTree)),
			),
		).GROUP_BY(rootID.From(topicTree)),
	)

	var dest []struct {
		RootID string
		Count  int
	}
	if err := stmt.Query(db, &dest); err != nil {
		return nil, err
	}
	counts := make(map[string]int)
	for _, r := range dest {
		counts[r.RootID] = r.Count
	}
	return counts, nil
}

// getSubtopicsByParent returns a map of root_tag_id → []all_descendant_names.
func getSubtopicsByParent(db *sql.DB) (map[string][]string, error) {
	topicTree := sqlite.CTE("topic_tree")
	rootID := sqlite.StringColumn("root_id")
	tagID := sqlite.StringColumn("tag_id")
	tagName := sqlite.StringColumn("name")

	stmt := sqlite.WITH_RECURSIVE(
		topicTree.AS(
			sqlite.SELECT(
				table.Tags.ID.AS("root_id"),
				table.Tags.ID.AS("tag_id"),
				table.Tags.Name.AS("name"),
			).FROM(table.Tags).
				WHERE(table.Tags.ParentID.IS_NULL()).
				UNION_ALL(
					sqlite.SELECT(
						rootID.From(topicTree),
						table.Tags.ID,
						table.Tags.Name,
					).FROM(
						table.Tags.INNER_JOIN(
							topicTree,
							table.Tags.ParentID.EQ(tagID.From(topicTree)),
						),
					),
				),
		),
	)(
		sqlite.SELECT(
			rootID.From(topicTree),
			tagName.From(topicTree),
		).FROM(topicTree).
			WHERE(tagID.From(topicTree).NOT_EQ(rootID.From(topicTree))).
			ORDER_BY(tagName.From(topicTree)),
	)

	var dest []struct {
		RootID string
		Name   string
	}
	if err := stmt.Query(db, &dest); err != nil {
		return nil, err
	}

	m := make(map[string][]string)
	for _, r := range dest {
		m[r.RootID] = append(m[r.RootID], r.Name)
	}
	return m, nil
}
