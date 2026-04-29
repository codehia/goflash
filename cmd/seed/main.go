package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"slices"
	"strings"
	"sync"

	"go.uber.org/zap"
)

var (
	logger, _    = zap.NewDevelopment()
	sugar        = logger.Sugar()
	systemPrompt string
)

const (
	Model           = "deepseek-v4-flash"
	ReasoningEffort = "medium"
	Stream          = false
	fakeUserPrompt  = `
		Generate flashcards for this concept. Fully decompose it into atomic ideas and generate one card per atomic idea.
		Use the tag and tag_path exactly as given - do not modify them or create new tags.

		Concept: TTL
		Tag path: ["Caching", "Cache Eviction Policies"]
		Anchor: TTL (time-to-live) eviction associates each cache entry with an expiration timestamp. 
			Once the timestamp passes, the entry is treated as invalid: either purged eagerly by a background sweeper or lazily on the next read.
			TTL is independent of access patterns, unlike LRU or LFU. Redis EXPIRE and SET with EX option. Memcached entries take an explicit TTL.
			CDN cache headers (Cache-Control: max-age) implement TTL at the HTTP layer.
	`
	assistantPrompt = `
	{
	   "entries":[
	    {"tag":"TTL","tag_path":["Caching","Cache Eviction Policies"],
	   "cards":[ 
		{"question":"What is TTL-based cache eviction and how does it work?",
		 "answer":"TTL eviction associates each cache entry with an expiration timestamp. Once the timestamp passes the entry is invalid, purged either eagerly by a background sweeper or lazily on the next read. TTL is independent of access patterns unlike LRU or LFU, meaning a hot entry will still be evicted once it expires.",
		 "examples":"Redis EXPIRE and SET with EX option. Memcached entries take an explicit TTL. CDN cache headers (Cache-Control: max-age) implement TTL at the HTTP layer.",
		 "tradeoffs":"TTL is simple and predictable but evicts entries even when still hot, and keeps cold entries until they expire. Combine with LRU when both freshness and access patterns matter.",
		 "card_type":"definition"}, 
		{"question":"How does Redis implement TTL expiry internally?",
		 "answer":"Redis uses two strategies combined. Lazy expiry: when a key is accessed Redis checks if it has expired and deletes it before returning. Active expiry: a periodic background job samples random keys with TTLs and deletes expired ones. This hybrid avoids scanning all keys while still reclaiming memory from unaccessed expired entries.","examples":"Redis commands: EXPIRE key seconds, PEXPIRE key milliseconds, TTL key to inspect remaining time.",
		 "tradeoffs":"The sampling approach means expired but unaccessed keys can linger and consume memory. Under heavy expiry load the background job can cause latency spikes.",
		 "card_type":"mechanism"}, 
		{"question":"When would you choose TTL eviction over LRU?",
		 "answer":"Use TTL when correctness depends on freshness: session tokens, rate limit counters, DNS records, or pricing data that must not be served stale. Also when downstream contracts dictate expiry via HTTP Cache-Control headers or CDN edge caches. Use LRU when the goal is keeping the working set in memory regardless of age and stale data is acceptable.",
		 "examples":"Session stores in Redis use TTL matching the session lifetime. Rate limiters use short TTL windows (1s, 1m). Application data caches typically prefer LRU.",
		 "tradeoffs":"","card_type":"tradeoff"}]}
	   ]
	}
	`
)

type Node struct {
	Name     string `json:"name"`
	Notes    string `json:"notes,omitempty"`
	Children []Node `json:"children,omitempty"`
}

type LeafNode struct {
	Node
	Path string
}

