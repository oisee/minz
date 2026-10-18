package mir2

import (
	"strings"
	"testing"
)

func TestAgonFrontendCleanupUsesOwnedStack(t *testing.T) {
	m := &Module{Funcs: []*Func{{Name: "main", Blocks: []*Block{{Insts: []*Inst{{Op: OpCall, Sym: "@mir.agon.fs.close", Ty: TyU8}}}}}}, Globals: []Global{
		{Name: "_abap_fs_dma_buffer", Ty: NewArray(TyU8, 128)},
		{Name: "_abap_fs_agon_name_buffer", Ty: NewArray(TyU8, 13)},
		{Name: "_abap_fs_agon_info_buffer", Ty: NewArray(TyU8, 278)},
	}}
	s, err := addAgonFrontendRuntime(agonZ80Tail(1024), m)
	if err != nil {
		t.Fatal(err)
	}
	cleanup := strings.Split(strings.Split(s, "__agon_cleanup:\n")[1], "__agon_guard_loop:")[0]
	want := "    LD SP, __agon_stack_top\n    LD.LIL SP, ($040000+__agon_frame_spl)\n    CALL __agon_fs_cleanup\n    LD SP, (__agon_saved_sps)\n"
	if !strings.HasPrefix(cleanup, want) {
		t.Fatalf("cleanup pushes onto caller stack before reset: %s", cleanup)
	}
}
