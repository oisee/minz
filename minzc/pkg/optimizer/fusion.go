package optimizer

import (
	"fmt"
	"strings"

	"github.com/minz/minzc/pkg/ir"
)

// FusionOptimizer performs iterator chain fusion optimization.
// It inlines small callback functions within DJNZ iterator loops,
// eliminating CALL/RET overhead (~27 T-states per call per element)
// and SMC parameter patching overhead.
//
// The semantic analyzer already fuses multi-stage chains (map+filter+forEach)
// into single DJNZ loops. This pass goes further by inlining the callback
// function bodies directly into the loop, replacing OpCall with the callee's
// instructions. This is specifically needed because the standard InliningPass
// skips functions with OpTrueSMCLoad or OpLoadParam (iterator callbacks use these).
type FusionOptimizer struct {
	module   *ir.Module
	optimized int
	fusionID  int // Unique ID for inlined label disambiguation
}

// NewFusionOptimizer creates a new fusion optimizer
func NewFusionOptimizer() *FusionOptimizer {
	return &FusionOptimizer{}
}

// NewIteratorFusionPass creates a fusion optimizer as a Pass
func NewIteratorFusionPass() Pass {
	return NewFusionOptimizer()
}

// Name returns the pass name (implements Pass interface)
func (f *FusionOptimizer) Name() string {
	return "IteratorFusion"
}

// Run executes the fusion pass on a module (implements Pass interface)
func (f *FusionOptimizer) Run(module *ir.Module) (bool, error) {
	f.module = module
	f.optimized = 0

	changed := false
	for _, fn := range module.Functions {
		if f.optimizeFunction(fn) {
			changed = true
		}
	}

	return changed, nil
}

// Optimize performs fusion optimization on the module (standalone API)
func (f *FusionOptimizer) Optimize(module *ir.Module) error {
	_, err := f.Run(module)
	if err != nil {
		return err
	}
	if f.optimized > 0 {
		fmt.Printf("Fusion optimizer: Fused %d iterator callbacks\n", f.optimized)
	}
	return nil
}

// djnzLoop represents a detected DJNZ iterator loop in MIR
type djnzLoop struct {
	nopIdx     int         // Index of NOP marker ("DJNZ OPTIMIZED LOOP")
	counterIdx int         // Index of OpLoadConst for counter
	ptrIdx     int         // Index of OpMove for pointer init
	labelIdx   int         // Index of OpLabel for loop start
	elementIdx int         // Index of OpLoad for element (*ptr)
	pushIdx    int         // Index of OpPush (save array pointer)
	popIdx     int         // Index of OpPop (restore array pointer)
	incIdx     int         // Index of OpInc (advance pointer)
	djnzIdx    int         // Index of OpDJNZ
	counterReg ir.Register
	ptrReg     ir.Register
	elementReg ir.Register
}

// callSite represents a fusible OpCall inside a DJNZ loop
type callSite struct {
	idx    int             // Instruction index in function
	inst   ir.Instruction  // The OpCall instruction
	callee *ir.Function    // The called function
}

// optimizeFunction finds DJNZ iterator loops and inlines fusible callbacks.
func (f *FusionOptimizer) optimizeFunction(fn *ir.Function) bool {
	loops := f.findDJNZLoops(fn)
	if len(loops) == 0 {
		return false
	}

	changed := false
	// Process loops in reverse order so index shifts don't affect earlier loops
	for i := len(loops) - 1; i >= 0; i-- {
		if f.fuseLoopCallbacks(fn, loops[i]) {
			changed = true
		}
	}
	return changed
}

