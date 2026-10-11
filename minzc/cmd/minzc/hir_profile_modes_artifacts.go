package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

func validateCompilerModeDestination(directory string) error {
	if _, err := os.Lstat(directory); err == nil {
		return fmt.Errorf("compiler modes: artifact directory already exists; choose a new directory")
	} else if !os.IsNotExist(err) {
		return err
	}
	return nil
}

func compilerModePathContains(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	return err == nil && rel != ".." && rel != "." && len(rel) > 0 && (len(rel) < 3 || rel[:3] != ".."+string(filepath.Separator))
}

func validateCompilerModePaths(directory string, inputs, names []string) error {
	if err := validateCompilerModeDestination(directory); err != nil {
		return err
	}
	if err := validateMemoryHostCLIArtifactAliases(directory, inputs, names); err != nil {
		return err
	}
	dir, err := canonicalMemoryHostCLIPath(directory)
	if err != nil {
		return err
	}
	for _, path := range append(append([]string{}, inputs...), outputFile) {
		if path == "" {
			continue
		}
		canonical, err := canonicalMemoryHostCLIPath(path)
		if err != nil {
			return err
		}
		if dir == canonical || compilerModePathContains(dir, canonical) || compilerModePathContains(canonical, dir) {
			return fmt.Errorf("compiler modes: artifact directory and input/output paths must not contain each other")
		}
	}
	if outputFile != "" {
		if info, err := os.Lstat(outputFile); err == nil && !info.Mode().IsRegular() {
			return fmt.Errorf("compiler modes: -o must be a regular file path")
		} else if err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func stageCompilerModeFile(path string, data []byte) (string, error) {
	file, err := os.CreateTemp(filepath.Dir(path), ".compiler-modes-output-*")
	if err != nil {
		return "", err
	}
	temp := file.Name()
	success := false
	defer func() {
		file.Close()
		if !success {
			os.Remove(temp)
		}
	}()
	if err = file.Chmod(0644); err != nil {
		return "", err
	}
	if _, err = file.Write(data); err != nil {
		return "", err
	}
	if err = file.Sync(); err != nil {
		return "", err
	}
	if err = file.Close(); err != nil {
		return "", err
	}
	success = true
	return temp, nil
}

func publishCompilerModeArtifacts(directory string, payloads map[string][]byte, output string, chosen []byte) error {
	if err := validateCompilerModeDestination(directory); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(directory), 0755); err != nil {
		return err
	}
	stage, err := os.MkdirTemp(filepath.Dir(directory), ".compiler-modes-artifacts-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	names := make([]string, 0, len(payloads))
	for name := range payloads {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if filepath.IsAbs(name) || filepath.Clean(name) != name || name == "." || !compilerModePathContains(stage, filepath.Join(stage, name)) {
			return fmt.Errorf("compiler modes: invalid artifact name %q", name)
		}
		path := filepath.Join(stage, name)
		if err = os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}
		temp, err := stageCompilerModeFile(path, payloads[name])
		if err != nil {
			return err
		}
		if err = os.Rename(temp, path); err != nil {
			os.Remove(temp)
			return err
		}
	}
	if err = os.Chmod(stage, 0755); err != nil {
		return err
	}
	stagedOutput := ""
	if output != "" {
		stagedOutput, err = stageCompilerModeFile(output, chosen)
		if err != nil {
			return err
		}
		defer os.Remove(stagedOutput)
	}
	if err = publishCompilerModeDirectory(stage, directory); err != nil {
		return err
	}
	// Each path is atomic independently. A final output rename failure preserves
	// the complete published evidence and returns an error; it never truncates -o.
	if output != "" {
		return os.Rename(stagedOutput, output)
	}
	return nil
}
