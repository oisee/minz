// Package qbe2mir2 parses QBE IL source text into a *mir2.Module.
//
// It is the reverse of pkg/mir2qbe — primarily for round-trip testing and
// for importing QBE IL from external compilers (cproc, hare, ...) into the
// MIR2 → Z80 pipeline.
//
// # Phi nodes → block params
//
// QBE uses SSA phi nodes; MIR2 uses Cranelift-style block parameters.
// We convert during parsing:
//
//	QBE:  @loop  %v =w phi @entry %a, @body %b
//	MIR2: block @loop(v: u16); @entry's jmp passes a; @body's jmp passes b
//
// # Type mapping
//
//	QBE w (32-bit word) → TyU16  (closest Z80 integer; precision is lost)
//	QBE l (64-bit long) → TyPtr  (pointer / address)
//	QBE b / h           → TyU8 / TyU16  (store/load width hints)
//	QBE s / d (floats)  → unsupported; treated as TyU16
package qbe2mir2

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/minz/minzc/pkg/mir2"
)

// Parse parses QBE IL and returns a *mir2.Module.
func Parse(src string) (*mir2.Module, error) {
	p := &parser{
		lines: strings.Split(strings.ReplaceAll(src, "\r\n", "\n"), "\n"),
		m:     &mir2.Module{},
	}
	if err := p.module(); err != nil {
		return nil, err
	}
	return p.m, nil
}

// ── parser ────────────────────────────────────────────────────────────────────

type parser struct {
	lines []string
	pos   int
	m     *mir2.Module
}

// peek returns the current non-blank non-comment line without advancing.
func (p *parser) peek() string {
	for i := p.pos; i < len(p.lines); i++ {
		l := strings.TrimSpace(p.lines[i])
		if l != "" && !strings.HasPrefix(l, "#") {
			return l
		}
	}
	return ""
}

// next returns the current non-blank non-comment line and advances.
func (p *parser) next() string {
	for p.pos < len(p.lines) {
		l := strings.TrimSpace(p.lines[p.pos])
		p.pos++
		if l != "" && !strings.HasPrefix(l, "#") {
			return l
		}
	}
	return ""
}

// ── module ────────────────────────────────────────────────────────────────────

func (p *parser) module() error {
	for p.peek() != "" {
		l := p.peek()
		switch {
		case strings.HasPrefix(l, "data "):
			if err := p.parseData(); err != nil {
				return err
			}
		case strings.HasPrefix(l, "export function "), strings.HasPrefix(l, "function "):
			if err := p.parseFunction(); err != nil {
				return err
			}
		default:
			p.next()
		}
	}
	return nil
}

// ── data ──────────────────────────────────────────────────────────────────────

// parseData handles:  data $name = { b 0, b 1, ... }  or  { z N }
func (p *parser) parseData() error {
	l := p.next()
	i := strings.Index(l, "$")
	if i < 0 {
		return fmt.Errorf("data: missing $name: %q", l)
	}
	rest := l[i+1:]
	j := strings.IndexAny(rest, " \t=")
	if j < 0 {
		return fmt.Errorf("data: malformed: %q", l)
	}
	name := rest[:j]
	open := strings.Index(rest, "{")
	if open < 0 {
		return fmt.Errorf("data: missing '{': %q", l)
	}
	body := rest[open+1:]
	if c := strings.Index(body, "}"); c >= 0 {
		body = body[:c]
	}
	body = strings.TrimSpace(body)

	if strings.HasPrefix(body, "z ") {
		n, _ := strconv.Atoi(strings.TrimSpace(body[2:]))
		p.m.Globals = append(p.m.Globals, mir2.Global{
			Name: name, Ty: mir2.NewArray(mir2.TyU8, n),
		})
		return nil
	}

	var init []byte
	for _, part := range strings.Split(body, ",") {
		fields := strings.Fields(strings.TrimSpace(part))
		if len(fields) == 2 {
			v, _ := strconv.ParseInt(fields[1], 10, 64)
			init = append(init, byte(v))
		}
	}
	p.m.Globals = append(p.m.Globals, mir2.Global{
		Name: name, Ty: mir2.NewArray(mir2.TyU8, len(init)), Init: init,
	})
	return nil
}

// ── function ──────────────────────────────────────────────────────────────────

