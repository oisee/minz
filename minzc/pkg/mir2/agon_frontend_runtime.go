package mir2

import (
	"fmt"
	"strings"
)

var agonFrontendCalls = map[string]string{
	"@mir.agon.fs.open":      "__agon_fs_open",
	"@mir.agon.fs.close":     "__agon_fs_close",
	"@mir.agon.fs.read":      "__agon_fs_read",
	"@mir.agon.dir.open":     "__agon_dir_open",
	"@mir.agon.dir.close":    "__agon_dir_close",
	"@mir.agon.dir.read":     "__agon_dir_read",
	"@mir.agon.key.poll":     "__agon_key_poll",
	"@mir.agon.screen.begin": "__agon_screen_begin",
	"@mir.agon.screen.end":   "__agon_screen_end",
}

func agonFrontendRequirements(m *Module) (files, keys, screen bool, err error) {
	for _, f := range m.Funcs {
		for _, b := range f.Blocks {
			for _, i := range b.Insts {
				if _, ok := agonFrontendCalls[i.Sym]; !ok || i.Op != OpCall && i.Op != OpCallCond {
					continue
				}
				if len(i.Args) != 0 || i.Ty != TyU8 || len(i.ExtraRets) != 0 {
					return false, false, false, fmt.Errorf("agon-z80 frontend intrinsic %s requires no arguments and one byte result", i.Sym)
				}
				if strings.HasPrefix(i.Sym, "@mir.agon.screen.") {
					screen = true
				} else if i.Sym == "@mir.agon.key.poll" {
					keys = true
				} else {
					files = true
				}
			}
		}
	}
	if files {
		for name, size := range map[string]int{"_abap_fs_dma_buffer": 128, "_abap_fs_agon_name_buffer": 13, "_abap_fs_agon_info_buffer": 278} {
			var found bool
			for _, g := range m.Globals {
				if g.Name == name {
					a, ok := g.Ty.(*ArrayTy)
					found = ok && a.Len == size && a.Elem == TyU8 && a.Layout == LayoutAoS && g.Storage == StorageNormal && g.At == nil && !g.IsConst
				}
			}
			if !found {
				return false, false, false, fmt.Errorf("agon-z80 frontend requires owned %s byte buffer of size%d", name, size)
			}
		}
	}
	return
}

func addAgonFrontendRuntime(assembly string, m *Module) (string, error) {
	files, keys, screen, e := agonFrontendRequirements(m)
	if e != nil {
		return "", e
	}
	if files {
		// FatFS uses stack-local LFN storage. Its contract requires 3584 long
		// bytes above BF000, separately from the console-only ABI allowance.
		assembly = strings.Replace(assembly, "    CP $F2\n", "    CP $FE\n", 1)
		assembly = strings.Replace(assembly, "    LD.LIL SP, ($040000+__agon_frame_spl)\n", "    LD.LIL SP, ($040000+__agon_frame_spl)\n    CALL __agon_fs_cleanup\n", 1)
		assembly += agonFilesystemRuntime()
	}
	if keys {
		assembly += agonKeyboardRuntime()
	}
	if screen {
		assembly = strings.Replace(assembly, "    LD.LIL SP, ($040000+__agon_frame_spl)\n", "    LD.LIL SP, ($040000+__agon_frame_spl)\n    CALL __agon_screen_cleanup\n", 1)
		assembly += agonScreenRuntime()
	}
	if files || screen {
		// Resource cleanup must not push onto the caller's admitted short stack:
		// at its lower bound that stack can immediately border the image.
		assembly = strings.Replace(assembly, "    LD SP, (__agon_saved_sps)\n", "    LD SP, __agon_stack_top\n", 1)
		assembly = strings.Replace(assembly, "    LD HL, __agon_stack_guard\n", "    LD SP, (__agon_saved_sps)\n    LD HL, __agon_stack_guard\n", 1)
	}
	if files || keys || screen {
		assembly = strings.Replace(assembly, "__agon_image_end:\n", "", 1)
		assembly += "\n__agon_native_result: DB 0\n__agon_image_end:\n"
	}
	return assembly, nil
}

