package mir2

import (
	"reflect"
	"testing"
)

func TestVMDataAccessReadLoop(t *testing.T) {
	m := &Module{}
	f := m.AddFunc("reads")
	b := NewBuilder(f)
	b.SwitchToNewBlock("entry")
	p := b.Param("p", TyPtr, ClassPointer)
	n := b.Param("n", TyU8, ClassGeneral)
	zero := b.Const(0, TyU8, ClassGeneral)
	b.Jmp("head", zero, zero)
	head := b.SwitchToNewBlock("head")
	i := b.BlockParam(head, TyU8, ClassGeneral)
	sum := b.BlockParam(head, TyU8, ClassGeneral)
	cond := b.Cmp(CmpUlt, i, n, ClassFlag, false)
	b.BrIf(cond, "body", nil, "exit", nil)
	b.SwitchToNewBlock("body")
	v := b.Load(p, TyU8, ClassGeneral)
	nextSum := b.Add(sum, v, TyU8, ClassGeneral)
	one := b.Const(1, TyU8, ClassGeneral)
	nextI := b.Add(i, one, TyU8, ClassGeneral)
	b.Jmp("head", nextI, nextSum)
	b.SwitchToNewBlock("exit")
	b.Ret(sum)
	b.SwitchToNewBlock("unreachable")
	b.Store(p, zero, TyU8)
	b.Ret(zero)
	for _, trips := range []int64{0, 1, 5} {
		vm := NewVM(m)
		var events []VMDataAccess
		vm.ObserveDataAccess = func(a VMDataAccess) { events = append(events, a) }
		ptr := vm.AllocHeap([]byte{7, 99})
		vm.EnsureHeap(ptr.I + 4)
		vm.WriteHeap(ptr.I, 7)
		vm.WriteHeapSlice(ptr.I+1, []byte{88})
		vm.WriteHeapBytes(ptr.I+2, []byte{77})
		before := vm.ReadHeap(ptr.I, 4)
		if len(events) != 0 {
			t.Fatal("setup/snapshot emitted access")
		}
		got, err := vm.Call(f.Name, []Value{ptr, {I: trips}})
		if err != nil || !reflect.DeepEqual(got, []Value{{I: trips * 7}}) || len(events) != int(trips) {
			t.Fatalf("trips %d: %v, %v, trace %v", trips, got, err, events)
		}
		for _, a := range events {
			if a != (VMDataAccess{OpLoad, ptr.I, 7}) {
				t.Fatalf("unexpected access %v", a)
			}
		}
		if !reflect.DeepEqual(before, vm.ReadHeap(ptr.I, 4)) || len(events) != int(trips) {
			t.Fatal("oracle snapshot changed trace/heap")
		}
		baseline := NewVM(m)
		bp := baseline.AllocHeap(before)
		want, err := baseline.Call(f.Name, []Value{bp, {I: trips}})
		if err != nil || !reflect.DeepEqual(got, want) || vm.StepsExecuted() != baseline.StepsExecuted() {
			t.Fatal("observer changed execution")
		}
	}
}

func TestVMDataAccessOverlappingWords(t *testing.T) {
	m := &Module{}
	f := m.AddFunc("words")
	b := NewBuilder(f)
	b.SwitchToNewBlock("entry")
	p := b.Param("p", TyPtr, ClassPointer)
	v := b.Const(0x1234, TyU16, ClassGeneral)
	b.Store(p, v, TyU16)
	q := b.Field(p, 1, ClassPointer)
	v2 := b.Const(0xabcd, TyU16, ClassGeneral)
	b.Store(q, v2, TyU16)
	r := b.Load(p, TyU16, ClassGeneral)
	s := b.Load(q, TyU16, ClassGeneral)
	b.Ret(r, s)
	vm := NewVM(m)
	ptr := vm.AllocHeap([]byte{0, 0, 0, 99})
	var events []VMDataAccess
	vm.ObserveDataAccess = func(a VMDataAccess) { events = append(events, a) }
	got, err := vm.Call(f.Name, []Value{ptr})
	want := []VMDataAccess{
		{OpStore, ptr.I, 0x34}, {OpStore, ptr.I + 1, 0x12},
		{OpStore, ptr.I + 1, 0xcd}, {OpStore, ptr.I + 2, 0xab},
		{OpLoad, ptr.I, 0x34}, {OpLoad, ptr.I + 1, 0xcd},
		{OpLoad, ptr.I + 1, 0xcd}, {OpLoad, ptr.I + 2, 0xab},
	}
	if err != nil || !reflect.DeepEqual(got, []Value{{I: 0xcd34}, {I: 0xabcd}}) || !reflect.DeepEqual(events, want) {
		t.Fatalf("result %v, error %v, events %v; want %v", got, err, events, want)
	}
	baseline := NewVM(m)
	bp := baseline.AllocHeap([]byte{0, 0, 0, 99})
	baseGot, baseErr := baseline.Call(f.Name, []Value{bp})
	if baseErr != nil || !reflect.DeepEqual(got, baseGot) || !reflect.DeepEqual(vm.ReadHeap(ptr.I, 4), baseline.ReadHeap(bp.I, 4)) || vm.StepsExecuted() != baseline.StepsExecuted() {
		t.Fatal("observer changed result/heap/steps")
	}
}