// findDJNZLoops scans for DJNZ iterator loop patterns in MIR.
// Pattern:
//   OpNop "DJNZ OPTIMIZED LOOP"
//   OpLoadConst  r_counter = N
//   OpMove       r_ptr = r_source
//   OpLabel      djnz_loop_X:
//   OpLoad       r_element = *r_ptr
//   OpPush       r_ptr
//   ... operations (OpCall, OpLoadConst, OpJumpIfFlag, etc.) ...
//   OpPop        r_ptr
//   OpInc        r_ptr
//   OpDJNZ       r_counter, djnz_loop_X
func (f *FusionOptimizer) findDJNZLoops(fn *ir.Function) []*djnzLoop {
	var loops []*djnzLoop
	instrs := fn.Instructions

	for i := 0; i < len(instrs); i++ {
		if instrs[i].Op == ir.OpNop && strings.Contains(instrs[i].Comment, "DJNZ OPTIMIZED LOOP") {
			loop := f.parseDJNZLoop(instrs, i)
			if loop != nil {
				loops = append(loops, loop)
			}
		}
	}
	return loops
}

// parseDJNZLoop validates and extracts a DJNZ iterator loop structure.
func (f *FusionOptimizer) parseDJNZLoop(instrs []ir.Instruction, nopIdx int) *djnzLoop {
	n := len(instrs)
	loop := &djnzLoop{nopIdx: nopIdx}

	// OpLoadConst for counter (immediately after NOP)
	idx := nopIdx + 1
	if idx >= n || instrs[idx].Op != ir.OpLoadConst {
		return nil
	}
	loop.counterIdx = idx
	loop.counterReg = instrs[idx].Dest

	// OpMove for pointer init
	idx++
	if idx >= n || instrs[idx].Op != ir.OpMove {
		return nil
	}
	loop.ptrIdx = idx
	loop.ptrReg = instrs[idx].Dest

	// OpLabel for loop start
	idx++
	if idx >= n || instrs[idx].Op != ir.OpLabel {
		return nil
	}
	loop.labelIdx = idx

	// OpLoad for element load (*ptr)
	idx++
	if idx >= n || (instrs[idx].Op != ir.OpLoad && instrs[idx].Op != ir.OpLoadPtr) {
		return nil
	}
	loop.elementIdx = idx
	loop.elementReg = instrs[idx].Dest

	// OpPush for saving pointer
	idx++
	if idx >= n || instrs[idx].Op != ir.OpPush {
		return nil
	}
	loop.pushIdx = idx

	// Find OpDJNZ to locate loop end
	for j := idx + 1; j < n; j++ {
		if instrs[j].Op == ir.OpDJNZ {
			loop.djnzIdx = j
			break
		}
	}
	if loop.djnzIdx == 0 {
		return nil
	}

	// Find OpInc and OpPop before DJNZ (scanning backwards)
	for k := loop.djnzIdx - 1; k > loop.pushIdx; k-- {
		if instrs[k].Op == ir.OpInc {
			loop.incIdx = k
			break
		}
	}
	for k := loop.incIdx - 1; k > loop.pushIdx; k-- {
		if instrs[k].Op == ir.OpPop {
			loop.popIdx = k
			break
		}
	}

	if loop.incIdx == 0 || loop.popIdx == 0 {
		return nil
	}

	return loop
}

// fuseLoopCallbacks finds and inlines fusible OpCall instructions within a DJNZ loop.
// After inlining, checks if no OpCall remains and sets BareDJNZ hint.
func (f *FusionOptimizer) fuseLoopCallbacks(fn *ir.Function, loop *djnzLoop) bool {
	calls := f.findFusibleCalls(fn.Instructions, loop)

	changed := false
	// Process in reverse order to preserve indices after splice
	for i := len(calls) - 1; i >= 0; i-- {
		if f.inlineCallInLoop(fn, calls[i]) {
			changed = true
			f.optimized++
		}
	}

	// After inlining, check if any OpCall remains in the loop body.
	// If none, B register won't be clobbered → can use bare DJNZ instruction.
	f.maybeSetBareDJNZ(fn, loop)

	return changed
}