// blockLines holds raw instruction lines for one QBE block.
type blockLines struct {
	label string
	lines []string // non-phi lines (instructions + terminator)
}

// phiDef records one QBE phi node converted to a MIR2 block param.
type phiDef struct {
	dstName string            // QBE reg name (without %)
	ty      mir2.Ty           // resolved MIR2 type
	srcs    map[string]string // predLabel → source reg name (without %)
}

func (p *parser) parseFunction() error {
	header := p.next()
	header = strings.TrimPrefix(header, "export ")
	header = strings.TrimPrefix(header, "function ")
	header = strings.TrimSpace(header)

	// Optional return type before $name.
	var retTy mir2.Ty = mir2.TyVoid
	if !strings.HasPrefix(header, "$") {
		sp := strings.IndexByte(header, ' ')
		if sp < 0 {
			return fmt.Errorf("function: bad header: %q", header)
		}
		retTy = parseQBEType(header[:sp])
		header = strings.TrimSpace(header[sp+1:])
	}

	// $name
	if !strings.HasPrefix(header, "$") {
		return fmt.Errorf("function: expected $name, got: %q", header)
	}
	header = header[1:]
	parenOpen := strings.IndexByte(header, '(')
	if parenOpen < 0 {
		return fmt.Errorf("function: missing '(': %q", header)
	}
	fnName := header[:parenOpen]
	rest := header[parenOpen+1:]
	parenClose := strings.IndexByte(rest, ')')
	if parenClose < 0 {
		return fmt.Errorf("function: missing ')': %q", header)
	}
	paramStr := strings.TrimSpace(rest[:parenClose])

	// Collect body lines until closing '}'.
	var bodyLines []string
	for {
		l := p.next()
		if l == "" || l == "}" {
			break
		}
		bodyLines = append(bodyLines, l)
	}

	// Build function.
	f := p.m.AddFunc(fnName)

	// regMap: QBE reg name → MIR2 Reg.
	regMap := make(map[string]mir2.Reg)
	alloc := func(name string) mir2.Reg {
		if r, ok := regMap[name]; ok {
			return r
		}
		r := f.AllocReg()
		regMap[name] = r
		return r
	}

	// Parse parameters.
	if paramStr != "" {
		for _, part := range splitCommaOutsideParens(paramStr) {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			fields := strings.Fields(part)
			if len(fields) < 2 {
				continue
			}
			ty := parseQBEType(fields[0])
			rname := strings.TrimPrefix(fields[1], "%")
			r := alloc(rname)
			f.Contract.Params = append(f.Contract.Params, mir2.Param{
				Name: rname, Reg: r, Ty: ty, Class: classFor(ty),
			})
		}
	}
	if retTy != mir2.TyVoid {
		f.Contract.Returns = []mir2.Return{{Ty: retTy, Class: classFor(retTy)}}
	}

	// ── Pass 1: group lines by block label, extract phi nodes ──
	blocks, phis := groupBlocks(bodyLines)

	// Pre-allocate every reg name that appears anywhere in the body
	// so that forward references (block params used before their block) work.
	for _, bl := range blocks {
		for _, phi := range phis[bl.label] {
			alloc(phi.dstName)
			for _, srcName := range phi.srcs {
				alloc(srcName)
			}
		}
		for _, line := range bl.lines {
			for _, t := range tokenizeLine(line) {
				if t.kind == tkReg {
					alloc(t.text)
				}
			}
		}
	}

	// ── Pass 2: build MIR2 ──
	// Create all blocks first so block params and labels are known.
	mirBlocks := make(map[string]*mir2.Block, len(blocks))
	for _, bl := range blocks {
		mb := f.NewBlock(bl.label)
		mirBlocks[bl.label] = mb
		for _, phi := range phis[bl.label] {
			mb.Params = append(mb.Params, mir2.BlockParam{
				Dst:   regMap[phi.dstName],
				Ty:    phi.ty,
				Class: classFor(phi.ty),
			})
		}
	}

	// argsFor(target, from) builds the terminator arg list by matching
	// phi sources for the given predecessor label.
	argsFor := func(target, from string) []mir2.Reg {
		phiList := phis[target]
		if len(phiList) == 0 {
			return nil
		}
		args := make([]mir2.Reg, len(phiList))
		for i, phi := range phiList {
			if srcName, ok := phi.srcs[from]; ok {
				args[i] = regMap[srcName]
			}
		}
		return args
	}

	bld := mir2.NewBuilder(f)
	for _, bl := range blocks {
		bld.SwitchTo(mirBlocks[bl.label])
		for _, line := range bl.lines {
			if err := emitLine(bld, line, bl.label, regMap, alloc, argsFor); err != nil {
				return fmt.Errorf("%s @%s: %w", fnName, bl.label, err)
			}
		}
	}
	return nil
}

