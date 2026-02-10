package semantic

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	
	"github.com/minz/minzc/pkg/ast"
	"github.com/minz/minzc/pkg/parser"
)

// ModuleLoader handles loading and caching of modules
type ModuleLoader struct {
	// Base paths to search for modules
	searchPaths []string
	// Cache of loaded modules
	cache map[string]*LoadedModule
	// Parser for module files
	parser *parser.Parser
}

// LoadedModule represents a parsed and analyzed module
type LoadedModule struct {
	Name    string
	File    *ast.File
	Exports map[string]Symbol // Exported symbols
}

// NewModuleLoader creates a new module loader
func NewModuleLoader() *ModuleLoader {
	loader := &ModuleLoader{
		searchPaths: []string{
			"stdlib",
			"../stdlib",        // When running from minzc directory
			"../../stdlib",     // When running from minzc subdirectory
			"../../../stdlib",  // When running from deeper subdirectories
			".",
		},
		cache:  make(map[string]*LoadedModule),
		parser: parser.New(),
	}

	// Also check for MINZ_STDLIB environment variable
	if stdlibPath := os.Getenv("MINZ_STDLIB"); stdlibPath != "" {
		loader.searchPaths = append([]string{stdlibPath}, loader.searchPaths...)
	}

	// Try to find stdlib relative to the executable
	if execPath, err := os.Executable(); err == nil {
		execDir := filepath.Dir(execPath)
		loader.searchPaths = append(loader.searchPaths,
			filepath.Join(execDir, "stdlib"),
			filepath.Join(execDir, "../stdlib"),
			filepath.Join(execDir, "../../stdlib"),
		)
	}

	return loader
}

// LoadModule loads a module by its import path
func (ml *ModuleLoader) LoadModule(importPath string) (*LoadedModule, error) {
	// Check cache first
	if module, ok := ml.cache[importPath]; ok {
		return module, nil
	}
	
	// Convert module path to file path
	// e.g., "zx.screen" -> "zx/screen.minz"
	filePath := strings.ReplaceAll(importPath, ".", "/") + ".minz"
	
	
	// Search for the module file
	var fullPath string
	for _, searchPath := range ml.searchPaths {
		candidate := filepath.Join(searchPath, filePath)
		if _, err := os.Stat(candidate); err == nil {
			fullPath = candidate
			break
		}
	}
	
	if fullPath == "" {
		return nil, fmt.Errorf("module not found: %s", importPath)
	}
	
	// Parse the module file
	moduleFile, err := ml.parser.ParseFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse module %s: %w", importPath, err)
	}
	
	// Create loaded module
	module := &LoadedModule{
		Name:    importPath,
		File:    moduleFile,
		Exports: make(map[string]Symbol),
	}
	
	// Extract exports (this will be done by the analyzer)
	// For now, we just cache the parsed file
	ml.cache[importPath] = module
	
	return module, nil
}

// AddSearchPath adds a directory to search for modules
func (ml *ModuleLoader) AddSearchPath(path string) {
	ml.searchPaths = append(ml.searchPaths, path)
}