package cparse

import (
	"strings"
	"testing"
)

func TestPredefinedClockDeterministic(t *testing.T) {
	for _, tc := range []struct{ epoch, date, clock string }{{"", "Jan  1 1970", "00:00:00"}, {"946684801", "Jan  1 2000", "00:00:01"}} {
		t.Setenv("SOURCE_DATE_EPOCH", tc.epoch)
		var out strings.Builder
		err := Preprocess(&Config{ABI: defaultABI()}, []Source{{Name: "clock.c", Value: "__DATE__ __TIME__\n"}}, &out)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out.String(), tc.date) || !strings.Contains(out.String(), tc.clock) {
			t.Fatalf("clock macros depend on wall clock: %q", out.String())
		}
	}
}