// groupBlocks splits body lines into per-block groups and extracts phi nodes.
// Phi lines are removed from the instruction list (they become block params).
func groupBlocks(lines []string) ([]blockLines, map[string][]phiDef) {
	phis := make(map[string][]phiDef)
	var blocks []blockLines
	var cur *blockLines

	for _, raw := range lines {
		l := strings.TrimSpace(raw)
		if l == "" || strings.HasPrefix(l, "#") {
			continue
		}
		if strings.HasPrefix(l, "@") {
			label := l[1:]
			label = strings.TrimSuffix(label, ":") // some emitters append ':'
			blocks = append(blocks, blockLines{label: label})
			cur = &blocks[len(blocks)-1]
			continue
		}
		if cur == nil {
			// Lines before first @label — create implicit entry block.
			blocks = append(blocks, blockLines{label: "entry"})
			cur = &blocks[0]
		}
		if phi, ok := parsePhi(l); ok {
			phis[cur.label] = append(phis[cur.label], phi)
			continue
		}
		cur.lines = append(cur.lines, l)
	}
	return blocks, phis
}

// parsePhi tries to match:  %dst =ty phi @p1 %s1, @p2 %s2 ...
func parsePhi(line string) (phiDef, bool) {
	toks := tokenizeLine(line)
	// Minimum: %dst =ty phi @p1 %s1
	if len(toks) < 5 {
		return phiDef{}, false
	}
	if toks[0].kind != tkReg || toks[1].kind != tkTypeAssign {
		return phiDef{}, false
	}
	if toks[2].kind != tkIdent || toks[2].text != "phi" {
		return phiDef{}, false
	}
	phi := phiDef{
		dstName: toks[0].text,
		ty:      parseQBEType(toks[1].text),
		srcs:    make(map[string]string),
	}
	// Parse (@label %reg)* pairs.
	i := 3
	for i+1 < len(toks) {
		if toks[i].kind != tkLabel {
			i++
			continue
		}
		pred := toks[i].text
		i++
		if i < len(toks) && toks[i].kind == tkReg {
			phi.srcs[pred] = toks[i].text
			i++
		}
		// Skip optional comma.
		if i < len(toks) && toks[i].kind == tkPunct && toks[i].text == "," {
			i++
		}
	}
	return phi, true
}

// ── instruction emission ──────────────────────────────────────────────────────

