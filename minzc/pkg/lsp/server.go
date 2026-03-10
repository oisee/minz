package lsp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/minz/minzc/pkg/ast"
	"github.com/minz/minzc/pkg/ir"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/parser"
	"github.com/minz/minzc/pkg/semantic"
)

// Server implements the Language Server Protocol over stdio JSON-RPC.
type Server struct {
	mu     sync.Mutex
	reader *bufio.Reader
	writer io.Writer
	logger *log.Logger

	initialized bool
	rootURI     string

	// Document state
	documents map[string]string // URI → content

	// Analysis cache (per-file)
	astCache    map[string]*ast.File
	irCache     map[string]*ir.Module
	symbolCache map[string][]SymbolInfo
}

// SymbolInfo caches a symbol's type and definition location.
type SymbolInfo struct {
	Name     string
	Kind     string // "function", "variable", "struct", "enum", "const", "param"
	Type     string // Human-readable type string
	Line     int    // 1-based
	Col      int    // 1-based
	File     string // File path
	Detail   string // Signature or extra info
}

// NewServer creates a new LSP server.
func NewServer() *Server {
	return &Server{
		documents:   make(map[string]string),
		astCache:    make(map[string]*ast.File),
		irCache:     make(map[string]*ir.Module),
		symbolCache: make(map[string][]SymbolInfo),
		logger:      log.New(os.Stderr, "[mzlsp] ", log.Ltime),
	}
}

// Run starts the LSP server on stdin/stdout.
func (s *Server) Run() error {
	return s.RunWithIO(os.Stdin, os.Stdout)
}

// RunWithIO starts the LSP server with custom I/O.
func (s *Server) RunWithIO(in io.Reader, out io.Writer) error {
	s.reader = bufio.NewReader(in)
	s.writer = out

	for {
		msg, err := s.readMessage()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("read message: %w", err)
		}

		if err := s.handleMessage(msg); err != nil {
			s.logger.Printf("handle error: %v", err)
		}
	}
}

// --- JSON-RPC Transport ---

func (s *Server) readMessage() (json.RawMessage, error) {
	var contentLength int
	for {
		line, err := s.reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			break
		}
		if strings.HasPrefix(line, "Content-Length:") {
			lengthStr := strings.TrimSpace(strings.TrimPrefix(line, "Content-Length:"))
			contentLength, err = strconv.Atoi(lengthStr)
			if err != nil {
				return nil, fmt.Errorf("invalid Content-Length: %s", lengthStr)
			}
		}
	}

	if contentLength == 0 {
		return nil, fmt.Errorf("missing Content-Length")
	}

	content := make([]byte, contentLength)
	if _, err := io.ReadFull(s.reader, content); err != nil {
		return nil, err
	}

	return content, nil
}

func (s *Server) sendMessage(msg interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	content, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(content))
	if _, err := s.writer.Write([]byte(header)); err != nil {
		return err
	}
	_, err = s.writer.Write(content)
	return err
}

func (s *Server) sendResponse(id interface{}, result interface{}) {
	s.sendMessage(Response{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	})
}

func (s *Server) sendError(id interface{}, code int, message string) {
	s.sendMessage(Response{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &Error{Code: code, Message: message},
	})
}

func (s *Server) sendNotification(method string, params interface{}) {
	s.sendMessage(Notification{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	})
}

// --- Message Dispatch ---

