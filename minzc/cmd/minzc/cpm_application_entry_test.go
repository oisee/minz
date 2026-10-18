package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/minz/minzc/pkg/abap"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/z80asm"
)

func applicationCLIState(t *testing.T) {
	t.Helper()
	hirProfileTestState(t)
	old, stack := runtimeMode, runtimeStackBytes
	t.Cleanup(func() { runtimeMode, runtimeStackBytes = old, stack })
	runtimeMode, runtimeStackBytes = "", 0
}

func compileApplicationPair(t *testing.T, source string) (*z80asm.Result, []byte) {
	t.Helper()
	dir := t.TempDir()
	outputFile = filepath.Join(dir, "report.com")
	if err := compile(source); err != nil {
		t.Fatal(err)
	}
	bin, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatal(err)
	}
	outputFile = filepath.Join(dir, "report.a80")
	if err := compile(source); err != nil {
		t.Fatal(err)
	}
	text, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatal(err)
	}
	a, err := z80asm.NewAssembler().AssembleString(string(text))
	if err != nil || len(a.Errors) != 0 {
		t.Fatalf("assemble: %v %v", err, a.Errors)
	}
	if a.Origin != 0x100 || !bytes.Equal(bin, a.Binary) {
		t.Fatal("ordinary CLI .a80/.com byte mismatch")
	}
	if !bytes.Contains(text, []byte("__cpm_app_entry:")) || bytes.Contains(text, []byte("__cpm_entry:")) {
		t.Fatal("wrong application contract")
	}
	if dir := os.Getenv("MINZ_ABAP_CPM_ARTIFACTS"); dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		name := strings.TrimSuffix(filepath.Base(source), ".abap")
		sourceBytes, err := os.ReadFile(source)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name+".source.sha256"), []byte(fmt.Sprintf("%x\n", sha256.Sum256(sourceBytes))), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name+".abap"), sourceBytes, 0644); err != nil {
			t.Fatal(err)
		}
		for ext, data := range map[string][]byte{"a80": text, "com": bin} {
			if err := os.WriteFile(filepath.Join(dir, name+"."+ext), data, 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
	return a, bin
}

func runApplication(t *testing.T, name string, a *z80asm.Result, bin []byte, wantOutput string, wantObjects map[string][]byte, fault byte) {
	t.Helper()
	z := emulator.NewRemogattoZ80WithScreen()
	z.SetROMEnd(0)
	mem := z.Memory()
	copy(mem, bytes.Repeat([]byte{0xa5}, 65536))
	copy(mem[0x100:], bin)
	binary.LittleEndian.PutUint16(mem[6:8], 0xFE00)
	// Deliberately give the application no caller stack budget: the sole return
	// word is in low CP/M memory, independently of its resident stack.
	const callerSP = 0x80
	const sentinel = 0xe000
	binary.LittleEndian.PutUint16(mem[callerSP:callerSP+2], sentinel)
	before := append([]byte(nil), mem...)
	z.SetRegisters(emulator.Registers{PC: 0x100, SP: callerSP})
	stack, top := a.Symbols["__cpm_app_stack"], a.Symbols["__cpm_app_stack_top"]
	if top-stack != mir2.CPMApplicationStackBytes || a.Symbols["__cpm_app_image_end"] != 0x100+len(bin) {
		t.Fatal("resident stack layout")
	}
	minSP := top
	var out []byte
	for n := 0; n < 30000000 && z.GetPC() != sentinel && !z.IsHalted(); n++ {
		pc, sp := int(z.GetPC()), int(z.GetSP())
		if sp != callerSP && (sp < stack || sp > top) {
			t.Fatalf("SP outside resident/caller stack: %04x", sp)
		}
		if sp >= stack && sp < minSP {
			minSP = sp
		}
		if pc == 5 {
			r := z.GetRegisters()
			if byte(r.BC) != 2 {
				t.Fatalf("unexpected BDOS %d", byte(r.BC))
			}
			out = append(out, byte(r.DE))
			r.PC = binary.LittleEndian.Uint16(mem[r.SP : r.SP+2])
			r.SP += 2
			z.SetRegisters(r)
		} else {
			if pc < 0x100 || pc >= 0x100+len(bin) {
				t.Fatalf("foreign fetch %04x", pc)
			}
			z.Step()
		}
	}
	if z.IsHalted() || z.GetPC() != sentinel || z.GetSP() != callerSP+2 {
		t.Fatalf("application return halted=%t PC=%04x SP=%04x", z.IsHalted(), z.GetPC(), z.GetSP())
	}
	if string(out) != wantOutput {
		t.Fatalf("output %q want %q", out, wantOutput)
	}
	if fault != 0 && z.GetRegisters().A != fault {
		t.Fatalf("terminal status A=%02x want %02x", z.GetRegisters().A, fault)
	}
	if mem[a.Symbols["_abap_error_code"]] != fault {
		t.Fatalf("fault %d want %d", mem[a.Symbols["_abap_error_code"]], fault)
	}
	if minSP == top || mem[a.Symbols["__cpm_app_stack_canary"]] != 0xa5 {
		t.Fatalf("resident stack minSP=%04x canary=%02x", minSP, mem[a.Symbols["__cpm_app_stack_canary"]])
	}
	if binary.LittleEndian.Uint16(mem[a.Symbols["__cpm_app_saved_sp"]:]) != callerSP {
		t.Fatal("incoming SP not saved")
	}
	for i, b := range mem {
		if i >= 0x100 && i < 0x100+len(bin) {
			continue
		}
		if b != before[i] {
			t.Fatalf("foreign write %04x", i)
		}
	}
	for name, want := range wantObjects {
		addr, ok := a.Symbols[name]
		if !ok {
			addr, ok = a.Symbols["v_"+name]
		}
		if !ok {
			t.Fatal("missing", name)
		}
		if !bytes.Equal(mem[addr:addr+len(want)], want) {
			t.Fatalf("%s=% x want % x", name, mem[addr:addr+len(want)], want)
		}
	}
	t.Logf("%s imageEnd=%04x stack=%04x..%04x minSP=%04x used=%d PC=%04x SP=%04x canary=A5 fault=%d", name, 0x100+len(bin), stack, top, minSP, top-minSP, z.GetPC(), z.GetSP(), fault)
	if dir := os.Getenv("MINZ_ABAP_CPM_ARTIFACTS"); dir != "" {
		data, _ := json.MarshalIndent(map[string]any{"image_sha256": fmt.Sprintf("%x", sha256.Sum256(bin)), "image_end": 0x100 + len(bin), "stack_bottom": stack, "stack_top": top, "min_sp": minSP, "final_pc": z.GetPC(), "final_sp": z.GetSP(), "canary": 165, "fault": fault, "output": string(out), "symbols": a.Symbols}, "", "  ")
		if err := os.WriteFile(filepath.Join(dir, name+"-native.json"), data, 0644); err != nil {
			t.Fatal(err)
		}
	}
}

// EXPECT = unchanged report executed by the pinned independent ABAP runtime.
func applicationReference(t *testing.T, source string) (string, map[string][]byte) {
	t.Helper()
	src, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(strings.TrimSuffix(source, ".abap") + ".json")
	if err != nil {
		t.Fatal(err)
	}
	var ref struct {
		SHA      string                     `json:"source_sha256"`
		Provider string                     `json:"provider"`
		Entry    string                     `json:"entry"`
		Output   string                     `json:"output"`
		Values   map[string]json.RawMessage `json:"values"`
		SY       map[string]int32           `json:"sy"`
	}
	if err := json.Unmarshal(data, &ref); err != nil {
		t.Fatal(err)
	}
	if ref.SHA != fmt.Sprintf("%x", sha256.Sum256(src)) || ref.Provider != "@abaplint/transpiler + @abaplint/runtime 2.14.8" || ref.Entry != "full report" {
		t.Fatal("reference identity")
	}
	p, err := abap.Parse(string(src), "ref")
	if err != nil {
		t.Fatal(err)
	}
	types := map[string]*abap.TypesDecl{}
	decls := map[string]*abap.DataDecl{}
	for _, d := range p.Decls {
		switch d := d.(type) {
		case *abap.TypesDecl:
			types[d.Name] = d
		case *abap.DataDecl:
			decls[d.Name] = d
		}
	}
	encode := func(ty string, raw json.RawMessage) []byte {
		var row map[string]json.RawMessage
		if err := json.Unmarshal(raw, &row); err != nil {
			t.Fatal(err)
		}
		var image []byte
		for _, c := range types[ty].Components {
			switch c.AbapTy {
			case "i":
				var n int32
				if err := json.Unmarshal(row[c.Name], &n); err != nil {
					t.Fatal(err)
				}
				image = binary.LittleEndian.AppendUint32(image, uint32(n))
			case "c":
				var s string
				if err := json.Unmarshal(row[c.Name], &s); err != nil {
					t.Fatal(err)
				}
				length := c.Length
				if length == 0 {
					length = 1
				}
				if len(s) != length {
					t.Fatal("reference character length")
				}
				image = append(image, s...)
			}
		}
		return image
	}
	objects := map[string][]byte{}
	for name, raw := range ref.Values {
		d := decls[name]
		if d == nil {
			t.Fatal("missing DATA", name)
		}
		if d.Table != nil {
			var rows []json.RawMessage
			if err := json.Unmarshal(raw, &rows); err != nil {
				t.Fatal(err)
			}
			for _, row := range rows {
				objects[name] = append(objects[name], encode(d.AbapTy, row)...)
			}
			objects["_itab_count_"+name] = binary.LittleEndian.AppendUint32(nil, uint32(len(rows)))
		} else if types[d.AbapTy] != nil {
			objects[name] = encode(d.AbapTy, raw)
		} else if d.AbapTy == "i" {
			var n int32
			if err := json.Unmarshal(raw, &n); err != nil {
				t.Fatal(err)
			}
			objects[name] = binary.LittleEndian.AppendUint32(nil, uint32(n))
		} else {
			t.Fatal("unsupported reference object", name)
		}
	}
	if len(ref.SY) != 3 {
		t.Fatal("missing SY observations")
	}
	for field, n := range ref.SY {
		objects["_abap_"+field+"_value"] = binary.LittleEndian.AppendUint32(nil, uint32(n))
	}
	return ref.Output, objects
}

func TestABAPCPMApplicationReferenceCLI(t *testing.T) {
	applicationCLIState(t)
	mze := buildApplicationMZE(t)
	for _, name := range []string{"fleet", "ledger"} {
		t.Run(name, func(t *testing.T) {
			source := "../../pkg/abap/testdata/itab-reference/" + name + ".abap"
			out, objects := applicationReference(t, source)
			a, bin := compileApplicationPair(t, source)
			runApplication(t, name, a, bin, out, objects, 0)
			runApplicationMZE(t, mze, name, a, bin, out, 0)
		})
	}
}

func TestABAPCPMApplicationTailAndNestedFaults(t *testing.T) {
	applicationCLIState(t)
	mze := buildApplicationMZE(t)
	for _, tc := range []struct {
		name, src string
		fault     byte
		objects   map[string][]byte
	}{
		{"tail", `REPORT ztail. DATA seen TYPE i. PERFORM helper. FORM helper. seen = 42. ENDFORM.`, 0, map[string][]byte{"seen": {42, 0, 0, 0}}},
		{"capacity", `REPORT zfault.
TYPES: BEGIN OF ty, n TYPE i, END OF ty.
DATA row TYPE ty.
DATA rows TYPE STANDARD TABLE OF ty WITH EMPTY KEY INITIAL SIZE 2.
DATA suffix TYPE i.
row-n = 41.
APPEND row TO rows.
PERFORM outer.
suffix = 99.
FORM outer.
 PERFORM inner.
 suffix = 88.
ENDFORM.
FORM inner.
 APPEND row TO rows.
 APPEND row TO rows.
 suffix = 77.
ENDFORM.`, 7, map[string][]byte{"rows": {41, 0, 0, 0, 41, 0, 0, 0}, "_itab_count_rows": {2, 0, 0, 0}, "suffix": {0, 0, 0, 0}}},
		{"overflow", `REPORT zoverflow.
DATA n TYPE i VALUE 2147483647.
DATA suffix TYPE i.
PERFORM outer.
suffix = 99.
FORM outer.
 PERFORM inner.
 suffix = 88.
ENDFORM.
FORM inner.
 n = n + 1.
 suffix = 77.
ENDFORM.`, 2, map[string][]byte{"n": {255, 255, 255, 127}, "suffix": {0, 0, 0, 0}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := filepath.Join(t.TempDir(), tc.name+".abap")
			if err := os.WriteFile(source, []byte(tc.src), 0644); err != nil {
				t.Fatal(err)
			}
			a, bin := compileApplicationPair(t, source)
			wantOutput := ""
			if tc.fault != 0 {
				wantOutput = fmt.Sprintf("ABAP runtime error 0x%02X\r\n", tc.fault)
			}
			runApplicationMZE(t, mze, tc.name, a, bin, wantOutput, tc.fault)
			runApplication(t, tc.name, a, bin, wantOutput, tc.objects, tc.fault)
		})
	}
}

func TestABAPCPMApplicationExplicitCOM(t *testing.T) {
	applicationCLIState(t)
	oldFormat := outputFormat
	t.Cleanup(func() { outputFormat = oldFormat })
	source := filepath.Join(t.TempDir(), "explicit.abap")
	if err := os.WriteFile(source, []byte("REPORT zexplicit. DATA seen TYPE i. seen = 42."), 0644); err != nil {
		t.Fatal(err)
	}
	outputFormat = ""
	_, implicit := compileApplicationPair(t, source)
	outputFormat = "com"
	a, explicit := compileApplicationPair(t, source)
	if !bytes.Equal(implicit, explicit) {
		t.Fatal("explicit COM changes application bytes")
	}
	runApplication(t, "explicit-com", a, explicit, "", map[string][]byte{"seen": {42, 0, 0, 0}}, 0)
	runApplicationMZE(t, buildApplicationMZE(t), "explicit-com", a, explicit, "", 0)
}

func TestABAPCPMApplicationSelection(t *testing.T) {
	applicationCLIState(t)
	hm := &hir.Module{}
	for _, be := range []string{"", "z80"} {
		backend = be
		if !ordinaryABAPCPMEntry("z.abap", hm, false, "none") {
			t.Fatalf("default backend %q not selected", be)
		}
	}
	backend = "z80"
	for _, tc := range []struct {
		name  string
		set   func()
		reset func()
	}{
		{"guarded", func() { runtimeMode = "cpm" }, func() { runtimeMode = "" }},
		{"generic", func() { target = "generic" }, func() { target = "cpm" }},
		{"ZX", func() { target = "zxspectrum" }, func() { target = "cpm" }},
		{"ez80", func() { backend = "ez80" }, func() { backend = "z80" }},
		{"HIR", func() { emitFormat = "hir" }, func() { emitFormat = "" }},
		{"MIR", func() { emitFormat = "mir2" }, func() { emitFormat = "" }},
		{"raw code", func() { outputFormat = "code" }, func() { outputFormat = "" }},
		{"snapshot", func() { outputFormat = "sna" }, func() { outputFormat = "" }},
		{"profile", func() { hirProfile.profile = "p.json" }, func() { hirProfile = hirProfileFlags{} }},
	} {
		tc.set()
		if ordinaryABAPCPMEntry("z.abap", hm, false, "none") {
			t.Error("unexpected selection", tc.name)
		}
		tc.reset()
	}
	if ordinaryABAPCPMEntry("z.nanz", hm, false, "none") || ordinaryABAPCPMEntry("z.abap", hm, true, "none") {
		t.Fatal("non-application selection")
	}
	hm.Asserts = []hir.Assert{{}}
	if ordinaryABAPCPMEntry("z.abap", hm, false, "mir2") {
		t.Fatal("assert workload selected")
	}
	runtimeStackBytes = 1024
	if err := validateCLIRuntime("z.abap"); err == nil {
		t.Fatal("--runtime-stack admitted without --runtime")
	}
	runtimeStackBytes = 0
	// Actual intermediate products must not acquire entry semantics.
	source := filepath.Join(t.TempDir(), "mode.abap")
	if err := os.WriteFile(source, []byte("REPORT zmode. DATA n TYPE i. n = 1."), 0644); err != nil {
		t.Fatal(err)
	}
	for _, format := range []string{"hir", "mir2"} {
		emitFormat = format
		outputFile = filepath.Join(t.TempDir(), "mode."+format)
		if err := compile(source); err != nil {
			t.Fatal(err)
		}
		b, err := os.ReadFile(outputFile)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(b, []byte("__cpm_app_")) {
			t.Fatal("entry leaked into intermediate")
		}
	}
	emitFormat = ""
}

func TestABAPCPMApplicationRejectsInsufficientTPA(t *testing.T) {
	applicationCLIState(t)
	source := filepath.Join(t.TempDir(), "boundary.abap")
	if err := os.WriteFile(source, []byte("REPORT zboundary. DATA seen TYPE i. seen = 42."), 0644); err != nil {
		t.Fatal(err)
	}
	a, bin := compileApplicationPair(t, source)
	end := uint16(a.Symbols["__cpm_app_image_end"])
	for _, tc := range []struct {
		name    string
		tpa, sp uint16
	}{
		{"TPA ends within image", end - 1, 0xeffe},
		{"return overlaps image", 0xfe00, end - 2},
		{"return crosses image start", 0xfe00, 0xff},
		{"return beyond TPA", 0xfe00, 0xfe00},
		{"return wraps", 0xfe00, 0xffff},
	} {
		t.Run(tc.name, func(t *testing.T) {
			z := emulator.NewRemogattoZ80WithScreen()
			z.SetROMEnd(0)
			mem := z.Memory()
			copy(mem, bytes.Repeat([]byte{0xa5}, 65536))
			copy(mem[0x100:], bin)
			binary.LittleEndian.PutUint16(mem[6:8], tc.tpa)
			// The high byte at SP=00FF overlaps opcode 0100 (LD HL,nn).
			// Preserve that opcode so this case exercises the real guard.
			const sentinel = 0x2a00
			mem[tc.sp] = byte(sentinel & 0xff)
			mem[uint16(tc.sp+1)] = byte(sentinel >> 8)
			before := append([]byte(nil), mem...)
			z.SetRegisters(emulator.Registers{PC: 0x100, SP: tc.sp})
			for n := 0; n < 200 && z.GetPC() != sentinel && !z.IsHalted(); n++ {
				z.Step()
			}
			if z.GetPC() != sentinel || z.GetSP() != tc.sp+2 || z.IsHalted() {
				t.Fatalf("rejection PC=%04x SP=%04x", z.GetPC(), z.GetSP())
			}
			if !bytes.Equal(mem, before) {
				t.Fatal("rejected entry changed memory before returning")
			}
		})
	}
}

// Exercise the real MZE executable's standard CP/M loader and BDOS path. The
// semantic trap harness above remains an independent stack/memory observer.
func buildApplicationMZE(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "mze")
	cmd := exec.Command(filepath.Join(runtime.GOROOT(), "bin", "go"), "build", "-o", path, "../mze")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build MZE: %v\n%s", err, out)
	}
	return path
}