func agonNativeLeaf(name, body string) string {
	return name + `:
    PUSH BC
    PUSH DE
    PUSH HL
    PUSH.LIL IX
    PUSH.LIL IY
` + body + `
    LD (__agon_native_result), A
    POP.LIL IY
    POP.LIL IX
    POP HL
    POP DE
    POP BC
    LD A, (__agon_native_result)
    RET
`
}

func agonFilesystemRuntime() string {
	s := agonNativeLeaf("__agon_fs_open", `
    LD A, (__agon_fs_active)
    OR A
    JR NZ, __agon_fs_open_bad
    CALL __agon_fs_validate_name
    JR NC, __agon_fs_open_bad
    LD.LIL HL, $040000+__agon_fs_object
    LD.LIL DE, $040000+_abap_fs_agon_name_buffer
    LD C, 1
    LD A, $80
    RST.LIS $08
    OR A
    JR NZ, __agon_fs_open_bad
    LD A, 1
    LD (__agon_fs_active), A
    XOR A
    JR __agon_fs_open_done
__agon_fs_open_bad:
    LD A, 255
__agon_fs_open_done:
`)
	s += agonNativeLeaf("__agon_fs_close", `
    LD A, (__agon_fs_active)
    OR A
    JR Z, __agon_fs_close_done
    XOR A
    LD (__agon_fs_active), A
    LD.LIL HL, $040000+__agon_fs_object
    LD A, $81
    RST.LIS $08
    OR A
    JR Z, __agon_fs_close_done
    LD A, 255
__agon_fs_close_done:
`)
	s += agonNativeLeaf("__agon_fs_read", `
    LD A, (__agon_fs_active)
    OR A
    JR Z, __agon_fs_read_bad
    LD HL, _abap_fs_dma_buffer
    LD B, 128
    LD A, 26
__agon_fs_read_fill:
    LD (HL), A
    INC HL
    DJNZ __agon_fs_read_fill
    LD.LIL HL, $040000+__agon_fs_object
    LD.LIL DE, $040000+_abap_fs_dma_buffer
    LD.LIL BC, 128
    LD A, $82
    RST.LIS $08
    OR A
    JR NZ, __agon_fs_read_bad
    LD.LIL ($040000+__agon_fs_read_count), BC
    LD A, (__agon_fs_read_count+2)
    LD B, A
    LD A, (__agon_fs_read_count+1)
    OR B
    JR NZ, __agon_fs_read_bad
    LD A, (__agon_fs_read_count)
    CP 129
    JR NC, __agon_fs_read_bad
    OR A
    JR Z, __agon_fs_read_end
    CP 128
    JR Z, __agon_fs_read_success
    LD E, A
    LD D, 0
    LD HL, _abap_fs_dma_buffer
    ADD HL, DE
    LD A, 128
    SUB E
    LD B, A
    LD A, 26
__agon_fs_read_pad:
    LD (HL), A
    INC HL
    DJNZ __agon_fs_read_pad
__agon_fs_read_success:
    XOR A
    JR __agon_fs_read_done
__agon_fs_read_end:
    LD A, 1
    JR __agon_fs_read_done
__agon_fs_read_bad:
    LD A, 2
__agon_fs_read_done:
`)
	s += agonNativeLeaf("__agon_dir_open", `
    LD A, (__agon_dir_active)
    OR A
    JR NZ, __agon_dir_open_bad
    LD.LIL HL, $040000+__agon_dir_object
    LD.LIL DE, $040000+__agon_dir_path
    LD A, $91
    RST.LIS $08
    OR A
    JR NZ, __agon_dir_open_bad
    LD A, 1
    LD (__agon_dir_active), A
    XOR A
    JR __agon_dir_open_done
__agon_dir_open_bad:
    LD A, 2
__agon_dir_open_done:
`)
	s += agonNativeLeaf("__agon_dir_read", `
    LD A, (__agon_dir_active)
    OR A
    JR Z, __agon_dir_read_bad
    LD.LIL HL, $040000+__agon_dir_object
    LD.LIL DE, $040000+_abap_fs_agon_info_buffer
    LD A, $93
    RST.LIS $08
    OR A
    JR Z, __agon_dir_read_done
__agon_dir_read_bad:
    LD A, 2
__agon_dir_read_done:
`)
	s += agonNativeLeaf("__agon_dir_close", `
    LD A, (__agon_dir_active)
    OR A
    JR Z, __agon_dir_close_done
    XOR A
    LD (__agon_dir_active), A
    LD.LIL HL, $040000+__agon_dir_object
    LD A, $92
    RST.LIS $08
    OR A
    JR Z, __agon_dir_close_done
    LD A, 2
__agon_dir_close_done:
`)
	s += `
; Bound the native filename independently of the compiled frontend caller.
; Accept one ASCII8.3 basename, never a path, wildcard or unterminated buffer.
__agon_fs_validate_name:
    LD HL, _abap_fs_agon_name_buffer
    LD B, 13
    LD C, 0
    LD D, 0
__agon_fs_name_loop:
    LD A, (HL)
    OR A
    JR Z, __agon_fs_name_end
    CP '.'
    JR Z, __agon_fs_name_dot
    CP 33
    JR C, __agon_fs_name_bad
    CP 127
    JR NC, __agon_fs_name_bad
    CP '<'
    JR Z, __agon_fs_name_bad
    CP '>'
    JR Z, __agon_fs_name_bad
    CP ','
    JR Z, __agon_fs_name_bad
    CP ';'
    JR Z, __agon_fs_name_bad
    CP ':'
    JR Z, __agon_fs_name_bad
    CP '='
    JR Z, __agon_fs_name_bad
    CP '*'
    JR Z, __agon_fs_name_bad
    CP '?'
    JR Z, __agon_fs_name_bad
    CP '['
    JR Z, __agon_fs_name_bad
    CP ']'
    JR Z, __agon_fs_name_bad
    CP '/'
    JR Z, __agon_fs_name_bad
    CP 92
    JR Z, __agon_fs_name_bad
    LD A, D
    OR A
    LD A, C
    JR NZ, __agon_fs_name_ext
    CP 8
    JR NC, __agon_fs_name_bad
    JR __agon_fs_name_char
__agon_fs_name_ext:
    CP 3
    JR NC, __agon_fs_name_bad
__agon_fs_name_char:
    INC C
    JR __agon_fs_name_advance
__agon_fs_name_dot:
    LD A, D
    OR A
    JR NZ, __agon_fs_name_bad
    LD A, C
    OR A
    JR Z, __agon_fs_name_bad
    LD D, 1
    LD C, 0
__agon_fs_name_advance:
    INC HL
    DJNZ __agon_fs_name_loop
__agon_fs_name_bad:
    OR A
    RET
__agon_fs_name_end:
    LD A, C
    OR A
    RET Z
    SCF
    RET
__agon_fs_cleanup:
    CALL __agon_fs_close
    OR A
    JR Z, __agon_fs_cleanup_dir
    LD A, 6
    LD (__agon_status), A
__agon_fs_cleanup_dir:
    CALL __agon_dir_close
    OR A
    JR Z, __agon_fs_cleanup_guard
    LD A, 6
    LD (__agon_status), A
__agon_fs_cleanup_guard:
    LD HL, __agon_fs_object_guard
    LD B, 8
    CALL __agon_fs_check_guard
    LD HL, __agon_dir_object_guard
    LD B, 8
    CALL __agon_fs_check_guard
    RET
__agon_fs_check_guard:
    LD A, (HL)
    CP $A5
    JR NZ, __agon_fs_bad_guard
    INC HL
    DJNZ __agon_fs_check_guard
    RET
__agon_fs_bad_guard:
    LD A, 4
    LD (__agon_status), A
    RET
__agon_fs_active: DB 0
__agon_dir_active: DB 0
__agon_fs_object: DS 36, 0
__agon_fs_object_guard: DS 8, $A5
__agon_dir_object: DS 49, 0
__agon_dir_object_guard: DS 8, $A5
__agon_fs_read_count: DS 3, 0
__agon_dir_path: DB '.', 0
`
	return s
}

