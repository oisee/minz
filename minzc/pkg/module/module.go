package module

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/minz/minzc/pkg/ast"
	"github.com/minz/minzc/pkg/semantic"
)

// Module represents a MinZ module
type Module struct {
	Name        string              // Fully qualified module name (e.g., "graphics.sprite")
	Path        string              // File path
	AST         *ast.File           // Parsed AST
	Imports     []*Import           // Import declarations
	Exports     map[string]Export   // Exported symbols
	Scope       *semantic.Scope     // Module-level scope
	IsCompiled  bool                // Whether module has been compiled
	ObjectFile  string              // Path to compiled object file
}

// Import represents an import declaration
type Import struct {
	Path  string // Module path (e.g., "std.io")
	Alias string // Import alias (empty if no alias)
}

// Export represents an exported symbol
type Export struct {
	Name      string
	Kind      ExportKind
	IsPublic  bool
	Symbol    semantic.Symbol
}

// ExportKind represents the kind of export
type ExportKind int

const (
	ExportFunction ExportKind = iota
	ExportType
	ExportVariable
	ExportConstant
)

// ModuleResolver resolves module imports
type ModuleResolver struct {
	searchPaths []string                // Directories to search for modules
	modules     map[string]*Module      // Loaded modules by name
	stdlibPath  string                  // Path to standard library
}

// NewModuleResolver creates a new module resolver
func NewModuleResolver(projectRoot string) *ModuleResolver {
	return &ModuleResolver{
		searchPaths: []string{
			projectRoot,
			filepath.Join(projectRoot, "src"),
			// TODO: Add standard library path
		},
		modules: make(map[string]*Module),
	}
}

// ResolveImport resolves an import path to a module
func (r *ModuleResolver) ResolveImport(importPath string, currentFile string) (*Module, error) {
	// Check if module is already loaded
	if mod, exists := r.modules[importPath]; exists {
		return mod, nil
	}

	// Convert module path to file path
	filePath := r.findModuleFile(importPath, currentFile)
	if filePath == "" {
		return nil, fmt.Errorf("module not found: %s", importPath)
	}

	// Create new module
	mod := &Module{
		Name:    importPath,
		Path:    filePath,
		Exports: make(map[string]Export),
	}

	r.modules[importPath] = mod
	return mod, nil
}

// findModuleFile finds the file path for a module
func (r *ModuleResolver) findModuleFile(modulePath string, currentFile string) string {
	// Convert module path to file path (e.g., "graphics.sprite" -> "graphics/sprite.minz")
	relativePath := strings.ReplaceAll(modulePath, ".", "/") + ".minz"

	// Try relative to current file first
	if currentFile != "" {
		dir := filepath.Dir(currentFile)
		candidate := filepath.Join(dir, relativePath)
		if fileExists(candidate) {
			return candidate
		}
	}

	// Try search paths
	for _, searchPath := range r.searchPaths {
		candidate := filepath.Join(searchPath, relativePath)
		if fileExists(candidate) {
			return candidate
		}
	}

	return ""
}

// LoadModule loads and parses a module
func (r *ModuleResolver) LoadModule(mod *Module) error {
	// This will be implemented to use the parser
	// For now, return a placeholder error
	return fmt.Errorf("module loading not yet implemented")
}

// GetModule returns a loaded module by name
func (r *ModuleResolver) GetModule(name string) *Module {
	return r.modules[name]
}

// GetAllModules returns all loaded modules
func (r *ModuleResolver) GetAllModules() []*Module {
	modules := make([]*Module, 0, len(r.modules))
	for _, mod := range r.modules {
		modules = append(modules, mod)
	}
	return modules
}

// fileExists checks if a file exists
func fileExists(path string) bool {
	// TODO: Implement actual file check
	return false
}

// ModuleManager manages the compilation of modules
type ModuleManager struct {
	resolver     *ModuleResolver
	mainModule   *Module
	dependencies map[string][]string // Module dependency graph
}

// NewModuleManager creates a new module manager
func NewModuleManager(projectRoot string) *ModuleManager {
	return &ModuleManager{
		resolver:     NewModuleResolver(projectRoot),
		dependencies: make(map[string][]string),
	}
}

// CompileModule compiles a module and its dependencies
func (m *ModuleManager) CompileModule(modulePath string) error {
	// Load the module
	mod, err := m.resolver.ResolveImport(modulePath, "")
	if err != nil {
		return err
	}

	// Compile dependencies first
	for _, imp := range mod.Imports {
		if err := m.CompileModule(imp.Path); err != nil {
			return err
		}
	}

	// Compile this module
	if !mod.IsCompiled {
		// TODO: Implement actual compilation
		mod.IsCompiled = true
	}

	return nil
}

// CheckCircularDependencies checks for circular module dependencies
func (m *ModuleManager) CheckCircularDependencies() error {
	// TODO: Implement cycle detection
	return nil
}

// ExtractModuleName extracts module name from file path
func ExtractModuleName(filePath string) string {
	// Remove extension
	name := strings.TrimSuffix(filepath.Base(filePath), ".minz")
	
	// Convert path separators to dots
	dir := filepath.Dir(filePath)
	if dir != "." && dir != "" {
		parts := strings.Split(dir, string(filepath.Separator))
		// Remove common prefixes like "src"
		if len(parts) > 0 && parts[0] == "src" {
			parts = parts[1:]
		}
		if len(parts) > 0 {
			return strings.Join(parts, ".") + "." + name
		}
	}
	
	return name
}