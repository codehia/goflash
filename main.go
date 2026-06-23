package main

import (
	"flag"
	"log"

	tea "charm.land/bubbletea/v2"
	cmd "github.com/codehia/goflash/cmd"
	"github.com/codehia/goflash/internal/store"
	"github.com/codehia/goflash/internal/tui"
)

func runCommand(commandString *string) {
	switch *commandString {
	case "seed":
		cmd.FetchCards()
	case "import":
		cmd.StoreCards()
	default:
		log.Fatalf("invalid command %s. Only allowed seed and import", *commandString)
	}
}

func runTUI() {
	db, err := store.Open()
	if err != nil {
		log.Fatalf("failed to open store: %v", err)
	}
	defer db.Close() //nolint:errcheck

	tuiProgram := tea.NewProgram(tui.NewRootModel(db))
	_, err = tuiProgram.Run()
	if err != nil {
		log.Fatalf("failed to run the tea new program: %v", err)
	}
}

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

	commandString := flag.String("cmd", "", "command to run (seed, import)")
	flag.Parse()

	if *commandString != "" {
		runCommand(commandString)
	} else {
		runTUI()
	}
}
