package nanz

import (
	"github.com/minz/minzc/pkg/hir"
	"reflect"
	"testing"
)

func TestInterfaceCandidatesDeterministic(t *testing.T) {
	p := &parser{interfaces: map[string]*hir.InterfaceDecl{"I": {Methods: []string{"read"}}}, methodTable: map[string]map[string]methodInfo{"A": {"read": {}}, "B": {"read": {}}, "C": {"other": {}}}}
	for run := 0; run < 100; run++ {
		if got := p.findImplementors("I", "read"); !reflect.DeepEqual(got, []string{"A", "B"}) {
			t.Fatalf("unstable ambiguity diagnostic: %v", got)
		}
	}
}