func (s *Server) handleMessage(raw json.RawMessage) error {
	var req Request
	if err := json.Unmarshal(raw, &req); err != nil {
		return fmt.Errorf("parse request: %w", err)
	}

	s.logger.Printf("← %s (id=%v)", req.Method, req.ID)

	switch req.Method {
	case "initialize":
		return s.handleInitialize(req)
	case "initialized":
		return nil // Client confirms initialization
	case "shutdown":
		s.sendResponse(req.ID, nil)
		return nil
	case "exit":
		os.Exit(0)
		return nil

	case "textDocument/didOpen":
		return s.handleDidOpen(raw)
	case "textDocument/didChange":
		return s.handleDidChange(raw)
	case "textDocument/didSave":
		return s.handleDidSave(raw)
	case "textDocument/didClose":
		return s.handleDidClose(raw)

	case "textDocument/hover":
		return s.handleHover(req)
	case "textDocument/definition":
		return s.handleDefinition(req)
	case "textDocument/completion":
		return s.handleCompletion(req)

	default:
		// Unknown method — ignore notifications, error for requests
		if req.ID != nil {
			s.sendError(req.ID, -32601, fmt.Sprintf("method not found: %s", req.Method))
		}
		return nil
	}
}

// --- Initialization ---

func (s *Server) handleInitialize(req Request) error {
	s.initialized = true

	result := InitializeResult{
		Capabilities: ServerCapabilities{
			TextDocumentSync:   1, // Full sync
			HoverProvider:      true,
			DefinitionProvider: true,
			CompletionProvider: &CompletionOptions{
				TriggerCharacters: []string{".", "@", ":"},
			},
		},
		ServerInfo: &ServerInfo{
			Name:    "mzlsp",
			Version: "0.1.0",
		},
	}

	s.sendResponse(req.ID, result)
	return nil
}

// --- Document Sync ---

func (s *Server) handleDidOpen(raw json.RawMessage) error {
	var msg struct {
		Params DidOpenTextDocumentParams `json:"params"`
	}
	if err := json.Unmarshal(raw, &msg); err != nil {
		return err
	}

	uri := msg.Params.TextDocument.URI
	s.documents[uri] = msg.Params.TextDocument.Text
	s.analyzeAndPublish(uri)
	return nil
}

func (s *Server) handleDidChange(raw json.RawMessage) error {
	var msg struct {
		Params DidChangeTextDocumentParams `json:"params"`
	}
	if err := json.Unmarshal(raw, &msg); err != nil {
		return err
	}

	uri := msg.Params.TextDocument.URI
	if len(msg.Params.ContentChanges) > 0 {
		// Full sync — last change event has the full text
		s.documents[uri] = msg.Params.ContentChanges[len(msg.Params.ContentChanges)-1].Text
	}
	s.analyzeAndPublish(uri)
	return nil
}

func (s *Server) handleDidSave(raw json.RawMessage) error {
	var msg struct {
		Params DidSaveTextDocumentParams `json:"params"`
	}
	if err := json.Unmarshal(raw, &msg); err != nil {
		return err
	}

	s.analyzeAndPublish(msg.Params.TextDocument.URI)
	return nil
}

func (s *Server) handleDidClose(raw json.RawMessage) error {
	var msg struct {
		Params DidCloseTextDocumentParams `json:"params"`
	}
	if err := json.Unmarshal(raw, &msg); err != nil {
		return err
	}

	uri := msg.Params.TextDocument.URI
	delete(s.documents, uri)
	delete(s.astCache, uri)
	delete(s.irCache, uri)
	delete(s.symbolCache, uri)

	// Clear diagnostics
	s.sendNotification("textDocument/publishDiagnostics", PublishDiagnosticsParams{
		URI:         uri,
		Diagnostics: []Diagnostic{},
	})
	return nil
}

// --- Analysis + Diagnostics ---

