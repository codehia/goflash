package cmd

import (
	_ "embed"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"slices"
	"sync"

	"github.com/codehia/goflash/internal/ai"
	"github.com/codehia/goflash/internal/types"
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

type LeafNode struct {
	types.Node
	Path string
}

type entriesWrapper struct {
	Entries []types.Response `json:"entries"`
}

func getLeafNodes(node types.Node, path string) []LeafNode {
	if len(node.Children) == 0 {
		return []LeafNode{{Node: node, Path: path}}
	}
	var childPath string
	if path == "" {
		childPath = node.Name
	} else {
		childPath = path + " > " + node.Name
	}
	var leaves []LeafNode
	for _, child := range node.Children {
		leaves = append(leaves, getLeafNodes(child, childPath)...)
	}
	return leaves
}

func findChildrenNodes(node types.Node, childrenNames []string) []types.Node {
	var childrenNodes []types.Node
	for _, child := range node.Children {
		if slices.Contains(childrenNames, child.Name) {
			childrenNodes = append(childrenNodes, child)
		}
	}
	return childrenNodes
}

func createUserMessage(leaf LeafNode) (types.APIMessage, error) {
	content := fmt.Sprintf(`
		Generate flashcards for this concept. Fully decompose it into atomic ideas and generate one card per atomic idea.
		Use the tag and tag_path exactly as given - do not modify them or create new tags.

		Concept: %s
		Tag path: %s
		Anchor: %s`, leaf.Name, leaf.Path, leaf.Notes)
	return types.NewMessage(content, "user")
}

func createFakeUserMessage() (types.APIMessage, error) {
	return types.NewMessage(fakeUserPrompt, "user")
}

func createAssistantMessage() (types.APIMessage, error) {
	return types.NewMessage(assistantPrompt, "assistant")
}

func createSystemMessage() (types.APIMessage, error) {
	return types.NewMessage(systemPrompt, "system")
}

func createPayload(node LeafNode) (types.RequestPayload, error) {
	systemMessage, err := createSystemMessage()
	if err != nil {
		return types.RequestPayload{}, err
	}
	userMessage, err := createUserMessage(node)
	if err != nil {
		return types.RequestPayload{}, err
	}
	fakeUserMessage, err := createFakeUserMessage()
	if err != nil {
		return types.RequestPayload{}, err
	}
	assistantMessage, err := createAssistantMessage()
	if err != nil {
		return types.RequestPayload{}, err
	}

	messages := []types.APIMessage{systemMessage, fakeUserMessage, assistantMessage, userMessage}
	return types.NewRequestPayload(messages)
}

func makeRequest(payloadData types.RequestPayload, cfg types.Config, results chan<- []types.Response, retry chan<- types.RequestPayload, wg *sync.WaitGroup) {
	sugar.Infow("sending request", "model", payloadData.Model)

	content, err := ai.MakeRequest(payloadData, cfg)
	if err != nil {
		sugar.Warnw("request failed, retrying", "error", err)
		wg.Add(1)
		retry <- payloadData
		wg.Done()
		return
	}

	responses, err := validateResponse(content)
	if err != nil {
		sugar.Warnw("validation failed, retrying", "error", err)
		wg.Add(1)
		retry <- payloadData
		wg.Done()
		return
	}

	sugar.Infow("request successful", "responses", len(responses))
	results <- responses
	wg.Done()
}

func validateResponse(content []byte) ([]types.Response, error) {
	var wrapper entriesWrapper
	if err := json.Unmarshal(content, &wrapper); err != nil {
		return nil, err
	}
	for _, entry := range wrapper.Entries {
		if err := entry.Validate(); err != nil {
			return nil, err
		}
	}
	return wrapper.Entries, nil
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

func FetchCards() {
	defer logger.Sync() //nolint:errcheck
	// CREATE CONFIG -> CAN DELETE
	cfg, err := types.NewConfig()
	if err != nil {
		sugar.Errorw("config creation failed", "error", err)
		os.Exit(1)
	}

	// REMOVE THIS CHILDREN LOGIC ENTIRELY.
	children := flag.Args()

	root, err := types.LoadNode("system-design-hierarchy.json")
	if err != nil {
		sugar.Errorw("failed to read root node", "error", err)
		os.Exit(1)
	}

	var childrenNodes []types.Node
	if len(children) == 0 {
		sugar.Infow("no topics specified, seeding all top-level topics", "count", len(root.Children))
		childrenNodes = root.Children
	} else {
		childrenNodes = findChildrenNodes(root, children)
	}
	var leafNodes []LeafNode
	for _, node := range childrenNodes {
		leafNodes = append(leafNodes, getLeafNodes(node, "")...)
	}
	if len(leafNodes) == 0 {
		sugar.Errorw("no matching nodes found", "args", children)
		os.Exit(1)
	}
	sugar.Infow("found leaf nodes", "count", len(leafNodes))

	results := make(chan []types.Response)
	retry := make(chan types.RequestPayload)

	var wg sync.WaitGroup
	for _, node := range leafNodes {
		payload, err := createPayload(node)
		if err != nil {
			sugar.Errorw("failed to create payload", "node", node.Name, "error", err)
			continue
		}
		sugar.Infow("launching request", "node", node.Name)
		wg.Add(1) // only count goroutines that actually launch
		go makeRequest(payload, cfg, results, retry, &wg)
	}

	go func() {
		wg.Wait()
		sugar.Infow("all requests done, closing channels")
		close(results)
		close(retry)
	}()

	go func() {
		for r := range retry {
			sugar.Infow("retrying request", "model", r.Model)
			// TODO: no retry limit — a permanently failing request retries forever.
			// Fix: track attempt count per payload and drop after N retries.
			go makeRequest(r, cfg, results, retry, &wg)
		}
	}()

	var collected []types.Response
	for r := range results {
		collected = append(collected, r...)
	}
	sugar.Infow("collection complete", "total_responses", len(collected))
	writeToResultJson("output.json", collected)
}
