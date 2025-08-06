package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type PackageInfo struct {
	Name       string
	Path       string
	Files      []string
	Imports    map[string]bool
	Functions  map[string]*FunctionInfo
	Types      map[string]bool
	Exported   map[string]bool
	References map[string]int // How many times each function is called
}

type FunctionInfo struct {
	Name       string
	Package    string
	Exported   bool
	Calls      []string // Functions this function calls
	CalledBy   []string // Functions that call this function
	LineCount  int
	HasBody    bool
}

var packages = make(map[string]*PackageInfo)
var allFunctions = make(map[string]*FunctionInfo)

func main() {
	fmt.Println("# MinZ Compiler Static Analysis Report")
	fmt.Println("\nAnalyzing Go codebase structure...\n")

	// Walk through all Go files
	err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip vendor and test directories for now
		if strings.Contains(path, "vendor/") || strings.Contains(path, ".git/") {
			return nil
		}

		if strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
			analyzeFile(path)
		}

		return nil
	})

	if err != nil {
		fmt.Printf("Error walking files: %v\n", err)
	}

	// Analyze call relationships
	analyzeCallGraph()

	// Generate reports
	fmt.Println("\n## Package Structure\n")
	printPackageStructure()

	fmt.Println("\n## Import Dependency Graph\n")
	printImportGraph()

	fmt.Println("\n## Dead Code Analysis\n")
	printDeadCode()

	fmt.Println("\n## Core Components\n")
	printCoreComponents()

	fmt.Println("\n## Call Graph Summary\n")
	printCallGraphSummary()
}

func analyzeFile(filename string) {
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		return
	}

	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filename, content, parser.ParseComments)
	if err != nil {
		return
	}

	// Get package info
	pkgName := node.Name.Name
	pkgPath := filepath.Dir(filename)
	
	if _, exists := packages[pkgPath]; !exists {
		packages[pkgPath] = &PackageInfo{
			Name:       pkgName,
			Path:       pkgPath,
			Files:      []string{},
			Imports:    make(map[string]bool),
			Functions:  make(map[string]*FunctionInfo),
			Types:      make(map[string]bool),
			Exported:   make(map[string]bool),
			References: make(map[string]int),
		}
	}

	pkg := packages[pkgPath]
	pkg.Files = append(pkg.Files, filename)

	// Analyze imports
	for _, imp := range node.Imports {
		impPath := strings.Trim(imp.Path.Value, "\"")
		pkg.Imports[impPath] = true
	}

	// Analyze declarations
	ast.Inspect(node, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncDecl:
			funcName := x.Name.Name
			fullName := fmt.Sprintf("%s.%s", pkgPath, funcName)
			
			funcInfo := &FunctionInfo{
				Name:      funcName,
				Package:   pkgPath,
				Exported:  ast.IsExported(funcName),
				Calls:     []string{},
				CalledBy:  []string{},
				HasBody:   x.Body != nil,
			}

			if x.Body != nil {
				funcInfo.LineCount = fset.Position(x.Body.End()).Line - fset.Position(x.Body.Pos()).Line
			}

			pkg.Functions[funcName] = funcInfo
			allFunctions[fullName] = funcInfo

			if ast.IsExported(funcName) {
				pkg.Exported[funcName] = true
			}

			// Analyze function calls within this function
			if x.Body != nil {
				ast.Inspect(x.Body, func(n ast.Node) bool {
					if call, ok := n.(*ast.CallExpr); ok {
						if ident, ok := call.Fun.(*ast.Ident); ok {
							funcInfo.Calls = append(funcInfo.Calls, ident.Name)
						} else if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
							funcInfo.Calls = append(funcInfo.Calls, sel.Sel.Name)
						}
					}
					return true
				})
			}

		case *ast.TypeSpec:
			typeName := x.Name.Name
			pkg.Types[typeName] = true
			if ast.IsExported(typeName) {
				pkg.Exported[typeName] = true
			}
		}
		return true
	})
}

func analyzeCallGraph() {
	// Build reverse call graph
	for fullName, fn := range allFunctions {
		for _, calledFunc := range fn.Calls {
			// Try to find the called function
			for otherName, otherFn := range allFunctions {
				if strings.HasSuffix(otherName, "."+calledFunc) {
					otherFn.CalledBy = append(otherFn.CalledBy, fullName)
					
					// Update reference count
					pkgPath := otherFn.Package
					if pkg, exists := packages[pkgPath]; exists {
						pkg.References[calledFunc]++
					}
				}
			}
		}
	}
}