func (s *Server) analyzeAndPublish(uri string) {
	content, ok := s.documents[uri]
	if !ok {
		return
	}

	filePath := uriToPath(uri)
	diagnostics := []Diagnostic{}

	// Route .nanz files through the Nanz parser → HIR pipeline.
	if filepath.Ext(filePath) == ".nanz" {
		diagnostics = s.analyzeNanz(filePath, content)
		s.sendNotification("textDocument/publishDiagnostics", PublishDiagnosticsParams{
			URI:         uri,
			Diagnostics: diagnostics,
		})
		return
	}

	// Parse — write content to temp file since parser reads from disk
	tmpFile, tmpErr := os.CreateTemp("", "mzlsp-*.minz")
	if tmpErr != nil {
		return
	}
	tmpName := tmpFile.Name()
	tmpFile.Write([]byte(content))
	tmpFile.Close()
	defer os.Remove(tmpName)

	p := parser.New()
	astFile, err := p.ParseFile(tmpName)
	if astFile != nil {
		astFile.Name = filePath // Restore original filename
	}
	if err != nil {
		diags := extractDiagnostics(err.Error(), filePath)
		if len(diags) > 0 {
			diagnostics = append(diagnostics, diags...)
		} else {
			diagnostics = append(diagnostics, Diagnostic{
				Range:    Range{Start: Position{0, 0}, End: Position{0, 20}},
				Severity: DiagnosticSeverityError,
				Source:   "minz",
				Message:  err.Error(),
			})
		}
	}

	if astFile != nil {
		s.astCache[uri] = astFile

		// Semantic analysis
		analyzer := semantic.NewAnalyzer()
		irModule, err := analyzer.Analyze(astFile)
		if err != nil {
			diags := extractDiagnostics(err.Error(), filePath)
			if len(diags) > 0 {
				diagnostics = append(diagnostics, diags...)
			} else {
				diagnostics = append(diagnostics, Diagnostic{
					Range:    Range{Start: Position{0, 0}, End: Position{0, 20}},
					Severity: DiagnosticSeverityError,
					Source:   "minz",
					Message:  err.Error(),
				})
			}
		}
		analyzer.Close()

		if irModule != nil {
			s.irCache[uri] = irModule
			s.symbolCache[uri] = buildSymbolTable(astFile, irModule)
		}
	}

	s.sendNotification("textDocument/publishDiagnostics", PublishDiagnosticsParams{
		URI:         uri,
		Diagnostics: diagnostics,
	})
}

// analyzeNanz parses a Nanz file and returns LSP diagnostics.
func (s *Server) analyzeNanz(filePath, content string) []Diagnostic {
	var diagnostics []Diagnostic
	m, err := nanz.Parse(content, filepath.Base(filePath))
	if err != nil {
		// Nanz errors have the form "line N: message"
		diags := extractNanzDiagnostics(err.Error(), filePath)
		if len(diags) > 0 {
			diagnostics = append(diagnostics, diags...)
		} else {
			diagnostics = append(diagnostics, Diagnostic{
				Range:    Range{Start: Position{0, 0}, End: Position{0, 20}},
				Severity: DiagnosticSeverityError,
				Source:   "nanz",
				Message:  err.Error(),
			})
		}
	}
	// Surface use-before-init and other compile-time warnings.
	if m != nil {
		for _, w := range m.Warnings {
			diag := extractNanzWarningDiagnostic(w)
			diagnostics = append(diagnostics, diag)
		}
	}
	return diagnostics
}

// nanzWarnLineRe extracts "declared at line N" from a use-before-init warning.
var nanzWarnLineRe = regexp.MustCompile(`declared at line (\d+)`)

// extractNanzWarningDiagnostic converts a use-before-init warning string to an LSP Diagnostic.
func extractNanzWarningDiagnostic(w string) Diagnostic {
	line := 0
	if m := nanzWarnLineRe.FindStringSubmatch(w); m != nil {
		n, _ := strconv.Atoi(m[1])
		line = max0(n - 1)
	}
	return Diagnostic{
		Range:    Range{Start: Position{line, 0}, End: Position{line, 80}},
		Severity: DiagnosticSeverityWarning,
		Source:   "nanz",
		Message:  strings.TrimPrefix(w, "warning: "),
	}
}

// extractNanzDiagnostics parses Nanz error strings of the form "line N: message".
var nanzLineRe = regexp.MustCompile(`(?m)line\s+(\d+):\s*(.+)`)

