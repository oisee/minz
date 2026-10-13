package pipeline

import (
	"bytes"
	"encoding/json"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/spectrum"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"
)

func TestZMachineRestoreNative(t *testing.T) {
	command := exec.Command("python3", "-c", `
import json
from restart_rle import pack
from save_format import encode,HEADER,crc16
from native_shared_reader import emit as reader
from native_state_stream import emit as state
from native_state_validate import emit as validate
from native_save_preflight import emit as preflight
from native_delta_apply import emit as apply
from native_restore import emit as restore
from native_save_stream import emit as packer
from native_save import emit as save
story=bytearray(1024);story[0]=3;story[2:4]=b'\x00\x01';story[14:16]=b'\x01\x00';story[18:24]=b'260101';story[28:30]=b'\x12\x34'
saved=bytearray(1786);saved[:16]=b'MZ3S\x00\x01260101\x12\x34\x01\x00';saved[16]=128;saved[22:26]=b'\xcd\xab\x01\x01';saved[26]=64
for i in range(58,len(saved)):saved[i]=(i*37)&255
saved[19]=2;saved[20:22]=(8).to_bytes(2,'little');saved[25]=2;saved[27]=80
for i in [1,2]:
 base=1082;saved[base+i]=128;saved[base+32+i]=0;saved[base+64+i]=0;saved[base+128+i]=i;saved[base+160+i]=0
memory=bytearray(story[:256]);memory[100]=17;memory[16:18]=b'\x12\x34'
valid=encode(story,saved,memory)
cases=[{'record':list(valid),'want':1}]
def forge(s,d):
 first=pack(s);second=pack(bytes(a^b for a,b in zip(d,story)));h=bytearray(valid[:32]);h[12:16]=len(first).to_bytes(2,'little')+len(second).to_bytes(2,'little');payload=first+second;h[30:32]=crc16(h[:30]+payload).to_bytes(2,'little');return bytes(h)+payload
for offset,value in [(19,32),(21,2),(25,17),(42,255),(16,0),(1083,0),(1147,255),(1244,2)]:
 bad=bytearray(saved);bad[offset]=value;cases.append({'record':list(forge(bad,memory)),'want':0})
bad=bytearray(memory);bad[14]^=1;cases.append({'record':list(forge(saved,bad)),'want':0})
bad=bytearray(valid);bad[-1]^=1;cases.append({'record':list(bad),'want':0})
source=reader(0x17,0)+state()+validate(story)+preflight(story)+apply()+restore(story)+packer()+save(story,0x16,14180)+save(story,0x16,14180,32).replace("global native_save_length:u16\n","").replace("fun native_save()","fun native_save_short()")
print(json.dumps({'source':source,'archive':list(pack(story[:256])),'state':list(saved),'memory':list(memory),'cases':cases}))
`)
	command.Dir = "../../../scripts/zmachine"
	output, err := command.Output()
	if err != nil {
		t.Fatal(err)
	}
	var f struct {
		Source                 string
		Archive, State, Memory []byte
		Cases                  []struct {
			Record []byte
			Want   byte
		}
	}
	if err = json.Unmarshal(output, &f); err != nil {
		t.Fatal(err)
	}
	globals := `global pc:u32
 global frame:u8
 global v_sp:u16
 global random_state:u16
 global screen_enabled:u8
 global stream3_depth:u8
 global stream3_table_lo:[u8;16]
 global stream3_table_hi:[u8;16]
 global local_words:[u16;512]
 global ret_pc_lo:[u8;32]
 global ret_pc_hi:[u8;32]
 global ret_pc_bank:[u8;32]
 global ret_store:[u8;32]
 global ret_sp_lo:[u8;32]
 global ret_sp_hi:[u8;32]
 global stack_words:[u16;256]
 `
	core, err := os.ReadFile("../../../examples/zmachine/spectrum128/zvm_core.nanz")
	if err != nil {
		t.Fatal(err)
	}
	begin := strings.Index(string(core), "fun random_number(")
	end := strings.Index(string(core)[begin+4:], "\nfun ") + begin + 4
	rngSource := string(core)[begin:end]
	h, err := nanz.ParseWithOpts(globals+f.Source+rngSource+"\nfun main()->u8 {return native_restore(6,0,0)}", "restore.nanz", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
	if err != nil {
		t.Fatal(err)
	}
	s, err := CompileHIRSteps(h, Options{AssertMode: "none"})
	if err != nil {
		t.Fatal(err)
	}
	a, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + s.Assembly)
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Errors) > 0 {
		t.Fatal(a.Errors)
	}
	names := []string{"pc", "frame", "v_sp", "random_state", "screen_enabled", "stream3_depth", "stream3_table_lo", "stream3_table_hi", "local_words", "ret_pc_lo", "ret_pc_hi", "ret_pc_bank", "ret_store", "ret_sp_lo", "ret_sp_hi", "stack_words"}
	sizes := []int{3, 1, 2, 2, 1, 1, 16, 16, 1024, 32, 32, 32, 32, 32, 32, 512}
	oracleSequence := func(seed uint16, ranges []uint16) []uint16 {
		var result []uint16
		for _, r := range ranges {
			if r >= 32768 {
				seed = 0 - r
				result = append(result, 0)
				continue
			}
			if r == 0 {
				seed = 1
				result = append(result, 0)
				continue
			}
			limit := uint16(65535 - ((65535%uint32(r) + 1) % uint32(r)))
			for {
				seed = seed*25173 + 13849
				if seed <= limit {
					result = append(result, seed%r+1)
					break
				}
			}
		}
		return result
	}
	ranges := []uint16{1, 2, 3, 7, 100, 32767, 0, 0xffd6, 32767, 3, 1000, 17}
	for ci, c := range f.Cases {
		m, err := spectrum.NewPentagon128(nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		for p := 0; p < 8; p++ {
			for i := range m.Memory.RAM[p] {
				m.Memory.RAM[p][i] = byte(i*37 + p*17)
			}
		}
		for i, b := range a.Binary {
			m.Memory.Write(0x8000+uint16(i), b, false)
		}
		copy(m.Memory.RAM[7][:], f.Archive)
		offset := 16384 - len(c.Record)
		copy(m.Memory.RAM[6][offset:], c.Record)

		draw := func() []uint16 {
			var results []uint16
			for _, r := range ranges {
				m.CPU.SetPC(uint16(a.Symbols["random_number"]))
				m.CPU.SetSP(0xbf00)
				m.CPU.SetHL(r)
				for i := 0; i < 100000; i++ {
					if m.CPU.PC() == 0x7000 && m.CPU.SP() == 0xbf02 {
						break
					}
					m.CPU.DoOpcode()
				}
				if m.CPU.PC() != 0x7000 || m.CPU.SP() != 0xbf02 || m.Memory.PagingState() != 0x10 || m.CPU.IFF1() || m.CPU.IFF2() {
					t.Fatal("RNG return", r)
				}
				results = append(results, m.CPU.HL())
			}
			return results
		}
		var beforeRestoreSequence []uint16
		if ci == 0 {
			// Exercise actual native SAVE against independently encoded record.
			for i, value := range f.Memory {
				m.Memory.RAM[0][i] = value
			}
			at := 16
			for j, name := range names {
				for i := 0; i < sizes[j]; i++ {
					m.Memory.Write(uint16(a.Symbols[name]+i), f.State[at], false)
					at++
				}
			}
			m.Memory.Write(0xbf00, 0, false)
			m.Memory.Write(0xbf01, 0x70, false)
			m.Memory.SetPagingForce(0x10)
			m.CPU.SetPC(uint16(a.Symbols["native_save_short"]))
			m.CPU.SetSP(0xbf00)
			m.CPU.SetIFF1(false)
			m.CPU.SetIFF2(false)
			for i := 0; i < 4000000; i++ {
				if m.CPU.PC() == 0x7000 && m.CPU.SP() == 0xbf02 {
					break
				}
				m.CPU.DoOpcode()
			}
			lengthAddress := uint16(a.Symbols["native_save_length"])
			if byte(m.CPU.AF()>>8) != 0 || m.Memory.Read(lengthAddress) != 0 || m.Memory.Read(lengthAddress+1) != 0 || m.CPU.PC() != 0x7000 || m.CPU.SP() != 0xbf02 || m.Memory.PagingState() != 0x10 {
				t.Fatalf("short SAVE status%d length%02x%02x PC/SP%04x/%04x paging%x", m.CPU.AF()>>8, m.Memory.Read(lengthAddress+1), m.Memory.Read(lengthAddress), m.CPU.PC(), m.CPU.SP(), m.Memory.PagingState())
			}
			m.CPU.SetPC(uint16(a.Symbols["native_save"]))
			m.CPU.SetSP(0xbf00)
			m.CPU.SetIFF1(false)
			m.CPU.SetIFF2(false)
			for i := 0; i < 4000000; i++ {
				if m.CPU.PC() == 0x7000 && m.CPU.SP() == 0xbf02 {
					break
				}
				m.CPU.DoOpcode()
			}
			lengthAddr := uint16(a.Symbols["native_save_length"])
			savedLength := int(m.Memory.Read(lengthAddr)) | int(m.Memory.Read(lengthAddr+1))<<8
			if byte(m.CPU.AF()>>8) != 1 || savedLength != len(c.Record) || m.CPU.PC() != 0x7000 || m.CPU.SP() != 0xbf02 || m.Memory.PagingState() != 0x10 || m.CPU.IFF1() || m.CPU.IFF2() {
				t.Fatalf("SAVE got%d length%d want%d PC/SP%04x/%04x", m.CPU.AF()>>8, savedLength, len(c.Record), m.CPU.PC(), m.CPU.SP())
			}
			if !bytes.Equal(m.Memory.RAM[6][14180:14180+savedLength], c.Record) {
				t.Fatal("native SAVE differs from reference record")
			}
			offset = 14180
			beforeRestoreSequence = draw()
			expected := oracleSequence(0xabcd, ranges)
			if !slices.Equal(beforeRestoreSequence, expected) {
				t.Fatalf("before restore RNG got%v want%v", beforeRestoreSequence, expected)
			}
		}
		for j, name := range names {
			for i := 0; i < sizes[j]; i++ {
				m.Memory.Write(uint16(a.Symbols[name]+i), 0xee, false)
			}
		}
		m.Memory.Write(uint16(a.Symbols["pc"]+3), 0xff, false)
		m.Memory.Write(0xbf00, 0, false)
		m.Memory.Write(0xbf01, 0x70, false)
		before := m.Memory.RAM
		want := before
		if c.Want == 1 {
			for i, value := range f.Memory {
				if i != 16 && i != 17 {
					want[0][i] = value
				}
			}
			at := 16
			for j, name := range names {
				for i := 0; i < sizes[j]; i++ {
					want[2][(a.Symbols[name]+i)%16384] = f.State[at]
					at++
				}
			}
			want[2][(a.Symbols["pc"]+3)%16384] = 0
		}
		m.Memory.SetPagingForce(0x10)
		m.CPU.SetPC(uint16(a.Symbols["native_restore"]))
		m.CPU.SetSP(0xbf00)
		m.CPU.SetHL(uint16(offset))
		m.CPU.SetDE(uint16(len(c.Record)))
		m.CPU.SetAF(0x1600)
		m.CPU.SetIFF1(false)
		m.CPU.SetIFF2(false)
		for i := 0; i < 4000000; i++ {
			if m.CPU.PC() == 0x7000 && m.CPU.SP() == 0xbf02 {
				break
			}
			m.CPU.DoOpcode()
		}
		if byte(m.CPU.AF()>>8) != c.Want || m.CPU.PC() != 0x7000 || m.CPU.SP() != 0xbf02 || m.Memory.PagingState() != 0x10 || m.CPU.IFF1() || m.CPU.IFF2() {
			t.Fatalf("case%d got%d want%d PC/SP%04x/%04x", ci, m.CPU.AF()>>8, c.Want, m.CPU.PC(), m.CPU.SP())
		}
		scratch := map[int]bool{}
		for name, size := range map[string]int{"original_cursor": 12, "saved_cursor": 12, "state_check_frame": 1, "state_check_depth": 2, "state_check_last": 2, "save_check_bank": 1, "save_check_offset": 2, "save_check_length": 2, "save_check_first": 2, "save_check_second": 2, "save_check_crc": 2} {
			for i := 0; i < size; i++ {
				scratch[a.Symbols[name]+i] = true
			}
		}
		for name, addr := range a.Symbols {
			if strings.HasPrefix(name, "_spill_") {
				scratch[addr] = true
			}
		}
		for p := 0; p < 8; p++ {
			for i, value := range want[p] {
				if p == 2 && (i >= 0x3e00 && i < 0x3f00 || scratch[0x8000+i]) {
					continue
				}
				if m.Memory.RAM[p][i] != value {
					t.Fatalf("case%d bank%d offset%x got%x want%x", ci, p, i, m.Memory.RAM[p][i], value)
				}
			}
		}
		if ci == 0 {
			afterRestoreSequence := draw()
			if !slices.Equal(afterRestoreSequence, beforeRestoreSequence) {
				t.Fatalf("RNG continuation differs after RESTORE: before%v after%v", beforeRestoreSequence, afterRestoreSequence)
			}
			t.Logf("native RNG save/restore sequence matches arithmetic oracle: %v", afterRestoreSequence)
		}

	}
	t.Logf("%d restore cases: exact full state/dynamic apply; rejected records leave game state untouched", len(f.Cases))
}
