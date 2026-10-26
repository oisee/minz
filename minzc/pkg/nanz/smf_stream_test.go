package nanz_test

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pipeline"
	"github.com/minz/minzc/pkg/z80asm"
)

const smfHarness = `import mem.bytes
import midi.smf
global body_bytes:[u8;514]
global body:ByteBuffer
global decoder:SmfDecoder
global event:MidiEvent
global input:u8
global result:u16
global chunk_bytes:[u8;256]
global chunk:ByteView
global chunk_len:u16
global report:SmfReport
fun boot() {
    body_bytes[0] = 197
    body_bytes[513] = 197
    var data:^u8 = &body_bytes
    body.init(data + 1,512)
    decoder.init(&body)
}
fun step() { result = decoder.feed_byte(input,&event) }
fun ack() { result = u16(decoder.ack()) }
fun finish() { result = u16(decoder.finish()) }
fun chunk_step() {
    chunk.init(&chunk_bytes,chunk_len)
    result = u16(decoder.feed(&chunk,&event,&report))
}
`

type wireEvent struct {
	Track   uint16 `json:"track"`
	Tick    uint32 `json:"tick"`
	Delta   uint32 `json:"delta"`
	Status  byte   `json:"status"`
	Data1   byte   `json:"data1"`
	Data2   byte   `json:"data2"`
	Meta    byte   `json:"meta"`
	Payload []byte `json:"payload"`
}

// Ruby's JSON arrays intentionally use []int rather than Go's base64 []byte.
func (e *wireEvent) UnmarshalJSON(b []byte) error {
	type plain wireEvent
	var p struct {
		*plain
		Payload []int `json:"payload"`
	}
	p.plain = (*plain)(e)
	if err := json.Unmarshal(b, &p); err != nil {
		return err
	}
	e.Payload = make([]byte, len(p.Payload))
	for i, v := range p.Payload {
		e.Payload[i] = byte(v)
	}
	return nil
}

type smfEngine struct {
	name  string
	call  func(string)
	read  func(string, int) []byte
	write func(string, []byte)
}

