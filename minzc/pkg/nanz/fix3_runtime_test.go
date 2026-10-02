package nanz_test

import (
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pipeline"
	"strings"
	"testing"
)

func TestExternCannotClaimMultiplyRuntime(t *testing.T) {
	for _, attr := range []string{"@extern", `@extern(clobbers: "")`} {
		t.Run(attr, func(t *testing.T) {
			hm, err := nanz.Parse(attr+` fun __mul8(a: u8, b: u8) -> u8
fun g(x: u8, y: u8) -> u8 { return x * y }
fun main() -> u8 { return g(3, 5) + __mul8(2, 3) }`, "runtime.nanz")
			if err != nil {
				t.Fatal(err)
			}
			_, err = pipeline.CompileHIR(hm)
			if err == nil || !strings.Contains(err.Error(), "__mul8") {
				t.Fatalf("compile error = %v", err)
			}
		})
	}
}
