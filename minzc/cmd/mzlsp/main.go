package main

import (
	"fmt"
	"os"

	"github.com/minz/minzc/pkg/lsp"
)

func main() {
	server := lsp.NewServer()
	if err := server.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "mzlsp: %v\n", err)
		os.Exit(1)
	}
}