// maybeSetBareDJNZ checks if a DJNZ loop body has no remaining OpCall.
// If so, sets the BareDJNZ CodegenHint on the OpDJNZ instruction,
// allowing the Z80 codegen to emit a single DJNZ instruction instead
// of the manual DEC B + LD A,B + store + JR NZ sequence.
func (f *FusionOptimizer) maybeSetBareDJNZ(fn *ir.Function, loop *djnzLoop) {
	// Re-find the DJNZ instruction (indices may have shifted after inlining)
	djnzIdx := -1
	loopLabelIdx := -1
	for i, inst := range fn.Instructions {
		if inst.Op == ir.OpLabel && inst.Label == fn.Instructions[loop.labelIdx].Label {
			loopLabelIdx = i
		}
		if inst.Op == ir.OpDJNZ && loopLabelIdx >= 0 && i > loopLabelIdx {
			djnzIdx = i
			break
		}
	}
	if djnzIdx < 0 || loopLabelIdx < 0 {
		return
	}

	// Check for any OpCall between loop label and DJNZ
	for i := loopLabelIdx + 1; i < djnzIdx; i++ {
		if fn.Instructions[i].Op == ir.OpCall {
			return // Still has calls — B might be clobbered
		}
	}

	// No calls in loop body — set BareDJNZ hint
	inst := &fn.Instructions[djnzIdx]
	if inst.CodegenHint == nil {
		inst.CodegenHint = &ir.CodegenHints{}
	}
	inst.CodegenHint.BareDJNZ = true
}

// findFusibleCalls finds OpCall instructions in the loop body that can be inlined.
func (f *FusionOptimizer) findFusibleCalls(instrs []ir.Instruction, loop *djnzLoop) []callSite {
	var calls []callSite

	// Scan the operation region (between PUSH and POP)
	for i := loop.pushIdx + 1; i < loop.popIdx; i++ {
		if instrs[i].Op != ir.OpCall {
			continue
		}

		callee := f.findFunction(instrs[i].Symbol)
		if callee == nil {
			continue
		}

		if f.isFusibleCallback(callee) {
			calls = append(calls, callSite{
				idx:    i,
				inst:   instrs[i],
				callee: callee,
			})
		}
	}
	return calls
}

// findFunction looks up a function by name in the module.
func (f *FusionOptimizer) findFunction(name string) *ir.Function {
	for _, fn := range f.module.Functions {
		if fn.Name == name {
			return fn
		}
	}
	return nil
}

// isFusibleCallback checks if a function is suitable for inlining into a DJNZ loop.
// Criteria:
//   - ≤ 8 instructions (small enough to not bloat the loop)
//   - No inline assembly (uses physical registers directly)
//   - No nested function calls (would reintroduce CALL overhead)
//   - No loops (would create nested loops)
//   - Exactly 1 parameter (standard iterator callback signature)
func (f *FusionOptimizer) isFusibleCallback(fn *ir.Function) bool {
	if len(fn.Instructions) > 8 {
		return false
	}

	if fn.NumParams != 1 {
		return false
	}

	for _, inst := range fn.Instructions {
		switch inst.Op {
		case ir.OpAsm:
			return false // Physical register dependency
		case ir.OpCall:
			return false // Nested calls defeat the purpose
		case ir.OpDJNZ, ir.OpJump:
			return false // Loops inside the callback
		}
	}

	return true
}

