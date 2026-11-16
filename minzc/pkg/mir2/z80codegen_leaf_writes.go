package mir2

import (
	"slices"
	"strings"

	"github.com/minz/minzc/pkg/z80effects"
)

// z80CodegenEmit is the single emission used by Z80Codegen. Tests replace it
// to check that a second emission adding writes falls back to the original.
var z80CodegenEmit = z80CodegenWithLeafWrites

// Z80Codegen uses the emitted bodies of parameterless call-free callees to
// refine caller saves. It emits once with the conservative ABI, proves each
// callee's register writes from that assembly, emits again with the proven
// sets and re-proves every narrowed callee in the final product. Unknown forms
// retain the full ABI. Runtime products keep the ordinary single emission
// path.
//
// Native-annotated products (memory host) must assemble to the same bytes as
// the plain product. Their sinks record every emission, so the proof runs on
// plain emissions first and only the final emission is annotated, with the
// accepted sets; it is re-proved like any other final product.
func Z80Codegen(m *Module, ar *AllocResult, opts ...Z80CodegenOptions) (string, error) {
	var o Z80CodegenOptions
	if len(opts) > 0 {
		o = opts[0]
	}
	native := o.NativeMemoryAccesses != nil || o.NativeMemoryAccessOrigins != nil || o.NativeCallFrames != nil || o.NativeCallSpills != nil
	if o.Runtime != "" || o.RuntimeStackBytes != 0 || native && o.NoExactClobbers {
		return z80CodegenEmit(m, ar, nil, opts...)
	}
	if native {
		plain := o
		plain.NativeMemoryAccesses, plain.NativeMemoryAccessOrigins, plain.NativeCallFrames, plain.NativeCallSpills = nil, nil, nil, nil
		_, cache, err := exactCodegen(m, ar, plain)
		if err != nil || cache == nil {
			return z80CodegenEmit(m, ar, nil, o)
		}
		lens := nativeSinkLens(o)
		annotated, err := z80CodegenEmit(m, ar, cache, o)
		if err == nil && exactProductHolds(m, annotated, cache, emittedExactWrites) {
			return annotated, nil
		}
		truncateNativeSinks(o, lens)
		return z80CodegenEmit(m, ar, nil, o)
	}
	asm, _, err := exactCodegen(m, ar, o)
	return asm, err
}

// exactCodegen is the two-emission proof. It returns the accepted product and
// the clobber sets it was emitted with (nil when the original was kept).
func exactCodegen(m *Module, ar *AllocResult, o Z80CodegenOptions) (string, map[string]map[string]bool, error) {
	original, err := z80CodegenEmit(m, ar, nil, o)
	if err != nil {
		return original, nil, err
	}
	proof, legacy := emittedExactWrites, false
	if o.NoExactClobbers {
		proof, legacy = emittedLeafByteWrites, true
	}
	cache := map[string]map[string]bool{}
	for _, caller := range m.Funcs {
		for _, b := range caller.Blocks {
			for _, x := range b.Insts {
				// The #245 proof covers argument-free sites only; exact sets
				// also serve sites with arguments, whose copies are measured.
				if x.Op == OpCall && (len(x.Args) == 0 || !legacy) && len(x.ExtraRets) == 0 {
					if f := m.FuncByName(x.Sym); f != nil && (legacy || !o.FullABI[f.Name]) {
						if writes, ok := proof(f, original); ok {
							cache[f.Name] = writes
						}
					}
				}
			}
		}
	}
	if len(cache) == 0 {
		return original, nil, nil
	}
	candidate, err := z80CodegenEmit(m, ar, cache, o)
	if err != nil || !exactProductHolds(m, candidate, cache, proof) {
		return original, nil, nil
	}
	return candidate, cache, nil
}

// exactProductHolds re-checks a final product emitted with cache: removing
// caller saves must not change any proof itself, so every narrowed callee must
// still be provable and write nothing new, and each call site's argument
// copies, measured where they were emitted, are re-measured.
func exactProductHolds(m *Module, asm string, cache map[string]map[string]bool, proof func(*Func, string) (map[string]bool, bool)) bool {
	for name, before := range cache {
		after, ok := proof(m.FuncByName(name), asm)
		if !ok {
			return false
		}
		for reg := range after {
			if !before[reg] {
				return false
			}
		}
	}
	return verifyExactArgSites(asm)
}

func nativeSinkLens(o Z80CodegenOptions) [3]int {
	var n [3]int
	if o.NativeMemoryAccesses != nil {
		n[0] = len(*o.NativeMemoryAccesses)
	}
	if o.NativeCallFrames != nil {
		n[1] = len(*o.NativeCallFrames)
	}
	if o.NativeCallSpills != nil {
		n[2] = len(*o.NativeCallSpills)
	}
	return n
}