type requestPayload struct {
	Model          string            `json:"model"`
	Temperature    float64           `json:"temperature"`
	MaxTokens      int               `json:"max_tokens"`
	ResponseFormat map[string]string `json:"response_format"`
	Messages       []message         `json:"messages"`
	Thinking       map[string]string `json:"thinking"`
	Stream         bool              `json:"stream"`
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Card struct {
	Question  string `json:"question"`
	Answer    string `json:"answer"`
	Examples  string `json:"examples"`
	TradeOffs string `json:"tradeoffs"`
	CardType  string `json:"card_type"`
}

func (c Card) validate() (bool, error) {
	validCardTypes := map[string]bool{
		"definition":  true,
		"mechanism":   true,
		"tradeoff":    true,
		"application": true,
	}
	if c.Question == "" {
		sugar.Warnw("card validation failed", "reason", "missing question")
		return false, errors.New("missing question")
	} else if c.Answer == "" {
		sugar.Warnw("card validation failed", "reason", "missing answer")
		return false, errors.New("missing answer")
	} else if !validCardTypes[c.CardType] {
		sugar.Warnw("card validation failed", "reason", "invalid card type", "card_type", c.CardType)
		return false, errors.New("invalid card type")
	}
	return true, nil
}

type Response struct {
	Tag     string   `json:"tag"`
	TagPath []string `json:"tag_path"`
	Cards   []Card   `json:"cards"`
}

type entriesWrapper struct {
	Entries []Response `json:"entries"`
}

func (r Response) validate() (bool, error) {
	if r.Tag == "" {
		return false, errors.New("tag field is empty")
	} else if len(r.TagPath) == 0 {
		return false, errors.New("tag_path field is empty")
	} else if len(r.Cards) == 0 {
		return false, errors.New("cards field is empty")
	}
	for _, card := range r.Cards {
		_, err := card.validate()
		if err != nil {
			return false, err
		}
	}
	return true, nil
}

type Choice struct {
	Index   int     `json:"index"`
	Message message `json:"message"`
}

type APIResponse struct {
	Choices []Choice `json:"choices"`
}

func newMessage(content string, role string) (message, error) {
	if content == "" {
		return message{}, errors.New("empty strings are not allowed")
	}
	if role == "" {
		role = "user"
	}
	return message{Role: role, Content: content}, nil
}

func newRequestPayload(messages []message) (requestPayload, error) {
	if len(messages) == 0 {
		return requestPayload{}, errors.New("empty list of messages is not allowed")
	}

	thinking := map[string]string{"type": "disabled"}

	responseFormat := map[string]string{"type": "json_object"}
	return requestPayload{
		Model:          Model,
		Temperature:    0.0,
		MaxTokens:      32000,
		ResponseFormat: responseFormat,
		Messages:       messages,
		Thinking:       thinking,
		Stream:         Stream,
	}, nil
}

func getRootNode(path string) (Node, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Node{}, errors.New("error reading file")
	}

	var root Node
	if err := json.Unmarshal(data, &root); err != nil {
		return Node{}, errors.New("parsing json failed")
	}

	return root, nil
}

func getLeafNodes(node Node, path string) []LeafNode {
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

func findChildrenNodes(node Node, childrenNames []string) []Node {
	var childrenNodes []Node
	for _, child := range node.Children {
		if slices.Contains(childrenNames, child.Name) {
			childrenNodes = append(childrenNodes, child)
		}
	}
	return childrenNodes
}

func formatNode(node Node, depth int) string {
	indent := strings.Repeat("  ", depth)
	var sb strings.Builder

	if depth == 0 {
		fmt.Fprintf(&sb, "%s\n", node.Name)
	} else {
		fmt.Fprintf(&sb, "%s└── %s\n", indent, node.Name)
	}

	if node.Notes != "" {
		fmt.Fprintf(&sb, "%s  Anchor: %s\n\n", indent, node.Notes)
	}

	for _, child := range node.Children {
		sb.WriteString(formatNode(child, depth+1))
	}

	return sb.String()
}

func formatSubtrees(node Node) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Root: %s\nSubtree:\n\n", node.Name)
	sb.WriteString(formatNode(node, 0))
	return sb.String()
}

func createUserMessage(leaf LeafNode) (message, error) {
	content := fmt.Sprintf(`
		Generate flashcards for this concept. Fully decompose it into atomic ideas and generate one card per atomic idea. 
		Use the tag and tag_path exactly as given - do not modify them or create new tags.
		
		Concept: %s
		Tag path: %s
		Anchor: %s`, leaf.Name, leaf.Path, leaf.Notes)
	return newMessage(content, "user")
}

func createFakeUserMessage() (message, error) {
	return newMessage(fakeUserPrompt, "user")
}

func createAssistantMessage() (message, error) {
	return newMessage(assistantPrompt, "assistant")
}

func createSystemMessage() (message, error) {
	return newMessage(systemPrompt, "system")
}

func createPayload(node LeafNode) (requestPayload, error) {
	systemMessage, err := createSystemMessage()
	if err != nil {
		return requestPayload{}, err
	}
	userMessage, err := createUserMessage(node)
	if err != nil {
		return requestPayload{}, err
	}
	fakeUserMessage, err := createFakeUserMessage()
	if err != nil {
		return requestPayload{}, err
	}
	assistantMessage, err := createAssistantMessage()
	if err != nil {
		return requestPayload{}, err
	}

	messages := []message{systemMessage, fakeUserMessage, assistantMessage, userMessage}
	payload, err := newRequestPayload(messages)
	if err != nil {
		return requestPayload{}, err
	}

	return payload, nil
}

