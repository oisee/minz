package main

import (
	"fmt"
	"github.com/minz/minzc/pkg/ir"
)

func main() {
	fmt.Printf("OpNop: %d\n", int(ir.OpNop))
	fmt.Printf("OpLabel: %d\n", int(ir.OpLabel))
	fmt.Printf("OpJump: %d\n", int(ir.OpJump))
	fmt.Printf("OpJumpIf: %d\n", int(ir.OpJumpIf))
	fmt.Printf("OpJumpIfNot: %d\n", int(ir.OpJumpIfNot))
	fmt.Printf("OpJumpIfZero: %d\n", int(ir.OpJumpIfZero))
	fmt.Printf("OpJumpIfNotZero: %d\n", int(ir.OpJumpIfNotZero))
	fmt.Printf("OpCall: %d\n", int(ir.OpCall))
	fmt.Printf("OpReturn: %d\n", int(ir.OpReturn))
	
	// Data movement
	fmt.Printf("OpLoadConst: %d\n", int(ir.OpLoadConst))
	fmt.Printf("OpLoadVar: %d\n", int(ir.OpLoadVar))
	fmt.Printf("OpStoreVar: %d\n", int(ir.OpStoreVar))
	fmt.Printf("OpLoadParam: %d\n", int(ir.OpLoadParam))
	fmt.Printf("OpLoadField: %d\n", int(ir.OpLoadField))
	fmt.Printf("OpStoreField: %d\n", int(ir.OpStoreField))
	fmt.Printf("OpLoadIndex: %d\n", int(ir.OpLoadIndex))
	fmt.Printf("OpStoreIndex: %d\n", int(ir.OpStoreIndex))
	
	// More...
	fmt.Printf("OpMove: %d\n", int(ir.OpMove))
	fmt.Printf("OpLoadLabel: %d\n", int(ir.OpLoadLabel))
	fmt.Printf("OpLoadDirect: %d\n", int(ir.OpLoadDirect))
	fmt.Printf("OpStoreDirect: %d\n", int(ir.OpStoreDirect))
	
	// SMC
	fmt.Printf("OpSMCLoadConst: %d\n", int(ir.OpSMCLoadConst))
	fmt.Printf("OpSMCStoreConst: %d\n", int(ir.OpSMCStoreConst))
	
	// Let's find what opcode 36 is
	fmt.Printf("\nLooking for opcode 36...\n")
	for i := 30; i <= 40; i++ {
		switch ir.Opcode(i) {
		case ir.OpAdd:
			fmt.Printf("Opcode %d: OpAdd\n", i)
		case ir.OpSub:
			fmt.Printf("Opcode %d: OpSub\n", i)
		case ir.OpMul:
			fmt.Printf("Opcode %d: OpMul\n", i)
		case ir.OpDiv:
			fmt.Printf("Opcode %d: OpDiv\n", i)
		case ir.OpMod:
			fmt.Printf("Opcode %d: OpMod\n", i)
		case ir.OpNeg:
			fmt.Printf("Opcode %d: OpNeg\n", i)
		case ir.OpInc:
			fmt.Printf("Opcode %d: OpInc\n", i)
		case ir.OpDec:
			fmt.Printf("Opcode %d: OpDec\n", i)
		case ir.OpAnd:
			fmt.Printf("Opcode %d: OpAnd\n", i)
		case ir.OpOr:
			fmt.Printf("Opcode %d: OpOr\n", i)
		case ir.OpXor:
			fmt.Printf("Opcode %d: OpXor\n", i)
		case ir.OpNot:
			fmt.Printf("Opcode %d: OpNot\n", i)
		case ir.OpShl:
			fmt.Printf("Opcode %d: OpShl\n", i)
		case ir.OpShr:
			fmt.Printf("Opcode %d: OpShr\n", i)
		case ir.OpAddr:
			fmt.Printf("Opcode %d: OpAddr\n", i)
		default:
			fmt.Printf("Opcode %d: UNKNOWN\n", i)
		}
	}
}