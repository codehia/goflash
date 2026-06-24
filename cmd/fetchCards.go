package cmd

import (
	"context"
	_ "embed"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"slices"

	"github.com/codehia/goflash/internal/ai"
	"github.com/codehia/goflash/internal/types"
	"github.com/zendev-sh/goai"
	"go.uber.org/zap"
)

var (
	logger = zap.Must(zap.NewDevelopment())
	sugar  = logger.Sugar()
	//go:embed prompts/system.txt
	systemPrompt string
	//go:embed prompts/fakeUser.txt
	fakeUserPrompt string
	//go:embed prompts/assistant.txt
	assistantPrompt string
)

type Leaf struct {
	Name       string
	Notes      string
	ParentPath string
}

type seedArgs struct {
	inputFile  string
	outputFile string
	children   []string
}

type entriesWrapper struct {
	Entries []types.Response `json:"entries"`
}

func (w entriesWrapper) Validate() error {
	for _, entry := range w.Entries {
		if err := entry.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func collectLeaves(node types.Node, path string) []Leaf {
	if len(node.Children) == 0 {
		return []Leaf{{Name: node.Name, Notes: node.Notes, ParentPath: path}}
	}
	var childPath string
	if path == "" {
		childPath = node.Name
	} else {
		childPath = path + " > " + node.Name
	}
	var leaves []Leaf
	for _, child := range node.Children {
		leaves = append(leaves, collectLeaves(child, childPath)...)
	}
	return leaves
}

func createUserMessage(leaf Leaf) string {
	return fmt.Sprintf(`
		Generate flashcards for this concept. Fully decompose it into atomic ideas and generate one card per atomic idea.
		Use the tag and tag_path exactly as given - do not modify them or create new tags.

		Concept: %s
		Tag path: %s
		Anchor: %s`, leaf.Name, leaf.ParentPath, leaf.Notes)
}

func writeToResultJson(path string, results []types.Response) {
	sugar.Infow("writing results to file", "path", path, "count", len(results))
	jsonFileData, err := os.ReadFile(path)

	if err == nil && len(jsonFileData) > 0 {
		var responsesFromJson []types.Response
		err := json.Unmarshal(jsonFileData, &responsesFromJson)
		if err != nil {
			sugar.Errorw("error reading file", "path", path, "error", err)
			return
		}
		results = append(results, responsesFromJson...)

		seen := map[string]bool{}
		var distinct []types.Response
		for _, result := range results {
			key, err := json.Marshal(result)
			if err != nil {
				sugar.Errorw("failed to marshal response for dedup", "tag", result.Tag, "error", err)
				continue
			}
			if !seen[string(key)] {
				seen[string(key)] = true
				distinct = append(distinct, result)
			}
		}
		results = distinct

	}

	marshaledResultData, err := json.MarshalIndent(results, "", " ")
	if err != nil {
		sugar.Errorw("failed to marshal results", "error", err)
		return
	}

	err = os.WriteFile(path, marshaledResultData, 0o644)
	if err != nil {
		sugar.Errorw("failed to write file", "path", path, "error", err)
		return
	}
	sugar.Infow("results written successfully", "path", path)
}

func parseArgs(args []string) seedArgs {
	fs := flag.NewFlagSet("fetchCards", flag.ExitOnError)
	inputFile := fs.String("source", "source.json", "input file name")
	outputFile := fs.String("output", "output.json", "output file name")
	fs.Parse(args) //nolint:errcheck
	return seedArgs{inputFile: *inputFile, outputFile: *outputFile, children: fs.Args()}
}

func FetchCards(args []string) {
	defer logger.Sync() //nolint:errcheck

	providerName := ai.GetProviderName()
	model, err := ai.GetModel()
	if err != nil {
		sugar.Errorw("failed to get the model", "error", err)
	}
	parsedArgs := parseArgs(args)
	// Get the nodes based on the parsedArgs
	root, err := types.LoadNode(parsedArgs.inputFile)
	if err != nil {
		sugar.Errorw("failed to read root node", "error", err)
		os.Exit(1)
	}

	var leaves []Leaf
	for _, child := range root.Children {
		if len(parsedArgs.children) == 0 || slices.Contains(parsedArgs.children, child.Name) {
			leaves = append(leaves, collectLeaves(child, "")...)
		}
	}

	var collected []types.Response
	for _, leaf := range leaves {
		result, err := ai.GenerateStructured[entriesWrapper](
			context.Background(), model, providerName,
			goai.WithSystem(systemPrompt),
			goai.WithMessages(
				goai.UserMessage(fakeUserPrompt),
				goai.AssistantMessage(assistantPrompt),
				goai.UserMessage(createUserMessage(leaf)),
			), goai.WithMaxRetries(3),
		)
		if err != nil {
			sugar.Warnw("request failed", "leaf", leaf.Name, "error", err)
			continue
		}
		collected = append(collected, result.Entries...)
	}
	writeToResultJson(parsedArgs.outputFile, collected)
}
