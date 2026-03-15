// Package interop implements cross-language #import resolution.
//
// Any frontend source file can contain #import directives:
//
//	#import "math_helpers.pas"    // Pascal → HIR
//	#import "utils.c"            // C89 → HIR
//	#import "core.lizp"          // Lizp → HIR
//	#import "base.minz"          // Nanz → HIR
//	#import "defs.hir"           // HIR directly
//
// The resolver compiles each imported file via the appropriate frontend,
// then merges all resulting HIR modules into the caller's module.
//
// This layer sits ABOVE individual parsers — it is the foundation for
// multi-language interop and future Objective-C #import support.
package interop

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/minz/minzc/pkg/hir"
)

// importRe matches: #import "filename.ext"
// Works in C (#import), and can be used as // #import in other languages.
var importRe = regexp.MustCompile(`(?m)^\s*#\s*import\s+"([^"]+)"\s*$`)

// CompileFunc compiles a source file and returns an HIR module.
// The resolver uses this callback to dispatch to the right frontend.
type CompileFunc func(src, name string) (*hir.Module, error)

// Resolver resolves #import directives by compiling imported files.
type Resolver struct {
	// BaseDir is the directory of the importing source file.
	BaseDir string

	// Compilers maps file extensions to compile functions.
	// Example: {".c": c89.Compile, ".pas": pascal.Compile, ...}
	Compilers map[string]CompileFunc

	// already tracks resolved imports to prevent cycles.
	already map[string]bool
}

// ExtractImports scans source for #import directives and returns:
// - imports: list of filenames to import
// - cleaned: source with #import lines removed (safe for cc/v4 etc.)
func ExtractImports(src string) (imports []string, cleaned string) {
	matches := importRe.FindAllStringSubmatchIndex(src, -1)
	if len(matches) == 0 {
		return nil, src
	}

	var b strings.Builder
	prev := 0
	for _, m := range matches {
		b.WriteString(src[prev:m[0]])
		// Replace with blank line to preserve line numbers.
		b.WriteString("/* #import */")
		imports = append(imports, src[m[2]:m[3]])
		prev = m[1]
	}
	b.WriteString(src[prev:])
	return imports, b.String()
}

// Resolve processes #import directives in source, compiles imported files,
// and returns the cleaned source plus a list of imported HIR modules.
func (r *Resolver) Resolve(src string) (string, []*hir.Module, error) {
	if r.already == nil {
		r.already = make(map[string]bool)
	}

	imports, cleaned := ExtractImports(src)
	if len(imports) == 0 {
		return src, nil, nil
	}

	var modules []*hir.Module
	for _, imp := range imports {
		absPath := imp
		if !filepath.IsAbs(imp) {
			absPath = filepath.Join(r.BaseDir, imp)
		}

		// Cycle detection.
		absPath, _ = filepath.Abs(absPath)
		if r.already[absPath] {
			continue // already imported
		}
		r.already[absPath] = true

		ext := filepath.Ext(imp)
		compiler, ok := r.Compilers[ext]
		if !ok {
			return "", nil, fmt.Errorf("#import %q: unsupported extension %s (available: %s)",
				imp, ext, r.supportedExts())
		}

		data, err := os.ReadFile(absPath)
		if err != nil {
			return "", nil, fmt.Errorf("#import %q: %w", imp, err)
		}

		// Recursively resolve imports in the imported file.
		childResolver := &Resolver{
			BaseDir:   filepath.Dir(absPath),
			Compilers: r.Compilers,
			already:   r.already,
		}
		childSrc, childMods, err := childResolver.Resolve(string(data))
		if err != nil {
			return "", nil, fmt.Errorf("#import %q: %w", imp, err)
		}
		modules = append(modules, childMods...)

		mod, err := compiler(childSrc, filepath.Base(absPath))
		if err != nil {
			return "", nil, fmt.Errorf("#import %q: %w", imp, err)
		}
		modules = append(modules, mod)
	}

	return cleaned, modules, nil
}

// MergeInto merges functions, globals, structs, and asserts from imported
// modules into the target module. Duplicate function names are skipped.
func MergeInto(target *hir.Module, imports []*hir.Module) {
	seen := make(map[string]bool)
	for _, f := range target.Funcs {
		seen[f.Name] = true
	}
	for _, m := range imports {
		for _, f := range m.Funcs {
			if !seen[f.Name] {
				target.Funcs = append(target.Funcs, f)
				seen[f.Name] = true
			}
		}
		target.Globals = append(target.Globals, m.Globals...)
		target.Structs = append(target.Structs, m.Structs...)
		target.Strings = append(target.Strings, m.Strings...)
	}
}

func (r *Resolver) supportedExts() string {
	var exts []string
	for ext := range r.Compilers {
		exts = append(exts, ext)
	}
	return strings.Join(exts, ", ")
}
