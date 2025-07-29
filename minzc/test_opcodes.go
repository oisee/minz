package main

import (
	"fmt"
	"github.com/minz/minzc/pkg/ir"
)

func main() {
	fmt.Printf("OpLoad = %d\n", ir.OpLoad)
	fmt.Printf("OpStore = %d\n", ir.OpStore)
}