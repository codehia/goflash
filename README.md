# goflash

> AI-evaluated spaced-repetition flashcards in your terminal.

![demo](demo.gif)

Most flashcard tools make you rate yourself, which is unreliable and easy to game. goflash feeds your answers to an AI that scores them objectively (0-5), then uses SM-2 spaced repetition to resurface cards at the right time. Everything runs in the terminal and stores locally in SQLite.

---

## Features

- **AI evaluation** - answers scored by DeepSeek against the stored reference answer
- **SM-2 scheduling** - cards resurface based on your performance; easy cards disappear longer, hard cards come back sooner
- **Topic hierarchy** - cards are organized as a browsable tree; bring your own topic or start with the included system design deck (114 topics, 1996 cards)
- **Local-first** - SQLite database, no account, no cloud sync
- **Terminal UI** - keyboard-driven, Catppuccin Mocha theme

---

## Prerequisites

- Go 1.21 or higher
- A [DeepSeek API key](https://platform.deepseek.com/) (for evaluating answers and seeding)
- [direnv](https://direnv.net/) for environment variable management (recommended)

---

## Quick Start

```bash
git clone https://github.com/codehia/goflash.git
cd goflash
cp .envrc.example .envrc
# edit .envrc and set DEEPSEEK_API_KEY
direnv allow
go run main.go
```

The repo ships with a pre-seeded system design database (1996 cards) so you can start immediately. To study your own topics, see [Seeding Your Own Topics](#seeding-your-own-topics).

> If you use Nix, a `devenv` shell is included: `devenv shell` sets up the full environment.

---

## Keybindings

| Key | Action |
|-----|--------|
| `up` / `down` | Navigate topic list |
| `enter` | Select topic / advance |
| `ctrl+s` | Submit answer |
| `n` | Next card |
| `q` / `ctrl+c` | Quit |

---

## How It Works

1. Pick a topic from the list
2. Read the question, type your answer freely, submit with `ctrl+s`
3. DeepSeek scores your answer (0-5) and shows feedback alongside the reference answer
4. SM-2 calculates the next due date - nail it and the card will not appear for days; struggle and it comes back tomorrow

---

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Language | Go |
| TUI | [bubbletea v2](https://github.com/charmbracelet/bubbletea) + [bubbles](https://github.com/charmbracelet/bubbles) + [lipgloss](https://github.com/charmbracelet/lipgloss) |
| Database | SQLite via [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite) (pure Go, no CGo) |
| Query builder | [go-jet/jet](https://github.com/go-jet/jet) - type-safe SQL, schema-generated models |
| Migrations | [goose](https://github.com/pressly/goose) |
| AI eval | DeepSeek |
| Card seeding | DeepSeek via OpenRouter |
| Scheduling | SM-2 algorithm |

---

## Architecture

```
goflash/
├── cmd/
│   ├── seed/       reads topic JSON -> calls DeepSeek -> writes output.json
│   └── import/     reads output.json -> upserts cards + tags into SQLite
├── internal/
│   ├── ai/         DeepSeek eval client
│   ├── scheduler/  pure SM-2 math, no DB deps
│   ├── store/      DB layer: cards, topics, attempts (jet queries + goose migrations)
│   └── tui/        Elm-style screens: topic list -> question -> attempt -> eval -> done
└── main.go         opens DB -> launches bubbletea TUI
```

Data flow per review:

```
user answer -> DeepSeek eval -> score -> SM-2 -> new due_date written to DB
```

---

## Seeding Your Own Topics

Prepare a JSON file describing your topic hierarchy:

```json
{
  "name": "Your Topic",
  "children": [
    {
      "name": "Subtopic",
      "children": [
        { "name": "Leaf Topic", "notes": "Your notes here." }
      ]
    }
  ]
}
```

```bash
go run cmd/seed/main.go seedfile.json   # generates output.json (resumable)
go run cmd/import/main.go               # imports into SQLite
go run main.go                          # start studying
```

---

## License

MIT. See [LICENSE](LICENSE)
