// Research-only adapter: Crystal marker -> Nanz meta VM -> verified HIR.
package main

import (
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/internal/crystalmeta"
	"os"
	"strconv"
)

func run() error {
	if len(os.Args) != 4 || os.Args[1] != "repeat" {
		return fmt.Errorf("usage: crystal-meta-bridge repeat COUNT DELTA")
	}
	count, err := strconv.Atoi(os.Args[2])
	if err != nil {
		return fmt.Errorf("invalid count: %w", err)
	}
	delta, err := strconv.Atoi(os.Args[3])
	if err != nil {
		return fmt.Errorf("invalid delta: %w", err)
	}
	out, err := crystalmeta.GenerateRepeat(count, delta)
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(out)
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