// truncateNativeSinks drops the records of a refused annotated emission.
func truncateNativeSinks(o Z80CodegenOptions, n [3]int) {
	if o.NativeMemoryAccesses != nil {
		*o.NativeMemoryAccesses = (*o.NativeMemoryAccesses)[:n[0]]
	}
	if o.NativeCallFrames != nil {
		*o.NativeCallFrames = (*o.NativeCallFrames)[:n[1]]
	}
	if o.NativeCallSpills != nil {
		*o.NativeCallSpills = (*o.NativeCallSpills)[:n[2]]
	}
}

// SpliceKeepsClobbers reports whether asm, a complete product in which f's
// body was replaced after Z80Codegen, still honours the clobber set f's header
// declares. Under exact clobbers that header is what callers were narrowed
// against; a header declaring every caller-save register constrains nothing.
// A spliced body that cannot be proved, or writes a register the header does
// not declare, must not be used.
func SpliceKeepsClobbers(f *Func, asm string) bool {
	if f == nil {
		return false
	}
	label := sanitizeIdent(f.Name)
	at := strings.Index(asm, "\n"+label+":\n")
	if at < 0 {
		return false
	}
	h := strings.LastIndex("\n"+asm[:at], "\n; fun ") // index into asm
	if h < 0 {
		return false
	}
	header, _, _ := strings.Cut(asm[h:], "\n")
	_, clause, ok := strings.Cut(header, " ; clobbers: ")
	if !ok {
		return false
	}
	var declared z80effects.Registers
	for _, name := range strings.Split(clause, ",") {
		declared |= z80effects.Register(name)
	}
	if declared&exactClobberMask() == exactClobberMask() {
		return true
	}
	writes, reason := exactWritesReason(f, asm)
	if reason != "" {
		return false
	}
	for name := range writes {
		if declared&z80effects.Register(name) == 0 {
			return false
		}
	}
	return true
}

// emittedLeafByteWrites is the #245 proof, kept for --no-exact-clobbers:
// one straight-line block of a few ALU/LD forms returning a byte in A.
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
	main := exactClobberMask()
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
			return writeNames(written), true
		}
		if effect.Flow != z80effects.Fallthrough || effect.Writes&z80effects.SP != 0 {
			return nil, false
		}
	}
	return nil, false
}

// exactClobberNames are the caller-save byte resources, in clobber-cache spelling.
var exactClobberNames = []string{"A", "F", "B", "C", "D", "E", "H", "L", "IXH", "IXL", "IYH", "IYL"}

func exactClobberMask() z80effects.Registers {
	var main z80effects.Registers
	for _, name := range exactClobberNames {
		main |= z80effects.Register(name)
	}
	return main
}

func writeNames(written z80effects.Registers) map[string]bool {
	out := map[string]bool{}
	for _, name := range exactClobberNames {
		if written&z80effects.Register(name) != 0 {
			out[name] = true
		}
	}
	return out
}

// clobberNames spells a proven write set like computeClobbers: sorted byte
// registers plus every pair whose halves are both written.
func clobberNames(w map[string]bool) []string {
	var names []string
	for name := range w {
		names = append(names, name)
	}
	for pair, halves := range map[string][2]string{"BC": {"B", "C"}, "DE": {"D", "E"}, "HL": {"H", "L"}, "IX": {"IXH", "IXL"}, "IY": {"IYH", "IYL"}} {
		if w[halves[0]] && w[halves[1]] {
			names = append(names, pair)
		}
	}
	sortStrings(names)
	return names
}

func emittedExactWrites(f *Func, asm string) (map[string]bool, bool) {
	writes, reason := exactWritesReason(f, asm)
	return writes, reason == ""
}