func runApplicationMZE(t *testing.T, mze, name string, a *z80asm.Result, bin []byte, want string, fault byte) {
	t.Helper()
	dir := t.TempDir()
	if root := os.Getenv("MINZ_ABAP_CPM_ARTIFACTS"); root != "" {
		dir = root
	}
	image := filepath.Join(dir, name+"-mze.com")
	profile := filepath.Join(dir, name+"-mze-profile.json")
	if err := os.WriteFile(image, bin, 0644); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, mze, "-t", "cpm", "--timeout", "1000000000", "--profile", profile, image)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil || string(out) != want {
		t.Fatalf("MZE normal BDOS: %v output=%q want=%q stderr=%s", err, out, want, stderr.String())
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
	depth, ok := prof.Meta["stack_depth"].(string)
	if !ok {
		t.Fatal("MZE did not measure stack")
	}
	minSP, err := strconv.ParseInt(depth, 16, 32)
	if err != nil {
		t.Fatal(err)
	}
	bottom, top := a.Symbols["__cpm_app_stack"], a.Symbols["__cpm_app_stack_top"]
	guard := a.Symbols["__cpm_app_stack_canary"]
	if int(minSP) < bottom || int(minSP) >= top || prof.Write[fmt.Sprintf("%04X", guard)] != 0 || bin[guard-0x100] != 0xa5 {
		t.Fatal("MZE resident stack/canary", minSP, bottom, top)
	}
	verbose := exec.CommandContext(ctx, mze, "-t", "cpm", "-v", "--timeout", "1000000000", image)
	regs, err := verbose.Output()
	if err != nil {
		t.Fatalf("MZE verbose: %v", err)
	}
	if fault != 0 {
		if prof.MemSnapshot[fmt.Sprintf("%04X", a.Symbols["_abap_error_code"])] != fmt.Sprintf("%02X", fault) {
			t.Fatal("MZE lost stored ABAP fault", fault)
		}
		if !bytes.Contains(regs, []byte(fmt.Sprintf("A=$%02X", fault))) {
			t.Fatalf("MZE lost terminal A status: %s", regs)
		}
	}
	if !bytes.Contains(regs, []byte("PC=$0000  SP=$FE00")) {
		t.Fatalf("MZE did not restore CP/M caller: %s", regs)
	}
	t.Logf("MZE real BDOS %s minSP=%04x stack=%04x..%04x untouched canary=A5 final PC=0000 SP=FE00", name, minSP, bottom, top)
	if root := os.Getenv("MINZ_ABAP_CPM_ARTIFACTS"); root != "" {
		for suffix, data := range map[string][]byte{"stdout": out, "stderr": stderr.Bytes(), "registers.log": regs, "invocation.txt": []byte(strings.Join(cmd.Args, " ") + "\n"), "image.sha256": []byte(fmt.Sprintf("%x\n", sha256.Sum256(bin)))} {
			if err := os.WriteFile(filepath.Join(root, name+"-mze-"+suffix), data, 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
}