func extractNanzDiagnostics(errStr, filePath string) []Diagnostic {
	var diags []Diagnostic
	for _, m := range nanzLineRe.FindAllStringSubmatch(errStr, -1) {
		lineNum, _ := strconv.Atoi(m[1])
		line := max0(lineNum - 1)
		diags = append(diags, Diagnostic{
			Range:    Range{Start: Position{line, 0}, End: Position{line, 80}},
			Severity: DiagnosticSeverityError,
			Source:   "nanz",
			Message:  strings.TrimSpace(m[2]),
		})
	}
	return diags
}

func max0(n int) int {
	if n < 0 {
		return 0
	}
	return n
}

// --- Hover ---

func (s *Server) handleHover(req Request) error {
	var params TextDocumentPositionParams
	raw, _ := json.Marshal(req.Params)
	if err := json.Unmarshal(raw, &params); err != nil {
		s.sendError(req.ID, -32602, "invalid params")
		return nil
	}

	uri := params.TextDocument.URI
	line := params.Position.Line + 1 // LSP is 0-based, our symbols are 1-based
	col := params.Position.Character + 1

	// Find word at position
	content := s.documents[uri]
	word := wordAtPosition(content, params.Position.Line, params.Position.Character)
	if word == "" {
		s.sendResponse(req.ID, nil)
		return nil
	}

	// Look up in symbol table
	symbols := s.symbolCache[uri]
	for _, sym := range symbols {
		if sym.Name == word || strings.HasSuffix(sym.Name, "."+word) {
			hover := Hover{
				Contents: MarkupContent{
					Kind:  "markdown",
					Value: formatSymbolHover(sym),
				},
			}
			s.sendResponse(req.ID, hover)
			return nil
		}
	}

	// Check built-in types
	if isBuiltinType(word) {
		hover := Hover{
			Contents: MarkupContent{
				Kind:  "markdown",
				Value: fmt.Sprintf("**%s** — built-in type", word),
			},
		}
		s.sendResponse(req.ID, hover)
		return nil
	}

	_ = line
	_ = col
	s.sendResponse(req.ID, nil)
	return nil
}

// --- Go to Definition ---

func (s *Server) handleDefinition(req Request) error {
	var params DefinitionParams
	raw, _ := json.Marshal(req.Params)
	if err := json.Unmarshal(raw, &params); err != nil {
		s.sendError(req.ID, -32602, "invalid params")
		return nil
	}

	uri := params.TextDocument.URI
	content := s.documents[uri]
	word := wordAtPosition(content, params.Position.Line, params.Position.Character)
	if word == "" {
		s.sendResponse(req.ID, nil)
		return nil
	}

	symbols := s.symbolCache[uri]
	for _, sym := range symbols {
		if sym.Name == word || strings.HasSuffix(sym.Name, "."+word) {
			if sym.Line > 0 {
				fileURI := uri
				if sym.File != "" {
					fileURI = pathToURI(sym.File)
				}
				loc := Location{
					URI: fileURI,
					Range: Range{
						Start: Position{Line: sym.Line - 1, Character: sym.Col - 1},
						End:   Position{Line: sym.Line - 1, Character: sym.Col - 1 + len(sym.Name)},
					},
				}
				s.sendResponse(req.ID, loc)
				return nil
			}
		}
	}

	s.sendResponse(req.ID, nil)
	return nil
}

// --- Completion ---

