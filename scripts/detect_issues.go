package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// SuspiciousPattern represents a pattern that might indicate a bug
type SuspiciousPattern struct {
	Name        string
	Pattern     *regexp.Regexp
	Description string
	Severity    string // "critical", "warning", "info"
}

// Issue represents a detected problem
type Issue struct {
	Pattern     string
	File        string
	Line        int
	Context     string
	Severity    string
	Suggestion  string
}

var patterns = []SuspiciousPattern{
	{
		Name:        "Parameter Overwrite",
		Pattern:     regexp.MustCompile(`LD\s+A,\s*\d+\s*\n\s*LD\s+A,\s*\d+`),
		Description: "Second LD A instruction overwrites the first parameter",
		Severity:    "critical",
	},
	{
		Name:        "Redundant Stack Operation",
		Pattern:     regexp.MustCompile(`PUSH\s+(\w+)\s*\n[^\n]*\n\s*POP\s+\1\s*\n\s*RET`),
		Description: "Unnecessary push/pop before return",
		Severity:    "warning",
	},
	{
		Name:        "Duplicate Load",
		Pattern:     regexp.MustCompile(`LD\s+(\w+),\s*([^\n]+)\n[^\n]*\n\s*LD\s+\1,\s*\2`),
		Description: "Same value loaded twice to same register",
		Severity:    "warning",
	},
	{
		Name:        "Dead Jump",
		Pattern:     regexp.MustCompile(`JP\s+\.(\w+)\s*\n\.\1:`),
		Description: "Jump to immediately following label",
		Severity:    "info",
	},
	{
		Name:        "Unoptimized Clear",
		Pattern:     regexp.MustCompile(`LD\s+A,\s*0(?:\s*;[^\n]*)?\n`),
		Description: "Using LD A,0 instead of XOR A",
		Severity:    "info",
	},
	{
		Name:        "Register Reload",
		Pattern:     regexp.MustCompile(`LD\s+(\w+),\s*\(([^\)]+)\)\s*\n[^\n]*\n\s*LD\s+\1,\s*\(\2\)`),
		Description: "Same memory location loaded twice",
		Severity:    "warning",
	},
}

func detectIssues(filename string) ([]Issue, error) {
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var issues []Issue
	lines := strings.Split(string(content), "\n")

	for _, pattern := range patterns {
		matches := pattern.Pattern.FindAllStringIndex(string(content), -1)
		
		for _, match := range matches {
			lineNum := strings.Count(string(content[:match[0]]), "\n") + 1
			contextStart := lineNum - 2
			if contextStart < 0 {
				contextStart = 0
			}
			contextEnd := lineNum + 2
			if contextEnd >= len(lines) {
				contextEnd = len(lines) - 1
			}
			
			context := strings.Join(lines[contextStart:contextEnd], "\n")
			
			issue := Issue{
				Pattern:    pattern.Name,
				File:       filename,
				Line:       lineNum,
				Context:    context,
				Severity:   pattern.Severity,
				Suggestion: getSuggestion(pattern.Name),
			}
			issues = append(issues, issue)
		}
	}

	return issues, nil
}

func getSuggestion(patternName string) string {
	suggestions := map[string]string{
		"Parameter Overwrite":      "Use different registers for each parameter (e.g., LD B, 0 for second param)",
		"Redundant Stack Operation": "Remove unnecessary PUSH/POP pairs",
		"Duplicate Load":           "Remove the second load instruction",
		"Dead Jump":                "Remove the unnecessary jump",
		"Unoptimized Clear":        "Replace with XOR A (faster, smaller)",
		"Register Reload":          "Reuse the already loaded value",
	}
	return suggestions[patternName]
}

func generateIssueReport(issues []Issue) string {
	if len(issues) == 0 {
		return "No issues detected! 🎉"
	}

	report := fmt.Sprintf("# MinZ Assembly Analysis Report\n\n")
	report += fmt.Sprintf("**Generated:** %s\n", time.Now().Format("2006-01-02 15:04:05"))
	report += fmt.Sprintf("**Total Issues:** %d\n\n", len(issues))

	// Group by severity
	critical := filterBySeverity(issues, "critical")
	warnings := filterBySeverity(issues, "warning")
	info := filterBySeverity(issues, "info")

	if len(critical) > 0 {
		report += fmt.Sprintf("## 🚨 Critical Issues (%d)\n\n", len(critical))
		for _, issue := range critical {
			report += formatIssue(issue)
		}
	}

	if len(warnings) > 0 {
		report += fmt.Sprintf("## ⚠️ Warnings (%d)\n\n", len(warnings))
		for _, issue := range warnings {
			report += formatIssue(issue)
		}
	}

	if len(info) > 0 {
		report += fmt.Sprintf("## ℹ️ Information (%d)\n\n", len(info))
		for _, issue := range info {
			report += formatIssue(issue)
		}
	}

	return report
}

func filterBySeverity(issues []Issue, severity string) []Issue {
	var filtered []Issue
	for _, issue := range issues {
		if issue.Severity == severity {
			filtered = append(filtered, issue)
		}
	}
	return filtered
}

func formatIssue(issue Issue) string {
	return fmt.Sprintf("### %s\n\n**File:** `%s:%d`\n\n**Context:**\n```asm\n%s\n```\n\n**Suggestion:** %s\n\n---\n\n",
		issue.Pattern, issue.File, issue.Line, issue.Context, issue.Suggestion)
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: detect_issues <directory|file>")
		os.Exit(1)
	}

	path := os.Args[1]
	var allIssues []Issue

	// Check if path is directory or file
	info, err := os.Stat(path)
	if err != nil {
		log.Fatal(err)
	}

	if info.IsDir() {
		// Process all .a80 files in directory
		err := filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if strings.HasSuffix(filePath, ".a80") {
				issues, err := detectIssues(filePath)
				if err != nil {
					fmt.Printf("Error processing %s: %v\n", filePath, err)
					return nil
				}
				allIssues = append(allIssues, issues...)
			}
			return nil
		})
		if err != nil {
			log.Fatal(err)
		}
	} else {
		// Process single file
		issues, err := detectIssues(path)
		if err != nil {
			log.Fatal(err)
		}
		allIssues = issues
	}

	// Generate and print report
	report := generateIssueReport(allIssues)
	fmt.Println(report)

	// Write to file if requested
	if len(os.Args) > 2 && os.Args[2] == "--output" {
		outputFile := "assembly_analysis_report.md"
		if len(os.Args) > 3 {
			outputFile = os.Args[3]
		}
		err := ioutil.WriteFile(outputFile, []byte(report), 0644)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("\nReport written to %s\n", outputFile)
	}

	// Exit with error code if critical issues found
	for _, issue := range allIssues {
		if issue.Severity == "critical" {
			os.Exit(1)
		}
	}
}