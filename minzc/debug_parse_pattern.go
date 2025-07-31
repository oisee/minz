package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
)

func main() {
	// Run tree-sitter parse with JSON output
	cmd := exec.Command("tree-sitter", "parse", "examples/test_pattern_guard_simple.minz", "--json")
	output, err := cmd.Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error running tree-sitter: %v\n", err)
		return
	}

	// Parse the JSON
	var result map[string]interface{}
	if err := json.Unmarshal(output, &result); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing JSON: %v\n", err)
		return
	}

	// Pretty print the JSON
	prettyJSON, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(prettyJSON))
}