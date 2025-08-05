package mir

import (
	"fmt"
	"io"
	"strings"

	"github.com/minz/minzc/pkg/ir"
)

// Visualizer generates Graphviz DOT output for MIR visualization
type Visualizer struct {
	writer io.Writer
	module *ir.Module
}

// NewVisualizer creates a new MIR visualizer
func NewVisualizer(w io.Writer) *Visualizer {
	return &Visualizer{
		writer: w,
	}
}

// Visualize generates DOT output for the IR module
func (v *Visualizer) Visualize(module *ir.Module) error {
	v.module = module

	// Start the graph
	v.emit("digraph MinZ_MIR {")
	v.emit("  rankdir=TB;")
	v.emit("  node [shape=box, style=rounded];")
	v.emit("")

	// Module information
	v.emit("  // Module: %s", module.Name)
	v.emit("  subgraph cluster_module {")
	v.emit("    label=\"Module: %s\";", module.Name)
	v.emit("    style=dashed;")
	v.emit("")

	// Visualize globals
	if len(module.Globals) > 0 {
		v.visualizeGlobals()
	}

	// Visualize each function
	for i, fn := range module.Functions {
		if err := v.visualizeFunction(fn, i); err != nil {
			return err
		}
	}

	v.emit("  }")
	v.emit("}")

	return nil
}

// visualizeGlobals creates nodes for global variables
func (v *Visualizer) visualizeGlobals() {
	v.emit("    // Global variables")
	v.emit("    subgraph cluster_globals {")
	v.emit("      label=\"Globals\";")
	v.emit("      style=filled;")
	v.emit("      fillcolor=lightgray;")
	
	for _, global := range v.module.Globals {
		v.emit("      \"%s\" [label=\"%s\\n%s\", shape=octagon];", 
			global.Name, global.Name, global.Type.String())
	}
	
	v.emit("    }")
	v.emit("")
}

// visualizeFunction creates a control flow graph for a function
func (v *Visualizer) visualizeFunction(fn *ir.Function, fnIndex int) error {
	funcID := fmt.Sprintf("func_%d", fnIndex)
	
	v.emit("    // Function: %s", fn.Name)
	v.emit("    subgraph cluster_%s {", funcID)
	v.emit("      label=\"%s\";", fn.Name)
	v.emit("      style=filled;")
	v.emit("      fillcolor=lightyellow;")
	
	// Function metadata
	if fn.IsSMCEnabled {
		v.emit("      \"%s_meta\" [label=\"SMC Enabled\", shape=note, style=filled, fillcolor=lightgreen];", funcID)
	}
	if fn.IsRecursive {
		v.emit("      \"%s_rec\" [label=\"Recursive\", shape=note, style=filled, fillcolor=lightcoral];", funcID)
	}
	
	// Parameters
	if len(fn.Params) > 0 {
		v.emit("      \"%s_params\" [label=\"Parameters:\\n%s\", shape=invhouse];", 
			funcID, v.formatParams(fn.Params))
	}
	
	// Return type
	if fn.ReturnType != nil && fn.ReturnType.String() != "void" {
		v.emit("      \"%s_return\" [label=\"Returns: %s\", shape=house];", 
			funcID, fn.ReturnType.String())
	}
	
	// Create basic blocks
	blocks := v.identifyBasicBlocks(fn)
	
	// Visualize basic blocks
	for i, block := range blocks {
		v.visualizeBasicBlock(funcID, i, block)
	}
	
	// Connect blocks with control flow edges
	v.connectBasicBlocks(funcID, blocks)
	
	v.emit("    }")
	v.emit("")
	
	return nil
}

// BasicBlock represents a sequence of instructions without branches
type BasicBlock struct {
	StartIndex int
	EndIndex   int
	Label      string
	Instructions []ir.Instruction
}

