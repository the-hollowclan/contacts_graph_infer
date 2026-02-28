package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/its-ernest/osintrace/sdk"
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

func (m *Module) Run(input sdk.Input, ctx sdk.Context) error {
	// -------- Parse config --------
	var cfg config
	rawCfg, err := json.Marshal(input.Config)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(rawCfg, &cfg); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	if cfg.Model == "" {
		return fmt.Errorf("config.model is required")
	}

	if _, err := os.Stat(cfg.Model); err != nil {
		return fmt.Errorf("model not found: %s", cfg.Model)
	}

	if len(cfg.Subject) == 0 || len(cfg.Relation) == 0 {
		return fmt.Errorf("config.subject and config.relation are required")
	}

	subject := cfg.Subject[0]
	target := cfg.Relation[0]

	fmt.Fprintln(os.Stderr, "loading graph:", input.Input)
	fmt.Fprintln(os.Stderr, "subject:", subject, "target:", target)

	// -------- Load graph artifact --------
	rawGraph, err := os.ReadFile(input.Input)
	if err != nil {
		return fmt.Errorf("failed to read graph artifact: %w", err)
	}

	var graph Graph
	if err := json.Unmarshal(rawGraph, &graph); err != nil {
		return fmt.Errorf("invalid graph JSON: %w", err)
	}

	if len(graph.Nodes) == 0 || len(graph.Edges) == 0 {
		return fmt.Errorf("graph is empty or malformed")
	}

	// -------- Feature extraction + inference --------
	signals := extractFeatures(graph, subject, target)
	confidence := runONNX(cfg.Model, signals)

	result := Relationship{
		With:        target,
		Confidence:  confidence,
		Signals:     signals,
		Explanation: "Graph co-occurrence and reciprocal contact inference",
	}

	// -------- Write result artifact --------
	resultPath := filepath.Join(ctx.StepDir, "relationship.json")
	rawResult, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(resultPath, rawResult, 0o644); err != nil {
		return err
	}

	fmt.Fprintln(os.Stderr, "relationship written:", resultPath)
	fmt.Fprintln(os.Stderr, "confidence:", confidence)

	// -------- Write output index --------
	outputIndex := map[string]any{
		"artifacts": map[string]any{
			"relationship": map[string]any{
				"path": "relationship.json",
				"type": "application/json",
			},
		},
	}

	indexPath := filepath.Join(ctx.StepDir, "output.json")
	rawIndex, _ := json.MarshalIndent(outputIndex, "", "  ")

	if err := os.WriteFile(indexPath, rawIndex, 0o644); err != nil {
		return err
	}

	return nil
}

// ---------------- FEATURE ENGINEERING ----------------

func extractFeatures(graph Graph, subject, target string) map[string]float32 {
	var coOccurrence float32
	var reciprocal float32
	var sharedLinks float32

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
			sharedLinks++
		}
	}

	return map[string]float32{
		"co_occurrence": coOccurrence,
		"reciprocal":    reciprocal,
		"shared_links":  sharedLinks,
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