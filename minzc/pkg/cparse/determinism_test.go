package cparse

import (
	"os"
	"strings"
	"testing"
	"time"
)

func preprocessClock() (string, error) {
	var out strings.Builder
	err := Preprocess(&Config{ABI: defaultABI()}, []Source{{Name: "clock.c", Value: "__DATE__ __TIME__\n"}}, &out)
	return out.String(), err
}

func TestPredefinedClockDeterministic(t *testing.T) {
	t.Setenv("SOURCE_DATE_EPOCH", "946684801")
	out, err := preprocessClock()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Jan  1 2000") || !strings.Contains(out, "00:00:01") {
		t.Fatalf("unexpected clock: %q", out)
	}
}

func TestPredefinedClockUnset(t *testing.T) {
	t.Setenv("SOURCE_DATE_EPOCH", "")
	if err := os.Unsetenv("SOURCE_DATE_EPOCH"); err != nil {
		t.Fatal(err)
	}
	before := time.Now()
	out, err := preprocessClock()
	after := time.Now()
	if err != nil {
		t.Fatal(err)
	}
	if (!strings.Contains(out, before.Format("Jan _2 2006")) && !strings.Contains(out, after.Format("Jan _2 2006"))) ||
		(!strings.Contains(out, before.Format("15:04:05")) && !strings.Contains(out, after.Format("15:04:05"))) {
		t.Fatalf("clock does not use wall time: %q", out)
	}
}

func TestPredefinedClockInvalidEpoch(t *testing.T) {
	for _, epoch := range []string{"", "garbage", "-1", "1.5", "253402300800", "9223372036854775808"} {
		t.Run(epoch, func(t *testing.T) {
			t.Setenv("SOURCE_DATE_EPOCH", epoch)
			_, err := preprocessClock()
			if err == nil || !strings.Contains(err.Error(), "SOURCE_DATE_EPOCH") {
				t.Fatalf("expected clear epoch error, got %v", err)
			}
		})
	}
}
