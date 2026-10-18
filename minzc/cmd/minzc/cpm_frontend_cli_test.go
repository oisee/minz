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

// EXPECT = SAP-documented frontend contract plus exact ASCII fixtures. Frontend
// services are unavailable in the independent abaplint reference runtime.
func TestABAPFrontendCPMCLI(t *testing.T) {
	applicationCLIState(t)
	const source = "../../../examples/abap/zpickfile_cpm.abap"
	files := map[string][]byte{}
	for path, want := range map[string]string{
		source:                                   "df1637cf8701472602df14f2e101395c36477d34507be6cb519e03aaa798c93a",
		"../../../examples/abap/fleet/FLEET.CSV": "4bcb4a855c60745e65513227695bc1b11701cd2b3511c1bb7ff1e421fe8f6f49",
		"../../../examples/abap/fleet/EXTRA.CSV": "cf07eca0dfcff2cc1e7160e30b917c63c18ea7119915f4724401eb2148dfbb62",
	} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if fmt.Sprintf("%x", sha256.Sum256(data)) != want {
			t.Fatal("frozen fixture modified", path)
		}
		files[filepath.Base(path)] = data
	}
	a, bin := compileApplicationPair(t, source)
	mze := buildApplicationMZE(t)
	const prompt = "P_FILE [EXTRA.CSV]: "
	const menu = "Choose CSV\r\n>1 EXTRA.CSV\r\n 2 FLEET.CSV\r\n"
	for _, tc := range []struct {
		name, keys, output, replacement string
		fault                           byte
	}{
		{name: "select", keys: "\x1bOS\x1b[B\r\r", output: prompt + menu + "\r\n 1 EXTRA.CSV\r\n>2 FLEET.CSV\r\nP_FILE [FLEET.CSV]: \r\n\r\n\r\nAtlas North 40\r\nBeacon South 32\r\nTotal seats: 72"},
		{name: "fragmented-select", keys: "\x1bOS\x1b\x00\x00[\x00\x00B\r\r", output: prompt + menu + "\r\n 1 EXTRA.CSV\r\n>2 FLEET.CSV\r\nP_FILE [FLEET.CSV]: \r\n\r\n\r\nAtlas North 40\r\nBeacon South 32\r\nTotal seats: 72"},
		{name: "cancel", keys: "\x1bOS\x1b\r", output: prompt + menu + prompt + "\r\n\r\n\r\nSpare Depot 8\r\nTotal seats: 8"},
		{name: "missing", keys: "MISSING.CSV\r", output: prompt + "MISSING.CSV\r\n\r\n\r\nFile not found"},
		{name: "capacity", keys: "\r", replacement: strings.Repeat("A,B,1\r\n", 17), output: prompt + "\r\n\r\nABAP runtime error 0x07\r\n", fault: 7},
		{name: "invalid-number", keys: "\r", replacement: "A,B,BAD\r\n", output: prompt + "\r\n\r\nABAP runtime error 0x04\r\n", fault: 4},
		{name: "empty-seat", keys: "\r", replacement: "A,B,\r\n", output: prompt + "\r\n\r\n\r\nA B 0\r\nTotal seats: 0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			image := filepath.Join(dir, "report.com")
			profile := filepath.Join(dir, "profile.json")
			for name, data := range files {
				if name == "zpickfile_cpm.abap" {
					continue
				}
				if name == "EXTRA.CSV" && tc.replacement != "" {
					data = []byte(tc.replacement)
				}
				if err := os.WriteFile(filepath.Join(dir, name), data, 0644); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(image, bin, 0644); err != nil {
				t.Fatal(err)
			}
			run := func(verbose bool) []byte {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				args := []string{image, "-t", "cpm", "--timeout", "100000000"}
				if verbose {
					args = append(args, "-v")
				} else {
					args = append(args, "--profile", profile)
				}
				cmd := exec.CommandContext(ctx, mze, args...)
				cmd.Stdin = strings.NewReader(tc.keys)
				var stderr bytes.Buffer
				cmd.Stderr = &stderr
				out, err := cmd.Output()
				if err != nil || !verbose && stderr.Len() != 0 {
					t.Fatalf("MZE %v stderr=%s output=%q", err, stderr.String(), out)
				}
				return out
			}
			out := run(false)
			if string(out) != tc.output {
				t.Fatalf("output=%q want=%q", out, tc.output)
			}
			data, err := os.ReadFile(profile)
			if err != nil {
				t.Fatal(err)
			}
			var prof struct {
				Meta        map[string]any    `json:"meta"`
				Write       map[string]uint32 `json:"write"`
				MemSnapshot map[string]string `json:"mem_snapshot"`
			}
			if err := json.Unmarshal(data, &prof); err != nil {
				t.Fatal(err)
			}
			minSP, err := strconv.ParseInt(fmt.Sprint(prof.Meta["stack_depth"]), 16, 32)
			if err != nil {
				t.Fatal(err)
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
				t.Fatalf("OS return: %s", regs)
			}
			if tc.fault != 0 && !bytes.Contains(regs, []byte(fmt.Sprintf("A=$%02X", tc.fault))) {
				t.Fatal("terminal fault A lost")
			}
			t.Logf("real MZE imageEnd=%04x stack=%04x..%04x minSP=%04x PC=0000 SP=FE00 canary=A5 fault=%d", 0x100+len(bin), bottom, top, minSP, tc.fault)
			if root := os.Getenv("MINZ_ABAP_FRONTEND_ARTIFACTS"); root != "" {
				if err := os.MkdirAll(root, 0755); err != nil {
					t.Fatal(err)
				}
				for ext, b := range map[string][]byte{"stdout": out, "profile.json": data, "registers.log": regs, "keys": []byte(tc.keys)} {
					if err := os.WriteFile(filepath.Join(root, tc.name+"."+ext), b, 0644); err != nil {
						t.Fatal(err)
					}
				}
			}
		})
	}
}
