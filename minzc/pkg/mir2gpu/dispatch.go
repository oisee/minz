// dispatch.go — Route detected GPU candidates to kernel generation.
//
// Takes DetectGPUCandidates output and generates GPU kernels for each candidate.
// Map candidates become embarrassingly parallel kernels.
// Reduce candidates use parallel map + tree reduction.
//
// This is the bridge between detection (detect.go) and codegen (codegen.go).
package mir2gpu

import (
	"fmt"
	"strings"

	"github.com/minz/minzc/pkg/mir2"
)

// GPUDispatchResult holds the generated GPU code for a candidate.
type GPUDispatchResult struct {
	Candidate   GPUCandidate
	Backend     Backend
	KernelCode  string // GPU kernel source
	HostCode    string // host dispatch code (for standalone testing)
	Error       error
}

// DispatchOptions controls GPU kernel generation.
type DispatchOptions struct {
	Backend Backend // target GPU backend
}

// DispatchAll generates GPU kernels for all candidates in a module.
func DispatchAll(m *mir2.Module, opts DispatchOptions) []GPUDispatchResult {
	candidates := DetectGPUCandidates(m)
	var results []GPUDispatchResult

	for _, c := range candidates {
		r := dispatchCandidate(m, c, opts)
		results = append(results, r)
	}
	return results
}

// dispatchCandidate generates GPU code for a single candidate.
func dispatchCandidate(m *mir2.Module, c GPUCandidate, opts DispatchOptions) GPUDispatchResult {
	result := GPUDispatchResult{
		Candidate: c,
		Backend:   opts.Backend,
	}

	switch c.Kind {
	case GPUMap:
		code, err := generateMapKernel(m, c, opts.Backend)
		result.KernelCode = code
		result.Error = err

	case GPUReduce:
		code, err := generateReduceKernel(m, c, opts.Backend)
		result.KernelCode = code
		result.Error = err

	case GPUFilter:
		// Filter = map (compute predicate) + compact (atomic counter)
		// Phase 1: just generate the map part
		code, err := generateMapKernel(m, c, opts.Backend)
		result.KernelCode = code
		result.Error = err

	default:
		result.Error = fmt.Errorf("unsupported GPU candidate kind: %s", c.Kind)
	}

	return result
}

// generateMapKernel compiles the body function as a GPU kernel.
// Each thread computes results[tid] = f(tid).
func generateMapKernel(m *mir2.Module, c GPUCandidate, backend Backend) (string, error) {
	if c.BodyFunc != "" {
		// Body is a named function — compile it directly
		return Compile(m, CompileOptions{Backend: backend, EntryFunc: c.BodyFunc})
	}

	// Body is inline — compile the containing function
	// The loop body already contains the computation
	return Compile(m, CompileOptions{Backend: backend})
}

// generateReduceKernel generates a two-phase kernel:
// Phase 1: Each thread computes partial[tid] = f(tid) (map phase)
// Phase 2: Tree reduction: partial[0] = sum(partial[0..N])
//
// For now, generates the map phase — host does sequential reduce.
// Future: warp-level tree reduction in a second kernel.
func generateReduceKernel(m *mir2.Module, c GPUCandidate, backend Backend) (string, error) {
	spec := specs[backend]

	var sb strings.Builder

	// Header
	sb.WriteString(spec.Header)
	sb.WriteString("\n")

	// If there's a body function, emit it as a device/inline function
	if c.BodyFunc != "" {
		f := m.FuncByName(c.BodyFunc)
		if f == nil {
			return "", fmt.Errorf("body function %s not found", c.BodyFunc)
		}

		// Emit as device function
		name := gpuSym(c.BodyFunc)
		switch backend {
		case CUDA:
			sb.WriteString(fmt.Sprintf("__device__ %s %s(%s x) {\n", spec.UintType, name, spec.UintType))
		case OpenCL:
			sb.WriteString(fmt.Sprintf("%s %s(%s x) {\n", spec.UintType, name, spec.UintType))
		case Metal:
			sb.WriteString(fmt.Sprintf("%s %s(%s x) {\n", spec.UintType, name, spec.UintType))
		default:
			sb.WriteString(fmt.Sprintf("%s %s(%s x) {\n", spec.UintType, name, spec.UintType))
		}

		// Emit body — device function uses 'x' param, not 'tid'
		emitDeviceBody(&sb, f, spec)
		sb.WriteString("}\n\n")
	}

	// Emit reduce kernel: map phase writes partial results, host reduces
	kernelName := fmt.Sprintf("reduce_%s", gpuSym(c.Func.Name))
	sb.WriteString(spec.KernelDecl(kernelName))
	sb.WriteString(" {\n")
	sb.WriteString("    " + spec.ThreadID + "\n")

	if c.BodyFunc != "" {
		sb.WriteString(fmt.Sprintf("    %s val = %s(tid);\n", spec.UintType, gpuSym(c.BodyFunc)))
	} else {
		// Inline body: for simple accumulations like sum(0..N), just use tid
		sb.WriteString(fmt.Sprintf("    %s val = tid;\n", spec.UintType))
	}
	sb.WriteString(fmt.Sprintf("    %s\n", spec.ResultWrite("val")))
	sb.WriteString("    return;\n")
	sb.WriteString("}\n")

	return sb.String(), nil
}