func printPackageStructure() {
	var pkgPaths []string
	for path := range packages {
		pkgPaths = append(pkgPaths, path)
	}
	sort.Strings(pkgPaths)

	for _, path := range pkgPaths {
		pkg := packages[path]
		fmt.Printf("### %s (%s)\n", path, pkg.Name)
		fmt.Printf("- Files: %d\n", len(pkg.Files))
		fmt.Printf("- Functions: %d (Exported: %d)\n", len(pkg.Functions), countExported(pkg.Functions))
		fmt.Printf("- Types: %d\n", len(pkg.Types))
		fmt.Println()
	}
}

func printImportGraph() {
	fmt.Println("```mermaid")
	fmt.Println("graph TD")
	
	for path, pkg := range packages {
		pkgNode := strings.ReplaceAll(path, "/", "_")
		pkgNode = strings.ReplaceAll(pkgNode, ".", "_")
		
		for imp := range pkg.Imports {
			if strings.HasPrefix(imp, "github.com/minz/minzc") {
				impNode := strings.ReplaceAll(imp, "/", "_")
				impNode = strings.ReplaceAll(impNode, ".", "_")
				impNode = strings.ReplaceAll(impNode, "github_com_minz_minzc_", "")
				fmt.Printf("    %s --> %s\n", pkgNode, impNode)
			}
		}
	}
	
	fmt.Println("```")
}

func printDeadCode() {
	var deadFunctions []string
	var suspiciousFunctions []string

	for fullName, fn := range allFunctions {
		if fn.HasBody && len(fn.CalledBy) == 0 && !fn.Exported && fn.Name != "main" && fn.Name != "init" {
			deadFunctions = append(deadFunctions, fullName)
		} else if fn.HasBody && len(fn.CalledBy) == 1 && fn.LineCount < 5 && !fn.Exported {
			suspiciousFunctions = append(suspiciousFunctions, fullName)
		}
	}

	if len(deadFunctions) > 0 {
		fmt.Println("### Potentially Dead Functions")
		fmt.Println("These functions are never called and not exported:")
		for _, fn := range deadFunctions {
			fmt.Printf("- `%s`\n", fn)
		}
		fmt.Println()
	}

	if len(suspiciousFunctions) > 0 {
		fmt.Println("### Suspicious Functions")
		fmt.Println("These functions are called only once and are very small:")
		for _, fn := range suspiciousFunctions {
			fmt.Printf("- `%s` (%d lines)\n", fn, allFunctions[fn].LineCount)
		}
	}
}

func printCoreComponents() {
	// Identify core components by package importance
	corePackages := []string{
		"cmd/minzc",
		"pkg/parser",
		"pkg/ast", 
		"pkg/semantic",
		"pkg/ir",
		"pkg/optimizer",
		"pkg/codegen",
	}

	for _, pkgPath := range corePackages {
		if pkg, exists := packages[pkgPath]; exists {
			fmt.Printf("### %s\n", pkgPath)
			
			// List key exported functions
			var exportedFuncs []string
			for name, fn := range pkg.Functions {
				if fn.Exported {
					exportedFuncs = append(exportedFuncs, name)
				}
			}
			sort.Strings(exportedFuncs)
			
			if len(exportedFuncs) > 0 {
				fmt.Println("**Key Functions:**")
				for _, fn := range exportedFuncs {
					refs := pkg.References[fn]
					fmt.Printf("- `%s` (referenced %d times)\n", fn, refs)
				}
			}
			fmt.Println()
		}
	}
}

func printCallGraphSummary() {
	// Find most called functions
	type funcRef struct {
		name  string
		count int
	}
	
	var refs []funcRef
	for _, pkg := range packages {
		for fn, count := range pkg.References {
			refs = append(refs, funcRef{fn, count})
		}
	}
	
	sort.Slice(refs, func(i, j int) bool {
		return refs[i].count > refs[j].count
	})
	
	fmt.Println("### Most Referenced Functions")
	for i := 0; i < 10 && i < len(refs); i++ {
		fmt.Printf("- `%s`: %d references\n", refs[i].name, refs[i].count)
	}
}

func countExported(functions map[string]*FunctionInfo) int {
	count := 0
	for _, fn := range functions {
		if fn.Exported {
			count++
		}
	}
	return count
}