func agonKeyboardRuntime() string {
	return agonNativeLeaf("__agon_key_poll", `
    LD A, (__agon_key_pending)
    OR A
    JR Z, __agon_key_snapshot
    DEC A
    LD (__agon_key_pending), A
    LD A, (__agon_key_queue)
    LD B, A
    LD A, (__agon_key_queue+1)
    LD (__agon_key_queue), A
    LD A, B
    JP __agon_key_done
__agon_key_snapshot:
    LD A, $08
    RST.LIS $08
__agon_key_snapshot_retry:
    LD.LIL A, (IX+25)
    LD B, A
    LD A, (__agon_key_initialized)
    OR A
    JR NZ, __agon_key_changed
    LD A, B
    LD (__agon_key_seen), A
    LD A, 1
    LD (__agon_key_initialized), A
    JP __agon_key_none
__agon_key_changed:
    LD A, (__agon_key_seen)
    CP B
    JP Z, __agon_key_none
    LD.LIL A, (IX+24)
    LD C, A
    LD.LIL A, (IX+23)
    LD D, A
    LD.LIL A, (IX+5)
    LD E, A
    LD.LIL A, (IX+25)
    CP B
    JR NZ, __agon_key_snapshot_retry
    LD A, B
    LD (__agon_key_seen), A
    LD A, C
    OR A
    JP Z, __agon_key_none
    LD A, D
    CP 162
    JR Z, __agon_key_f4
    CP 150
    JR Z, __agon_key_up
    CP 151
    JR Z, __agon_key_up
    CP 152
    JR Z, __agon_key_down
    CP 153
    JR Z, __agon_key_down
    CP 143
    JR Z, __agon_key_enter
    CP 144
    JR Z, __agon_key_enter
    CP 125
    JR Z, __agon_key_escape
    LD A, E
    JR __agon_key_done
__agon_key_f4:
    LD B, 'O'
    LD C, 'S'
    JR __agon_key_sequence
__agon_key_up:
    LD B, '['
    LD C, 'A'
    JR __agon_key_sequence
__agon_key_down:
    LD B, '['
    LD C, 'B'
__agon_key_sequence:
    LD A, B
    LD (__agon_key_queue), A
    LD A, C
    LD (__agon_key_queue+1), A
    LD A, 2
    LD (__agon_key_pending), A
__agon_key_escape:
    LD A, 27
    JR __agon_key_done
__agon_key_enter:
    LD A, 13
    JR __agon_key_done
__agon_key_none:
    XOR A
__agon_key_done:
`) + `
__agon_key_initialized: DB 0
__agon_key_seen: DB 0
__agon_key_pending: DB 0
__agon_key_queue: DS 2, 0
`
}