// exactWritesReason proves the registers a call to f can change, from f's
// emitted body. It returns the written byte registers, or a non-empty reason
// when no proof is possible and the caller must assume the full ABI.
//
// The MIR side keeps out everything whose emitted body does not show all of
// its effects: calls of any kind, inline asm and pinned parameters, SMC patch
// slots, fallible CY, volatile accesses and struct/wide results. Parameters
// are accepted: the caller's argument copies write registers too, and each
// call site measures those from its own emitted copies (genCall).
//
// Memory stores are accepted. A store can change a register only by
// rewriting code that runs later; the compiler emits such stores only for SMC
// (patch slots and TSMC spills, both rejected here) and everywhere else
// assumes that data stores do not hit code. PUSH writes the stack below the
// return address, and the depth check in bodyWrites makes every POP read a
// slot this body pushed.
func exactWritesReason(f *Func, asm string) (map[string]bool, string) {
	switch {
	case f == nil:
		return nil, "unknown callee"
	case f.Attrs.IsExtern || len(f.Blocks) == 0 || f.Attrs.ExternAddr != 0:
		return nil, "extern"
	case funcHasAsm(f):
		return nil, "asm"
	case f.Attrs.FallibleCY:
		return nil, "fallible"
	case f.Attrs.IsRecursive || isRecursive(f):
		return nil, "recursive"
	case slices.ContainsFunc(f.Contract.Params, func(p Param) bool { return p.ClassHard }):
		return nil, "pinned param"
	case len(f.Contract.Returns) > 1:
		return nil, "multi-return"
	case len(f.Contract.StructReturnBuffers) != 0:
		return nil, "struct-return"
	case len(f.Contract.Returns) == 1 && isZ80WideInt(f.Contract.Returns[0].Ty):
		return nil, "wide-return"
	}
	for _, b := range f.Blocks {
		for _, x := range b.Insts {
			switch {
			case x.Op == OpCall || x.Op == OpCallIndirect || x.Op == OpCallCond:
				return nil, "calls"
			case x.Op == OpPatch || x.Op == OpPatchSlot || x.Op == OpLoadPatched:
				return nil, "patch"
			case x.Op == OpAsm:
				return nil, "asm"
			case x.Volatile:
				return nil, "volatile"
			}
		}
	}
	return bodyWrites(sanitizeIdent(f.Name), asm)
}

type bodyInsn struct {
	op, operands, target string
	effect               z80effects.Effect
}

// emittedBody returns the instructions from label to the next top-level label
// or directive (local labels start with '.'), and the instruction index of
// each label in between.
func emittedBody(label, asm string) ([]bodyInsn, map[string]int, string) {
	start := strings.Index(asm, "\n"+label+":\n")
	if start < 0 {
		return nil, nil, "no emitted body"
	}
	insns, labels, reason := parseInsns(asm[start+len(label)+3:])
	if reason == "" {
		if _, own := labels[label]; !own {
			labels[label] = 0
		}
	}
	return insns, labels, reason
}

// parseInsns reads indented instructions and local labels up to the first
// top-level label or directive.
func parseInsns(text string) ([]bodyInsn, map[string]int, string) {
	var insns []bodyInsn
	labels := map[string]int{}
	for _, line := range strings.Split(text, "\n") {
		code := strings.TrimRight(strings.SplitN(line, ";", 2)[0], " \t\r")
		if strings.TrimSpace(code) == "" {
			continue
		}
		if code[0] != ' ' && code[0] != '\t' && code[0] != '.' {
			break // next top-level label or directive
		}
		code = strings.TrimSpace(code)
		if code[0] == '.' { // local label, possibly followed by an instruction
			name, rest, ok := strings.Cut(code, ":")
			if !ok || strings.ContainsAny(name, " \t") {
				return nil, nil, "unparsed line " + code
			}
			labels[name] = len(insns)
			if code = strings.TrimSpace(rest); code == "" {
				continue
			}
		}
		if strings.Contains(code, "_tsmc_") {
			return nil, nil, "TSMC patch"
		}
		mnemonic := strings.Fields(code)[0]
		op, operands := strings.ToUpper(mnemonic), strings.TrimSpace(code[len(mnemonic):])
		if op == "JRS" { // assembler pseudo-op for a short JR
			op = "JR"
		}
		in := bodyInsn{op: op, operands: operands, effect: z80effects.Instruction(op + " " + operands)}
		if op == "JP" || op == "JR" || op == "DJNZ" || op == "CALL" || op == "RST" {
			args := strings.Split(operands, ",")
			in.target = strings.TrimSpace(args[len(args)-1])
		}
		insns = append(insns, in)
	}
	return insns, labels, ""
}

// bodyWrites walks every path of an emitted body from its entry. Paths may
// branch only to the body's own labels and must end in RET at the entry stack
// depth; reached instructions must have fully known effects and must not
// touch ports, interrupt state, I/R, or SP other than by balanced PUSH/POP
// (EX (SP) only on a slot this body pushed), nor store into this body's code.
// Unreached lines (data after the last RET) are ignored. Writes to the
// alternate bank count as writes to the main registers they pair with, so a
// caller keeping a wide value in a pair and its shadow saves both.
func bodyWrites(label, asm string) (map[string]bool, string) {
	insns, labels, reason := emittedBody(label, asm)
	if reason != "" {
		return nil, reason
	}
	if len(insns) == 0 {
		return nil, "empty body"
	}
	written, reason := walkWrites(insns, labels, false)
	if reason != "" {
		return nil, reason
	}
	return writeNames(written), ""
}