func emitLine(
	bld *mir2.Builder,
	line, curLabel string,
	regMap map[string]mir2.Reg,
	alloc func(string) mir2.Reg,
	argsFor func(target, from string) []mir2.Reg,
) error {
	toks := tokenizeLine(line)
	if len(toks) == 0 {
		return nil
	}

	// ── Terminators and bare statements (no leading %dst =) ──
	if toks[0].kind == tkIdent {
		switch toks[0].text {

		case "jmp":
			// jmp @target
			labels := labelsIn(toks[1:])
			if len(labels) == 0 {
				return fmt.Errorf("jmp: missing label")
			}
			target := labels[0]
			bld.Jmp(target, argsFor(target, curLabel)...)

		case "jnz":
			// jnz %cond, @then, @else
			if len(toks) < 2 || toks[1].kind != tkReg {
				return fmt.Errorf("jnz: expected %%cond")
			}
			cond := alloc(toks[1].text)
			labels := labelsIn(toks[2:])
			if len(labels) < 2 {
				return fmt.Errorf("jnz: expected 2 labels, got %v", labels)
			}
			then, els := labels[0], labels[1]
			bld.BrIf(cond, then, argsFor(then, curLabel), els, argsFor(els, curLabel))

		case "ret":
			regs := regsIn(toks[1:], alloc)
			if len(regs) > 0 {
				bld.Ret(regs[0])
			} else {
				bld.Ret()
			}

		case "hlt":
			bld.Unreachable()

		case "storeb", "storeh", "storew":
			ty := map[string]mir2.Ty{
				"storeb": mir2.TyU8, "storeh": mir2.TyU16, "storew": mir2.TyU16,
			}[toks[0].text]
			rs := regsIn(toks[1:], alloc)
			if len(rs) < 2 {
				return fmt.Errorf("%s: need val and ptr", toks[0].text)
			}
			bld.Store(rs[1], rs[0], ty) // Store(ptr, val, ty)

		case "call":
			// Void direct call: call $fn(ty %a, ...)
			emitVoidCall(bld, toks, alloc)
		}
		// Unknown bare statement: skip silently.
		return nil
	}

	// ── Value instructions: %dst =ty op ... ──
	if toks[0].kind != tkReg || len(toks) < 3 || toks[1].kind != tkTypeAssign {
		return nil // unrecognised line shape — skip
	}
	dst := alloc(toks[0].text)
	ty := parseQBEType(toks[1].text)
	if toks[2].kind != tkIdent {
		return nil
	}
	opName := toks[2].text
	cls := classFor(ty)

	// Collect sources: all reg tokens and the first integer token after the op.
	rs := regsIn(toks[3:], alloc)
	imm, hasImm := firstInt(toks[3:])

	src0, src1 := mir2.NoReg, mir2.NoReg
	if len(rs) > 0 {
		src0 = rs[0]
	}
	if len(rs) > 1 {
		src1 = rs[1]
	}

	blk := bld.Cur
	emit := func(inst *mir2.Inst) { blk.Append(inst) }
	bin := func(op mir2.Op) {
		emit(&mir2.Inst{Op: op, Dst: dst, Src: [2]mir2.Reg{src0, src1}, Ty: ty, Cls: cls})
	}
	una := func(op mir2.Op) {
		emit(&mir2.Inst{Op: op, Dst: dst, Src: [2]mir2.Reg{src0}, Ty: ty, Cls: cls})
	}
	cmp := func(cond mir2.CmpCond) {
		emit(&mir2.Inst{Op: mir2.OpCmp, Dst: dst, Src: [2]mir2.Reg{src0, src1}, Cond: cond, Ty: mir2.TyBool, Cls: mir2.ClassFlag})
	}

	switch opName {
	// Binary arithmetic
	case "add":
		bin(mir2.OpAdd)
	case "sub":
		bin(mir2.OpSub)
	case "mul":
		bin(mir2.OpMul)
	case "udiv":
		bin(mir2.OpDiv)
	case "div":
		bin(mir2.OpSDiv)
	case "urem":
		bin(mir2.OpMod)
	case "rem":
		bin(mir2.OpMod)

	// Bitwise
	case "and":
		bin(mir2.OpAnd)
	case "or":
		bin(mir2.OpOr)
	case "xor":
		bin(mir2.OpXor)
	case "shl":
		bin(mir2.OpShl)
	case "shr":
		bin(mir2.OpShr)
	case "sar":
		bin(mir2.OpSar)

	// Unary
	case "neg":
		una(mir2.OpNeg)
	case "not": // QBE: xor %r, -1
		una(mir2.OpNot)

	// Copy / const
	case "copy":
		if src0 != mir2.NoReg {
			una(mir2.OpMove)
		} else if hasImm {
			emit(&mir2.Inst{Op: mir2.OpConst, Dst: dst, Imm: imm, Ty: ty, Cls: cls})
		}

	// Extensions
	case "extsb":
		emit(&mir2.Inst{Op: mir2.OpSext, Dst: dst, Src: [2]mir2.Reg{src0}, SrcTy: mir2.TyI8, Ty: ty, Cls: cls})
	case "extsh":
		emit(&mir2.Inst{Op: mir2.OpSext, Dst: dst, Src: [2]mir2.Reg{src0}, SrcTy: mir2.TyI16, Ty: ty, Cls: cls})
	case "extsw":
		emit(&mir2.Inst{Op: mir2.OpExt, Dst: dst, Src: [2]mir2.Reg{src0}, SrcTy: mir2.TyU16, Ty: ty, Cls: cls})
	case "extub":
		emit(&mir2.Inst{Op: mir2.OpExt, Dst: dst, Src: [2]mir2.Reg{src0}, SrcTy: mir2.TyU8, Ty: ty, Cls: cls})
	case "extuh":
		emit(&mir2.Inst{Op: mir2.OpExt, Dst: dst, Src: [2]mir2.Reg{src0}, SrcTy: mir2.TyU16, Ty: ty, Cls: cls})
	case "truncw":
		emit(&mir2.Inst{Op: mir2.OpTrunc, Dst: dst, Src: [2]mir2.Reg{src0}, SrcTy: mir2.TyU16, Ty: ty, Cls: cls})

	// Comparisons
	case "ceqw", "ceql":
		cmp(mir2.CmpEq)
	case "cnew", "cnel":
		cmp(mir2.CmpNe)
	case "csltw", "csltl":
		cmp(mir2.CmpLt)
	case "cslew", "cslel":
		cmp(mir2.CmpLe)
	case "csgtw", "csgtl":
		cmp(mir2.CmpGt)
	case "csgew", "csgel":
		cmp(mir2.CmpGe)
	case "cultw", "cultl":
		cmp(mir2.CmpUlt)
	case "culew", "culel":
		cmp(mir2.CmpUle)
	case "cugtw", "cugtl":
		cmp(mir2.CmpUgt)
	case "cugew", "cugel":
		cmp(mir2.CmpUge)

	// Loads
	case "loadub":
		emit(&mir2.Inst{Op: mir2.OpLoad, Dst: dst, Src: [2]mir2.Reg{src0}, Ty: mir2.TyU8, Cls: cls})
	case "loaduh", "loadh":
		emit(&mir2.Inst{Op: mir2.OpLoad, Dst: dst, Src: [2]mir2.Reg{src0}, Ty: mir2.TyU16, Cls: cls})
	case "loadw", "loadsw":
		emit(&mir2.Inst{Op: mir2.OpLoad, Dst: dst, Src: [2]mir2.Reg{src0}, Ty: mir2.TyU16, Cls: cls})
	case "loadl":
		emit(&mir2.Inst{Op: mir2.OpLoad, Dst: dst, Src: [2]mir2.Reg{src0}, Ty: mir2.TyPtr, Cls: mir2.ClassPointer})

	// Alloca
	case "alloc4", "alloc8", "alloc16":
		sz := imm
		if sz == 0 {
			sz = 8
		}
		emit(&mir2.Inst{Op: mir2.OpAlloca, Dst: dst, Imm: sz, Ty: mir2.TyPtr})

	// Call with return value
	case "call":
		emitCall(bld, toks, dst, ty, alloc)

	default:
		// Unknown op — emit zero const as a stub so the function stays valid.
		emit(&mir2.Inst{Op: mir2.OpConst, Dst: dst, Imm: 0, Ty: ty, Cls: cls})
	}
	return nil
}

