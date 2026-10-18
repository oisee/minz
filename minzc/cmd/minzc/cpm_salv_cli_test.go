package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Actual MZE executes the unchanged complete F4/CSV/SALV report. Cell-by-cell
// oracle checks and typed-memory canaries also run in pkg/abap; these cases prove
// public CLI assembly/COM equivalence, real BDOS I/O, faults, and OS return.
func TestABAPSALVCPMCLI(t *testing.T) {
	applicationCLIState(t)
	if root := os.Getenv("MINZ_ABAP_SALV_ARTIFACTS"); root != "" {
		t.Setenv("MINZ_ABAP_CPM_ARTIFACTS", root)
	}
	const source = "../../../examples/abap/zfleet_csv_alv.abap"
	src, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprintf("%x", sha256.Sum256(src)) != "50a48e0726154b3d37115bfdcd09d4f6222c7c2e3380293128cb489c75fb7979" {
		t.Fatal("frozen source changed")
	}
	a, bin := compileApplicationPair(t, source)
	mze := buildApplicationMZE(t)
	files := map[string][]byte{}
	for name, sha := range map[string]string{"FLEET.CSV": "4bcb4a855c60745e65513227695bc1b11701cd2b3511c1bb7ff1e421fe8f6f49", "EXTRA.CSV": "cf07eca0dfcff2cc1e7160e30b917c63c18ea7119915f4724401eb2148dfbb62"} {
		data, e := os.ReadFile("../../../examples/abap/fleet/" + name)
		if e != nil {
			t.Fatal(e)
		}
		if fmt.Sprintf("%x", sha256.Sum256(data)) != sha {
			t.Fatal("frozen CSV changed")
		}
		files[name] = data
	}
	for _, tc := range []struct {
		name, keys, replacement string
		want                    []string
		fault                   byte
	}{
		{name: "fleet", keys: "\x1bOS\x1b[B\r\rq", want: []string{"Total seats: 72", "ALV Table | Rows: 2 | First: 1", "|Atlas               |North               |         40|", "|Beacon              |South               |         32|"}},
		{name: "cancel-then-extra", keys: "\x1bOS\x1b\rq", want: []string{"Total seats: 8", "ALV Table | Rows: 1 | First: 1", "|Spare               |Depot               |          8|"}},
		{name: "missing", keys: "MISSING.CSV\r", want: []string{"File not found"}},
		{name: "capacity", keys: "\r", replacement: strings.Repeat("A,B,1\r\n", 17), want: []string{"ABAP runtime error 0x07\r\n"}, fault: 7},
		{name: "invalid-number", keys: "\r", replacement: "A,B,BAD\r\n", want: []string{"ABAP runtime error 0x04\r\n"}, fault: 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			image := filepath.Join(dir, "report.com")
			profile := filepath.Join(dir, "profile.json")
			if e := os.WriteFile(image, bin, 0644); e != nil {
				t.Fatal(e)
			}
			for name, data := range files {
				if name == "EXTRA.CSV" && tc.replacement != "" {
					data = []byte(tc.replacement)
				}
				if e := os.WriteFile(filepath.Join(dir, name), data, 0644); e != nil {
					t.Fatal(e)
				}
			}
			run := func(verbose bool) []byte {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				args := []string{image, "-t", "cpm", "--timeout", "200000000"}
				if verbose {
					args = append(args, "-v")
				} else {
					args = append(args, "--profile", profile)
				}
				cmd := exec.CommandContext(ctx, mze, args...)
				cmd.Stdin = strings.NewReader(tc.keys)
				var stderr bytes.Buffer
				cmd.Stderr = &stderr
				out, e := cmd.Output()
				if e != nil || !verbose && stderr.Len() != 0 {
					t.Fatalf("MZE %v stderr=%s out=%q", e, stderr.String(), out)
				}
				return out
			}
			out := run(false)
			for _, want := range tc.want {
				if !bytes.Contains(out, []byte(want)) {
					t.Fatalf("missing %q in %q", want, out)
				}
			}
			hasGrid := bytes.Contains(out, []byte("ALV Table | Rows:"))
			if hasGrid != (tc.name == "fleet" || tc.name == "cancel-then-extra") {
				t.Fatal("unexpected grid reachability")
			}
			if hasGrid && !bytes.HasSuffix(out, []byte("\x1b[0m\x1b[?25h\x1b[24;1H\x1b[2K\r\n")) {
				t.Fatal("terminal state not restored")
			}
			data, e := os.ReadFile(profile)
			if e != nil {
				t.Fatal(e)
			}
			var prof struct {
				Meta        map[string]any    `json:"meta"`
				Write       map[string]uint32 `json:"write"`
				MemSnapshot map[string]string `json:"mem_snapshot"`
			}
			if e := json.Unmarshal(data, &prof); e != nil {
				t.Fatal(e)
			}
			minSP, e := strconv.ParseInt(fmt.Sprint(prof.Meta["stack_depth"]), 16, 32)
			if e != nil {
				t.Fatal(e)
			}
			bottom, top, guard := a.Symbols["__cpm_app_stack"], a.Symbols["__cpm_app_stack_top"], a.Symbols["__cpm_app_stack_canary"]
			if int(minSP) < bottom || int(minSP) >= top || prof.Write[fmt.Sprintf("%04X", guard)] != 0 || bin[guard-0x100] != 0xa5 {
				t.Fatal("resident stack/canary", minSP, bottom, top)
			}
			if tc.fault != 0 && prof.MemSnapshot[fmt.Sprintf("%04X", a.Symbols["_abap_error_code"])] != fmt.Sprintf("%02X", tc.fault) {
				t.Fatal("stored fault lost")
			}
			regs := run(true)
			if !bytes.Contains(regs, []byte("PC=$0000  SP=$FE00")) {
				t.Fatalf("OS return %s", regs)
			}
			if tc.fault != 0 && !bytes.Contains(regs, []byte(fmt.Sprintf("A=$%02X", tc.fault))) {
				t.Fatal("fault A lost")
			}
			t.Logf("real MZE COM%d imageEnd%04x stack%04x..%04x minSP%04x OSreturn PC0000 SPFE00 fault%d", len(bin), 0x100+len(bin), bottom, top, minSP, tc.fault)
			if root := os.Getenv("MINZ_ABAP_SALV_ARTIFACTS"); root != "" {
				if e := os.MkdirAll(root, 0755); e != nil {
					t.Fatal(e)
				}
				for ext, b := range map[string][]byte{"stdout": out, "profile.json": data, "registers.log": regs, "keys": []byte(tc.keys)} {
					if e := os.WriteFile(filepath.Join(root, "cli-"+tc.name+"."+ext), b, 0644); e != nil {
						t.Fatal(e)
					}
				}
			}
		})
	}
}