// BBC VDU viewport/cursor lifecycle. Clearing active before restoration makes a
// console failure during cleanup finite: the second cleanup skips the screen.
func agonScreenRuntime() string {
	s := agonNativeLeaf("__agon_screen_begin", `
    LD A, 1
    LD (__agon_screen_active), A
    LD HL, __agon_screen_begin_bytes
    CALL __agon_screen_emit
    XOR A
`)
	s += agonNativeLeaf("__agon_screen_end", `
    CALL __agon_screen_cleanup
    XOR A
`)
	s += `
__agon_screen_cleanup:
    LD A, (__agon_screen_active)
    OR A
    RET Z
    XOR A
    LD (__agon_screen_active), A
    LD HL, __agon_screen_end_bytes
__agon_screen_emit:
    LD B, (HL)
    INC HL
__agon_screen_emit_loop:
    LD A, (HL)
    CALL __agon_putchar
    INC HL
    DJNZ __agon_screen_emit_loop
    RET
__agon_screen_active: DB 0
; Reset viewport, clear, 80x24 text viewport (left,bottom,right,top), hide.
__agon_screen_begin_bytes: DB 10,26,12,28,0,23,79,0,23,1,0
; Reset to normal full-screen viewport, show cursor, next output below grid.
__agon_screen_end_bytes: DB 7,26,23,1,1,31,0,24
`
	return s
}