// identifyBasicBlocks splits function instructions into basic blocks
func (v *Visualizer) identifyBasicBlocks(fn *ir.Function) []BasicBlock {
	blocks := []BasicBlock{}
	if len(fn.Instructions) == 0 {
		return blocks
	}
	
	// Find block boundaries (labels and jumps)
	blockStarts := map[int]bool{0: true}
	
	for i, inst := range fn.Instructions {
		switch inst.Op {
		case ir.OpLabel:
			blockStarts[i] = true
		case ir.OpJump, ir.OpJumpIf, ir.OpJumpIfNot, ir.OpReturn:
			if i+1 < len(fn.Instructions) {
				blockStarts[i+1] = true
			}
		}
	}
	
	// Create blocks
	starts := []int{}
	for start := range blockStarts {
		starts = append(starts, start)
	}
	
	// Sort starts (simple bubble sort)
	for i := 0; i < len(starts); i++ {
		for j := i + 1; j < len(starts); j++ {
			if starts[i] > starts[j] {
				starts[i], starts[j] = starts[j], starts[i]
			}
		}
	}
	
	// Create blocks from sorted starts
	for i := 0; i < len(starts); i++ {
		start := starts[i]
		end := len(fn.Instructions)
		if i+1 < len(starts) {
			end = starts[i+1]
		}
		
		block := BasicBlock{
			StartIndex: start,
			EndIndex:   end,
			Instructions: fn.Instructions[start:end],
		}
		
		// Set label if first instruction is a label
		if len(block.Instructions) > 0 && block.Instructions[0].Op == ir.OpLabel {
			block.Label = block.Instructions[0].Label
		}
		
		blocks = append(blocks, block)
	}
	
	return blocks
}

// visualizeBasicBlock creates a node for a basic block
func (v *Visualizer) visualizeBasicBlock(funcID string, blockIndex int, block BasicBlock) {
	blockID := fmt.Sprintf("%s_bb%d", funcID, blockIndex)
	
	// Build label with instructions
	var label strings.Builder
	if block.Label != "" {
		label.WriteString(fmt.Sprintf("%s:\\n", block.Label))
	} else {
		label.WriteString(fmt.Sprintf("BB%d:\\n", blockIndex))
	}
	
	for _, inst := range block.Instructions {
		if inst.Op != ir.OpLabel { // Skip label, already shown
			label.WriteString(v.formatInstruction(&inst))
			label.WriteString("\\n")
		}
	}
	
	// Choose color based on block type
	color := "lightblue"
	if blockIndex == 0 {
		color = "lightgreen" // Entry block
	} else if len(block.Instructions) > 0 {
		lastInst := block.Instructions[len(block.Instructions)-1]
		if lastInst.Op == ir.OpReturn {
			color = "lightcoral" // Exit block
		}
	}
	
	v.emit("      \"%s\" [label=\"%s\", style=filled, fillcolor=%s];", 
		blockID, strings.TrimSpace(label.String()), color)
}

// connectBasicBlocks adds control flow edges between blocks
func (v *Visualizer) connectBasicBlocks(funcID string, blocks []BasicBlock) {
	for i, block := range blocks {
		if len(block.Instructions) == 0 {
			continue
		}
		
		lastInst := block.Instructions[len(block.Instructions)-1]
		blockID := fmt.Sprintf("%s_bb%d", funcID, i)
		
		switch lastInst.Op {
		case ir.OpJump:
			// Find target block
			for j, target := range blocks {
				if target.Label == lastInst.Label {
					targetID := fmt.Sprintf("%s_bb%d", funcID, j)
					v.emit("      \"%s\" -> \"%s\" [label=\"jump\"];", blockID, targetID)
					break
				}
			}
			
		case ir.OpJumpIf, ir.OpJumpIfNot:
			// Conditional jump - two edges
			// Find target block
			for j, target := range blocks {
				if target.Label == lastInst.Label {
					targetID := fmt.Sprintf("%s_bb%d", funcID, j)
					cond := "true"
					if lastInst.Op == ir.OpJumpIfNot {
						cond = "false"
					}
					v.emit("      \"%s\" -> \"%s\" [label=\"%s\", style=dashed];", 
						blockID, targetID, cond)
					break
				}
			}
			// Fall through to next block
			if i+1 < len(blocks) {
				nextID := fmt.Sprintf("%s_bb%d", funcID, i+1)
				cond := "false"
				if lastInst.Op == ir.OpJumpIfNot {
					cond = "true"
				}
				v.emit("      \"%s\" -> \"%s\" [label=\"%s\"];", blockID, nextID, cond)
			}
			
		case ir.OpReturn:
			// No outgoing edges
			
		default:
			// Fall through to next block
			if i+1 < len(blocks) {
				nextID := fmt.Sprintf("%s_bb%d", funcID, i+1)
				v.emit("      \"%s\" -> \"%s\";", blockID, nextID)
			}
		}
	}
}

