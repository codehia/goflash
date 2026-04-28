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
	// "github.com/codehia/goflash/internal/store"
)

const (
	Model           = "deepseek-v4-pro"
	ReasoningEffort = "medium"
	Stream          = false
	SystemPrompt    = `
		You are a senior backend engineer and educator generating flashcards for
		system design interview preparation. Your audience is experienced software
		engineers preparing for senior-level interviews.

		OUTPUT RULES:
			- Respond ONLY with valid JSON. No preamble, no markdown fences, no commentary.
			- Match the response shape exactly. Do not add or rename fields.

		CARD QUALITY RULES:
			- Each card covers exactly one atomic concept. If a topic has multiple ideas, produce multiple cards.
			- Questions are specific and unambiguous. Prefer "Why X over Y in scenario Z" over "What is X".
			- Answers are accurate, current, and complete enough to stand alone.
			- Examples reference real systems with concrete details (Postgres, Cassandra, Kafka, DynamoDB, etc.). No vague "many databases".
			- Tradeoffs are honest: include when NOT to use something.
			- card_type must be one of: definition, mechanism, tradeoff, application.

		AUTHORITY ANCHORS:
			- For storage, indexing, replication, transactions: frame as Designing Data-Intensive Applications (Kleppmann) does.
			- For end-to-end system design and estimation: frame as Alex Xu's System Design Interview does.

		RESPONSE SHAPE:
			[
			  {
			    "tag": "<leaf concept name>",
			    "parent_tag": "<immediate parent>",
			    "root_tag": "<top-level branch>",
			    "cards": [
			      {
			        "question": "...",
			        "answer": "...",
			        "examples": "...",
			        "tradeoffs": "...",
			        "card_type": "definition | mechanism | tradeoff | application"
			      }
			    ]
			  }
			]

		EXAMPLE OF A GOOD CARD (do not include this in output, this is for reference):
			{
			  "question": "Why does Postgres use B-trees as the default index type?",
			  "answer": "B-trees provide O(log n) lookups and efficient range scans on
			    disk-based storage, which match the most common OLTP query patterns:
			    equality lookups (WHERE id = ?) and range scans (WHERE created_at
			    BETWEEN ? AND ?). B-trees update in place on fixed-size pages, which
			    fits Postgres's page-oriented storage engine.",
			  "examples": "Postgres CREATE INDEX defaults to btree. MySQL InnoDB stores
			    the primary key as a clustered B-tree. SQLite also uses B-trees.",
			  "tradeoffs": "B-trees degrade for write-heavy workloads with random keys
			    due to page splits; LSM-trees are better there (RocksDB, Cassandra).
			    B-trees do poorly on full-text search; use GIN/inverted indexes
			    (Elasticsearch). Every B-tree index is a copy of the indexed columns,
			    consuming significant disk.",
			  "card_type": "mechanism"
			}
	`
)

type Node struct {
	Name     string `json:"name"`
	Notes    string `json:"notes,omitempty"`
	Children []Node `json:"children,omitempty"`
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
	Role    string `json:"role"` // check if this can be limited to a set of options
	Content string `json:"content"`
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
		MaxTokens:      8000,
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

func formatSubtrees(nodes []Node) string {
	var sb strings.Builder
	for _, node := range nodes {
		fmt.Fprintf(&sb, "Root: %s\nSubtree:\n\n", node.Name)
		sb.WriteString(formatNode(node, 0))
	}
	return sb.String()
}

func createUserMessage(nodes []Node) (message, error) {
	content := formatSubtrees(nodes)
	return newMessage(content, "user")
}

func createSystemMessage() (message, error) {
	return newMessage(SystemPrompt, "system")
}

func createPayload(nodes []Node) (requestPayload, error) {
	systemMessage, err := createSystemMessage()
	if err != nil {
		return requestPayload{}, err
	}
	userMessage, err := createUserMessage(nodes[:1])
	if err != nil {
		return requestPayload{}, err
	}

	messages := []message{systemMessage, userMessage}
	payload, err := newRequestPayload(messages)
	if err != nil {
		return requestPayload{}, err
	}

	return payload, nil
}

func makeRequest(payloadData requestPayload) {
	apiKey := os.Getenv("DEEPSEEK_API_KEY")
	baseUrl := os.Getenv("DEEPSEEK_BASE_URL")

	if apiKey == "" {
		fmt.Println("DEEPSEEK_API_KEY is not set")
		os.Exit(1)
	}
	if baseUrl == "" {
		fmt.Println("DEEPSEEK_BASE_URL is not set")
		os.Exit(1)
	}

	payload, err := json.Marshal(payloadData)
	if err != nil {
		fmt.Println("marshalling payload data failed", err)
	}

	req, _ := http.NewRequest("POST", baseUrl, bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("api request failed", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error reading response body", err)
		os.Exit(1)
	}

	var pretty bytes.Buffer
	if err := json.Indent(&pretty, body, "", "  "); err != nil {
		fmt.Println(string(body))
		return
	}
	fmt.Println(pretty.String())
}

func main() {
	children := os.Args[1:]
	if len(children) == 0 {
		fmt.Println("No children specified")
		os.Exit(1)
	}

	root, error := getRootNode("system-design-hierarchy.json")
	if error != nil {
		fmt.Println(error)
		os.Exit(1)
	}

	childrenNodes := findChildrenNodes(root, children)
	payload, err := createPayload(childrenNodes)
	if err != nil {
		fmt.Println("Failed to create payload", err)
	}
	makeRequest(payload)
}
