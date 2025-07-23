package main

import (
	"fmt"
	"github.com/minz/minzc/pkg/ir"
)

func main() {
	fmt.Printf("OpLoadAddr = %d\n", ir.OpLoadAddr)
	fmt.Printf("OpCopyToBuffer = %d\n", ir.OpCopyToBuffer)
	fmt.Printf("OpCopyFromBuffer = %d\n", ir.OpCopyFromBuffer)
	fmt.Printf("OpDJNZ = %d\n", ir.OpDJNZ)
	fmt.Printf("OpLoadImm = %d\n", ir.OpLoadImm)
	fmt.Printf("OpAddImm = %d\n", ir.OpAddImm)
}