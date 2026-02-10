; MinZ generated code
; Generated: 2026-02-10 20:17:08


; Code section
    ORG $8000

; Using hierarchical register allocation (physical → shadow → memory)

; Function: agon_test.main
main:
; IsSMCDefault=true, IsSMCEnabled=true
; Using absolute addressing for locals (SMC style)
    PUSH BC
    PUSH DE
    ; r3 = 65
    LD A, 65
    LD ($F006), A     ; Virtual register 3 to memory
    ; r4 = call agon_test.mos_putchar$u8(r3)
    ; Call to agon_test.mos_putchar$u8 (args: 1)
    ; Default register passing for extern function
    ; c in A (default)
    RST $10    ; extern agon_test.mos_putchar$u8 (optimized from CALL)
    LD ($F008), HL    ; Virtual register 4 to memory
    ; r6 = 71
    LD A, 71
    LD C, A         ; Store to physical register C
    ; r7 = call agon_test.mos_putchar$u8(r6)
    ; Call to agon_test.mos_putchar$u8 (args: 1)
    ; Default register passing for extern function
    ; c in A (default)
    RST $10    ; extern agon_test.mos_putchar$u8 (optimized from CALL)
    ; r9 = 79
    LD A, 79
    LD E, A         ; Store to physical register E
    ; r10 = call agon_test.mos_putchar$u8(r9)
    ; Call to agon_test.mos_putchar$u8 (args: 1)
    ; Default register passing for extern function
    ; c in A (default)
    RST $10    ; extern agon_test.mos_putchar$u8 (optimized from CALL)
    ; r12 = 78
    LD A, 78
    LD L, A         ; Store to physical register L
    ; r13 = call agon_test.mos_putchar$u8(r12)
    ; Call to agon_test.mos_putchar$u8 (args: 1)
    ; Default register passing for extern function
    ; c in A (default)
    RST $10    ; extern agon_test.mos_putchar$u8 (optimized from CALL)
    ; r15 = 32
    LD A, 32
    EXX               ; Switch to shadow registers
    LD C, A         ; Store to shadow C' (now active)
    EXX               ; Switch back to main registers
    ; r16 = call agon_test.mos_putchar$u8(r15)
    ; Call to agon_test.mos_putchar$u8 (args: 1)
    ; Default register passing for extern function
    LD A, 32       ; Constant
    ; c in A (default)
    RST $10    ; extern agon_test.mos_putchar$u8 (optimized from CALL)
    ; r18 = 79
    LD A, 79
    EXX               ; Switch to shadow registers
    LD E, A         ; Store to shadow E' (now active)
    EXX               ; Switch back to main registers
    ; r19 = call agon_test.mos_putchar$u8(r18)
    ; Call to agon_test.mos_putchar$u8 (args: 1)
    ; Default register passing for extern function
    LD A, 79       ; Constant
    ; c in A (default)
    RST $10    ; extern agon_test.mos_putchar$u8 (optimized from CALL)
    ; r21 = 75
    LD A, 75
    LD B, A         ; Store to physical register B
    ; r22 = call agon_test.mos_putchar$u8(r21)
    ; Call to agon_test.mos_putchar$u8 (args: 1)
    ; Default register passing for extern function
    ; c in A (default)
    RST $10    ; extern agon_test.mos_putchar$u8 (optimized from CALL)
    ; Allocate struct Sprite
    LD HL, -5
    ADD HL, SP
    LD SP, HL
    EX DE, HL
    LD HL, SP
    LD ($F032), HL    ; Virtual register 25 to memory
    ; r26 = 100
    LD A, 100
    ; Register 26 already in A
    ; r27 = 50
    LD A, 50
    LD L, A         ; Store to HL (low byte)
    ; r28 = 1
    LD A, 1
    LD H, A         ; Store to physical register H
    ; r30 = 5
    LD A, 5
    LD L, A         ; Store to physical register L
    ; Store to Sprite.x
    LD HL, ($F032)    ; Virtual register 25 from memory
    PUSH HL
    LD HL, 100      ; Constant
    POP DE
    LD A, L
    LD (DE), A
    INC DE
    LD A, H
    LD (DE), A
    ; store spr, r25
    LD HL, ($F032)    ; Virtual register 25 from memory
    LD ($F030), HL
    ; Store to Sprite.y
    LD HL, ($F032)    ; Virtual register 25 from memory
    LD DE, 2
    ADD HL, DE
    PUSH HL
    LD HL, 50      ; Constant
    POP DE
    LD A, L
    LD (DE), A
    INC DE
    LD A, H
    LD (DE), A
    ; Store to Sprite.id
    LD HL, ($F032)    ; Virtual register 25 from memory
    LD DE, 4
    ADD HL, DE
    PUSH HL
    LD HL, 1      ; Constant
    POP DE
    LD A, L
    LD (DE), A
    INC DE
    LD A, H
    LD (DE), A
    ; r31 = call agon_test.countdown$u8(r30)
    ; Call to agon_test.countdown$u8 (args: 1)
    ; Found function, UsesTrueSMC=true
    ; TRUE SMC call to agon_test.countdown$u8
    LD A, 5       ; Constant
    LD (countdown_u8_param_start$imm0), A        ; Patch start
    CALL agon_test.countdown$u8
    LD ($F03E), HL    ; Virtual register 31 to memory
    ; r34 = call agon_test.test_for

    ; *** SMC ANNOTATED CALL to agon_test.test_for ***
    ; Pattern: store_u8, Dest: temp_result
    LD A, #00               ; NOP opcode
    LD (test_for_return_patch.op), A
    LD HL, temp_result
    LD (test_for_store_addr), HL
    CALL test_for
    ; *** END SMC CALL ***

    ; store sum, r34
    LD A, H
    LD ($F042), A
    ; r36 = load sum
    LD A, ($F042)
    LD L, A         ; Store to HL (low byte)
    ; r37 = call agon_test.print_hex8$u8(r36)
    ; Call to agon_test.print_hex8$u8 (args: 1)
    ; Found function, UsesTrueSMC=true
    ; TRUE SMC call to agon_test.print_hex8$u8
    LD A, L
    LD (print_hex8_u8_param_n$imm0), A        ; Patch n
    CALL agon_test.print_hex8$u8
    ; return
    POP DE
    POP BC
    RET
