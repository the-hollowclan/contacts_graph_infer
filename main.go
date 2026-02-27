package main

import (
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"
    "strings"

    "github.com/its-ernest/opentrace/sdk"
)

type Module struct{}

type config struct {
    LeakPaths      []string `json:"leak_paths"`
    MinOccurrences int      `json:"min_occurrences"`
    MaxContacts    int      `json:"max_contacts"`
    OutputDir      string   `json:"output_dir"`
}

type output struct {
    Subject      string `json:"subject"`
    ContactsFile string `json:"contacts_file"`
    ContactCount int    `json:"contact_count"`
    Online       bool   `json:"online"`
    Source       string `json:"source"`
}

func (m *Module) Name() string {
    return "contact_graph_infer"
}

func (m *Module) Run(input sdk.Input) (sdk.Output, error) {
    // Parse config from map[string]any
    var cfg config
    rawCfg, err := json.Marshal(input.Config)
    if err != nil {
        return sdk.Output{}, fmt.Errorf("config marshal: %w", err)
    }
    if err := json.Unmarshal(rawCfg, &cfg); err != nil {
        return sdk.Output{}, fmt.Errorf("invalid config: %w", err)
    }

    // Ensure output dir exists
    outDir := os.ExpandEnv(cfg.OutputDir)
    if outDir != "" {
        _ = os.MkdirAll(outDir, os.ModePerm)
    }

    // Subject is the literal input string
    subject := input.Input

    // Scan for co-occurrences
    contactCounts := make(map[string]int)
    for _, p := range cfg.LeakPaths {
        data, err := os.ReadFile(p)
        if err != nil {
            continue
        }
        lines := strings.Split(string(data), "\n")
        for _, line := range lines {
            if strings.Contains(line, subject) {
                parts := strings.Fields(line)
                for _, f := range parts {
                    if f != subject {
                        contactCounts[f]++
                    }
                }
            }
        }
    }

    // Prepare CSV output
    csvPath := filepath.Join(outDir, fmt.Sprintf("%s_contacts.csv", strings.ReplaceAll(subject, "+", "")))
    outF, err := os.Create(csvPath)
    if err != nil {
        return sdk.Output{}, fmt.Errorf("could not create CSV: %w", err)
    }
    defer outF.Close()

    fmt.Fprintln(outF, "contact,occurrences")
    count := 0
    for c, ccount := range contactCounts {
        if ccount >= cfg.MinOccurrences && (cfg.MaxContacts == 0 || count < cfg.MaxContacts) {
            fmt.Fprintf(outF, "%s,%d\n", c, ccount)
            count++
        }
    }

    // Build structured result
    res := output{
        Subject:      subject,
        ContactsFile: csvPath,
        ContactCount: count,
        Online:       true,
        Source:       "leak_cooccurrence_inference",
    }

    // Marshal structured result as JSON string
    raw, err := json.Marshal(res)
    if err != nil {
        return sdk.Output{}, fmt.Errorf("marshal result: %w", err)
    }

    // You may also print human-friendly logs for operator
    fmt.Printf("Inferred %d contacts for %s\n", count, subject)
    fmt.Printf("Contacts list saved to: %s\n", csvPath)

    // Return the JSON string as required
    return sdk.Output{Result: string(raw)}, nil
}

func main() {
    sdk.Run(&Module{})
}