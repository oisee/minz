package hir

import (
	"fmt"
	"github.com/minz/minzc/pkg/mir2"
	"reflect"
)

// ProfileWorkloadReachableFunctions conservatively includes calls in every block,
// including constant-false branches. Controlled workloads have no host authority.
func ProfileWorkloadReachableFunctions(m *mir2.Module, w ProfileWorkload) (map[string]bool, error) {
	mw := MemoryHostWorkload{}
	for _, c := range w.Calls {
		mw.Calls = append(mw.Calls, MemoryHostInvocation{Function: c.Function})
	}
	return MemoryHostReachableFunctions(m, mw)
}

// ValidateProfileWorkloadMIR preserves source obligations even if an optimizer
// erases a call. Dead externs remain declarations, never runtime implementations.
func ValidateProfileWorkloadMIR(source *Module, candidate *mir2.Module, w ProfileWorkload) error {
	if source == nil || candidate == nil {
		return fmt.Errorf("loop profile: missing module")
	}
	if err := Verify(source); err != nil {
		return err
	}
	raw := LowerModule(source)
	// Final allocation may reuse virtual register IDs; this is admission, not
	// an SSA verifier. The compilation pipeline verifies its earlier SSA stages.
	sr, err := ProfileWorkloadReachableFunctions(raw, w)
	if err != nil {
		return err
	}
	cr, err := ProfileWorkloadReachableFunctions(candidate, w)
	if err != nil {
		return err
	}
	for _, m := range []*mir2.Module{raw, candidate} {
		names := map[string]bool{}
		for _, f := range m.Funcs {
			if f == nil || f.Name == "" || names[f.Name] {
				return fmt.Errorf("loop profile: duplicate/invalid function")
			}
			names[f.Name] = true
			if f.Attrs.IsExtern {
				if sr[f.Name] || cr[f.Name] {
					return fmt.Errorf("loop profile: reachable extern unsupported: %s", f.Name)
				}
				baseline := raw.FuncByName(f.Name)
				if baseline == nil || !baseline.Attrs.IsExtern || len(f.Blocks) != 0 || !memoryHostExternIdentityEqual(baseline, f) {
					return fmt.Errorf("loop profile: extern identity mismatch: %s", f.Name)
				}
			} else if baseline := raw.FuncByName(f.Name); baseline != nil && baseline.Attrs.IsExtern {
				return fmt.Errorf("loop profile: extern body mutation: %s", f.Name)
			}
			deadRuntime := !sr[f.Name] && !cr[f.Name] && ProfileDeadPascalRuntime(source, f)
			if f.Attrs.HasAsm && !deadRuntime {
				return fmt.Errorf("loop profile: ASM unsupported")
			}
			for _, b := range f.Blocks {
				for _, i := range b.Insts {
					switch i.Op {
					case mir2.OpAsm:
						if !deadRuntime {
							return fmt.Errorf("loop profile: noncanonical/reachable ASM unsupported")
						}
						continue
					case mir2.OpIn8, mir2.OpIn16, mir2.OpOut8, mir2.OpOut16, mir2.OpPatchSlot, mir2.OpLoadPatched, mir2.OpPatch, mir2.OpCallIndirect:
						return fmt.Errorf("loop profile: ASM/ports/SMC/indirect unsupported")
					case mir2.OpAddrOf:
						if raw.FuncByName(i.Sym) != nil || candidate.FuncByName(i.Sym) != nil {
							return fmt.Errorf("loop profile: function address escape unsupported")
						}
					case mir2.OpCall:
						if mir2.IsByteLoopIntrinsic(f, i) {
							continue
						}
						if m.FuncByName(i.Sym) == nil {
							// Known output intrinsics can remain in uncalled source
							// functions (e.g. a demo main). The closure above still
							// refuses them if reachable; no runtime host is installed.
							if !sr[f.Name] && !cr[f.Name] && profileKnownOutputIntrinsic(i.Sym) {
								continue
							}
							return fmt.Errorf("loop profile: undefined call unsupported: %s", i.Sym)
						}
					}
				}
			}
		}
	}
	if !profileCompiledGlobalsEqual(source, raw.Globals, candidate.Globals) {
		return fmt.Errorf("loop profile: compiled global metadata differs from source")
	}
	for _, f := range source.Funcs {
		for _, p := range f.Params {
			if p.SMC {
				return fmt.Errorf("loop profile: SMC parameter unsupported")
			}
		}
	}
	return nil
}