func makeRequest(payloadData requestPayload, results chan<- []Response, retry chan<- requestPayload, wg *sync.WaitGroup) {
	defer wg.Done()

	apiKey := os.Getenv("DEEPSEEK_API_KEY")
	baseUrl := os.Getenv("DEEPSEEK_BASE_URL")

	if apiKey == "" {
		sugar.Errorw("DEEPSEEK_API_KEY is not set")
		return
	}
	if baseUrl == "" {
		sugar.Errorw("DEEPSEEK_BASE_URL is not set")
		return
	}

	payload, err := json.Marshal(payloadData)
	if err != nil {
		sugar.Errorw("marshalling payload failed", "error", err)
		return
	}

	sugar.Infow("sending request", "model", payloadData.Model)
	req, _ := http.NewRequest("POST", baseUrl, bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		sugar.Warnw("http request failed, retrying", "error", err)
		retry <- payloadData
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		sugar.Warnw("reading response body failed, retrying", "error", err)
		retry <- payloadData
		return
	}

	responses, err := validateResponse(body)
	if err != nil {
		var pretty bytes.Buffer
		json.Indent(&pretty, body, "", " ")
		os.WriteFile("raw_response.json", pretty.Bytes(), 0o644)
		sugar.Warnw("validation failed, retrying", "error", err)
		retry <- payloadData
		return
	}

	sugar.Infow("request successful", "responses", len(responses))
	results <- responses
}

func validateResponse(response []byte) ([]Response, error) {
	apiResponse := APIResponse{}
	if err := json.Unmarshal(response, &apiResponse); err != nil {
		return nil, err
	}
	var responses []Response

	for _, choice := range apiResponse.Choices {
		var wrapper entriesWrapper
		if err := json.Unmarshal([]byte(choice.Message.Content), &wrapper); err != nil {
			return nil, err
		}
		for _, response := range wrapper.Entries {
			if _, err := response.validate(); err != nil {
				return nil, err
			}
		}
		responses = append(responses, wrapper.Entries...)

	}
	return responses, nil
}

func main() {
	defer logger.Sync()
	data, err := os.ReadFile("systemPrompt.txt")
	if err != nil {
		sugar.Errorw("failed to load the systemPrompt file", err)
	}
	systemPrompt = string(data)

	children := os.Args[1:]
	if len(children) == 0 {
		sugar.Errorw("no children specified")
		os.Exit(1)
	}

	root, error := getRootNode("system-design-hierarchy.json")
	if error != nil {
		sugar.Errorw("failed to read root node", "error", error)
		os.Exit(1)
	}

	childrenNodes := findChildrenNodes(root, children)
	var leafNodes []LeafNode
	for _, node := range childrenNodes {
		leafNodes = append(leafNodes, getLeafNodes(node, "")...)
	}
	sugar.Infow("found leaf nodes", "count", len(leafNodes))

	results := make(chan []Response)
	retry := make(chan requestPayload)

	var wg sync.WaitGroup
	wg.Add(len(leafNodes))
	for _, node := range leafNodes {
		payload, err := createPayload(node)
		if err != nil {
			sugar.Errorw("failed to create payload", "node", node.Name, "error", err)
			continue
		}
		sugar.Infow("launching request", "node", node.Name)
		go makeRequest(payload, results, retry, &wg)
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
			// wg.Add(1)
			// go makeRequest(r, results, retry, &wg)
		}
	}()

	var collected []Response
	for r := range results {
		collected = append(collected, r...)
	}
	sugar.Infow("collection complete", "total_responses", len(collected))
	writeToResultJson("output.json", collected)
}

func writeToResultJson(path string, results []Response) {
	sugar.Infow("writing results to file", "path", path, "count", len(results))
	data, err := os.ReadFile(path)

	if err == nil && len(data) > 0 {
		var outputData []Response
		error := json.Unmarshal(data, &outputData)
		if error != nil {
			sugar.Errorw("error reading file", "path", path, "error", error)
			return
		}
		results = append(results, outputData...)

		seen := map[string]bool{}
		var distinct []Response
		for _, r := range results {
			key, _ := json.Marshal(r)
			if !seen[string(key)] {
				seen[string(key)] = true
				distinct = append(distinct, r)
			}
		}
		results = distinct

	}

	marshaledResultData, marshalError := json.MarshalIndent(results, "", " ")
	if marshalError != nil {
		sugar.Errorw("failed to marshal results", "error", marshalError)
		return
	}

	err = os.WriteFile(path, marshaledResultData, 0o644)
	if err != nil {
		sugar.Errorw("failed to write file", "path", path, "error", err)
		return
	}
	sugar.Infow("results written successfully", "path", path)
}
