package main

import (
	"bytes"
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pipeline"
	"github.com/minz/minzc/pkg/spectrum"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"strings"
	"testing"
)

// Exercise the actual registered provider devices, not a copied protocol model.
// Numeric/string results are independent frozen SQLite expectations.
func TestMZXSQLiteProviderPublicProtocol(t *testing.T) {
	newPorts := func() *spectrum.Ports {
		m, err := spectrum.New48K(make([]byte, 16384))
		if err != nil {
			t.Fatal(err)
		}
		setupMZXSQLitePorts(m, false)
		return m.Ports
	}
	out := func(p *spectrum.Ports, port, v byte) { p.WritePort(uint16(v)<<8|uint16(port), v) }
	cmd := func(p *spectrum.Ports, v byte) byte { out(p, 0x41, v); return p.ReadPort(0xa543) }
	str := func(p *spectrum.Ports, s string) {
		for _, b := range append([]byte(s), 0) {
			out(p, 0x45, b)
		}
	}
	handle := func(p *spectrum.Ports, h byte) { out(p, 0x45, h) }
	query := func(p *spectrum.Ports, db byte, s string) byte { out(p, 0x47, db); str(p, s); return cmd(p, 3) }
	text := func(p *spectrum.Ports, sh, col byte) string {
		handle(p, sh)
		out(p, 0x47, col)
		cmd(p, 5)
		var b []byte
		for i := 0; i < 256; i++ {
			v := p.ReadPort(0xc145)
			if v == 0 {
				return string(b)
			}
			b = append(b, v)
		}
		t.Fatal("unterminated bounded response")
		return ""
	}
	integer := func(p *spectrum.Ports, sh, col byte) uint16 {
		handle(p, sh)
		out(p, 0x47, col)
		cmd(p, 6)
		return uint16(p.ReadPort(0x7845)) | uint16(p.ReadPort(0x9847))<<8
	}
	t.Run("handles_status_and_fresh_query", func(t *testing.T) {
		p := newPorts()
		str(p, ":memory:")
		db := cmd(p, 1)
		if db != 1 {
			t.Fatalf("open status43 got%d want1", db)
		}
		for _, s := range []string{"CREATE TABLE t(name TEXT,n INTEGER)", "INSERT INTO t VALUES('Alice',-32768),('Bob',65535)"} {
			out(p, 0x47, db)
			str(p, s)
			if got := cmd(p, 8); got != 0 {
				t.Fatal("exec", got, s)
			}
		}
		sh := query(p, db, "SELECT name,n FROM t ORDER BY rowid")
		if sh != 1 {
			t.Fatalf("query status43 got%d want1", sh)
		}
		for i, want := range []struct {
			s string
			n uint16
		}{{"Alice", 32768}, {"Bob", 65535}} {
			handle(p, sh)
			if got := cmd(p, 4); got != 1 {
				t.Fatal("row status", i, got)
			}
			if got := text(p, sh, 0); got != want.s {
				t.Fatal("text", got, want.s)
			}
			if got := integer(p, sh, 1); got != want.n {
				t.Fatal("int", got, want.n)
			}
		}
		handle(p, sh)
		if got := cmd(p, 4); got != 0 {
			t.Fatal("done", got)
		}
		handle(p, sh)
		cmd(p, 7)
		// Handle-only step/finalize must not prefix their bytes to the next SQL.
		second := query(p, db, "SELECT 256")
		if second != 2 {
			t.Fatal("second query status", second)
		}
		handle(p, second)
		if got := cmd(p, 4); got != 1 {
			t.Fatal("second step", got)
		}
		if got := integer(p, second, 0); got != 256 {
			t.Fatal("second int", got)
		}
		handle(p, second)
		cmd(p, 7)
		handle(p, db)
		cmd(p, 2)
		str(p, ":memory:")
		if got := cmd(p, 1); got != 2 {
			t.Fatal("reopen handle", got)
		}
		handle(p, 2)
		cmd(p, 2)
	})
	t.Run("errors_invalid_columns_and_integer_bounds", func(t *testing.T) {
		p := newPorts()
		str(p, ":memory:")
		cmd(p, 1)
		out(p, 0x47, 1)
		str(p, "NOT SQL")
		if got := cmd(p, 8); got != 1 {
			t.Fatal("exec error", got)
		}
		if got := query(p, 1, "NOT SQL"); got != 0 {
			t.Fatal("query error must return handle0", got)
		}
		if got := text(p, 255, 255); got != "" {
			t.Fatal("invalid text", got)
		}
		handle(p, 255)
		if got := cmd(p, 4); got != 0 {
			t.Fatal("invalid step", got)
		}
		if got := integer(p, 255, 255); got != 0 {
			t.Fatal("invalid int", got)
		}
		for _, n := range []int{0, 1, 255, 256, 32767, 32768, 65535, -1, -32768} {
			sh := query(p, 1, fmt.Sprintf("SELECT %d", n))
			if sh == 0 {
				t.Fatal("query", n)
			}
			handle(p, sh)
			if got := cmd(p, 4); got != 1 {
				t.Fatal("step", n, got)
			}
			if got := integer(p, sh, 0); got != uint16(n) {
				t.Fatal("integer", n, got)
			}
			if got := text(p, sh, 255); got != "" {
				t.Fatal("invalid column", got)
			}
			if got := integer(p, sh, 255); got != 0 {
				t.Fatal("invalid column int", got)
			}
			handle(p, sh)
			cmd(p, 7)
		}
		handle(p, 1)
		cmd(p, 2)
	})
}

