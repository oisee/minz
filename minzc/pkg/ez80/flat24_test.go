package ez80

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
)

func TestFlat24NativeBoundary(t *testing.T) {
	h, err := nanz.ParseWithOpts(`fun main()->u8{return ptr(327679)[1]}`, "boundary.nanz", nanz.ParseOpts{PointerBits: 24})
	if err != nil {
		t.Fatal(err)
	}
	h.Target = hir.TargetAgon
	m := hir.LowerModule(h)
	if err := mir2.Verify(m); err != nil {
		t.Fatal(err)
	}
	f := m.FuncByName("main")
	ar := mir2.PBQPAllocate(f, mir2.ComputeLiveness(f), EZ80CostTable{})
	asm, err := MIR2CodegenChecked(m, ar)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(asm, "TODO") {
		t.Fatalf("unimplemented operation:\n%s", asm)
	}
	a := z80asm.NewAssembler()
	a.SetCPUMode(z80asm.CPUModeEZ80ADL)
	result, err := a.AssembleString(asm)
	if err != nil {
		t.Fatalf("%v\n%s", err, asm)
	}
	t.Logf("%d bytes:\n%s", len(result.Binary), asm)
	runner := os.Getenv("MINZ_EZ80_RUNNER")
	if runner == "" {
		t.Log("CPU execution available with MINZ_EZ80_RUNNER; assembly verified")
		return
	}
	path := filepath.Join(t.TempDir(), "boundary.bin")
	if err := os.WriteFile(path, result.Binary, 0600); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"29", "91", "255"} {
		out, err := exec.Command(runner, path, want).CombinedOutput()
		if err != nil {
			t.Fatalf("%v: %s\n%s", err, out, asm)
		}
		t.Logf("%s", out)
	}
}

func TestFlat24NativeAdmission(t *testing.T) {
	for _, source := range []string{
		`fun main(base:^u8)->u8{return base[1]}`,
		`global value:u8
fun main()->u8{return value}`,
		`fun main()->u8{ptr(327680)[0]=1
return 1}`,
	} {
		h, err := nanz.ParseWithOpts(source, "rejected.nanz", nanz.ParseOpts{PointerBits: 24})
		if err != nil {
			t.Fatal(err)
		}
		m := hir.LowerModule(h)
		f := m.FuncByName("main")
		ar := mir2.PBQPAllocate(f, mir2.ComputeLiveness(f), EZ80CostTable{})
		if asm, err := MIR2CodegenChecked(m, ar); err == nil || asm != "" {
			t.Fatalf("unproved kernel admitted: %s", source)
		}
	}
	if _, err := MIR2CodegenChecked(&mir2.Module{}, &mir2.AllocResult{}); err == nil {
		t.Fatal("implicit 16-bit ABI accepted")
	}
}

func TestADLRegisterWidths(t *testing.T) {
	ct := EZ80CostTable{}
	for _, ty := range []mir2.Ty{mir2.PtrFor(24), mir2.TyU24, mir2.TyU16} {
		for _, name := range []string{"HL", "DE", "BC"} {
			if !ct.Compatible(ty, mir2.PhysLoc{Kind: mir2.LocReg, Name: name}) {
				t.Fatalf("%s rejected for %s", name, ty)
			}
		}
		if ct.Compatible(ty, mir2.PhysLoc{Kind: mir2.LocReg, Name: "A"}) {
			t.Fatalf("wide %s admitted in A", ty)
		}
	}
	if ct.Compatible(mir2.TyU32, mir2.PhysLoc{Kind: mir2.LocReg, Name: "HL"}) {
		t.Fatal("u32 admitted in 24-bit HL")
	}
}
