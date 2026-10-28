package main

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

// Hand-assembled CP/M 3 test programs.
var cpm3TestPrograms = map[string]string{
	// LD C,6Ch / LD DE,0 / CALL 5 / RET
	"OK.COM": "0e6c110000cd0500c9",
	// LD C,6Ch / LD DE,0FF05h / CALL 5 / RET
	"FAIL.COM": "0e6c1105ffcd0500c9",
	// read the code (DE=0FFFFh), set code+1
	"BUMP.COM": "0e6c11ffffcd050023eb0e6ccd0500c9",
	// set 1234h, copy "BUMP\0" to 80h, P_CHAIN; must not reach the
	// following set of 0FF7Fh
	"CHAINER.COM": "0e6c113412cd0500212301118000010500edb00e2f1e00cd05000e6c117fffcd0500c942554d5000",
}

func runCPM3Job(t *testing.T, job string) uint16 {
	t.Helper()
	dir := t.TempDir()
	for name, code := range cpm3TestPrograms {
		b, err := hex.DecodeString(code)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(dir, "JOB.SUB")
	if err := os.WriteFile(path, []byte(job), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cpm3, cpmDirOverride = nil, "" })
	code, err := runSubmit(path)
	if err != nil {
		t.Fatal(err)
	}
	return code
}

func TestCPM3SubmitReturnCodes(t *testing.T) {
	cases := []struct {
		name, job string
		want      uint16
	}{
		{"fatal code skips conditional line", "OK\nFAIL\n:OK\n", 0xFF05},
		{"CCP resets the code before each program", "FAIL\nOK\n", 0x0000},
		{"comments and blank lines are ignored", "; note\n\nFAIL\n", 0xFF05},
		{"chained program sees the caller's code", "CHAINER\n", 0x1235},
		{"conditional runs below 0FF00h and starts from zero", "CHAINER\n:BUMP\n", 0x0001},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := runCPM3Job(t, tc.job); got != tc.want {
				t.Fatalf("return code %04Xh, want %04Xh", got, tc.want)
			}
		})
	}
}

func TestCPM3FunctionsInactiveOutsideSubmit(t *testing.T) {
	if cpm3 != nil {
		t.Fatal("CP/M 3 state must be nil outside the SUBMIT runner")
	}
	if _, _, ok := cpm3BDOS(nil, 0x6C, 0xFFFF); ok {
		t.Fatal("P_CODE must be unhandled by the plain CP/M 2.2 target")
	}
}
