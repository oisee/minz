package main

import (
	"fmt"
	"github.com/minz/minzc/pkg/optimizer"
)

func main() {
	input := `; Test file for peephole optimization
    LD D, H
    LD E, L
    EX DE, HL
    
; Another pattern
    LD L, E
    LD H, D
    
; And the reverse
    LD H, D
    LD L, E`

	peephole := optimizer.NewAssemblyPeepholePass()
	output := peephole.OptimizeAssembly(input)
	
	fmt.Println("=== INPUT ===")
	fmt.Println(input)
	fmt.Println("\n=== OUTPUT ===")
	fmt.Println(output)
}