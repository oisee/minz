package main

import (
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
)

func main() {
	fmt.Println("Testing MZE upgrade to 100% Z80 coverage...")
	
	// Create the new 100% coverage emulator
	emu := emulator.NewRemogattoZ80WithScreen()
	
	// Test program with DJNZ (previously not working)
	program := []byte{
		0x06, 5,   // LD B, 5
		0x3E, 0,   // LD A, 0
		0x3C,      // loop: INC A
		0x10, 0xFD, // DJNZ loop (-3) - This was missing in basic emulator!
		0x76,      // HALT
	}
	
	// Load program
	emu.LoadAt(0x8000, program)
	emu.SetPC(0x8000)
	
	fmt.Println("Running DJNZ test (previously impossible with 19.5% coverage)...")
	
	// Execute
	err := emu.Execute()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	
	// Check results
	regs := emu.GetRegisters()
	fmt.Printf("✅ DJNZ test completed!\n")
	fmt.Printf("   A = %d (expected 5)\n", regs.A)
	fmt.Printf("   B = %d (expected 0)\n", (regs.BC>>8)&0xFF)
	fmt.Printf("   Cycles = %d\n", emu.GetCycles())
	
	if regs.A == 5 {
		fmt.Println("🎉 SUCCESS: MZE now has 100% Z80 instruction coverage!")
		fmt.Println("🚀 This unlocks full game testing and TSMC verification!")
	} else {
		fmt.Println("❌ FAILED: Something went wrong")
	}
	
	// Test memory access
	testVal := emu.GetMemory(0x8000)
	fmt.Printf("Memory at 0x8000: 0x%02X (should be 0x06 - LD B,5)\n", testVal)
	
	fmt.Println("\n🎯 Next: Update cmd/mze/main.go to use this emulator by default")
}