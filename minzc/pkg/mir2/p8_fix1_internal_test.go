package mir2

import (
	"fmt"
	"strings"
	"testing"
)

func TestP8WideSpillExhaustionFails(t *testing.T) {
	defer func() {
		err := recover()
		if err == nil || !strings.Contains(fmt.Sprint(err), "needs 1 wide spill staging pairs, only 0 available") {
			t.Fatalf("expected explicit staging failure, got %v", err)
		}
	}()
	wideSpillPairs("full", OpAdd, map[string]bool{"HL": true, "DE": true, "BC": true}, 1)
}