// fragmentWrites measures straight-line emitted code that is entered at its
// first line and left by falling off its last one, such as a call's argument
// copies. The rules are bodyWrites' with two changes: a path must leave at
// the entry stack depth instead of returning, and SP must not be read at all
// (an SP-relative offset would see the caller saves pushed before the
// fragment, whose count the measurement decides).
func fragmentWrites(text string) (z80effects.Registers, string) {
	insns, labels, reason := parseInsns(text)
	if reason != "" {
		return 0, reason
	}
	if len(insns) == 0 {
		return 0, ""
	}
	return walkWrites(insns, labels, true)
}

func walkWrites(insns []bodyInsn, labels map[string]int, fragment bool) (z80effects.Registers, string) {
	main := exactClobberMask()
	alt := map[z80effects.Registers]z80effects.Registers{
		z80effects.AAlt: z80effects.A, z80effects.FAlt: z80effects.F, z80effects.BAlt: z80effects.B, z80effects.CAlt: z80effects.C,
		z80effects.DAlt: z80effects.D, z80effects.EAlt: z80effects.E, z80effects.HAlt: z80effects.H, z80effects.LAlt: z80effects.L,
	}
	var altAll z80effects.Registers
	for r := range alt {
		altAll |= r
	}
	depth := make([]int, len(insns))
	for i := range depth {
		depth[i] = -1
	}
	depth[0] = 0
	work := []int{0}
	visit := func(i, d int) string {
		if i >= len(insns) {
			if fragment && d == 0 {
				return ""
			}
			if fragment {
				return "leaves with pushed bytes"
			}
			return "falls off the body"
		}
		if depth[i] == -1 {
			depth[i] = d
			work = append(work, i)
		} else if depth[i] != d {
			return "unbalanced stack at a join"
		}
		return ""
	}
	var written z80effects.Registers
	for len(work) > 0 {
		i := work[len(work)-1]
		work = work[:len(work)-1]
		in, d := insns[i], depth[i]
		e := in.effect
		switch in.op {
		case "CALL", "RST":
			return 0, "calls " + in.target
		case "IN", "OUT", "INI", "INIR", "IND", "INDR", "OUTI", "OTIR", "OUTD", "OTDR":
			return 0, "port I/O"
		case "HALT", "DI", "EI", "IM", "RETI", "RETN":
			return 0, "interrupt state " + in.op
		}
		if !e.Known {
			return 0, "unknown " + in.op
		}
		if e.PortRead || e.PortWrite {
			return 0, "port I/O"
		}
		if e.INT != z80effects.INTUnchanged || e.ReadsINT || e.Flow == z80effects.Wait {
			return 0, "interrupt state " + in.op
		}
		if e.Writes&^(main|altAll|z80effects.SP) != 0 {
			return 0, "writes I/R " + in.op
		}
		if e.MemoryWrite && in.op != "PUSH" {
			// A store into this code is SMC (see exactWritesReason).
			if strings.Contains(in.operands, "$") {
				return 0, "store into code " + in.operands
			}
			for _, word := range strings.FieldsFunc(in.operands, func(c rune) bool { return strings.ContainsRune(" \t(),+-*", c) }) {
				if _, ok := labels[word]; ok {
					return 0, "store into code " + in.operands
				}
			}
		}
		next := d
		switch {
		case in.op == "PUSH":
			next = d + 2
		case in.op == "POP":
			if d < 2 {
				return 0, "POP below entry stack"
			}
			next = d - 2
		case in.op == "RET":
			if fragment {
				return 0, "returns"
			}
		case in.op == "EX" && strings.Contains(in.operands, "(SP)"):
			if d < 2 { // would swap the return address or a caller save
				return 0, "EX (SP) at entry stack"
			}
		default:
			// Reading SP (ADD HL, SP) is harmless in a body; any change is not.
			if e.Writes&z80effects.SP != 0 || e.ReplacesSP || e.StackDelta != 0 || (fragment && e.Reads&z80effects.SP != 0) {
				return 0, "SP access " + in.op
			}
		}
		written |= e.Writes & main
		for a, m := range alt {
			if e.Writes&a != 0 {
				written |= m
			}
		}
		var r string
		switch e.Flow {
		case z80effects.Return:
			if d != 0 {
				return 0, "RET with pushed bytes"
			}
			if e.Conditional {
				r = visit(i+1, d)
			}
		case z80effects.Jump, z80effects.ConditionalJump:
			t, ok := labels[in.target]
			if !ok {
				return 0, "jumps out to " + in.target
			}
			r = visit(t, next)
			if r == "" && e.Flow == z80effects.ConditionalJump {
				r = visit(i+1, next)
			}
		case z80effects.Fallthrough:
			r = visit(i+1, next)
		default:
			return 0, "flow " + in.op
		}
		if r != "" {
			return 0, r
		}
	}
	return written, ""
}