func emitVoidCall(bld *mir2.Builder, toks []token, alloc func(string) mir2.Reg) {
	fnName, args := parseCallParts(toks, alloc)
	if fnName != "" {
		bld.CallVoid(fnName, args, mir2.CallAttrs{})
	}
}

func emitCall(bld *mir2.Builder, toks []token, dst mir2.Reg, retTy mir2.Ty, alloc func(string) mir2.Reg) {
	fnName, args := parseCallParts(toks, alloc)
	if fnName == "" {
		return
	}
	bld.Cur.Append(&mir2.Inst{
		Op: mir2.OpCall, Dst: dst, Sym: fnName, Args: args,
		Ty: retTy, Cls: classFor(retTy),
	})
}

// parseCallParts finds the $fnname and collects arg registers.
// Handles both: `call $fn(ty %a, ...)` and `%dst =ty call $fn(ty %a, ...)`
func parseCallParts(toks []token, alloc func(string) mir2.Reg) (string, []mir2.Reg) {
	var fnName string
	inArgs := false
	var args []mir2.Reg
	for _, t := range toks {
		if t.kind == tkGlobal {
			fnName = t.text
			inArgs = true
			continue
		}
		if !inArgs {
			continue
		}
		if t.kind == tkReg {
			args = append(args, alloc(t.text))
		}
	}
	return fnName, args
}

// ── tokenizer ─────────────────────────────────────────────────────────────────

type tokKind uint8

const (
	tkIdent      tokKind = iota // add, sub, jmp, phi, function, ...
	tkReg                       // %r42 → text="r42"
	tkLabel                     // @loop → text="loop"
	tkGlobal                    // $gcd → text="gcd"
	tkTypeAssign                // =w → text="w"  (the char after '=')
	tkInt                       // 42, -5
	tkPunct                     // , ( )
)

