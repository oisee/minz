package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func validateMemoryHostSelectedOutput(path string) error {
	if path == "" {
		return nil
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".bin", ".a80":
		return nil
	default:
		return fmt.Errorf("memory host CLI: -o must end in .bin or .a80")
	}
}

func writeMemoryHostSelectedOutput(path, assembly string, binary []byte) error {
	if err := validateMemoryHostSelectedOutput(path); err != nil {
		return err
	}
	data := []byte(assembly)
	if strings.EqualFold(filepath.Ext(path), ".bin") {
		data = binary
	}
	return os.WriteFile(path, data, 0644)
}
