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
	for name, value := range map[string]string{"grace": "true", "hir-grace": "false"} {
		f := rootCmd.Flags().Lookup(name)
		if f == nil || f.DefValue != value {
			t.Errorf("--%s must default %s: %v", name, value, f)
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
	pointer, err := os.ReadFile("../../../examples/nanz/01_sum_array.nanz")
	if err != nil {
		t.Fatal(err)
	}
	source := string(pointer) + `
 fun difference(a:u8,b:u8)->u8 {
 while a!=b { if a>b { a=a-b } else { b=b-a } }
 return a
 }`
	assemblies := map[string]string{}
	for _, tc := range []struct {
		name     string
		args     []string
		mir, hir bool
	}{
		{"default", nil, true, false},
		{"no-grace", []string{"--no-grace"}, false, false},
		{"no-mir", []string{"--grace=false"}, false, false},
		{"hir-only", []string{"--grace=false", "--hir-grace"}, false, true},
		{"no-hir", []string{"--hir-grace=false"}, true, false},
		{"explicit", []string{"--grace", "--hir-grace"}, true, true},
		{"disable-both", []string{"--grace", "--hir-grace", "--no-grace"}, false, false},
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
			assemblies[tc.name] = string(got)
			if string(got) != want.Assembly {
				t.Fatal("CLI pass overrides differ from explicit pipeline options")
			}
		})
	}
	if assemblies["default"] == assemblies["no-grace"] {
		t.Fatal("fixture does not exercise MIR Grace")
	}
	if assemblies["explicit"] == assemblies["default"] || assemblies["hir-only"] == assemblies["no-grace"] {
		t.Fatal("pointer fixture does not exercise HIR Grace independently")
	}
}