func smfEngines(t *testing.T) []smfEngine {
	t.Helper()
	parse := func() *hir.Module {
		h, e := nanz.ParseWithOpts(smfHarness, "smf-harness.nanz", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
		if e != nil {
			t.Fatal(e)
		}
		h.NonNullObjects = true
		if e := hir.Verify(h); e != nil {
			t.Fatal(e)
		}
		return h
	}
	raw := hir.LowerModule(parse())
	steps, e := pipeline.CompileHIRSteps(parse(), pipeline.Options{DisableVariantSearch: true, AssertMode: "none"})
	if e != nil {
		t.Fatal(e)
	}
	var engines []smfEngine
	for i, m := range []*mir2.Module{raw, steps.MIR2Module} {
		vm := mir2.NewVM(m)
		engines = append(engines, smfEngine{name: []string{"raw", "optimized"}[i], call: func(name string) {
			t.Helper()
			if name == "boot" {
				vm = mir2.NewVM(m)
			}
			if _, e := vm.Call(name, nil); e != nil {
				t.Fatal(name, e)
			}
		}, read: func(name string, n int) []byte { return append([]byte(nil), vm.ReadHeap(vm.GlobalAddr(name), n)...) }, write: func(name string, b []byte) { vm.WriteHeapSlice(vm.GlobalAddr(name), b) }})
	}
	a, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
	if e != nil {
		t.Fatal(e)
	}
	if len(a.Errors) != 0 {
		t.Fatal(a.Errors)
	}
	if 0x8000+len(a.Binary) >= 0xf000 {
		t.Fatalf("code/data %d bytes overlaps reserved stack", len(a.Binary))
	}
	z := emulator.NewRemogattoZ80()
	ram := z.Memory()
	copy(ram[0x8000:], a.Binary)
	sym := func(s string) int {
		v, ok := a.Symbols[s]
		if !ok {
			t.Fatal("missing symbol", s)
		}
		return v
	}
	engines = append(engines, smfEngine{name: "Z80", call: func(name string) {
		t.Helper()
		ram[0xff00], ram[0xff01] = 0, 0x70
		z.SetRegisters(emulator.Registers{PC: uint16(sym(name)), SP: 0xff00})
		for n := 0; n < 1000000 && (z.GetPC() != 0x7000 || z.GetSP() != 0xff02); n++ {
			z.Step()
		}
		if z.GetPC() != 0x7000 || z.GetSP() != 0xff02 {
			t.Fatalf("%s PC/SP %04x/%04x", name, z.GetPC(), z.GetSP())
		}
	}, read: func(name string, n int) []byte { return append([]byte(nil), ram[sym(name):sym(name)+n]...) }, write: func(name string, b []byte) { copy(ram[sym(name):], b) }})
	t.Logf("SMF code + static data: %d bytes (includes 512-byte payload buffer)", len(a.Binary))
	return engines
}
func (e smfEngine) result() uint16 { return binary.LittleEndian.Uint16(e.read("result", 2)) }
func (e smfEngine) snapshot() wireEvent {
	b := e.read("event", 18)
	n := binary.LittleEndian.Uint32(b[14:])
	payload := make([]byte, int(n))
	copy(payload, e.read("body_bytes", int(n)+1)[1:])
	return wireEvent{Track: binary.LittleEndian.Uint16(b), Tick: binary.LittleEndian.Uint32(b[2:]), Delta: binary.LittleEndian.Uint32(b[6:]), Status: b[10], Data1: b[11], Data2: b[12], Meta: b[13], Payload: payload}
}
func smfDecode(t *testing.T, e smfEngine, data []byte) ([]wireEvent, byte) {
	t.Helper()
	e.call("boot")
	var events []wireEvent
	for at, b := range data {
		e.write("input", []byte{b})
		e.call("step")
		r := e.result()
		guards := e.read("body_bytes", 514)
		if guards[0] != 197 || guards[513] != 197 {
			t.Fatalf("%s payload guard at %d", e.name, at)
		}
		if r == 257 {
			event := e.snapshot()
			events = append(events, event)
			// Pending output must survive an attempted next byte unchanged.
			e.call("step")
			if e.result() != 2 || !reflect.DeepEqual(e.snapshot(), event) {
				t.Fatalf("%s pending at byte%d", e.name, at)
			}
			e.call("ack")
			if e.result() != 1 {
				t.Fatal("ack", e.name, e.result())
			}
		} else if r != 256 {
			code := e.read("decoder", 6)[5]
			if code == 0 {
				t.Fatalf("%s invalid status %d at byte%d", e.name, r, at)
			}
			return events, code
		}
	}
	e.call("finish")
	if e.result() != 4 {
		return events, e.read("decoder", 6)[5]
	}
	return events, 0
}
func smfFile(track []byte) []byte {
	data := []byte{'M', 'T', 'h', 'd', 0, 0, 0, 6, 0, 0, 0, 1, 1, 224, 'M', 'T', 'r', 'k', 0, 0, 0, 0}
	binary.BigEndian.PutUint32(data[18:], uint32(len(track)))
	return append(data, track...)
}
func TestSMFStreamNative(t *testing.T) {
	data := smfFile([]byte{0, 0x90, 60, 100, 0x83, 0x80, 0, 61, 0, 0, 0xc2, 7, 0, 8, 0, 0xff, 1, 3, 'a', 'b', 'c', 0, 0xf0, 3, 1, 2, 0xf7, 0, 0xff, 0x2f, 0})
	want := []wireEvent{{Status: 0x90, Data1: 60, Data2: 100, Payload: []byte{}}, {Tick: 49152, Delta: 49152, Status: 0x90, Data1: 61, Payload: []byte{}}, {Tick: 49152, Status: 0xc2, Data1: 7, Payload: []byte{}}, {Tick: 49152, Status: 0xc2, Data1: 8, Payload: []byte{}}, {Tick: 49152, Status: 255, Meta: 1, Payload: []byte{'a', 'b', 'c'}}, {Tick: 49152, Status: 240, Payload: []byte{1, 2, 247}}, {Tick: 49152, Status: 255, Meta: 47, Payload: []byte{}}}
	for _, e := range smfEngines(t) {
		t.Run(e.name, func(t *testing.T) {
			got, err := smfDecode(t, e, data)
			if err != 0 || !reflect.DeepEqual(got, want) {
				t.Fatalf("error%d\ngot%+v\nwant%+v", err, got, want)
			}
		})
	}
}
func TestSMFAutosiril(t *testing.T) {
	dir := os.Getenv("AUTOSIRIL_MIDI_DIR")
	if dir == "" {
		t.Skip("set AUTOSIRIL_MIDI_DIR to Autosiril test directory for real MIDI differential test")
	}
	engines := smfEngines(t)
	for _, name := range []string{"chronos.mid", "flim.mid", "imrav.mid", "tottoro_example.mid"} {
		path := filepath.Join(dir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		out, err := exec.Command("ruby", "../../../research/binary-stream/midi_oracle.rb", path).CombinedOutput()
		if err != nil {
			t.Fatalf("oracle %v: %s", err, out)
		}
		var oracle struct {
			Header []uint16    `json:"header"`
			Events []wireEvent `json:"events"`
		}
		if err := json.Unmarshal(out, &oracle); err != nil {
			t.Fatal(err)
		}
		for _, e := range engines {
			t.Run(name+"/"+e.name, func(t *testing.T) {
				got, code := smfDecode(t, e, data)
				if code != 0 {
					t.Fatalf("error%d after %d events of %d", code, len(got), len(oracle.Events))
				}
				if len(got) != len(oracle.Events) {
					t.Fatalf("events%d want%d", len(got), len(oracle.Events))
				}
				for i, event := range got {
					if !reflect.DeepEqual(event, oracle.Events[i]) {
						t.Fatalf("event%d got%+v want%+v", i, event, oracle.Events[i])
					}
				}
				header := e.read("decoder", 12)
				for i, want := range oracle.Header {
					if binary.LittleEndian.Uint16(header[6+2*i:]) != want {
						t.Fatal("header", header, oracle.Header)
					}
				}
				maxTick := uint32(0)
				maxPayload := 0
				for _, v := range got {
					if v.Tick > maxTick {
						maxTick = v.Tick
					}
					if len(v.Payload) > maxPayload {
						maxPayload = len(v.Payload)
					}
				}
				t.Logf("%s: %d bytes, %d events, max tick %d, max payload %d", e.name, len(data), len(got), maxTick, maxPayload)
			})
		}
	}
}

func TestSMFMalformedNative(t *testing.T) {
	eot := []byte{0, 255, 47, 0}
	valid := smfFile(eot)
	cases := []struct {
		name string
		data []byte
		code byte
	}{
		{"missing-running-status", smfFile([]byte{0, 60, 0, 0}), 5},
		{"status-in-data", smfFile([]byte{0, 144, 128, 1}), 6},
		{"illegal-system-status", smfFile([]byte{0, 241, 0, 0}), 5},
		{"five-byte-delta", smfFile([]byte{128, 128, 128, 128, 0, 255, 47, 0}), 7},
		{"five-byte-length", smfFile([]byte{0, 255, 1, 128, 128, 128, 128, 0}), 7},
		{"payload-over-track", smfFile([]byte{0, 255, 1, 2}), 4},
		{"payload-cap", smfFile(append(append([]byte{0, 255, 1, 132, 1}, make([]byte, 513)...), eot...)), 8},
		{"nonempty-eot", smfFile([]byte{0, 255, 47, 1, 0}), 9},
		{"early-eot", smfFile(append(append([]byte{}, eot...), eot...)), 9},
		{"no-eot", smfFile([]byte{0, 144, 60, 1}), 10},
		{"track-exhausted", append(smfFile([]byte{0, 144, 60, 1, 0}), 0), 4},
		{"meta-cancels-running", smfFile(append([]byte{0, 144, 60, 1, 0, 255, 1, 0, 0, 61, 1}, eot...)), 5},
		{"sysex-cancels-running", smfFile(append([]byte{0, 144, 60, 1, 0, 240, 0, 0, 61, 1}, eot...)), 5},
		{"trailing-bytes", append(append([]byte{}, valid...), 0), 12},
	}
	for _, c := range []struct {
		name  string
		index int
		value byte
		code  byte
	}{
		{"magic", 0, 0, 1}, {"short-header", 7, 5, 2}, {"format2", 9, 2, 3},
		{"zero-tracks", 11, 0, 2}, {"format0-two-tracks", 11, 2, 2},
		{"zero-division", 12, 0, 2}, {"smpte", 12, 128, 3}, {"short-track", 21, 3, 4},
	} {
		b := append([]byte{}, valid...)
		b[c.index] = c.value
		if c.name == "zero-division" {
			b[13] = 0
		}
		cases = append(cases, struct {
			name string
			data []byte
			code byte
		}{c.name, b, c.code})
	}
	for n := 0; n < len(valid); n++ {
		cases = append(cases, struct {
			name string
			data []byte
			code byte
		}{fmt.Sprintf("eof-%d", n), valid[:n], 10})
	}
	// Largest legal four-byte VLQ, followed by enough deltas to overflow u32.
	long := []byte{}
	for i := 0; i < 17; i++ {
		long = append(long, 255, 255, 255, 127, 144, 60, 1)
	}
	cases = append(cases, struct {
		name string
		data []byte
		code byte
	}{"tick-overflow", smfFile(append(long, eot...)), 11})
	for _, e := range smfEngines(t) {
		for _, c := range cases {
			t.Run(e.name+"/"+c.name, func(t *testing.T) {
				_, code := smfDecode(t, e, c.data)
				if code != c.code {
					t.Fatalf("error%d want%d", code, c.code)
				}
				before := e.read("decoder", 44)
				e.call("step")
				if e.result() != 3 {
					t.Fatalf("sticky feed %d", e.result())
				}
				e.call("finish")
				if e.result() != 3 {
					t.Fatalf("sticky EOF %d", e.result())
				}
				if !bytes.Equal(before, e.read("decoder", 44)) {
					t.Fatal("sticky fault mutated state")
				}
			})
		}
	}
}

func TestSMFWideTicksNative(t *testing.T) {
	track := []byte{}
	want := []wireEvent{}
	for i := uint32(1); i <= 16; i++ {
		track = append(track, 255, 255, 255, 127, 144, 60, 1)
		want = append(want, wireEvent{Tick: i * 268435455, Delta: 268435455, Status: 144, Data1: 60, Data2: 1, Payload: []byte{}})
	}
	track = append(track, 15, 192, 7, 0, 255, 47, 0)
	want = append(want, wireEvent{Tick: 4294967295, Delta: 15, Status: 192, Data1: 7, Payload: []byte{}}, wireEvent{Tick: 4294967295, Status: 255, Meta: 47, Payload: []byte{}})
	for _, e := range smfEngines(t) {
		t.Run(e.name, func(t *testing.T) {
			got, code := smfDecode(t, e, smfFile(track))
			if code != 0 || !reflect.DeepEqual(got, want) {
				t.Fatalf("error%d got%+v want%+v", code, got, want)
			}
		})
	}
}

func TestSMFChunksNative(t *testing.T) {
	data := smfFile([]byte{0, 144, 60, 100, 1, 61, 0, 0, 255, 1, 3, 97, 98, 99, 0, 247, 2, 33, 44, 0, 255, 47, 0})
	for _, e := range smfEngines(t) {
		want, code := smfDecode(t, e, data)
		if code != 0 {
			t.Fatal(code)
		}
		for _, size := range []int{1, 2, 3, 7, 17, 64, 256} {
			t.Run(fmt.Sprintf("%s/%d", e.name, size), func(t *testing.T) {
				e.call("boot")
				var got []wireEvent
				for pos := 0; pos < len(data); {
					// Zero-length input is not EOF, even in the middle of a VLQ/payload.
					e.write("chunk_len", []byte{0, 0})
					e.call("chunk_step")
					if e.result() != 0 {
						t.Fatal("empty chunk", pos, e.result())
					}
					end := pos + size
					if end > len(data) {
						end = len(data)
					}
					e.write("chunk_bytes", data[pos:end])
					e.write("chunk_len", []byte{byte(end - pos), byte((end - pos) >> 8)})
					e.call("chunk_step")
					status := e.result()
					used := int(binary.LittleEndian.Uint16(e.read("report", 4)))
					if used <= 0 || used > end-pos {
						t.Fatal("consumed", pos, used, status)
					}
					pos += used
					if status == 1 {
						event := e.snapshot()
						got = append(got, event)
						e.call("chunk_step")
						if e.result() != 2 || binary.LittleEndian.Uint16(e.read("report", 4)) != 0 || !reflect.DeepEqual(event, e.snapshot()) {
							t.Fatal("backpressure")
						}
						e.call("ack")
					} else if status != 0 {
						t.Fatal("feed", status, e.read("decoder", 6))
					}
				}
				e.call("finish")
				if e.result() != 4 || !reflect.DeepEqual(got, want) {
					t.Fatal("EOF/events", e.result(), got, want)
				}
				e.write("chunk_len", []byte{0, 0})
				e.call("chunk_step")
				if e.result() != 4 {
					t.Fatal("empty after EOF")
				}
			})
		}
	}
}

func TestSMFNotesExampleNative(t *testing.T) {
	source, err := os.ReadFile("../../../research/binary-stream/midi_notes.nanz")
	if err != nil {
		t.Fatal(err)
	}
	data := smfFile([]byte{0, 144, 60, 100, 1, 61, 0, 0, 255, 47, 0})
	literals := []string{}
	for _, v := range data {
		literals = append(literals, fmt.Sprint(v))
	}
	code := string(source) + fmt.Sprintf("\nglobal test_file:[u8;%d] = [%s]\n", len(data), strings.Join(literals, ",")) + fmt.Sprintf(`
fun probe() -> u8 {
    midi_init()
    var i:u16 = 0
    while i < %d {
        let status:u8 = midi_byte(test_file[i])
        if status > 1 { return 1 }
        i = i + 1
    }
    if midi_finish() != SmfStatus.DONE { return 2 }
    if note_ons != 1 { return 3 }
    if note_offs != 1 { return 4 }
    if last_tick != 1 { return 5 }
    return 0
}`, len(data))
	m, err := nanz.ParseWithOpts(code, "midi-notes-example.nanz", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
	if err != nil {
		t.Fatal(err)
	}
	m.NonNullObjects = true
	m.Asserts = []hir.Assert{{FuncName: "probe", Expected: 0}}
	if err := pipeline.RunAssertsMIR2(m, hir.LowerModule(hir.CloneModule(m))); err != nil {
		t.Fatal(err)
	}
	if _, err := pipeline.CompileHIRSteps(m, pipeline.Options{DisableVariantSearch: true}); err != nil {
		t.Fatal(err)
	}
}
