package semantic

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/minz/minzc/pkg/ast"
)

// LogLevel represents the severity of a log message
type LogLevel int

const (
	LogLevelTrace LogLevel = iota
	LogLevelDebug
	LogLevelInfo
	LogLevelWarn
	LogLevelError
	LogLevelOut // Default output level
)

// LogHandler handles compile-time logging metafunctions
type LogHandler struct {
	Level      LogLevel
	ShowColors bool
	StartTime  time.Time
	Timings    map[string]time.Time
}

// NewLogHandler creates a new compile-time log handler
func NewLogHandler() *LogHandler {
	return &LogHandler{
		Level:      LogLevelInfo,
		ShowColors: true,
		StartTime:  time.Now(),
		Timings:    make(map[string]time.Time),
	}
}

// HandleLogMetafunction processes @log.* metafunctions during compilation
func (h *LogHandler) HandleLogMetafunction(call *ast.MetafunctionCall, analyzer *Analyzer) error {
	// Parse the log level from the function name
	level := h.parseLogLevel(call.Name)
	
	// Don't output if below configured level
	if level < h.Level && level != LogLevelOut {
		return nil
	}
	
	// Evaluate arguments at compile time
	var args []string
	for _, arg := range call.Arguments {
		value := h.evaluateCompileTime(arg, analyzer)
		args = append(args, value)
	}
	
	// Format and output the message
	message := h.formatMessage(level, args)
	fmt.Fprintln(os.Stderr, message)
	
	return nil
}

// parseLogLevel extracts the log level from the metafunction name
func (h *LogHandler) parseLogLevel(name string) LogLevel {
	// Handle @log.debug, @log.info, etc.
	if strings.HasPrefix(name, "@log.") {
		levelStr := strings.TrimPrefix(name, "@log.")
		switch levelStr {
		case "trace":
			return LogLevelTrace
		case "debug":
			return LogLevelDebug
		case "info":
			return LogLevelInfo
		case "warn":
			return LogLevelWarn
		case "error":
			return LogLevelError
		case "out":
			return LogLevelOut
		default:
			return LogLevelOut
		}
	}
	
	// Default @log maps to @log.out
	if name == "@log" {
		return LogLevelOut
	}
	
	return LogLevelOut
}

// evaluateCompileTime evaluates an expression at compile time
func (h *LogHandler) evaluateCompileTime(expr ast.Expression, analyzer *Analyzer) string {
	switch e := expr.(type) {
	case *ast.StringLiteral:
		return e.Value
	
	case *ast.NumberLiteral:
		return fmt.Sprintf("%v", e.Value)
	
	case *ast.BooleanLiteral:
		return fmt.Sprintf("%v", e.Value)
	
	case *ast.Identifier:
		// Look up compile-time constants
		if symbol := analyzer.currentScope.Lookup(e.Name); symbol != nil {
			// For now, just return the identifier name
			// TODO: Implement proper constant evaluation
			return e.Name
		}
		return fmt.Sprintf("<undefined: %s>", e.Name)
	
	case *ast.BinaryExpr:
		// Simple binary operations on compile-time values
		left := h.evaluateCompileTime(e.Left, analyzer)
		right := h.evaluateCompileTime(e.Right, analyzer)
		return fmt.Sprintf("(%s %s %s)", left, e.Operator, right)
	
	default:
		return fmt.Sprintf("<complex expression>")
	}
}

// formatMessage formats a log message with the appropriate prefix
func (h *LogHandler) formatMessage(level LogLevel, args []string) string {
	var prefix string
	var color string
	
	// ANSI color codes for terminal output
	const (
		colorReset  = "\033[0m"
		colorRed    = "\033[31m"
		colorYellow = "\033[33m"
		colorBlue   = "\033[34m"
		colorGray   = "\033[90m"
		colorGreen  = "\033[32m"
	)
	
	switch level {
	case LogLevelTrace:
		prefix = "[TRACE]"
		color = colorGray
	case LogLevelDebug:
		prefix = "[DEBUG]"
		color = colorGray
	case LogLevelInfo:
		prefix = "[INFO]"
		color = colorBlue
	case LogLevelWarn:
		prefix = "[WARN]"
		color = colorYellow
	case LogLevelError:
		prefix = "[ERROR]"
		color = colorRed
	case LogLevelOut:
		prefix = "[COMPILE]"
		color = colorGreen
	}
	
	message := strings.Join(args, " ")
	
	if h.ShowColors && color != "" {
		return fmt.Sprintf("%s%s%s %s", color, prefix, colorReset, message)
	}
	
	return fmt.Sprintf("%s %s", prefix, message)
}

// HandleTimingStart starts a timing measurement
func (h *LogHandler) HandleTimingStart(name string) {
	h.Timings[name] = time.Now()
	message := h.formatMessage(LogLevelInfo, []string{
		fmt.Sprintf("Started: %s", name),
	})
	fmt.Fprintln(os.Stderr, message)
}

// HandleTimingEnd ends a timing measurement
func (h *LogHandler) HandleTimingEnd(name string) {
	if start, exists := h.Timings[name]; exists {
		duration := time.Since(start)
		message := h.formatMessage(LogLevelInfo, []string{
			fmt.Sprintf("Completed: %s (%dms)", name, duration.Milliseconds()),
		})
		fmt.Fprintln(os.Stderr, message)
		delete(h.Timings, name)
	}
}

// SetLogLevel sets the minimum log level to display
func (h *LogHandler) SetLogLevel(levelStr string) {
	switch strings.ToLower(levelStr) {
	case "trace":
		h.Level = LogLevelTrace
	case "debug":
		h.Level = LogLevelDebug
	case "info":
		h.Level = LogLevelInfo
	case "warn":
		h.Level = LogLevelWarn
	case "error":
		h.Level = LogLevelError
	case "none":
		h.Level = LogLevelError + 1 // Suppress all logs
	default:
		h.Level = LogLevelInfo
	}
}