func profileKnownOutputIntrinsic(name string) bool {
	switch name {
	case "@mir.io.print.str", "@mir.io.print.sstr", "@mir.io.print.lstr", "@mir.io.print.char", "@mir.io.print.nl", "@mir.io.print.u8", "@mir.io.print.dec":
		return true
	}
	return false
}

// Strip only privately constructed collector events from a validation view.
// User declarations, even with collector-like names, have no such authority.
func validateProfileWorkloadInstrumentation(source *Module, p *LoopProfiler, w ProfileWorkload) error {
	view := *p.VM.Module
	view.Funcs = nil
	seen := map[string]bool{}
	for _, f := range p.VM.Module.Funcs {
		if _, ok := p.profileHooks[f.Name]; ok {
			globalCollision := false
			for _, g := range source.Globals {
				globalCollision = globalCollision || g.Name == f.Name
			}
			params := f.Contract.Params
			validParams := len(params) == 0 || len(params) == 2 && params[0].Ty == mir2.TyPtr && params[1].Ty == mir2.TyU8
			if globalCollision || source.FuncByName(f.Name) != nil || !f.Attrs.IsExtern || f.Attrs.HasAsm || len(f.Blocks) != 0 || !validParams || len(f.Contract.Returns) != 0 || f.Attrs.ExternAddr != 0 || f.Contract.ExternClobbers != nil {
				return fmt.Errorf("loop profile: invalid private collector declaration")
			}
			seen[f.Name] = true
			continue
		}
		ff := *f
		ff.Blocks = nil
		for _, b := range f.Blocks {
			bb := *b
			bb.Insts = nil
			for _, i := range b.Insts {
				if _, ok := p.profileHooks[i.Sym]; ok {
					hook := p.VM.Module.FuncByName(i.Sym)
					if i.Op != mir2.OpCall || len(i.Args) != len(hook.Contract.Params) || i.Src != [2]mir2.Reg{} || i.Dst != mir2.NoReg || i.Ty != mir2.TyVoid {
						return fmt.Errorf("loop profile: invalid private collector call")
					}
					continue
				}
				bb.Insts = append(bb.Insts, i)
			}
			ff.Blocks = append(ff.Blocks, &bb)
		}
		view.Funcs = append(view.Funcs, &ff)
	}
	if len(seen) != len(p.profileHooks) {
		return fmt.Errorf("loop profile: private collector set mismatch")
	}
	return ValidateProfileWorkloadMIR(source, &view, w)
}

func installProfileDeniedExternHooks(vm *mir2.VM, source *Module) {
	for _, f := range source.Funcs {
		if f.IsExtern {
			name := f.Name
			deny := func([]mir2.Value) ([]mir2.Value, error) {
				return nil, fmt.Errorf("loop profile: executed unbound extern %q", name)
			}
			vm.Hosts[name] = deny
			vm.Hosts["@"+name] = deny
		}
	}
}

// Source globals stay byte-for-byte identical. The sole permitted extension is
// the complete read-only LUT output independently regenerated from source MIR.
// Names or IsConst alone never grant a compiler-created global authority.
func profileCompiledGlobalsEqual(source *Module, baseline, candidate []mir2.Global) bool {
	if reflect.DeepEqual(baseline, candidate) {
		return true
	}
	if len(candidate) <= len(baseline) || len(baseline) > 0 && !reflect.DeepEqual(baseline, candidate[:len(baseline)]) {
		return false
	}
	regenerated := LowerModule(source)
	if !mir2.LUTGen(regenerated) {
		return false
	}
	return reflect.DeepEqual(regenerated.Globals, candidate)
}