; Using hierarchical register allocation (physical → shadow → memory)

; Function: agon_test.print_char$u8
agon_test.print_char$u8:
; TRUE SMC function with immediate anchors
print_char_u8_param_c$immOP:
    LD A, 0        ; c anchor (will be patched)
print_char_u8_param_c$imm0 EQU print_char_u8_param_c$immOP+1
    LD L, A         ; Store to physical register L
    ; r4 = call agon_test.mos_putchar$u8(r3)
    ; Call to agon_test.mos_putchar$u8 (args: 1)
    ; Default register passing for extern function
    LD A, 65       ; Constant
    ; c in A (default)
    RST $10    ; extern agon_test.mos_putchar$u8 (optimized from CALL)
    ; Register 4 already in HL
    ; return
    POP DE
    POP BC
    RET
; Using hierarchical register allocation (physical → shadow → memory)

; Function: agon_test.newline
newline:
; IsSMCDefault=true, IsSMCEnabled=true
; Using absolute addressing for locals (SMC style)
    PUSH BC
    PUSH DE
    ; r2 = 13
    LD A, 13
    LD H, A         ; Store to physical register H
    ; r3 = call agon_test.mos_putchar$u8(r2)
    ; Call to agon_test.mos_putchar$u8 (args: 1)
    ; Default register passing for extern function
    ; c in A (default)
    RST $10    ; extern agon_test.mos_putchar$u8 (optimized from CALL)
    ; r5 = 10
    LD A, 10
    LD L, A         ; Store to HL (low byte)
    ; r6 = call agon_test.mos_putchar$u8(r5)
    ; Call to agon_test.mos_putchar$u8 (args: 1)
    ; Default register passing for extern function
    ; c in A (default)
    RST $10    ; extern agon_test.mos_putchar$u8 (optimized from CALL)
    ; return
    POP DE
    POP BC
    RET
; Using hierarchical register allocation (physical → shadow → memory)

; Function: agon_test.print_hex_digit$u8
agon_test.print_hex_digit$u8:
; TRUE SMC function with immediate anchors
print_hex_digit_u8_param_n$immOP:
    LD A, 0        ; n anchor (will be patched)
