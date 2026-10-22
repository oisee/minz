package main

import (
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pipeline"
	"os"
	"path/filepath"
	"testing"
)

func TestGraceCLIFlagsDefaultOn(t *testing.T) {
	for _, name := range []string{"grace", "hir-grace"} {
		f := rootCmd.Flags().Lookup(name)
		if f == nil || f.DefValue != "true" {
			t.Errorf("--%s must default true: %v", name, f)
		}
	}
	if rootCmd.Flags().Lookup("no-grace") == nil {
		t.Error("missing --no-grace escape hatch")
	}
}

func TestGraceCLIOverrideAssembly(t *testing.T) {
	outputTestState(t)
	oldTarget, oldSize := target, optSize
	t.Cleanup(func() { target, optSize = oldTarget, oldSize })
	target, optSize = "cpm", false
	flags := rootCmd.Flags()
	for _, name := range []string{"grace", "hir-grace", "no-grace"} {
		f := flags.Lookup(name)
		if f == nil {
			t.Fatalf("missing --%s", name)
		}
		value, changed := f.Value.String(), f.Changed
		t.Cleanup(func() { _ = f.Value.Set(value); f.Changed = changed })
	}
	const source = `fun difference(a:u8,b:u8)->u8 {
 while a!=b { if a>b { a=a-b } else { b=b-a } }
 return a
 }`
	for _, tc := range []struct {
		name     string
		args     []string
		mir, hir bool
	}{
		{"default", nil, true, true},
		{"no-grace", []string{"--no-grace"}, false, false},
		{"no-mir", []string{"--grace=false"}, false, true},
		{"no-hir", []string{"--hir-grace=false"}, true, false},
		{"explicit", []string{"--grace", "--hir-grace"}, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, name := range []string{"grace", "hir-grace", "no-grace"} {
				f := flags.Lookup(name)
				if err := f.Value.Set(f.DefValue); err != nil {
					t.Fatal(err)
				}
				f.Changed = false
			}
			if err := flags.Parse(tc.args); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "source.nanz")
			if err := os.WriteFile(path, []byte(source), 0600); err != nil {
				t.Fatal(err)
			}
			outputFile = filepath.Join(t.TempDir(), "result.a80")
			if err := compileViaHIR(path); err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(outputFile)
			if err != nil {
				t.Fatal(err)
			}
			h, err := nanz.Parse(source, path)
			if err != nil {
				t.Fatal(err)
			}
			h.Target = hir.TargetFromString(target)
			want, err := pipeline.CompileHIRSteps(h, pipeline.Options{ContractOpt: true, UseGrace: tc.mir, UseHIRGrace: tc.hir})
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != want.Assembly {
				t.Fatal("CLI pass overrides differ from explicit pipeline options")
			}
		})
	}
}