func (s *Server) handleCompletion(req Request) error {
	var params CompletionParams
	raw, _ := json.Marshal(req.Params)
	if err := json.Unmarshal(raw, &params); err != nil {
		s.sendError(req.ID, -32602, "invalid params")
		return nil
	}

	uri := params.TextDocument.URI
	items := []CompletionItem{}

	// Keywords
	keywords := []string{
		"fun", "fn", "let", "const", "mut", "if", "else", "while", "for", "in",
		"return", "struct", "enum", "import", "global", "asm", "match", "break",
		"continue", "loop", "pub", "type", "gen", "yield", "as", "true", "false",
	}
	for _, kw := range keywords {
		items = append(items, CompletionItem{
			Label: kw,
			Kind:  CompletionItemKindKeyword,
		})
	}

	// Types
	types := []string{"u8", "u16", "u24", "i8", "i16", "bool", "void"}
	for _, t := range types {
		items = append(items, CompletionItem{
			Label:  t,
			Kind:   CompletionItemKindKeyword,
			Detail: "type",
		})
	}

	// Metafunctions
	metas := []string{"@define", "@print", "@if", "@elif", "@else", "@error", "@emit", "@include", "@target"}
	for _, m := range metas {
		items = append(items, CompletionItem{
			Label:  m,
			Kind:   CompletionItemKindFunction,
			Detail: "metafunction",
		})
	}

	// Symbols from analysis
	symbols := s.symbolCache[uri]
	for _, sym := range symbols {
		kind := CompletionItemKindVariable
		switch sym.Kind {
		case "function":
			kind = CompletionItemKindFunction
		case "struct":
			kind = CompletionItemKindStruct
		case "enum":
			kind = CompletionItemKindEnum
		}
		items = append(items, CompletionItem{
			Label:  sym.Name,
			Kind:   kind,
			Detail: sym.Detail,
		})
	}

	// Iterator methods (when preceded by a dot)
	content := s.documents[uri]
	if isPrecedingDot(content, params.Position.Line, params.Position.Character) {
		iterMethods := []string{"iter", "map", "filter", "forEach", "take", "skip", "reduce", "enumerate", "collect"}
		for _, m := range iterMethods {
			items = append(items, CompletionItem{
				Label:  m,
				Kind:   CompletionItemKindFunction,
				Detail: "iterator method",
			})
		}
	}

	s.sendResponse(req.ID, CompletionList{
		IsIncomplete: false,
		Items:        items,
	})
	return nil
}

// --- Helpers ---

var errorLineRe = regexp.MustCompile(`(?m)([^:\s]+):(\d+):(\d+):\s*(error|warning):\s*(.+)`)

func extractDiagnostics(errMsg string, filePath string) []Diagnostic {
	var diags []Diagnostic
	matches := errorLineRe.FindAllStringSubmatch(errMsg, -1)
	for _, m := range matches {
		line, _ := strconv.Atoi(m[2])
		col, _ := strconv.Atoi(m[3])
		severity := DiagnosticSeverityError
		if m[4] == "warning" {
			severity = DiagnosticSeverityWarning
		}
		diags = append(diags, Diagnostic{
			Range: Range{
				Start: Position{Line: line - 1, Character: col - 1},
				End:   Position{Line: line - 1, Character: col - 1 + 20},
			},
			Severity: severity,
			Source:   "minz",
			Message:  m[5],
		})
	}
	return diags
}