print_hex_digit_u8_param_n$imm0 EQU print_hex_digit_u8_param_n$immOP+1
    LD L, A         ; Store to physical register L
    ; r3 = 10
    LD A, 10
    LD ($F006), A     ; Virtual register 3 to memory
    ; r4 = r2 < r3
    ; 8-bit less-than comparison
    LD A, 13       ; Constant
    LD B, A
    LD A, 10       ; Constant
    LD C, A
    LD A, B       ; A = Src1
    CP C          ; Compare Src1 with Src2
    JR C, agon_test_print_hex_digit_u8_lt_true_0      ; Carry = Src1 < Src2
    LD HL, 0       ; False
    JR agon_test_print_hex_digit_u8_lt_done_0
agon_test_print_hex_digit_u8_lt_true_0:
    LD HL, 1       ; True
agon_test_print_hex_digit_u8_lt_done_0:
    ; jump_if_not r4, else_1
    LD A, H
    OR A
    JP Z, agon_test_print_hex_digit_u8_else_1_i4   ; Test for zero
    LD A, (n$imm0)    ; Reuse from anchor
    LD L, A         ; Store to physical register L
    ; r10 = r9
    LD ($F014), HL    ; Virtual register 10 to memory
    ; r11 = call agon_test.mos_putchar$u8(r10)
    ; Call to agon_test.mos_putchar$u8 (args: 1)
    ; Default register passing for extern function
    LD A, ($F014)     ; Virtual register 10 from memory
    ; c in A (default)
    RST $10    ; extern agon_test.mos_putchar$u8 (optimized from CALL)
    ; jump end_if_2
    JP agon_test_print_hex_digit_u8_end_if_2_i4
    ; else_1:
agon_test_print_hex_digit_u8_else_1_i4:
    LD A, (n$imm0)    ; Reuse from anchor
    LD L, A         ; Store to physical register L
    ; r17 = r16
    ; Register 17 already in HL
    ; r18 = call agon_test.mos_putchar$u8(r17)
    ; Call to agon_test.mos_putchar$u8 (args: 1)
    ; Default register passing for extern function
    LD A, L
    ; c in A (default)
    RST $10    ; extern agon_test.mos_putchar$u8 (optimized from CALL)
    ; end_if_2:
agon_test_print_hex_digit_u8_end_if_2_i4:
    ; return
    POP DE
    POP BC
    RET
; Using hierarchical register allocation (physical → shadow → memory)

; Function: agon_test.print_hex8$u8
agon_test.print_hex8$u8:
; TRUE SMC function with immediate anchors
    ; r11 = r0
    LD HL, ($F000)    ; Virtual register 0 from memory
    ; r12 = call agon_test.print_hex_digit$u8(r11)
    ; Call to agon_test.print_hex_digit$u8 (args: 1)
    ; Found function, UsesTrueSMC=true
    ; TRUE SMC call to agon_test.print_hex_digit$u8
    LD A, L
    LD (print_hex_digit_u8_param_n$imm0), A        ; Patch n
    CALL agon_test.print_hex_digit$u8
    ; Register 12 already in HL
    ; r18 = r0
    LD HL, ($F000)    ; Virtual register 0 from memory
    ; r19 = call agon_test.print_hex_digit$u8(r18)
    ; Call to agon_test.print_hex_digit$u8 (args: 1)
    ; Found function, UsesTrueSMC=true
    ; TRUE SMC call to agon_test.print_hex_digit$u8
    LD A, H
    LD (print_hex_digit_u8_param_n$imm0), A        ; Patch n
    CALL agon_test.print_hex_digit$u8
    ; return
    POP DE
    POP BC
    RET
; Using hierarchical register allocation (physical → shadow → memory)

; Function: agon_test.add_values$u16$u16
agon_test.add_values$u16$u16:
; TRUE SMC function with immediate anchors
add_values_u16_u16_param_a$immOP:
    LD HL, 0       ; a anchor (will be patched)
add_values_u16_u16_param_a$imm0 EQU add_values_u16_u16_param_a$immOP+1
    ; Register 3 already in HL
add_values_u16_u16_param_b$immOP:
    LD HL, 0       ; b anchor (will be patched)
