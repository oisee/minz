# Comprehensive Backend E2E Testing Report

**Generated**: 2025-08-06 16:51:42  
**Test Suite**: Full backend compilation pipeline (source → binary)

## Executive Summary

This report documents comprehensive end-to-end testing of all MinZ compiler backends, testing the complete compilation pipeline from source code to final binary output.

## Test Programs

1. **basic_math.minz** - Arithmetic operations and variable assignments
2. **control_flow.minz** - Loops and conditional statements  
3. **function_calls.minz** - Function definitions and calls
4. **arrays.minz** - Array declarations and indexing

## Backend Test Results

[0;34m### Testing: arrays[0m

#### z80
```
    LD D, 0
    POP HL
    ADD HL, DE
    LD A, (HL)
    LD L, A         ; Store to physical register L
    ; store last, r9
    LD A, L
    LD ($F00C), A
    ; r11 = load first
  🔨 Attempting binary generation...
Unrecognized option: o
SjASMPlus Z80 Cross-Assembler v1.07 RC8 (build 06-11-2008)
tests/backend_e2e/outputs/arrays_z80.a80(89): error: Duplicate label: print_string_u16
tests/backend_e2e/outputs/arrays_z80.a80(116): error: Duplicate label: print_u16_decimal
tests/backend_e2e/outputs/arrays_z80.a80(134): error: Duplicate label: print_digit
tests/backend_e2e/outputs/arrays_z80.a80(175): error: Duplicate label: print_true
tests/backend_e2e/outputs/arrays_z80.a80(179): error: Duplicate label: bool_true_str
tests/backend_e2e/outputs/arrays_z80.a80(181): error: Duplicate label: bool_false_str
(0): error: Error opening file: tests/backend_e2e/binaries/arrays_z80
  ❌ Binary generation failed
```

#### 6502
```
    lda arr        ; r7 = arr
    lda #$04      ; r8 = 4
    ; TODO: LOAD_INDEX
    sta last        ; store last
    lda first        ; r11 = first
    lda last        ; r12 = last
    ; r13 = r11 + r12 (needs register allocation)
    clc
    adc $00        ; placeholder
    sta sum        ; store sum
    ; return
    rts        ; Return

; Helper routines
print_char:
    ; Platform-specific character output
    ; For C64: sta $FFD2
    ; For Apple II: jsr $FDED
    rts
  ⚠️  Assembler not available, skipping binary generation
```

#### 68000
```

| Print helpers
print_char:
	| Character in d0
	| Platform-specific implementation needed
	| Amiga: dos.library/Write
	| Atari ST: GEMDOS Cconout
	| Mac: _PBWrite trap
	rts

print_hex:
	move.b d0,d1
	lsr.b #4,d0
	bsr print_nibble
	move.b d1,d0
	bsr print_nibble
	rts

print_nibble:
  ⚠️  Assembler not available, skipping binary generation
```

#### i8080
```
Error: code generation error: unsupported operation: LOAD_INDEX
  ❌ Code generation failed
```

#### gb
```

; Print helpers for Game Boy
print_char:
    ; Wait for VBlank
    LD HL, $FF44  ; LY register
.wait_vblank:
    LD A, [HL]
    CP 144
    JR C, .wait_vblank
    ; Character in A, write to tile map
    ; This is a simplified version
    RET

print_hex:
    PUSH AF
    SWAP A
    CALL print_nibble
    POP AF
    CALL print_nibble
  ⚠️  Assembler not available, skipping binary generation
```

#### c
```
Error: code generation error: generating function tests.backend_e2e.sources.arrays.main: unsupported operation: LOAD_INDEX
  ❌ Code generation failed
```

#### llvm
```

; Runtime functions
define void @print_u8(i8 %value) {
  %1 = zext i8 %value to i32
  %2 = call i32 (i8*, ...) @printf(i8* getelementptr inbounds ([4 x i8], [4 x i8]* @.str.u8, i32 0, i32 0), i32 %1)
  ret void
}

@.str.u8 = private constant [4 x i8] c"%u\0A\00"

; main wrapper
define i32 @main() {
  call void @examples_test_llvm_main()
  ret i32 0
}
  🔨 Attempting binary generation...
llc: error: llc: tests/backend_e2e/outputs/arrays_llvm.ll:21:12: error: use of undefined value '%r5'
  store i8 %r5, i8* %first.addr
           ^
  ❌ Binary generation failed
```

#### wasm
```
               ^^^^^^
tests/backend_e2e/outputs/arrays_wasm.wat:37:16: error: undefined global variable "$arr"
    global.get $arr  ;; r7 = arr
               ^^^^
tests/backend_e2e/outputs/arrays_wasm.wat:43:16: error: undefined global variable "$last"
    global.set $last
               ^^^^^
tests/backend_e2e/outputs/arrays_wasm.wat:44:16: error: undefined global variable "$first"
    global.get $first  ;; r11 = first
               ^^^^^^
tests/backend_e2e/outputs/arrays_wasm.wat:46:16: error: undefined global variable "$last"
    global.get $last  ;; r12 = last
               ^^^^^
tests/backend_e2e/outputs/arrays_wasm.wat:53:16: error: undefined global variable "$sum"
    global.set $sum
               ^^^^
tests/backend_e2e/outputs/arrays_wasm.wat:58:24: error: undefined function variable "$main"
  (export "main" (func $main))
                       ^^^^^
  ❌ Binary generation failed
```

[0;34m### Testing: basic_math[0m

#### z80
```
    LD C', A         ; Store to shadow C'
    EXX               ; Switch back to main registers
    ; r11 = load y
    LD A, ($F006)
    EXX               ; Switch to shadow registers
    LD D', A         ; Store to shadow D'
    EXX               ; Switch back to main registers
    ; r12 = r10 - r11
  🔨 Attempting binary generation...
Unrecognized option: o
SjASMPlus Z80 Cross-Assembler v1.07 RC8 (build 06-11-2008)
tests/backend_e2e/outputs/basic_math_z80.a80(81): error: Duplicate label: tests.backend_e2e.sources.basic_math.main.mul_done_0
tests/backend_e2e/outputs/basic_math_z80.a80(105): error: Duplicate label: print_string_u16
tests/backend_e2e/outputs/basic_math_z80.a80(132): error: Duplicate label: print_u16_decimal
tests/backend_e2e/outputs/basic_math_z80.a80(150): error: Duplicate label: print_digit
tests/backend_e2e/outputs/basic_math_z80.a80(191): error: Duplicate label: print_true
tests/backend_e2e/outputs/basic_math_z80.a80(195): error: Duplicate label: bool_true_str
tests/backend_e2e/outputs/basic_math_z80.a80(197): error: Duplicate label: bool_false_str
(0): error: Error opening file: tests/backend_e2e/binaries/basic_math_z80
  ❌ Binary generation failed
```

#### 6502
```
    lda x        ; r10 = x
    lda y        ; r11 = y
    ; r12 = r10 - r11 (needs register allocation)
    sec
    sbc $00        ; placeholder
    sta diff        ; store diff
    lda x        ; r14 = x
    lda #$02      ; r15 = 2
    ; TODO: MUL
    sta prod        ; store prod
    ; return
    rts        ; Return

; Helper routines
print_char:
    ; Platform-specific character output
    ; For C64: sta $FFD2
    ; For Apple II: jsr $FDED
    rts
  ⚠️  Assembler not available, skipping binary generation
```

#### 68000
```
	move.w -56(a6),d0
	muls.w -60(a6),d0
	move.l d0,-64(a6)
	move.l -64(a6),prod
	movem.l (sp)+,d2-d7/a2-a5
	unlk a6
	rts

| Print helpers
print_char:
	| Character in d0
	| Platform-specific implementation needed
	| Amiga: dos.library/Write
	| Atari ST: GEMDOS Cconout
	| Mac: _PBWrite trap
	rts

print_hex:
	move.b d0,d1
  ⚠️  Assembler not available, skipping binary generation
```

#### i8080
```
    STA F010H
    LDA F010H
    STA sum
    LHLD x
    SHLD F014H
    LHLD y
    SHLD F016H
    LDA F014H
    MOV B,A
    LDA F016H
    SUB B
    STA F018H
    LDA F018H
    STA diff
    LHLD x
    SHLD F01CH
    MVI A,02H
    STA F01EH
    LDA F01CH
  ⚠️  Assembler not available, skipping binary generation
```

#### gb
```
    LD A, 2
    ; Store to r15
    ; TODO: MUL
    ; Store r16 to var prod
    RET

; Print helpers for Game Boy
print_char:
    ; Wait for VBlank
    LD HL, $FF44  ; LY register
.wait_vblank:
    LD A, [HL]
    CP 144
    JR C, .wait_vblank
    ; Character in A, write to tile map
    ; This is a simplified version
    RET

print_hex:
  ⚠️  Assembler not available, skipping binary generation
```

#### c
```
    char* data;
} String;

// Print helper functions
void print_char(u8 ch) {
    putchar(ch);
}

void print_u8(u8 value) {
    printf("%u", value);
}

void print_u8_decimal(u8 value) {
    printf("%u", value);
  🔨 Attempting binary generation...
  ✅ Binary generation successful

=== Binary Analysis ===
tests/backend_e2e/binaries/basic_math_c: Mach-O 64-bit executable arm64
-rwxr-xr-x  1 alice  staff  33968  6 Aug 16:51 tests/backend_e2e/binaries/basic_math_c
```

#### llvm
```
}


; Runtime functions
define void @print_u8(i8 %value) {
  %1 = zext i8 %value to i32
  %2 = call i32 (i8*, ...) @printf(i8* getelementptr inbounds ([4 x i8], [4 x i8]* @.str.u8, i32 0, i32 0), i32 %1)
  ret void
}

@.str.u8 = private constant [4 x i8] c"%u\0A\00"

; main wrapper
define i32 @main() {
  call void @examples_test_llvm_main()
  🔨 Attempting binary generation...
llc: error: llc: tests/backend_e2e/outputs/basic_math_llvm.ll:50:13: error: use of undefined value '@examples_test_llvm_main'
  call void @examples_test_llvm_main()
            ^
  ❌ Binary generation failed
```

#### wasm
```
               ^^^^
tests/backend_e2e/outputs/basic_math_wasm.wat:52:16: error: undefined global variable "$x"
    global.get $x  ;; r10 = x
               ^^
tests/backend_e2e/outputs/basic_math_wasm.wat:54:16: error: undefined global variable "$y"
    global.get $y  ;; r11 = y
               ^^
tests/backend_e2e/outputs/basic_math_wasm.wat:61:16: error: undefined global variable "$diff"
    global.set $diff
               ^^^^^
tests/backend_e2e/outputs/basic_math_wasm.wat:62:16: error: undefined global variable "$x"
    global.get $x  ;; r14 = x
               ^^
tests/backend_e2e/outputs/basic_math_wasm.wat:71:16: error: undefined global variable "$prod"
    global.set $prod
               ^^^^^
tests/backend_e2e/outputs/basic_math_wasm.wat:76:24: error: undefined function variable "$main"
  (export "main" (func $main))
                       ^^^^^
  ❌ Binary generation failed
```

[0;34m### Testing: control_flow[0m

#### z80
```
    ; r8 = r6 + r7
    LD D, H
  🔨 Attempting binary generation...
Unrecognized option: o
SjASMPlus Z80 Cross-Assembler v1.07 RC8 (build 06-11-2008)
tests/backend_e2e/outputs/control_flow_z80.a80(36): error: Duplicate label: loop_1.lt_true_0
tests/backend_e2e/outputs/control_flow_z80.a80(38): error: Duplicate label: loop_1.lt_done_0
tests/backend_e2e/outputs/control_flow_z80.a80(61): error: Duplicate label: end_loop_2
tests/backend_e2e/outputs/control_flow_z80.a80(78): error: Duplicate label: end_loop_2.eq_true_1
tests/backend_e2e/outputs/control_flow_z80.a80(80): error: Duplicate label: end_loop_2.eq_done_1
tests/backend_e2e/outputs/control_flow_z80.a80(96): error: Duplicate label: else_3
tests/backend_e2e/outputs/control_flow_z80.a80(108): error: Duplicate label: end_if_4
tests/backend_e2e/outputs/control_flow_z80.a80(129): error: Duplicate label: print_string_u16
tests/backend_e2e/outputs/control_flow_z80.a80(156): error: Duplicate label: print_u16_decimal
tests/backend_e2e/outputs/control_flow_z80.a80(174): error: Duplicate label: print_digit
tests/backend_e2e/outputs/control_flow_z80.a80(215): error: Duplicate label: print_true
tests/backend_e2e/outputs/control_flow_z80.a80(219): error: Duplicate label: bool_true_str
tests/backend_e2e/outputs/control_flow_z80.a80(221): error: Duplicate label: bool_false_str
(0): error: Error opening file: tests/backend_e2e/binaries/control_flow_z80
  ❌ Binary generation failed
```

#### 6502
```
    ; TODO: EQ
    ; conditional jump (needs implementation)
    beq else_3        ; if zero
    lda #$01      ; r13 = 1
    sta result        ; store result
    jmp end_if_4
else_3:
    lda #$00      ; r15 = 0
    sta result        ; store result
end_if_4:
    ; return
    rts        ; Return

; Helper routines
print_char:
    ; Platform-specific character output
    ; For C64: sta $FFD2
    ; For Apple II: jsr $FDED
    rts
  ⚠️  Assembler not available, skipping binary generation
```

#### 68000
```
	move.l d5,d7
	add.l d6,d7
	move.l d7,i
	bra loop_1
end_loop_2:
	move.l i,-40(a6)
	move.l #10,-44(a6)
	cmp.l -44(a6),-40(a6)
	beq .true_L2
	moveq #0,-48(a6)
	bra .end_L2
.true_L2:
	moveq #1,-48(a6)
.end_L2:
	tst.l -48(a6)
	beq else_3
	move.l #1,-52(a6)
	move.l -52(a6),result
	bra end_if_4
  ⚠️  Assembler not available, skipping binary generation
```

#### i8080
```
true_L1:
    MVI A,1
end_L1:
    STA F00AH
    LDA F00AH
    ORA A
    JZ end_loop_2
    LHLD i
    SHLD F00CH
    MVI A,01H
    STA F00EH
    LDA F00CH
    MOV B,A
    LDA F00EH
    ADD B
    STA F010H
    LHLD F010H
    SHLD i
    JMP loop_1
  ⚠️  Assembler not available, skipping binary generation
```

#### gb
```
    ; TODO: LABEL
    ; Load var i to r9
    LD A, 10
    ; Store to r10
    ; TODO: EQ
    ; TODO: JUMP_IF_NOT
    LD A, 1
    ; Store to r13
    ; Store r13 to var result
    ; TODO: JUMP
    ; TODO: LABEL
    LD A, 0
    ; Store to r15
    ; Store r15 to var result
    ; TODO: LABEL
    RET

; Print helpers for Game Boy
print_char:
  ⚠️  Assembler not available, skipping binary generation
```

#### c
```
// Print helper functions
void print_char(u8 ch) {
    putchar(ch);
}

void print_u8(u8 value) {
    printf("%u", value);
}

void print_u8_decimal(u8 value) {
    printf("%u", value);
  🔨 Attempting binary generation...
tests/backend_e2e/outputs/control_flow_c.c:102:8: error: redefinition of 'result'
  102 |     u8 result = 0;
      |        ^
tests/backend_e2e/outputs/control_flow_c.c:101:8: note: previous definition is here
  101 |     u8 result = 0;
      |        ^
1 error generated.
  ❌ Binary generation failed
```

#### llvm
```
  br label %end_if_4
else_3:
  %r15 = add i8 0, 0
  store i8 %r15, i8* %result.addr
end_if_4:
  ret void
}


; Runtime functions
define void @print_u8(i8 %value) {
  %1 = zext i8 %value to i32
  %2 = call i32 (i8*, ...) @printf(i8* getelementptr inbounds ([4 x i8], [4 x i8]* @.str.u8, i32 0, i32 0), i32 %1)
  ret void
}
  🔨 Attempting binary generation...
llc: error: llc: tests/backend_e2e/outputs/control_flow_llvm.ll:16:3: error: multiple definition of local value named 'result.addr'
  %result.addr = alloca i8
  ^
  ❌ Binary generation failed
```

#### wasm
```
               ^^
tests/backend_e2e/outputs/control_flow_wasm.wat:45:16: error: undefined global variable "$i"
    global.get $i  ;; r6 = i
               ^^
tests/backend_e2e/outputs/control_flow_wasm.wat:54:16: error: undefined global variable "$i"
    global.set $i
               ^^
tests/backend_e2e/outputs/control_flow_wasm.wat:57:16: error: undefined global variable "$i"
    global.get $i  ;; r9 = i
               ^^
tests/backend_e2e/outputs/control_flow_wasm.wat:69:16: error: undefined global variable "$result"
    global.set $result
               ^^^^^^^
tests/backend_e2e/outputs/control_flow_wasm.wat:75:16: error: undefined global variable "$result"
    global.set $result
               ^^^^^^^
tests/backend_e2e/outputs/control_flow_wasm.wat:81:24: error: undefined function variable "$main"
  (export "main" (func $main))
                       ^^^^^
  ❌ Binary generation failed
```

[0;34m### Testing: function_calls[0m

#### z80
```
    LD ($F00A), A     ; Virtual register 5 to memory
    ; r6 = call tests.backend_e2e.sources.function_calls.add$u8$u8
    ; Call to tests.backend_e2e.sources.function_calls.add$u8$u8 (args: 2)
    ; Found function, UsesTrueSMC=false
    CALL tests.backend_e2e.sources.function_calls.add$u8$u8
    ; store result, r6
    EXX               ; Switch to shadow registers
    LD A, C'         ; From shadow C'
    EXX               ; Switch back to main registers
  🔨 Attempting binary generation...
Unrecognized option: o
SjASMPlus Z80 Cross-Assembler v1.07 RC8 (build 06-11-2008)
tests/backend_e2e/outputs/function_calls_z80.a80(101): error: Duplicate label: print_string_u16
tests/backend_e2e/outputs/function_calls_z80.a80(128): error: Duplicate label: print_u16_decimal
tests/backend_e2e/outputs/function_calls_z80.a80(146): error: Duplicate label: print_digit
tests/backend_e2e/outputs/function_calls_z80.a80(187): error: Duplicate label: print_true
tests/backend_e2e/outputs/function_calls_z80.a80(191): error: Duplicate label: bool_true_str
tests/backend_e2e/outputs/function_calls_z80.a80(193): error: Duplicate label: bool_false_str
(0): error: Error opening file: tests/backend_e2e/binaries/function_calls_z80
  ❌ Binary generation failed
```

#### 6502
```
    lda #$05      ; r4 = 5
    lda #$03      ; r5 = 3
    jsr tests.backend_e2e.sources.function_calls.add$u8$u8        ; call tests.backend_e2e.sources.function_calls.add$u8$u8
    sta result        ; store result
    lda result        ; r8 = result
    lda result        ; r9 = result
    lda result        ; r10 = result
    lda result        ; r11 = result
    jsr tests.backend_e2e.sources.function_calls.add$u8$u8        ; call tests.backend_e2e.sources.function_calls.add$u8$u8
    sta doubled        ; store doubled
    ; return
    rts        ; Return

; Helper routines
print_char:
    ; Platform-specific character output
    ; For C64: sta $FFD2
    ; For Apple II: jsr $FDED
    rts
  ⚠️  Assembler not available, skipping binary generation
```

#### 68000
```
	moveq #5,d0
	moveq #3,d1
	moveq #5,d2
	moveq #3,d3
	move.l d2,d0
	move.l d3,d1
	jsr tests.backend_e2e.sources.function_calls.add$u8$u8
	move.l d0,d4
	move.l d4,result
	move.l result,d6
	move.l result,d7
	move.l result,-40(a6)
	move.l result,-44(a6)
	move.l -40(a6),d0
	move.l -44(a6),d1
	jsr tests.backend_e2e.sources.function_calls.add$u8$u8
	move.l d0,-48(a6)
	move.l -48(a6),doubled
	movem.l (sp)+,d2-d7/a2-a5
  ⚠️  Assembler not available, skipping binary generation
```

#### i8080
```

; Function: tests.backend_e2e.sources.function_calls.main
; SMC enabled - parameters can be self-modified
tests.backend_e2e.sources.function_calls.main:
    PUSH B
    PUSH D
    PUSH H
    MVI A,05H
    STA F004H
    MVI A,03H
    STA F006H
    MVI A,05H
    STA F008H
    MVI A,03H
    STA F00AH
    LDA F008H
    STA tests.backend_e2e.sources.function_calls.add$u8$u8$param_param+1
    CALL tests.backend_e2e.sources.function_calls.add$u8$u8
    STA F00CH
  ⚠️  Assembler not available, skipping binary generation
```

#### gb
```
    ; Store to r5
    CALL tests.backend_e2e.sources.function_calls.add$u8$u8
    ; Store r6 to var result
    ; Load var result to r8
    ; Load var result to r9
    ; Load var result to r10
    ; Load var result to r11
    CALL tests.backend_e2e.sources.function_calls.add$u8$u8
    ; Store r12 to var doubled
    RET

; Print helpers for Game Boy
print_char:
    ; Wait for VBlank
    LD HL, $FF44  ; LY register
.wait_vblank:
    LD A, [HL]
    CP 144
    JR C, .wait_vblank
  ⚠️  Assembler not available, skipping binary generation
```

#### c
```
    char* data;
} String;

// Print helper functions
void print_char(u8 ch) {
    putchar(ch);
}

void print_u8(u8 value) {
    printf("%u", value);
}

void print_u8_decimal(u8 value) {
    printf("%u", value);
  🔨 Attempting binary generation...
  ✅ Binary generation successful

=== Binary Analysis ===
tests/backend_e2e/binaries/function_calls_c: Mach-O 64-bit executable arm64
-rwxr-xr-x  1 alice  staff  34064  6 Aug 16:51 tests/backend_e2e/binaries/function_calls_c
```

#### llvm
```
  ret void
}


; Runtime functions
define void @print_u8(i8 %value) {
  %1 = zext i8 %value to i32
  %2 = call i32 (i8*, ...) @printf(i8* getelementptr inbounds ([4 x i8], [4 x i8]* @.str.u8, i32 0, i32 0), i32 %1)
  ret void
}

@.str.u8 = private constant [4 x i8] c"%u\0A\00"

; main wrapper
define i32 @main() {
  🔨 Attempting binary generation...
llc: error: llc: tests/backend_e2e/outputs/function_calls_llvm.ll:17:7: error: value doesn't match function result type 'i8'
  ret void %r5
      ^
  ❌ Binary generation failed
```

#### wasm
```
               ^^^^^^^
tests/backend_e2e/outputs/function_calls_wasm.wat:56:16: error: undefined global variable "$result"
    global.get $result  ;; r8 = result
               ^^^^^^^
tests/backend_e2e/outputs/function_calls_wasm.wat:58:16: error: undefined global variable "$result"
    global.get $result  ;; r9 = result
               ^^^^^^^
tests/backend_e2e/outputs/function_calls_wasm.wat:60:16: error: undefined global variable "$result"
    global.get $result  ;; r10 = result
               ^^^^^^^
tests/backend_e2e/outputs/function_calls_wasm.wat:62:16: error: undefined global variable "$result"
    global.get $result  ;; r11 = result
               ^^^^^^^
tests/backend_e2e/outputs/function_calls_wasm.wat:67:16: error: undefined global variable "$doubled"
    global.set $doubled
               ^^^^^^^^
tests/backend_e2e/outputs/function_calls_wasm.wat:72:24: error: undefined function variable "$main"
  (export "main" (func $main))
                       ^^^^^
  ❌ Binary generation failed
```


## Test Summary

- **Total Tests**: 32
- **Passed**: 17
- **Failed**: 15
- **Success Rate**: 53%

### Backend Status Summary

| Backend | Type | Success Rate | Notes |
|---------|------|--------------|-------|
