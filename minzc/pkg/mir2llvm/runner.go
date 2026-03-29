// runner.go — Run LLVM IR via llvmlite (Python subprocess) and verify asserts.
//
// Uses llvmlite MCJIT: parse IR → JIT compile → call function → check result.
// This gives us native-speed execution of LLVM IR without installing
// the full LLVM toolchain (just pip install llvmlite).
package mir2llvm

import (
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
)

// RunAsserts compiles to LLVM IR, JIT-executes via llvmlite, checks results.
func RunAsserts(hm *hir.Module, m *mir2.Module, force bool) error {
	ll, err := Compile(m)
	if err != nil {
		return fmt.Errorf("llvm compile: %w", err)
	}

	// Collect asserts to run
	var asserts []hir.Assert
	for _, a := range hm.Asserts {
		if force || a.Via == "" || a.Via == "llvm" {
			asserts = append(asserts, a)
		}
	}
	if len(asserts) == 0 {
		return nil
	}

	// Build Python script that JIT-compiles and runs all asserts
	var py strings.Builder
	py.WriteString("import llvmlite.binding as llvm\n")
	py.WriteString("import ctypes, sys\n")
	py.WriteString("llvm.initialize()\n")
	py.WriteString("llvm.initialize_native_target()\n")
	py.WriteString("llvm.initialize_native_asmprinter()\n")
	py.WriteString(fmt.Sprintf("ir = %q\n", ll))
	py.WriteString("mod = llvm.parse_assembly(ir)\n")
	py.WriteString("mod.verify()\n")
	py.WriteString("tm = llvm.Target.from_default_triple().create_target_machine()\n")
	py.WriteString("ee = llvm.create_mcjit_compiler(mod, tm)\n")
	py.WriteString("errors = 0\n")

	for _, a := range asserts {
		fnName := a.FuncName
		nArgs := len(a.Args)

		// Build ctypes function type
		argTypes := strings.Repeat("ctypes.c_int, ", nArgs)
		if nArgs > 0 {
			argTypes = argTypes[:len(argTypes)-2] // remove trailing ", "
		}

		py.WriteString(fmt.Sprintf("try:\n"))
		py.WriteString(fmt.Sprintf("  ptr_%s = ee.get_function_address('%s')\n", fnName, fnName))
		py.WriteString(fmt.Sprintf("  fn_%s = ctypes.CFUNCTYPE(ctypes.c_int, %s)(ptr_%s)\n",
			fnName, argTypes, fnName))

		// Call with args
		argVals := make([]string, nArgs)
		for i, v := range a.Args {
			argVals[i] = fmt.Sprintf("%d", v)
		}
		py.WriteString(fmt.Sprintf("  result = fn_%s(%s) & 0xFF\n",
			fnName, strings.Join(argVals, ", ")))
		py.WriteString(fmt.Sprintf("  if result != %d:\n", a.Expected))
		py.WriteString(fmt.Sprintf("    print('FAIL: %s [llvm]: got %%d, want %d' %% result)\n",
			escapePy(a.Source), a.Expected))
		py.WriteString(fmt.Sprintf("    errors += 1\n"))
		py.WriteString(fmt.Sprintf("  else:\n"))
		py.WriteString(fmt.Sprintf("    print('PASS: %s [llvm]')\n", escapePy(a.Source)))
		py.WriteString(fmt.Sprintf("except Exception as e:\n"))
		py.WriteString(fmt.Sprintf("  print('FAIL: %s [llvm]: %%s' %% e)\n", escapePy(a.Source)))
		py.WriteString(fmt.Sprintf("  errors += 1\n"))
	}

	py.WriteString("sys.exit(errors)\n")

	// Run Python script
	cmd := exec.Command("python3", "-c", py.String())
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	output := stdout.String()

	// Parse output for failures
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(line, "FAIL:") {
			if err == nil {
				err = fmt.Errorf("%s", line)
			}
		}
	}

	if err != nil {
		return fmt.Errorf("llvm asserts: %s\n%s", output, err)
	}

	return nil
}

// RunSingleFunc compiles and runs a single function with given args.
// Returns the result as int64.
func RunSingleFunc(m *mir2.Module, funcName string, args ...int64) (int64, error) {
	ll, err := Compile(m)
	if err != nil {
		return 0, fmt.Errorf("llvm compile: %w", err)
	}

	nArgs := len(args)
	argTypes := strings.Repeat("ctypes.c_int, ", nArgs)
	if nArgs > 0 {
		argTypes = argTypes[:len(argTypes)-2]
	}
	argVals := make([]string, nArgs)
	for i, v := range args {
		argVals[i] = fmt.Sprintf("%d", v)
	}

	py := fmt.Sprintf(`
import llvmlite.binding as llvm
import ctypes
llvm.initialize()
llvm.initialize_native_target()
llvm.initialize_native_asmprinter()
ir = %q
mod = llvm.parse_assembly(ir)
mod.verify()
tm = llvm.Target.from_default_triple().create_target_machine()
ee = llvm.create_mcjit_compiler(mod, tm)
ptr = ee.get_function_address('%s')
fn = ctypes.CFUNCTYPE(ctypes.c_int, %s)(ptr)
result = fn(%s) & 0xFF
print(result)
`, ll, funcName, argTypes, strings.Join(argVals, ", "))

	cmd := exec.Command("python3", "-c", py)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return 0, fmt.Errorf("llvm run: %w", err)
	}

	result, err := strconv.ParseInt(strings.TrimSpace(stdout.String()), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("llvm parse result: %w", err)
	}
	return result, nil
}

func escapePy(s string) string {
	s = strings.ReplaceAll(s, "'", "\\'")
	s = strings.ReplaceAll(s, "\\", "\\\\")
	return s
}