add_values_u16_u16_param_b$imm0 EQU add_values_u16_u16_param_b$immOP+1
    LD B, H
    LD C, L
    ; r5 = r3 + r4
    LD HL, 10      ; Constant
    LD D, H
    LD E, L
    ; Could optimize: LD H,B / LD L,C
    ; Could optimize: LD H,B / LD L,C
    ; Could optimize: LD H,B / LD L,C
    ; Could optimize: LD H,B / LD L,C
    ; Could optimize: LD H,B / LD L,C
    LD H, B
    LD L, C
    ADD HL, DE
    ; Register 5 already in HL
    ; return r5
    LD HL, 10      ; Constant
    RET
; Using hierarchical register allocation (physical → shadow → memory)

; Function: agon_test.countdown$u8
agon_test.countdown$u8:
; TRUE SMC function with immediate anchors
countdown_u8_param_start$immOP:
    LD A, 0        ; start anchor (will be patched)
countdown_u8_param_start$imm0 EQU countdown_u8_param_start$immOP+1
    LD ($F006), A     ; Virtual register 3 to memory
    ; store i, r3
    LD A, 10       ; Constant
    LD ($F004), A
    ; loop_3:
agon_test_countdown_u8_loop_3_i7:
    ; test r4
    LD HL, ($F008)    ; Virtual register 4 from memory
    LD A, H
    OR L           ; Test HL (set flags)
    ; jump_if_not r6, end_loop_4
    LD A, L
    OR A
    JP Z, agon_test_countdown_u8_end_loop_4_i7   ; Test for zero
    ; r8 = load i
    LD A, ($F004)
    LD H, A         ; Store to physical register H
    ; r9 = call agon_test.print_hex8$u8(r8)
    ; Call to agon_test.print_hex8$u8 (args: 1)
    ; Found function, UsesTrueSMC=true
    ; TRUE SMC call to agon_test.print_hex8$u8
    LD A, H
    LD (print_hex8_u8_param_n$imm0), A        ; Patch n
    CALL agon_test.print_hex8$u8
    ; r11 = 32
    LD A, 32
    LD L, A         ; Store to HL (low byte)
    ; r12 = call agon_test.mos_putchar$u8(r11)
    ; Call to agon_test.mos_putchar$u8 (args: 1)
    ; Default register passing for extern function
    ; c in A (default)
    RST $10    ; extern agon_test.mos_putchar$u8 (optimized from CALL)
    ; r13 = load i
    LD A, ($F004)
    LD L, A         ; Store to physical register L
    ; store i, r13
    LD A, L
    LD ($F004), A
    ; jump loop_3
    JP agon_test_countdown_u8_loop_3_i7
    ; end_loop_4:
agon_test_countdown_u8_end_loop_4_i7:
    ; return
    POP DE
    POP BC
    RET
; Using hierarchical register allocation (physical → shadow → memory)

; Function: agon_test.test_for
test_for:
; IsSMCDefault=true, IsSMCEnabled=true
; Using absolute addressing for locals (SMC style)
    ; r2 = 0
    XOR A        ; MIR hint: zero via XOR
    LD ($F004), A     ; Virtual register 2 to memory
    ; DJNZ OPTIMIZED: for i in 0..10
    NOP
    ; store sum, r2
    XOR A          ; Constant 0
    LD ($F002), A
    ; djnz_loop_5:
agon_test_test_for_djnz_loop_5_i8:
    ; r5 = load sum
    LD A, ($F002)
    LD L, A         ; Store to physical register L
    ; Increment i
    LD A, L
    INC A
    LD L, A         ; Store to HL (low byte)
    ; DJNZ - decrement B and loop
    LD A, H
    LD B, A
    DJNZ agon_test_test_for_djnz_loop_5_i8
    LD A, B
    LD H, A         ; Store to physical register H
    ; store sum, r5
    LD A, L
    LD ($F002), A
    ; r8 = load sum
    LD A, ($F002)
    LD L, A         ; Store to physical register L
    LD A, L

    ; *** SMART PATCHABLE RETURN SEQUENCE ***
    ; Default: Store to memory (most common complex case)
    ; For immediate use: Patch first NOP to RET for early return
agon_test.test_for_return_patch.op:
    NOP                     ; PATCH POINT: NOP or RET (C9) for early return
agon_test.test_for_store_addr.op:
agon_test.test_for_store_addr equ agon_test.test_for_store_addr.op + 1
    LD (0000), A            ; DEFAULT: Store result (address gets patched)
    RET                     ; Return after store
; Using hierarchical register allocation (physical → shadow → memory)