func TestMZXSQLiteOriginalNativeConsumer(t *testing.T) {
	src, err := os.ReadFile("../../../examples/nanz/sql_test6.nanz")
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"ordinary", "grace-size", "grace-speed"} {
		t.Run(mode, func(t *testing.T) {
			h, err := nanz.ParseWithOpts(string(src), "sql_test6.nanz", nanz.ParseOpts{BaseDir: "../../../examples/nanz", StdlibDir: "../../../stdlib"})
			if err != nil {
				t.Fatal(err)
			}
			o := pipeline.Options{ContractOpt: true, AssertMode: "none"}
			if mode != "ordinary" {
				o.UseGrace = true
				o.UseHIRGrace = true
				o.OptSize = mode == "grace-size"
			}
			steps, err := pipeline.CompileHIRSteps(h, o)
			if err != nil {
				t.Fatal(err)
			}
			a, err := z80asm.NewAssembler().AssembleString("ORG 0x100\n" + steps.Assembly)
			if err != nil || len(a.Errors) > 0 {
				t.Fatal(err, a.Errors)
			}
			m, err := spectrum.New48K(make([]byte, 16384))
			if err != nil {
				t.Fatal(err)
			}
			setupMZXSQLitePorts(m, false)
			z := emulator.NewRemogattoZ80WithScreen()
			z.SetROMEnd(0)
			z.LoadMemory(0x100, a.Binary)
			target := a.Symbols["main"]
			z.LoadMemory(0x7000, []byte{0xcd, byte(target), byte(target >> 8), 0x76})
			z.SetRegisters(emulator.Registers{PC: 0x7000, SP: 0xf000})
			var cmds, status []byte
			z.SetIOHandlers(func(p uint16) byte {
				v := m.Ports.ReadPort(p)
				if byte(p) == 0x43 {
					status = append(status, v)
				}
				return v
			}, func(p uint16, v byte) {
				if byte(p) == 0x41 {
					cmds = append(cmds, v)
				}
				m.Ports.WritePort(p, v)
			})
			before := append([]byte(nil), z.Memory()...)
			var stdout []byte
			minSP := uint16(0xf000)
			done := false
			for i := 0; i < 100000; i++ {
				pc := z.GetPC()
				if pc == 0x7003 && z.GetSP() == 0xf000 {
					done = true
					break
				}
				if pc == 5 {
					r := z.GetRegisters()
					if byte(r.BC) != 2 {
						t.Fatal("BDOS", r.BC)
					}
					sp := r.SP
					ret := uint16(z.Memory()[sp]) | uint16(z.Memory()[sp+1])<<8
					stdout = append(stdout, byte(r.DE))
					r.A = 0xa5
					r.F = 0xff
					r.BC = 0x1234
					r.DE = 0x5678
					r.HL = 0x9abc
					r.IX = 0x2468
					r.IY = 0x1357
					r.SP = sp + 2
					r.PC = ret
					z.SetRegisters(r)
					continue
				}
				if pc != 0x7000 && (pc < 0x100 || int(pc) >= 0x100+len(a.Binary)) {
					t.Fatal("foreign fetch", pc)
				}
				z.Step()
				if z.GetSP() < minSP {
					minSP = z.GetSP()
				}
			}
			if !done || string(stdout) != "Open...\nExec...\nQuery...\nStep1...\nAlice\nStep2...\nBob\nDone.\n" || !bytes.Equal(cmds, []byte{1, 8, 8, 8, 3, 4, 5, 4, 5, 7, 2}) || !bytes.Equal(status, []byte{1, 0, 0, 0, 1, 1, 1, 0, 0}) {
				t.Fatal("native original main", done, string(stdout), cmds, status)
			}
			buf := a.Symbols["sql__sqlite___sql_buf"]
			wantBuf := []byte("Bob\x00e\x00")
			for p, v := range before {
				if p >= int(minSP) && p < 0xf000 {
					continue
				}
				want := v
				if p >= buf && p < buf+len(wantBuf) {
					want = wantBuf[p-buf]
				}
				if z.Memory()[p] != want {
					t.Fatal("RAM", p, z.Memory()[p], want)
				}
			}
			t.Logf("actual MZX provider native original main %d B exact59stdout status/commands/fullRAM/SP PASS", len(a.Binary))
		})
	}
}

func TestMZXSQLiteStringBoundary(t *testing.T) {
	m, err := spectrum.New48K(make([]byte, 16384))
	if err != nil {
		t.Fatal(err)
	}
	setupMZXSQLitePorts(m, false)
	p := m.Ports
	out := func(port, v byte) { p.WritePort(uint16(v)<<8|uint16(port), v) }
	send := func(s string) {
		for _, b := range append([]byte(s), 0) {
			out(0x45, b)
		}
	}
	send(":memory:")
	out(0x41, 1)
	for _, n := range []int{0, 1, 17, 255} {
		out(0x47, 1)
		send("SELECT '" + strings.Repeat("Q", n) + "'")
		out(0x41, 3)
		sh := p.ReadPort(0x4343)
		if sh == 0 {
			t.Fatal("query")
		}
		out(0x45, sh)
		out(0x41, 4)
		if p.ReadPort(0x4343) != 1 {
			t.Fatal("row")
		}
		out(0x45, sh)
		out(0x47, 0)
		out(0x41, 5)
		for i := 0; i <= n; i++ {
			want := byte('Q')
			if i == n {
				want = 0
			}
			if got := p.ReadPort(0xa545); got != want {
				t.Fatal("bounded text", n, i, got, want)
			}
		}
		out(0x45, sh)
		out(0x41, 7)
	}
	out(0x45, 1)
	out(0x41, 2)
}