func buildSymbolTable(astFile *ast.File, module *ir.Module) []SymbolInfo {
	var symbols []SymbolInfo

	for _, decl := range astFile.Declarations {
		switch d := decl.(type) {
		case *ast.FunctionDecl:
			sig := d.Name + "("
			for i, p := range d.Params {
				if i > 0 {
					sig += ", "
				}
				sig += p.Name + ": " + typeToString(p.Type)
			}
			sig += ")"
			if d.ReturnType != nil {
				sig += " -> " + typeToString(d.ReturnType)
			}
			symbols = append(symbols, SymbolInfo{
				Name:   d.Name,
				Kind:   "function",
				Type:   typeToString(d.ReturnType),
				Line:   d.Pos().Line,
				Col:    d.Pos().Column,
				File:   astFile.Name,
				Detail: sig,
			})
		case *ast.StructDecl:
			symbols = append(symbols, SymbolInfo{
				Name:   d.Name,
				Kind:   "struct",
				Type:   "struct",
				Line:   d.Pos().Line,
				Col:    d.Pos().Column,
				File:   astFile.Name,
				Detail: fmt.Sprintf("struct %s { %d fields }", d.Name, len(d.Fields)),
			})
		case *ast.EnumDecl:
			symbols = append(symbols, SymbolInfo{
				Name:   d.Name,
				Kind:   "enum",
				Type:   "enum",
				Line:   d.Pos().Line,
				Col:    d.Pos().Column,
				File:   astFile.Name,
				Detail: fmt.Sprintf("enum %s { %d variants }", d.Name, len(d.Variants)),
			})
		case *ast.VarDecl:
			kind := "variable"
			symbols = append(symbols, SymbolInfo{
				Name:   d.Name,
				Kind:   kind,
				Type:   typeToString(d.Type),
				Line:   d.Pos().Line,
				Col:    d.Pos().Column,
				File:   astFile.Name,
				Detail: fmt.Sprintf("%s: %s", d.Name, typeToString(d.Type)),
			})
		case *ast.ConstDecl:
			symbols = append(symbols, SymbolInfo{
				Name:   d.Name,
				Kind:   "const",
				Type:   typeToString(d.Type),
				Line:   d.Pos().Line,
				Col:    d.Pos().Column,
				File:   astFile.Name,
				Detail: fmt.Sprintf("const %s", d.Name),
			})
		}
	}

	return symbols
}

func typeToString(t ast.Type) string {
	if t == nil {
		return "void"
	}
	switch ty := t.(type) {
	case *ast.PrimitiveType:
		return ty.Name
	case *ast.ArrayType:
		return fmt.Sprintf("[%s; %d]", typeToString(ty.ElementType), ty.Size)
	case *ast.PointerType:
		return fmt.Sprintf("*%s", typeToString(ty.BaseType))
	case *ast.TypeIdentifier:
		return ty.Name
	default:
		return fmt.Sprintf("%T", t)
	}
}

func formatSymbolHover(sym SymbolInfo) string {
	switch sym.Kind {
	case "function":
		return fmt.Sprintf("```minz\n%s\n```", sym.Detail)
	case "struct":
		return fmt.Sprintf("```minz\n%s\n```", sym.Detail)
	case "enum":
		return fmt.Sprintf("```minz\n%s\n```", sym.Detail)
	case "variable", "global":
		return fmt.Sprintf("```minz\n%s\n```\n%s", sym.Detail, sym.Kind)
	case "const":
		return fmt.Sprintf("```minz\n%s\n```\nconstant", sym.Detail)
	default:
		return sym.Detail
	}
}

func wordAtPosition(content string, line, col int) string {
	lines := strings.Split(content, "\n")
	if line < 0 || line >= len(lines) {
		return ""
	}
	ln := lines[line]
	if col < 0 || col >= len(ln) {
		return ""
	}

	// Expand to word boundaries
	start := col
	for start > 0 && isWordChar(ln[start-1]) {
		start--
	}
	end := col
	for end < len(ln) && isWordChar(ln[end]) {
		end++
	}
	if start == end {
		return ""
	}
	return ln[start:end]
}

func isWordChar(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_'
}

func isPrecedingDot(content string, line, col int) bool {
	lines := strings.Split(content, "\n")
	if line < 0 || line >= len(lines) {
		return false
	}
	ln := lines[line]
	// Walk backward from col to find a dot
	for i := col - 1; i >= 0; i-- {
		if ln[i] == '.' {
			return true
		}
		if ln[i] != ' ' && ln[i] != '\t' {
			break
		}
	}
	return false
}

func isBuiltinType(word string) bool {
	switch word {
	case "u8", "u16", "u24", "i8", "i16", "i24", "bool", "void", "Error":
		return true
	}
	return false
}

func uriToPath(uri string) string {
	u, err := url.Parse(uri)
	if err != nil {
		return strings.TrimPrefix(uri, "file://")
	}
	return u.Path
}

func pathToURI(path string) string {
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}
	return "file://" + absPath
}