type token struct {
	kind tokKind
	text string
}

func tokenizeLine(line string) []token {
	var toks []token
	s := strings.TrimSpace(line)
	for len(s) > 0 {
		// Skip whitespace.
		i := 0
		for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
			i++
		}
		s = s[i:]
		if len(s) == 0 {
			break
		}
		switch {
		case s[0] == '#':
			return toks
		case s[0] == '@':
			n := wordLen(s[1:])
			toks = append(toks, token{tkLabel, s[1 : 1+n]})
			s = s[1+n:]
		case s[0] == '%':
			n := wordLen(s[1:])
			toks = append(toks, token{tkReg, s[1 : 1+n]})
			s = s[1+n:]
		case s[0] == '$':
			n := wordLen(s[1:])
			toks = append(toks, token{tkGlobal, s[1 : 1+n]})
			s = s[1+n:]
		case s[0] == '=':
			s = s[1:]
			if len(s) > 0 {
				c := s[0]
				if c == 'w' || c == 'l' || c == 's' || c == 'd' || c == 'b' || c == 'h' {
					toks = append(toks, token{tkTypeAssign, string(c)})
					s = s[1:]
				}
			}
		case s[0] == ',', s[0] == '(', s[0] == ')':
			toks = append(toks, token{tkPunct, string(s[0])})
			s = s[1:]
		case s[0] == '-' && len(s) > 1 && s[1] >= '0' && s[1] <= '9':
			n := 1
			for n < len(s) && s[n] >= '0' && s[n] <= '9' {
				n++
			}
			toks = append(toks, token{tkInt, s[:n]})
			s = s[n:]
		case s[0] >= '0' && s[0] <= '9':
			n := 0
			for n < len(s) && s[n] >= '0' && s[n] <= '9' {
				n++
			}
			toks = append(toks, token{tkInt, s[:n]})
			s = s[n:]
		default:
			n := wordLen(s)
			if n == 0 {
				s = s[1:]
				continue
			}
			toks = append(toks, token{tkIdent, s[:n]})
			s = s[n:]
		}
	}
	return toks
}

// wordLen returns the length of the leading identifier in s.
func wordLen(s string) int {
	i := 0
	for i < len(s) {
		c := s[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '_' || c == '.' {
			i++
		} else {
			break
		}
	}
	return i
}

// ── helpers ───────────────────────────────────────────────────────────────────

// parseQBEType maps a QBE type letter to a MIR2 Ty.
func parseQBEType(s string) mir2.Ty {
	switch s {
	case "l":
		return mir2.TyPtr
	case "b":
		return mir2.TyU8
	case "h":
		return mir2.TyU16
	default: // w, s, d → u16
		return mir2.TyU16
	}
}

// classFor returns a sensible default RegClass for a type.
func classFor(ty mir2.Ty) mir2.RegClass {
	switch ty {
	case mir2.TyPtr:
		return mir2.ClassPointer
	case mir2.TyBool:
		return mir2.ClassFlag
	}
	return mir2.ClassGeneral
}

// labelsIn returns all label-token texts in toks.
func labelsIn(toks []token) []string {
	var out []string
	for _, t := range toks {
		if t.kind == tkLabel {
			out = append(out, t.text)
		}
	}
	return out
}

// regsIn returns all reg tokens in toks, allocated via alloc.
func regsIn(toks []token, alloc func(string) mir2.Reg) []mir2.Reg {
	var out []mir2.Reg
	for _, t := range toks {
		if t.kind == tkReg {
			out = append(out, alloc(t.text))
		}
	}
	return out
}

// firstInt returns the first integer token value and whether one was found.
func firstInt(toks []token) (int64, bool) {
	for _, t := range toks {
		if t.kind == tkInt {
			v, _ := strconv.ParseInt(t.text, 10, 64)
			return v, true
		}
	}
	return 0, false
}

// splitCommaOutsideParens splits s on commas not inside parentheses.
func splitCommaOutsideParens(s string) []string {
	var parts []string
	depth, start := 0, 0
	for i, c := range s {
		switch c {
		case '(':
			depth++
		case ')':
			depth--
		case ',':
			if depth == 0 {
				parts = append(parts, s[start:i])
				start = i + 1
			}
		}
	}
	return append(parts, s[start:])
}
