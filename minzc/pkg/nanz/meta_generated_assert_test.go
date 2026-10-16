package nanz_test

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pipeline"
)

func TestMetaGeneratedAssertionsExecute(t *testing.T) {
	for _, expected := range []int{7, 8} {
		t.Run(fmt.Sprint(expected), func(t *testing.T) {
			generated := fmt.Sprintf("fun f()->u8 {return 7}\nassert f()==%d\n", expected)
			source := "fun @build()->void {emit(c" + strconv.Quote(generated) + ")}\n@build()\n"
			m, err := nanz.Parse(source, "generated-assert.nanz")
			if err != nil {
				t.Fatal(err)
			}
			if len(m.Asserts) != 1 {
				t.Fatalf("generated assertion lost: %v", m.Asserts)
			}
			_, err = pipeline.CompileHIRSteps(m, pipeline.Options{ContractOpt: true})
			if (err == nil) != (expected == 7) {
				t.Fatalf("generated check expected%d err=%v", expected, err)
			}
		})
	}
}

func TestMetaGeneratedSandboxRetainsState(t *testing.T) {
	generated := "global counter:u8=0\nfun next()->u8 {counter=counter+1 return counter}\nsandbox \"generated\" {assert next()==1 assert next()==2}\n"
	source := "fun @build()->void {emit(c\"\"\"" + generated + "\"\"\")}\n@build()\n"
	m, err := nanz.Parse(source, "generated-sandbox.nanz")
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Sandboxes) != 1 || len(m.Sandboxes[0].Asserts) != 2 {
		t.Fatalf("generated sandbox lost: %v", m.Sandboxes)
	}
	if _, err = pipeline.CompileHIRSteps(m, pipeline.Options{ContractOpt: true}); err != nil {
		t.Fatal(err)
	}
}