// formatInstruction formats an instruction for display
func (v *Visualizer) formatInstruction(inst *ir.Instruction) string {
	switch inst.Op {
	case ir.OpLoadConst:
		return fmt.Sprintf("r%d = %d", inst.Dest, inst.Imm)
	case ir.OpMove:
		return fmt.Sprintf("r%d = r%d", inst.Dest, inst.Src1)
	case ir.OpLoadVar:
		return fmt.Sprintf("r%d = load %s", inst.Dest, inst.Symbol)
	case ir.OpStoreVar:
		if inst.Symbol != "" {
			return fmt.Sprintf("store %s, r%d", inst.Symbol, inst.Src1)
		}
		return fmt.Sprintf("store r%d", inst.Src1)
	case ir.OpAdd:
		return fmt.Sprintf("r%d = r%d + r%d", inst.Dest, inst.Src1, inst.Src2)
	case ir.OpSub:
		return fmt.Sprintf("r%d = r%d - r%d", inst.Dest, inst.Src1, inst.Src2)
	case ir.OpMul:
		return fmt.Sprintf("r%d = r%d * r%d", inst.Dest, inst.Src1, inst.Src2)
	case ir.OpCall:
		if inst.Dest != 0 {
			return fmt.Sprintf("r%d = call %s", inst.Dest, inst.Symbol)
		}
		return fmt.Sprintf("call %s", inst.Symbol)
	case ir.OpReturn:
		if inst.Src1 != 0 {
			return fmt.Sprintf("return r%d", inst.Src1)
		}
		return "return"
	case ir.OpJump:
		return fmt.Sprintf("jump %s", inst.Label)
	case ir.OpJumpIf:
		return fmt.Sprintf("jump_if r%d, %s", inst.Src1, inst.Label)
	case ir.OpJumpIfNot:
		return fmt.Sprintf("jump_if_not r%d, %s", inst.Src1, inst.Label)
	case ir.OpLabel:
		return fmt.Sprintf("%s:", inst.Label)
	case ir.OpPrint:
		return "print char"
	case ir.OpPrintU8:
		return fmt.Sprintf("print_u8 r%d", inst.Src1)
	case ir.OpPrintStringDirect:
		return "print_string_direct"
	case ir.OpAsm:
		return fmt.Sprintf("asm { %s }", strings.Split(inst.Symbol, "\n")[0])
	default:
		return inst.Op.String()
	}
}

// formatParams formats function parameters
func (v *Visualizer) formatParams(params []ir.Parameter) string {
	var parts []string
	for _, p := range params {
		parts = append(parts, fmt.Sprintf("%s: %s", p.Name, p.Type.String()))
	}
	return strings.Join(parts, "\\n")
}

// emit writes a line to the output
func (v *Visualizer) emit(format string, args ...interface{}) {
	if len(args) > 0 {
		fmt.Fprintf(v.writer, format+"\n", args...)
	} else {
		fmt.Fprintf(v.writer, format+"\n")
	}
}