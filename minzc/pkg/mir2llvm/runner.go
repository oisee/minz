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
		if len(a.ExpectedMulti) > 1 {
			return fmt.Errorf("line %d: assert %q [llvm]: tuple returns are unsupported", a.Line, a.Source)
		}
		if len(a.ExpectedMulti) == 1 {
			a.Expected = a.ExpectedMulti[0]
		}
		fn := m.FuncByName(fnName)
		if fn == nil {
			return fmt.Errorf("line %d: assert %q [llvm]: function %q not found", a.Line, a.Source, fnName)
		}
		if len(fn.Contract.Returns) == 0 || fn.Contract.Returns[0].Ty == nil {
			return fmt.Errorf("line %d: assert %q [llvm]: missing return contract", a.Line, a.Source)
		}
		ty := mir2.BaseOf(fn.Contract.Returns[0].Ty)
		width := ty.Width()
		py.WriteString(fmt.Sprintf("  raw = fn_%s(%s)\n", fnName, strings.Join(argVals, ", ")))
		if width <= 0 || width > 32 {
			return fmt.Errorf("line %d: assert %q [llvm]: unsupported return width %d", a.Line, a.Source, width)
		}
		py.WriteString(fmt.Sprintf("  bits = raw & %d\n", (uint64(1)<<uint(width))-1))
		py.WriteString("  result = bits\n")
		if mir2.IsSigned(ty) {
			py.WriteString(fmt.Sprintf("  if bits & %d: result -= %d\n", uint64(1)<<uint(width-1), uint64(1)<<uint(width)))
			py.WriteString("  valid = raw == bits or raw == result\n")
		} else {
			// ctypes.c_int exposes the i32 transport as signed.
			if width == 32 {
				py.WriteString("  raw = raw & 0xffffffff\n")
			}
			py.WriteString("  valid = raw == bits\n")
		}
		py.WriteString(fmt.Sprintf("  if not valid or (%d != bits and %d != result):\n", a.Expected, a.Expected))
		py.WriteString("    display = str(result) if result == raw else '%d (raw %d)' % (result, raw)\n")
		py.WriteString(fmt.Sprintf("    print('FAIL: %s [llvm]: got %%s, want %d' %% display)\n", escapePy(a.Source), a.Expected))
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

	// Count only checks reported by the runner.
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(line, "PASS:") {
			hm.RecordAssert(nil)
		}
		if strings.HasPrefix(line, "FAIL:") {
			hm.RecordAssert(fmt.Errorf("%s", line))
		}
	}

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
