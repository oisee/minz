package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	
	"github.com/minz/minzc/pkg/codegen"
)

func main() {
	var (
		action      = flag.String("action", "", "Action: create, test, validate, doc")
		backendName = flag.String("backend", "", "Backend name")
		description = flag.String("desc", "", "Backend description (for create)")
		outputDir   = flag.String("out", ".", "Output directory")
	)
	
	flag.Parse()
	
	if *action == "" {
		fmt.Println("MinZ Backend Development Kit")
		fmt.Println()
		fmt.Println("Actions:")
		fmt.Println("  create   - Create a new backend scaffold")
		fmt.Println("  test     - Generate test suite for a backend")
		fmt.Println("  validate - Validate backend implementation")
		fmt.Println("  doc      - Generate documentation for a backend")
		fmt.Println()
		fmt.Println("Examples:")
		fmt.Println("  backend-devkit -action=create -backend=arm -desc=\"ARM processor\"")
		fmt.Println("  backend-devkit -action=test -backend=z80")
		fmt.Println("  backend-devkit -action=validate -backend=6502")
		fmt.Println("  backend-devkit -action=doc -backend=68000")
		os.Exit(1)
	}
	
	devkit := codegen.NewBackendDevKit()
	
	switch *action {
	case "create":
		if *backendName == "" || *description == "" {
			fmt.Println("Error: -backend and -desc required for create action")
			os.Exit(1)
		}
		createBackend(*backendName, *description, *outputDir, devkit)
		
	case "test":
		if *backendName == "" {
			fmt.Println("Error: -backend required for test action")
			os.Exit(1)
		}
		generateTests(*backendName, *outputDir, devkit)
		
	case "validate":
		if *backendName == "" {
			fmt.Println("Error: -backend required for validate action")
			os.Exit(1)
		}
		validateBackend(*backendName)
		
	case "doc":
		if *backendName == "" {
			fmt.Println("Error: -backend required for doc action")
			os.Exit(1)
		}
		generateDocs(*backendName, *outputDir, devkit)
		
	default:
		fmt.Printf("Unknown action: %s\n", *action)
		os.Exit(1)
	}
}

func createBackend(name, desc, outputDir string, devkit *codegen.BackendDevKit) {
	// Generate scaffold
	code := codegen.CreateBackendScaffold(name, desc)
	
	// Write backend file
	filename := filepath.Join(outputDir, fmt.Sprintf("%s_backend.go", name))
	err := ioutil.WriteFile(filename, []byte(code), 0644)
	if err != nil {
		fmt.Printf("Error writing backend file: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Printf("Created backend scaffold: %s\n", filename)
	
	// Generate initial test suite
	testCode := devkit.TestGen.GenerateTestSuite(name)
	testDir := filepath.Join(outputDir, "test", name)
	os.MkdirAll(testDir, 0755)
	
	testFile := filepath.Join(testDir, "test_suite.minz")
	err = ioutil.WriteFile(testFile, []byte(testCode), 0644)
	if err != nil {
		fmt.Printf("Error writing test file: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Printf("Created test suite: %s\n", testFile)
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("1. Implement the TODO sections in", filename)
	fmt.Println("2. Run: backend-devkit -action=validate -backend=" + name)
	fmt.Println("3. Test with: minzc", testFile, "-b", name)
}

func generateTests(backendName, outputDir string, devkit *codegen.BackendDevKit) {
	// Generate test suite
	testCode := devkit.TestGen.GenerateTestSuite(backendName)
	
	// Create test directory
	testDir := filepath.Join(outputDir, "test", backendName)
	os.MkdirAll(testDir, 0755)
	
	// Write test files
	testFile := filepath.Join(testDir, "test_suite.minz")
	err := ioutil.WriteFile(testFile, []byte(testCode), 0644)
	if err != nil {
		fmt.Printf("Error writing test file: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Printf("Generated test suite: %s\n", testFile)
	
	// TODO: Generate expected output patterns
	fmt.Println("\nTest categories:")
	fmt.Println("- Arithmetic operations")
	fmt.Println("- Control flow")
	fmt.Println("- Loops") 
	fmt.Println("- Memory operations")
	fmt.Println("- Function calls")
}

func validateBackend(backendName string) {
	// Get backend
	backend := getBackend(backendName)
	if backend == nil {
		fmt.Printf("Backend '%s' not found\n", backendName)
		os.Exit(1)
	}
	
	// Create validator
	devkit := codegen.NewBackendDevKit()
	
	// Get toolkit for the backend (we need to extract it somehow)
	// For now, create a dummy one
	toolkit := codegen.NewBackendToolkit()
	
	// Validate
	errors := devkit.Validator.ValidateBackend(backend, toolkit)
	
	if len(errors) == 0 {
		fmt.Printf("✅ Backend '%s' validation passed!\n", backendName)
	} else {
		fmt.Printf("❌ Backend '%s' validation failed:\n", backendName)
		for _, err := range errors {
			fmt.Printf("  - %s\n", err)
		}
	}
}

func generateDocs(backendName, outputDir string, devkit *codegen.BackendDevKit) {
	// Get backend
	backend := getBackend(backendName)
	if backend == nil {
		fmt.Printf("Backend '%s' not found\n", backendName)
		os.Exit(1)
	}
	
	// Create documentation directory
	docDir := filepath.Join(outputDir, "docs", "backends")
	os.MkdirAll(docDir, 0755)
	
	// Generate README
	// For now, use a dummy toolkit
	toolkit := codegen.NewBackendToolkit()
	readme := devkit.DocGen.GenerateREADME(backend, toolkit)
	
	readmeFile := filepath.Join(docDir, fmt.Sprintf("%s_backend.md", backendName))
	err := ioutil.WriteFile(readmeFile, []byte(readme), 0644)
	if err != nil {
		fmt.Printf("Error writing documentation: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Printf("Generated documentation: %s\n", readmeFile)
}

func getBackend(name string) codegen.Backend {
	// Create a backend based on name
	options := &codegen.BackendOptions{}
	
	switch name {
	case "z80":
		return codegen.NewZ80Backend(options)
	case "6502":
		return codegen.NewM6502Backend(options)
	case "68000", "68k":
		return codegen.NewM68kBackend(options)
	case "i8080":
		return codegen.NewI8080Backend(options)
	case "gb":
		return codegen.NewGBBackend(options)
	case "c":
		return codegen.NewCBackend(options)
	case "wasm":
		return codegen.NewWASMBackend(options)
	case "llvm":
		// LLVM backend uses init() registration
		return &codegen.LLVMBackend{}
	default:
		return nil
	}
}