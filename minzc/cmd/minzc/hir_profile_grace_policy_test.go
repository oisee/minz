package main

import (
	"strings"
	"testing"
)

func TestHIRProfileGraceCompilationPolicy(t *testing.T) {
	flags := rootCmd.Flags()
	for _, name := range []string{"grace", "hir-grace", "no-grace"} {
		f := flags.Lookup(name)
		value, changed := f.Value.String(), f.Changed
		t.Cleanup(func() { _ = f.Value.Set(value); f.Changed = changed })
	}
	for _, tc := range []struct {
		name           string
		args           []string
		joint, refused bool
	}{
		{name: "linear-default"},
		{name: "joint-default", joint: true},
		{name: "linear-explicit-mir", args: []string{"--grace"}, refused: true},
		{name: "joint-explicit-mir", args: []string{"--grace"}, joint: true, refused: true},
		{name: "linear-explicit-hir", args: []string{"--hir-grace"}, refused: true},
		{name: "joint-explicit-hir", args: []string{"--hir-grace"}, joint: true},
		{name: "escape-hatch", args: []string{"--grace", "--hir-grace", "--no-grace"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, name := range []string{"grace", "hir-grace", "no-grace"} {
				f := flags.Lookup(name)
				_ = f.Value.Set(f.DefValue)
				f.Changed = false
			}
			if err := flags.Parse(tc.args); err != nil {
				t.Fatal(err)
			}
			err := validateHIRProfileGracePolicy(tc.joint)
			if (err != nil) != tc.refused {
				t.Fatalf("refused=%v err=%v", tc.refused, err)
			}
			if err != nil && !strings.Contains(err.Error(), "--no-grace") {
				t.Fatal(err)
			}
		})
	}
}