// emitDeviceBody emits a function body for a device/helper function.
// Unlike kernel bodies, device functions use parameter 'x' instead of 'tid'
// and return a value instead of writing to results[tid].
func emitDeviceBody(sb *strings.Builder, f *mir2.Func, spec backendSpec) {
	// Declare vregs
	vregs := make(map[mir2.Reg]bool)
	for _, b := range f.Blocks {
		for _, inst := range b.Insts {
			if inst.Dst != mir2.NoReg {
				vregs[inst.Dst] = true
			}
		}
		for _, p := range b.Params {
			vregs[p.Dst] = true
		}
	}

	// Map first param to 'x' (device function parameter)
	for i, cp := range f.Contract.Params {
		if i == 0 {
			sb.WriteString(fmt.Sprintf("    %s r%d = x;\n", spec.UintType, cp.Reg))
		} else {
			sb.WriteString(fmt.Sprintf("    %s r%d = 0;\n", spec.UintType, cp.Reg))
		}
		delete(vregs, cp.Reg)
	}
	for vreg := range vregs {
		sb.WriteString(fmt.Sprintf("    %s r%d = 0;\n", spec.UintType, vreg))
	}
	sb.WriteString("\n")

	// Emit blocks — use 'return rN;' instead of 'results[tid] = rN; return;'
	for bi, b := range f.Blocks {
		if bi > 0 && b.Label != "" {
			sb.WriteString(fmt.Sprintf("  %s:\n", gpuSym(b.Label)))
		}
		for _, inst := range b.Insts {
			emitUniversalInst(sb, inst, spec)
		}
		// Custom terminator: return value instead of writing to results
		if b.Term != nil {
			if ret, ok := b.Term.(*mir2.TermRet); ok && len(ret.Vals) > 0 {
				sb.WriteString(fmt.Sprintf("    return r%d;\n", ret.Vals[0]))
			} else {
				emitUniversalTerm(sb, f, b, spec)
			}
		}
	}
}

// FormatDispatchResults returns a human-readable summary.
func FormatDispatchResults(results []GPUDispatchResult) string {
	if len(results) == 0 {
		return "No GPU dispatches generated."
	}
	var sb strings.Builder
	for i, r := range results {
		status := "OK"
		if r.Error != nil {
			status = fmt.Sprintf("ERROR: %s", r.Error)
		}
		sb.WriteString(fmt.Sprintf("  [%d] %s.%s → %s kernel (%s), %d threads [%s]\n",
			i+1,
			r.Candidate.Func.Name,
			r.Candidate.LoopHeader.Label,
			r.Candidate.Kind,
			r.Backend,
			r.Candidate.NumThreads,
			status))
		if r.Error == nil {
			sb.WriteString(fmt.Sprintf("       %d bytes kernel source\n", len(r.KernelCode)))
		}
	}
	return sb.String()
}
