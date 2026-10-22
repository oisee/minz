package mir2

import (
	"github.com/minz/minzc/pkg/z80effects"
	"strings"
)

// Z80Codegen uses emitted leaf instructions to refine caller saves only for
// parameterless byte-returning helpers. Unknown forms retain the full ABI.
// Instrumented/runtime products keep the ordinary single emission path.
func Z80Codegen(m *Module, ar *AllocResult, opts ...Z80CodegenOptions) (string, error) {
	original, err := z80CodegenWithLeafWrites(m, ar, nil, opts...)
	if err != nil {
		return original, err
	}
	if len(opts) > 0 {
		o := opts[0]
		if o.Runtime != "" || o.RuntimeStackBytes != 0 || o.NativeMemoryAccesses != nil || o.NativeMemoryAccessOrigins != nil || o.NativeCallFrames != nil || o.NativeCallSpills != nil {
			return original, nil
		}
	}
	cache := map[string]map[string]bool{}
	for _, caller := range m.Funcs {
		for _, b := range caller.Blocks {
			for _, x := range b.Insts {
				if x.Op == OpCall && len(x.Args) == 0 && len(x.ExtraRets) == 0 {
					if f := m.FuncByName(x.Sym); f != nil {
						if writes, ok := emittedLeafByteWrites(f, original); ok {
							cache[f.Name] = writes
						}
					}
				}
			}
		}
	}
	if len(cache) == 0 {
		return original, nil
	}
	candidate, err := z80CodegenWithLeafWrites(m, ar, cache, opts...)
	if err != nil {
		return original, nil
	}
	// Removing caller saves must not change the leaf proof itself. Check the
	// final product as well, after all assembly rewrites, before accepting it.
	for name, before := range cache {
		after, ok := emittedLeafByteWrites(m.FuncByName(name), candidate)
		if !ok {
			return original, nil
		}
		for reg := range after {
			if !before[reg] {
				return original, nil
			}
		}
	}
	return candidate, nil
}

func emittedLeafByteWrites(f *Func, asm string) (map[string]bool, bool) {
	if f == nil || f.Attrs.IsExtern || f.Attrs.HasAsm || f.Attrs.FallibleCY || f.Attrs.IsRecursive || len(f.Contract.Params) != 0 || len(f.Contract.Returns) != 1 || f.Contract.Returns[0].Ty != TyU8 || canonicalReturnLoc(f.Contract.Returns[0].Class, TyU8) != "A" || len(f.Blocks) != 1 {
		return nil, false
	}
	if _, ok := f.Entry().Term.(*TermRet); !ok {
		return nil, false
	}
	for _, x := range f.Entry().Insts {
		if x.Op == OpAsm || x.Op == OpPatch || x.Op == OpPatchSlot || x.Op == OpLoadPatched || x.Op == OpCall || x.Op == OpCallIndirect || x.Op == OpCallCond || x.Volatile {
			return nil, false
		}
	}
	start := strings.Index(asm, "\n"+sanitizeIdent(f.Name)+":\n")
	if start < 0 {
		return nil, false
	}
	body := asm[start+len(sanitizeIdent(f.Name))+3:]
	names := []string{"A", "F", "B", "C", "D", "E", "H", "L", "IXH", "IXL", "IYH", "IYL"}
	var main z80effects.Registers
	for _, name := range names {
		main |= z80effects.Register(name)
	}
	var written z80effects.Registers
	for _, line := range strings.Split(body, "\n") {
		code, _, _ := strings.Cut(line, ";")
		fields := strings.Fields(code)
		if len(fields) == 0 {
			continue
		}
		switch fields[0] {
		case "LD", "INC", "DEC", "ADD", "SUB", "AND", "OR", "XOR", "CPL", "NEG", "CP", "RET":
		default:
			return nil, false
		}
		effect := z80effects.Instruction(code)
		if !effect.Known || effect.MemoryWrite || effect.PortRead || effect.PortWrite || effect.ReplacesSP || effect.INT != z80effects.INTUnchanged || effect.Writes & ^(main|z80effects.SP) != 0 {
			return nil, false
		}
		written |= effect.Writes & main
		if effect.Flow == z80effects.Return {
			if effect.Conditional {
				return nil, false
			}
			out := map[string]bool{}
			for _, name := range names {
				if written&z80effects.Register(name) != 0 {
					out[name] = true
				}
			}
			return out, true
		}
		if effect.Flow != z80effects.Fallthrough || effect.Writes&z80effects.SP != 0 {
			return nil, false
		}
	}
	return nil, false
}
