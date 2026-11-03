package cardigann

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDefinitionsFallbackAndCustomDir(t *testing.T) {
	// 1. Missing directory falls back to built-in definitions
	fallbackDefs, err := LoadDefinitions(filepath.Join(t.TempDir(), "non-existent"), 50)
	if err != nil {
		t.Fatalf("LoadDefinitions fallback failed: %v", err)
	}
	if len(fallbackDefs) == 0 {
		t.Fatalf("expected non-empty fallback definitions")
	}

	// 2. Custom YAML file in temp directory
	dir := t.TempDir()
	sampleYML := `---
id: custom-tracker
name: Custom Tracker
description: "A test Cardigann tracker definition"
language: en-US
type: private
encoding: UTF-8
links:
  - https://tracker.example.org/
caps:
  modes:
    search: [q]
settings:
  - name: apikey
    type: password
    label: API Key
`
	if err := os.WriteFile(filepath.Join(dir, "custom-tracker.yml"), []byte(sampleYML), 0o644); err != nil {
		t.Fatalf("write sample yml: %v", err)
	}

	defs, err := LoadDefinitions(dir, 10)
	if err != nil {
		t.Fatalf("LoadDefinitions custom dir failed: %v", err)
	}
	if len(defs) != 1 {
		t.Fatalf("expected 1 definition, got %d", len(defs))
	}
	if defs[0].ID != "custom-tracker" || defs[0].FileName != "custom-tracker" || defs[0].Name != "Custom Tracker" || defs[0].Type != "private" {
		t.Fatalf("unexpected parsed definition: %+v", defs[0])
	}
	if len(defs[0].Links) != 1 || defs[0].Links[0] != "https://tracker.example.org/" {
		t.Fatalf("unexpected links: %+v", defs[0].Links)
	}
}