func TestVMDataAccessWidthsAndWrapping(t *testing.T) {
	for _, tc := range []struct {
		name        string
		ptrTy, ty   Ty
		addr, value int64
		addrs       []int64
		bytes       []byte
	}{
		{"u16-wrap", TyPtr, TyU16, 65535, 0x1234, []int64{65535, 0}, []byte{0x34, 0x12}},
		{"u32", PtrFor(24), TyU32, 65536, 0x89abcdef, []int64{65536, 65537, 65538, 65539}, []byte{0xef, 0xcd, 0xab, 0x89}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := &Module{}
			f := m.AddFunc("width")
			b := NewBuilder(f)
			b.SwitchToNewBlock("entry")
			p := b.Param("p", tc.ptrTy, ClassPointer)
			v := b.Const(tc.value, tc.ty, ClassGeneral)
			b.Store(p, v, tc.ty)
			r := b.Load(p, tc.ty, ClassGeneral)
			b.Ret(r)
			vm := NewVM(m)
			vm.EnsureHeap(65540)
			var events []VMDataAccess
			vm.ObserveDataAccess = func(a VMDataAccess) { events = append(events, a) }
			got, err := vm.Call(f.Name, []Value{{I: tc.addr}})
			var want []VMDataAccess
			for _, op := range []Op{OpStore, OpLoad} {
				for i, addr := range tc.addrs {
					want = append(want, VMDataAccess{op, addr, tc.bytes[i]})
				}
			}
			if err != nil || !reflect.DeepEqual(got, []Value{{I: tc.value}}) || !reflect.DeepEqual(events, want) {
				t.Fatalf("result %v, error %v, events %v; want %v", got, err, events, want)
			}
		})
	}
}

func TestVMDataAccessRejectedRange(t *testing.T) {
	for _, op := range []Op{OpLoad, OpStore} {
		m := &Module{}
		f := m.AddFunc("bounds")
		b := NewBuilder(f)
		b.SwitchToNewBlock("entry")
		p := b.Param("p", TyPtr, ClassPointer)
		v := b.Const(0x1234, TyU16, ClassGeneral)
		if op == OpStore {
			b.Store(p, v, TyU16)
		} else {
			b.Load(p, TyU16, ClassGeneral)
		}
		b.Ret()
		vm := NewVM(m)
		vm.EnsureHeap(32)
		before := vm.ReadHeap(0, 32)
		count := 0
		vm.ObserveDataAccess = func(VMDataAccess) { count++ }
		if _, err := vm.Call(f.Name, []Value{{I: 31}}); err == nil || count != 0 || !reflect.DeepEqual(before, vm.ReadHeap(0, 32)) {
			t.Fatalf("%v: invalid range emitted access or changed heap", op)
		}
		vm.MaxSteps = vm.StepsExecuted()
		if _, err := vm.Call(f.Name, []Value{{I: 0}}); err == nil || count != 0 {
			t.Fatalf("%v: gas failure emitted access", op)
		}
	}
}

func TestVMDataAccessExclusions(t *testing.T) {
	m := &Module{}
	str, _ := m.Strings.Intern("lazy initialization")
	f := m.AddFunc("excluded")
	b := NewBuilder(f)
	b.SwitchToNewBlock("entry")
	b.AddrOf(str, ClassPointer)
	p := b.Alloca(4)
	v := b.Const(0xab, TyU8, ClassGeneral)
	n := b.Const(4, TyU8, ClassGeneral)
	b.CallVoid("@mir.mem.memset", []Reg{p, v, n}, CallAttrs{})
	q := b.Alloca(4)
	b.CallVoid("@mir.mem.memcpy", []Reg{q, p, n}, CallAttrs{})
	slot := b.PatchSlot(0x1234, TyU16, ClassGeneral)
	v2 := b.Const(0x5678, TyU16, ClassGeneral)
	b.Patch(slot, v2, TyU16)
	r := b.LoadPatched(slot, TyU16, ClassGeneral)
	b.Asm("ld a,(hl)", nil, nil, 0, TyU8, ClassGeneral)
	for _, op := range []Op{OpIn8, OpOut8, OpIn16, OpOut16} {
		f.Blocks[0].Insts = append(f.Blocks[0].Insts, &Inst{Op: op, Dst: f.AllocReg(), Src: [2]Reg{v}, Ty: TyU8, Imm: 0x23})
	}
	b.Ret(r, q)
	vm := NewVM(m)
	ports := &accessTestPorts{}
	vm.Ports = ports
	count := 0
	vm.ObserveDataAccess = func(VMDataAccess) { count++ }
	got, err := vm.Call(f.Name, nil)
	if err != nil || len(got) != 2 || got[0].I != 0x5678 || !reflect.DeepEqual(vm.ReadHeap(got[1].I, 4), []byte{0xab, 0xab, 0xab, 0xab}) || count != 0 || ports.reads != 2 || ports.writes != 2 {
		t.Fatalf("excluded paths: %v, %v, count %d", got, err, count)
	}
}

type accessTestPorts struct{ reads, writes int }

func (p *accessTestPorts) ReadPort(uint16) byte   { p.reads++; return 0x42 }
func (p *accessTestPorts) WritePort(uint16, byte) { p.writes++ }
