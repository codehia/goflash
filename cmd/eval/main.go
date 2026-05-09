package main

import (
	"fmt"

	"github.com/codehia/goflash/internal/ai"
)

type scenario struct {
	name          string
	question      string
	correctAnswer string
	userAnswer    string
}

var scenarios = []scenario{
	{
		name:     "Strong answer — Vertical Scaling",
		question: "What is vertical scaling and what are its limitations?",
		correctAnswer: `Vertical scaling means adding more CPU, RAM, or disk to a single machine.
It is simple and requires no coordination overhead. However it is bounded by
single-machine hardware ceilings (around 1-2TB RAM, ~100 cores), and creates
a single point of failure. Common starting point for early-stage Postgres or
MySQL deployments.`,
		userAnswer: `Vertical scaling is upgrading a single machine with more resources like CPU
and RAM. It's simple because you don't need to change application code or
coordinate between nodes. The downsides are that there's a hard hardware
ceiling you can't go past, and the machine becomes a single point of failure.
Good for early-stage databases like Postgres before you need to shard.`,
	},
	{
		name:     "Partial answer — Nines and Downtime Math",
		question: "What do availability nines mean and how much downtime does each allow per year?",
		correctAnswer: `99% allows 3.65 days/year downtime. 99.9% allows 8.76 hours. 99.99% allows
52.6 minutes. 99.999% allows 5.26 minutes. Each additional nine costs
exponentially more. These are used for back-of-envelope estimation in system
design.`,
		userAnswer: `Higher nines mean higher availability. 99.9% is pretty good and means only
a few hours of downtime per year. 99.999% is five nines and is very hard to
achieve. Companies use SLAs to promise certain availability to customers.`,
	},
	{
		name:     "Weak answer — SLA vs SLO vs SLI",
		question: "What is the difference between SLA, SLO, and SLI?",
		correctAnswer: `SLI is the actual measurement (e.g. uptime percentage). SLO is the internal
target (e.g. 99.95%). SLA is the contractual promise to customers (e.g. 99.9%)
usually with financial penalties for breach. Companies set SLO stricter than
SLA to maintain an error budget. Canonical reference: Google SRE book.`,
		userAnswer: `SLA is a service level agreement, it's the contract with the customer about
uptime. SLO and SLI are similar things related to performance goals.`,
	},
}

func main() {
	for _, s := range scenarios {
		fmt.Printf("\n=== %s ===\n", s.name)
		result, err := ai.Evaluate(s.question, s.correctAnswer, s.userAnswer)
		if err != nil {
			fmt.Printf("ERROR: %v\n", err)
			continue
		}
		fmt.Printf("Score:    %d/5\n", result.Score)
		fmt.Printf("Feedback: %s\n", result.Feedback)
	}
}
