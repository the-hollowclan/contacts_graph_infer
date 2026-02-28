package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/its-ernest/opentrace/sdk"
)

type Module struct{}

// ---------------- CONFIG ----------------

type config struct {
	Model    string   `json:"model"`
	Subject  []string `json:"subject"`
	Relation []string `json:"relation"`
}

// ---------------- GRAPH TYPES ----------------

type Person struct {
	Name     string `json:"name"`
	Phone    string `json:"phone"`
	Location string `json:"location,omitempty"`
}

type Edge struct {
	OwnerPhone   string `json:"owner_phone"`
	ContactPhone string `json:"contact_phone"`
	Weight       int    `json:"weight"`
}

type Graph struct {
	Nodes []Person               `json:"nodes"`
	Edges []Edge                 `json:"edges"`
	Meta  map[string]interface{} `json:"meta"`
}

// ---------------- WRAPPED INPUT ----------------
// This matches opentrace chaining behavior

type WrappedOutput struct {
	Result string `json:"result"`
}

// ---------------- RESULT ----------------

type Relationship struct {
	With        string             `json:"with"`
	Confidence  float32            `json:"confidence"`
	Signals     map[string]float32 `json:"signals"`
	Explanation string             `json:"explanation"`
}

// ---------------- MODULE ----------------

func (m *Module) Name() string {
	return "contacts_graph_infer"
}

func (m *Module) Run(input sdk.Input) (sdk.Output, error) {
	var cfg config
	rawCfg, _ := json.Marshal(input.Config)
	if err := json.Unmarshal(rawCfg, &cfg); err != nil {
		return sdk.Output{}, err
	}

	if _, err := os.Stat(cfg.Model); err != nil {
		return sdk.Output{}, fmt.Errorf("onnx model not found: %s", cfg.Model)
	}

	if len(cfg.Subject) == 0 || len(cfg.Relation) == 0 {
		return sdk.Output{}, fmt.Errorf("subject and relation must be provided")
	}

	subject := cfg.Subject[0]
	target := cfg.Relation[0]

	// 🔥 THIS IS THE ONLY CORRECT PARSE
	var graph Graph
	if err := json.Unmarshal([]byte(input.Input), &graph); err != nil {
		return sdk.Output{}, fmt.Errorf("invalid graph input: %w", err)
	}

	signals := extractFeatures(graph, subject, target)
	confidence := runONNX(cfg.Model, signals)

	result := Relationship{
		With:        target,
		Confidence:  confidence,
		Signals:     signals,
		Explanation: "Graph co-occurrence and reciprocal contact inference",
	}

	raw, _ := json.Marshal(result)
	return sdk.Output{Result: string(raw)}, nil
}

// ---------------- FEATURE ENGINEERING ----------------

func extractFeatures(graph Graph, subject, target string) map[string]float32 {
	var coOccurrence float32
	var reciprocal float32
	var sharedOwners float32

	owners := make(map[string]bool)

	for _, e := range graph.Edges {
		if e.OwnerPhone == subject {
			owners[e.ContactPhone] = true
		}
	}

	for _, e := range graph.Edges {
		if e.OwnerPhone == subject && e.ContactPhone == target {
			coOccurrence += float32(e.Weight)
		}
		if e.OwnerPhone == target && e.ContactPhone == subject {
			reciprocal = 1
		}
		if owners[e.ContactPhone] && e.OwnerPhone == target {
			sharedOwners++
		}
	}

	return map[string]float32{
		"co_occurrence": coOccurrence,
		"reciprocal":    reciprocal,
		"shared_links":  sharedOwners,
	}
}

// ---------------- ONNX STUB ----------------

func runONNX(modelPath string, features map[string]float32) float32 {
	score := float32(0)

	score += features["co_occurrence"] * 0.15
	score += features["shared_links"] * 0.10

	if features["reciprocal"] > 0 {
		score += 0.35
	}

	if score > 1 {
		score = 1
	}

	return score
}

// ---------------- MAIN ----------------

func main() {
	sdk.Run(&Module{})
}