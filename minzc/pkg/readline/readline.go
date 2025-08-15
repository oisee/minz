// Package readline provides line editing and history for REPL
package readline

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Reader provides readline functionality with history
type Reader struct {
	input         io.Reader
	output        io.Writer
	prompt        string
	history       []string
	historyFile   string
	historyIndex  int
	maxHistory    int
	currentLine   string
	scanner       *bufio.Scanner
}

// Config holds readline configuration
type Config struct {
	Prompt      string
	HistoryFile string
	MaxHistory  int
	Input       io.Reader
	Output      io.Writer
}

// NewReader creates a new readline reader
func NewReader(config *Config) *Reader {
	if config.Input == nil {
		config.Input = os.Stdin
	}
	if config.Output == nil {
		config.Output = os.Stdout
	}
	if config.MaxHistory == 0 {
		config.MaxHistory = 1000
	}
	
	r := &Reader{
		input:       config.Input,
		output:      config.Output,
		prompt:      config.Prompt,
		historyFile: config.HistoryFile,
		maxHistory:  config.MaxHistory,
		history:     []string{},
		scanner:     bufio.NewScanner(config.Input),
	}
	
	// Load history from file
	if config.HistoryFile != "" {
		r.loadHistory()
	}
	
	return r
}

// ReadLine reads a line with editing and history support
func (r *Reader) ReadLine() (string, error) {
	// Print prompt
	fmt.Fprint(r.output, r.prompt)
	
	// Simple implementation for now - just read line
	// TODO: Add arrow key support with terminal raw mode
	if !r.scanner.Scan() {
		if err := r.scanner.Err(); err != nil {
			return "", err
		}
		return "", io.EOF
	}
	
	line := r.scanner.Text()
	
	// Add to history if not empty and not duplicate
	if line != "" && (len(r.history) == 0 || r.history[len(r.history)-1] != line) {
		r.AddHistory(line)
	}
	
	return line, nil
}

// AddHistory adds a line to history
func (r *Reader) AddHistory(line string) {
	r.history = append(r.history, line)
	
	// Trim history if too long
	if len(r.history) > r.maxHistory {
		r.history = r.history[len(r.history)-r.maxHistory:]
	}
	
	// Save to file
	if r.historyFile != "" {
		r.saveHistory()
	}
}

// GetHistory returns the command history
func (r *Reader) GetHistory() []string {
	return r.history
}

// ClearHistory clears the history
func (r *Reader) ClearHistory() {
	r.history = []string{}
	if r.historyFile != "" {
		os.Remove(r.historyFile)
	}
}

// SetPrompt changes the prompt
func (r *Reader) SetPrompt(prompt string) {
	r.prompt = prompt
}

// loadHistory loads history from file
func (r *Reader) loadHistory() error {
	// Ensure directory exists
	dir := filepath.Dir(r.historyFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	
	// Read history file
	data, err := os.ReadFile(r.historyFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // File doesn't exist yet
		}
		return err
	}
	
	// Parse history
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			r.history = append(r.history, line)
		}
	}
	
	// Trim to max size
	if len(r.history) > r.maxHistory {
		r.history = r.history[len(r.history)-r.maxHistory:]
	}
	
	return nil
}

// saveHistory saves history to file
func (r *Reader) saveHistory() error {
	// Ensure directory exists
	dir := filepath.Dir(r.historyFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	
	// Join history lines
	data := strings.Join(r.history, "\n")
	
	// Write to file
	return os.WriteFile(r.historyFile, []byte(data), 0644)
}

// SearchHistory searches history for lines containing the query
func (r *Reader) SearchHistory(query string) []string {
	var results []string
	query = strings.ToLower(query)
	
	for _, line := range r.history {
		if strings.Contains(strings.ToLower(line), query) {
			results = append(results, line)
		}
	}
	
	return results
}

// GetHistoryItem returns a specific history item by index
func (r *Reader) GetHistoryItem(index int) string {
	if index < 0 || index >= len(r.history) {
		return ""
	}
	return r.history[index]
}

// SaveHistoryToFile saves current history to a specific file
func (r *Reader) SaveHistoryToFile(filename string) error {
	data := strings.Join(r.history, "\n")
	return os.WriteFile(filename, []byte(data), 0644)
}

// LoadHistoryFromFile loads history from a specific file
func (r *Reader) LoadHistoryFromFile(filename string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}
	
	r.history = []string{}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			r.history = append(r.history, line)
		}
	}
	
	return nil
}