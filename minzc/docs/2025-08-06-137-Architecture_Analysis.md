# MinZ Compiler Architecture Analysis

Generated: Wed  6 Aug 2025 13:19:00 IST

## 1. Project Structure Overview

### Directory Tree (Main Components)
```
./cmd
./cmd/backend-info
./cmd/minzc
./cmd/repl
./cmd/tokentest
./docs
./examples
./pkg
./pkg/ast
./pkg/codegen
./pkg/emulator
./pkg/interpreter
./pkg/ir
./pkg/meta
./pkg/metafunction
./pkg/mir
./pkg/module
./pkg/optimizer
./pkg/parser
./pkg/semantic
./pkg/tas
./pkg/testing
./pkg/z80asm
./pkg/z80asm/regression
./pkg/z80asm/regression/testdata
./pkg/z80asm/testdata
./pkg/z80testing
./scripts
./scripts/analysis
./tests
```

## 2. Go Package Structure

### Core Packages
```
pkg/ast - 3 files
pkg/codegen - 22 files
pkg/emulator - 3 files
pkg/interpreter - 2 files
pkg/ir - 1 files
pkg/meta - 2 files
pkg/metafunction - 1 files
pkg/mir - 2 files
pkg/module - 1 files
pkg/optimizer - 27 files
pkg/parser - 3 files
pkg/semantic - 13 files
pkg/tas - 11 files
pkg/testing - 1 files
pkg/z80asm - 11 files
pkg/z80asm/regression - 1 files
pkg/z80testing - 18 files
```

## 3. Import Dependency Analysis

### Internal Package Dependencies
```
```

## 4. Entry Points and Commands

### Main Entry Points
- **cmd/minzc/main.go**
  func main() {
  	if err := rootCmd.Execute(); err != nil {
  		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
- **cmd/tokentest/main.go**
  func main() {
  	if len(os.Args) < 2 {
  		fmt.Println("Usage: tokentest <file>")
- **cmd/repl/main.go**
  func main() {
  	repl := New()
  	repl.Run()
- **cmd/backend-info/main.go**
  func main() {
  	if len(os.Args) > 1 && os.Args[1] == "features" {
  		showFeatureMatrix()

## 5. Core Types and Interfaces

### IR Types (pkg/ir/ir.go)
```go
type Instruction struct {
type AsmBlock struct {
type Type interface {
type BasicType struct {
type PointerType struct {
type ArrayType struct {
type LambdaType struct {
type StructType struct {
type IteratorType struct {
type EnumType struct {
```

### AST Types (pkg/ast/ast.go)
```go
type Node interface {
type Position struct {
type File struct {
type Statement interface {
type Declaration interface {
type Expression interface {
type ImportStmt struct {
type FunctionDecl struct {
type Parameter struct {
type Type interface {
```

## 6. Compilation Pipeline

Based on code analysis, the compilation flow is:
```
1. Source (.minz) → Parser (tree-sitter) → AST
2. AST → Semantic Analysis → IR + Type Checking
3. IR → Optimization Passes → Optimized IR
4. Optimized IR → Code Generation → Target Output
```

## 7. Backend Support

### Available Backends (pkg/codegen/)
```
backend_toolkit_test.go
backend_toolkit.go
backend.go
base
c
example
gb
i8080
llvm
m6502
m68k
wasm
z80
```

## 8. Build System and Scripts

### Key Scripts
- **Makefile** targets:
all 
build 
repl 
test 
clean 
run 
run-repl 
deps 
benchmark 
perf-report 

### Shell Scripts
- **scripts/analysis/score_true_smc.py**
  !/usr/bin/env python3
- **scripts/analyze_example.py**
  !/usr/bin/env python3
- **scripts/build_release_v0.9.7.sh**
  !/bin/bash
- **scripts/comprehensive_analysis.sh**
  !/bin/bash
- **scripts/comprehensive_backend_test.sh**
  !/bin/bash
- **scripts/create_backend.sh**
  !/bin/bash
- **scripts/generate_corpus_tests.py**
  !/usr/bin/env python3
- **scripts/run_corpus_tests.sh**
  !/bin/bash
- **scripts/run_e2e_tests.sh**
  !/bin/bash
- **scripts/run_tsmc_benchmarks.sh**
  !/bin/bash
- **scripts/test_all_backends.sh**
  !/bin/bash

## 9. Test Infrastructure

### Test Files
```
Go test files: 18
MinZ test programs: 48
Example programs: 22
```

## 10. Potential Dead Code

### Unused Go Files (not imported by any other file)
```
pkg/interpreter/mir_interpreter.go
pkg/optimizer/peephole.go
pkg/optimizer/mir_reordering.go
pkg/optimizer/asm_reordering.go
pkg/optimizer/inc_dec_optimizer.go
pkg/optimizer/asm_passes.go
pkg/optimizer/register_allocation.go
pkg/optimizer/constant_folding.go
pkg/optimizer/recursion_detector.go
pkg/optimizer/multi_level.go
```

## 11. Function Statistics

### Exported Functions per Package
```
ast: 0 exported / 265 total
codegen: 30 exported / 349 total
emulator: 3 exported / 53 total
interpreter: 8 exported / 25 total
ir: 2 exported / 44 total
meta: 3 exported / 34 total
metafunction: 1 exported / 19 total
mir: 2 exported / 22 total
module: 3 exported / 11 total
optimizer: 48 exported / 318 total
parser: 2 exported / 189 total
semantic: 9 exported / 212 total
tas: 21 exported / 244 total
testing: 1 exported / 14 total
z80asm: 11 exported / 126 total
z80testing: 61 exported / 183 total
```

## 12. External Dependencies
```
	github.com/spf13/cobra v1.8.0
	github.com/yuin/gopher-lua v1.1.0
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/remogatto/z80 v0.0.0-20130613161616-82656d11c96b // indirect
	github.com/spf13/pflag v1.0.5 // indirect
```