; Function: agon_test.vdp_cls
vdp_cls:
; IsSMCDefault=true, IsSMCEnabled=true
; Using absolute addressing for locals (SMC style)
    PUSH BC
    PUSH DE
    ; r2 = 12
    LD A, 12
    LD L, A         ; Store to HL (low byte)
    ; r3 = call agon_test.mos_putchar$u8(r2)
    ; Call to agon_test.mos_putchar$u8 (args: 1)
    ; Default register passing for extern function
    ; c in A (default)
    RST $10    ; extern agon_test.mos_putchar$u8 (optimized from CALL)
    ; return
    POP DE
    POP BC
    RET
; Using hierarchical register allocation (physical → shadow → memory)

; Function: agon_test.vdp_cursor$u8$u8
agon_test.vdp_cursor$u8$u8:
; TRUE SMC function with immediate anchors
    ; r4 = 31
    LD A, 31
    LD L, A         ; Store to physical register L
    ; r5 = call agon_test.mos_putchar$u8(r4)
    ; Call to agon_test.mos_putchar$u8 (args: 1)
    ; Default register passing for extern function
    ; c in A (default)
    RST $10    ; extern agon_test.mos_putchar$u8 (optimized from CALL)
    LD ($F00A), HL    ; Virtual register 5 to memory
vdp_cursor_u8_u8_param_x$immOP:
    LD A, 0        ; x anchor (will be patched)
vdp_cursor_u8_u8_param_x$imm0 EQU vdp_cursor_u8_u8_param_x$immOP+1
    LD H, A         ; Store to physical register H
    ; r8 = call agon_test.mos_putchar$u8(r7)
    ; Call to agon_test.mos_putchar$u8 (args: 1)
    ; Default register passing for extern function
    LD A, H
    ; c in A (default)
    RST $10    ; extern agon_test.mos_putchar$u8 (optimized from CALL)
vdp_cursor_u8_u8_param_y$immOP:
    LD A, 0        ; y anchor (will be patched)
vdp_cursor_u8_u8_param_y$imm0 EQU vdp_cursor_u8_u8_param_y$immOP+1
    LD L, A         ; Store to HL (low byte)
    ; r11 = call agon_test.mos_putchar$u8(r10)
    ; Call to agon_test.mos_putchar$u8 (args: 1)
    ; Default register passing for extern function
    LD A, L
    ; c in A (default)
    RST $10    ; extern agon_test.mos_putchar$u8 (optimized from CALL)
    ; return
    POP DE
    POP BC
    RET

; TRUE SMC PATCH-TABLE
; Format: DW anchor_addr, DB size, DB param_tag
PATCH_TABLE:
    DW print_char_u8_param_c$imm0           ; agon_test.print_char$u8.c
    DB 1              ; Size in bytes
    DB 0              ; Reserved for param tag
    DW print_hex_digit_u8_param_n$imm0           ; agon_test.print_hex_digit$u8.n
    DB 1              ; Size in bytes
    DB 0              ; Reserved for param tag
    DW print_hex8_u8_param_n$imm0           ; agon_test.print_hex8$u8.n
    DB 1              ; Size in bytes
    DB 0              ; Reserved for param tag
    DW add_values_u16_u16_param_a$imm0           ; agon_test.add_values$u16$u16.a
    DB 2              ; Size in bytes
    DB 0              ; Reserved for param tag
    DW add_values_u16_u16_param_b$imm0           ; agon_test.add_values$u16$u16.b
    DB 2              ; Size in bytes
    DB 0              ; Reserved for param tag
    DW countdown_u8_param_start$imm0           ; agon_test.countdown$u8.start
    DB 1              ; Size in bytes
    DB 0              ; Reserved for param tag
    DW vdp_cursor_u8_u8_param_x$imm0           ; agon_test.vdp_cursor$u8$u8.x
    DB 1              ; Size in bytes
    DB 0              ; Reserved for param tag
    DW vdp_cursor_u8_u8_param_y$imm0           ; agon_test.vdp_cursor$u8$u8.y
    DB 1              ; Size in bytes
    DB 0              ; Reserved for param tag
    DW 0              ; End of table
PATCH_TABLE_END:

; Runtime print helper functions

; Standard library routines
temp_result:
    DW 0           ; Temporary storage for function results


; Data section (follows code contiguously)

agon_test.tick_count:
    DW 0

    END


; Assembly peephole optimization: 16 patterns applied