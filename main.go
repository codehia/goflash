package main

import (
	"fmt"
	"log"

	tea "charm.land/bubbletea/v2"
	"github.com/codehia/goflash/internal/scheduler"
	"github.com/codehia/goflash/internal/store"
	"github.com/codehia/goflash/internal/tui"
	"github.com/codehia/goflash/internal/types"
	// "github.com/codehia/goflash/internal/types"
)

func main() {
	/*
		steps:
		1. Show a list of all the topics (ordered by something?) and ask the user to select 1 top level topic
		2. Get all the cards with due_date today for the topic -> Grouped by SubTopics (recursively?) -> And order by something
			(We might need to add a order field in the tag to know the order in the same nesting or can maintain a new table with these details, could also fallback to the json hierarchy and add order numbers there)
		3. Show the cards for -> Selected topic -> Grouped by subtopics (nested) and all the cards in them.
		4. User is shown the question -> Accept the answer
		5. Send the answer for eval to deepseek
		6. Validate the response from deepseek and send request again if the format/ values doesn't match
		7. Save the response to card_attempts table
		8. Call the sm2 for the latest data for cards and attempts etc
		9. Update the card with the due_date, intervalDays etc.
		10. Show the next card
		Extra :-> can we do the card score stuff after validation in background and unblock the user for the next card? Maybe accept answers from the user in a batch (assuming that's a valid flash card learning way)


		MVP: ->
		1. Show Topic list ordered by Topic name (only top level)
		2. Get a flat list of cards for the selected topics (ordered by due_date)
		3. Show Card -> Accept Answer -> AI eval -> show feedback -> sm2 update -> next card
	*/

	db, err := store.Open()
	if err != nil {
		log.Fatalf("failed to open store: %v", err)
	}
	defer db.Close() //nolint:errcheck

	// get top level topics list ordered by topic name
	topLevelTags, err := store.GetTopLevelTopics(db)
	if err != nil {
		log.Fatalf("failed to get top level topics: %v", err)
	}
	var topicsListToShow []types.Topic
	for _, tag := range topLevelTags {
		topicsListToShow = append(topicsListToShow, types.Topic{Name: tag.Name})
	}
	p := tea.NewProgram(tui.InitialModel(topicsListToShow))
	result, err := p.Run()
	if err != nil {
		log.Fatalf("failed to run the tea new program: %v", err)
	}
	finalModel := result.(tui.Model)
	fmt.Println(finalModel.Choices[finalModel.Cursor].Name)

	// receive score
	// result := types.EvalResult{
	// 	Score:    3,
	// 	Feedback: "You correctly identified that B-tree split nodes when full...",
	// }
	cardID := "0042b3d69a3c3a6ea9326176f40eba43"
	// Maybe send the score to attempts, to be verified and saved -> re request based on the err
	// get the latest n
	cardAttempt, err := store.SaveAttempt(db, cardID, 3)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	fmt.Println(cardAttempt)
	n, err := store.GetStreakLength(db, cardID)
	if err != nil {
		log.Fatalf("ERROR: %v", err)
	}
	fmt.Println(n)

	// Call the sm-2 file with the card (assume it was there) and n stuff (extract the fields)
	// call the cards.go to update the latest values for fields for the card with the id

	// 3 -> QualityScore -> Comes from attempt (after SaveAttempt)
	// intervalDays will come from card
	// easeFactor will come from card
	cardSchedule := scheduler.GetCardSchedule(3, n[cardID], 6, 1.3)
	fmt.Println(cardSchedule.EaseFactor)
	fmt.Println(cardSchedule.IntervalDays)
}