// inlineCallInLoop replaces an OpCall with the callee's body, performing:
//   - OpTrueSMCLoad → OpMove from the actual argument
//   - OpLoadParam/OpLoadVar(param) → OpMove from the actual argument
//   - Register remapping to avoid conflicts
//   - OpReturn → OpMove to the call's destination register
func (f *FusionOptimizer) inlineCallInLoop(fn *ir.Function, call callSite) bool {
	callee := call.callee
	f.fusionID++
	labelSuffix := fmt.Sprintf("_fused%d", f.fusionID)

	// Determine the callee's parameter register
	var paramReg ir.Register
	if len(callee.Params) > 0 && callee.Params[0].Reg != 0 {
		paramReg = callee.Params[0].Reg
	} else {
		paramReg = ir.Register(1) // Convention: first param = r1
	}

	// The actual argument register from the call site
	var argReg ir.Register
	if len(call.inst.Args) > 0 {
		argReg = call.inst.Args[0]
	} else {
		return false // No argument to pass
	}

	// Build register mapping: callee registers → new registers in caller
	regMap := make(map[ir.Register]ir.Register)
	regMap[paramReg] = argReg // Map parameter to actual argument

	// Build label mapping for disambiguation
	labelMap := make(map[string]string)
	for _, inst := range callee.Instructions {
		if inst.Op == ir.OpLabel && inst.Label != "" {
			labelMap[inst.Label] = inst.Label + labelSuffix
		}
	}

	// Allocate new registers for callee's internal registers
	nextReg := fn.NextRegister
	for _, inst := range callee.Instructions {
		if inst.Dest != 0 {
			if _, exists := regMap[inst.Dest]; !exists {
				regMap[inst.Dest] = nextReg
				nextReg++
			}
		}
	}

	// Parameter name for matching OpLoadVar/OpLoadParam
	paramName := ""
	if len(callee.Params) > 0 {
		paramName = callee.Params[0].Name
	}

	// Generate inlined instructions
	var inlined []ir.Instruction
	for _, inst := range callee.Instructions {
		// Handle SMC parameter load → move from actual argument
		if inst.Op == ir.OpTrueSMCLoad {
			destReg := f.mapReg(regMap, inst.Dest)
			inlined = append(inlined, ir.Instruction{
				Op:      ir.OpMove,
				Dest:    destReg,
				Src1:    argReg,
				Comment: fmt.Sprintf("Fused: param %s ← r%d", paramName, argReg),
			})
			continue
		}

		// Handle regular parameter load → move from actual argument
		if inst.Op == ir.OpLoadParam ||
			(inst.Op == ir.OpLoadVar && paramName != "" && inst.Symbol == paramName) {
			destReg := f.mapReg(regMap, inst.Dest)
			inlined = append(inlined, ir.Instruction{
				Op:      ir.OpMove,
				Dest:    destReg,
				Src1:    argReg,
				Comment: fmt.Sprintf("Fused: param %s ← r%d", paramName, argReg),
			})
			continue
		}

		// Handle return → move result to call destination
		if inst.Op == ir.OpReturn {
			if inst.Src1 != 0 && call.inst.Dest != 0 {
				src := f.mapReg(regMap, inst.Src1)
				inlined = append(inlined, ir.Instruction{
					Op:      ir.OpMove,
					Dest:    call.inst.Dest,
					Src1:    src,
					Comment: "Fused: return value",
				})
			}
			continue
		}

		// Remap registers in all other instructions
		newInst := inst
		if inst.Dest != 0 {
			newInst.Dest = f.mapReg(regMap, inst.Dest)
		}
		if inst.Src1 != 0 {
			newInst.Src1 = f.mapReg(regMap, inst.Src1)
		}
		if inst.Src2 != 0 {
			newInst.Src2 = f.mapReg(regMap, inst.Src2)
		}
		if inst.Label != "" {
			if mapped, ok := labelMap[inst.Label]; ok {
				newInst.Label = mapped
			}
		}
		if len(inst.Args) > 0 {
			newArgs := make([]ir.Register, len(inst.Args))
			for i, arg := range inst.Args {
				newArgs[i] = f.mapReg(regMap, arg)
			}
			newInst.Args = newArgs
		}

		if newInst.Comment != "" {
			newInst.Comment = "Fused: " + newInst.Comment
		} else {
			newInst.Comment = fmt.Sprintf("Fused from %s", callee.Name)
		}

		inlined = append(inlined, newInst)
	}

	if len(inlined) == 0 {
		return false
	}

	// Splice: replace OpCall at call.idx with inlined instructions
	old := fn.Instructions
	result := make([]ir.Instruction, 0, len(old)+len(inlined)-1)
	result = append(result, old[:call.idx]...)
	result = append(result, inlined...)
	result = append(result, old[call.idx+1:]...)
	fn.Instructions = result
	fn.NextRegister = nextReg

	return true
}

// mapReg returns the mapped register if one exists, otherwise the original.
func (f *FusionOptimizer) mapReg(regMap map[ir.Register]ir.Register, reg ir.Register) ir.Register {
	if mapped, ok := regMap[reg]; ok {
		return mapped
	}
	return reg
}
