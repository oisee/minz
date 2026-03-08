package mir2_test

import (
	"fmt"
	"testing"

	"github.com/minz/minzc/pkg/mir2"
)

func TestDumpFibonacci(t *testing.T) {
	m := &mir2.Module{Name: "fib"}
	buildFib(m)
	fmt.Println(m.Dump())
}
