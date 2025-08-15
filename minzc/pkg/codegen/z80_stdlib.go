package codegen

// generateStdlibFunction generates a specific stdlib function if it's used
func (g *Z80Generator) generateStdlibFunction(name string) {
	switch name {
	case "cls":
		g.generateCls()
	case "print_newline":
		g.generatePrintNewline()
	case "print_hex_u8":
		g.generatePrintHexU8()
	case "print_hex_nibble":
		g.generatePrintHexNibble()
	case "print_hex_digit":
		g.generatePrintHexDigit()
	case "print_string":
		g.generatePrintString()
	case "print_u8_decimal":
		g.generatePrintU8Decimal()
	case "print_u16_decimal":
		g.generatePrintU16Decimal()
	case "print_i8_decimal":
		g.generatePrintI8Decimal()
	case "print_i16_decimal":
		g.generatePrintI16Decimal()
	case "print_digit":
		g.generatePrintDigit()
	case "zx_set_border":
		g.generateZxSetBorder()
	case "zx_clear_screen":
		g.generateZxClearScreen()
	case "zx_set_pixel":
		g.generateZxSetPixel()
	case "zx_set_ink":
		g.generateZxSetInk()
	case "zx_set_paper":
		g.generateZxSetPaper()
	case "zx_read_keyboard":
		g.generateZxReadKeyboard()
	case "zx_wait_key":
		g.generateZxWaitKey()
	case "zx_is_key_pressed":
		g.generateZxIsKeyPressed()
	case "zx_beep":
		g.generateZxBeep()
	case "zx_click":
		g.generateZxClick()
	case "abs":
		g.generateAbs()
	case "min":
		g.generateMin()
	case "max":
		g.generateMax()
	}
}

func (g *Z80Generator) generateCls() {
	g.emit("cls:")
	switch g.targetPlatform {
	case "cpm":
		// CP/M clear screen using ANSI escape codes
		g.emit("    LD C, 2            ; BDOS function 2 (console output)")
		g.emit("    LD E, 27           ; ESC character")
		g.emit("    CALL 5             ; Call BDOS")
		g.emit("    LD E, '['          ; [")
		g.emit("    CALL 5")
		g.emit("    LD E, '2'          ; 2")
		g.emit("    CALL 5")
		g.emit("    LD E, 'J'          ; J (clear screen)")
		g.emit("    CALL 5")
	case "msx":
		g.emit("    CALL $00C3         ; MSX BIOS CLS")
	case "cpc", "amstrad":
		g.emit("    CALL $BC14         ; CPC SCR CLEAR")
	default: // ZX Spectrum
		g.emit("    LD HL, $4000       ; Screen start")
		g.emit("    LD DE, $4001")
		g.emit("    LD BC, $17FF       ; Screen size - 1")
		g.emit("    LD (HL), 0")
		g.emit("    LDIR               ; Clear screen")
		g.emit("    LD HL, $5800       ; Attribute start")
		g.emit("    LD DE, $5801")
		g.emit("    LD BC, $02FF       ; Attribute size - 1")
		g.emit("    LD (HL), $38       ; White ink on black paper")
		g.emit("    LDIR               ; Clear attributes")
	}
	g.emit("    RET")
	g.emit("")
}

func (g *Z80Generator) generatePrintNewline() {
	g.emit("print_newline:")
	g.emit("    LD A, 13           ; CR")
	g.emit("    RST 16")
	g.emit("    RET")
	g.emit("")
}

func (g *Z80Generator) generatePrintHexU8() {
	g.emit("print_hex_u8:")
	g.emit("    PUSH AF            ; Save value")
	g.emit("    RRA")
	g.emit("    RRA")
	g.emit("    RRA")
	g.emit("    RRA                ; High nibble to low")
	g.emit("    CALL print_hex_nibble")
	g.emit("    POP AF             ; Restore value")
	g.emit("    ; Fall through to print low nibble")
}

func (g *Z80Generator) generatePrintHexNibble() {
	g.emit("print_hex_nibble:")
	g.emit("    AND $0F            ; Isolate low nibble")
	g.emit("    ADD A, '0'         ; Convert to ASCII")
	g.emit("    CP '9' + 1")
	g.emit("    JR C, print_hex_digit")
	g.emit("    ADD A, 'A' - '0' - 10  ; Adjust for A-F")
}

func (g *Z80Generator) generatePrintHexDigit() {
	g.emit("print_hex_digit:")
	g.emit("    RST 16             ; ZX Spectrum print")
	g.emit("    RET")
	g.emit("")
}

func (g *Z80Generator) generatePrintString() {
	g.emit("print_string:")
	g.emit("    LD A, (HL)         ; A = first byte")
	g.emit("    CP 255             ; Check if extended format marker")
	g.emit("    JR NZ, print_string_u8")
	g.emit("    ; Extended format: skip marker, actual length follows")
	g.emit("    INC HL             ; Skip marker")
	g.emit("    LD E, (HL)         ; Load low byte of length")
	g.emit("    INC HL")
	g.emit("    LD D, (HL)         ; Load high byte of length")
	g.emit("    INC HL             ; HL points to string data")
	g.emit("    ; DE = length, HL = string data")
	g.emit("    JR print_string_loop_de")
	g.emit("print_string_u8:")
	g.emit("    LD E, A            ; E = length (u8)")
	g.emit("    LD D, 0            ; D = 0")
	g.emit("    INC HL             ; HL points to string data")
	g.emit("print_string_loop_de:")
	g.emit("    ; Check if DE (length) is zero")
	g.emit("    LD A, D")
	g.emit("    OR E")
	g.emit("    RET Z              ; Return if length is zero")
	g.emit("print_string_char_loop:")
	g.emit("    LD A, (HL)         ; Load character")
	g.emit("    RST 16             ; Print character")
	g.emit("    INC HL             ; Next character")
	g.emit("    DEC DE             ; Decrement length")
	g.emit("    LD A, D")
	g.emit("    OR E")
	g.emit("    JR NZ, print_string_char_loop")
	g.emit("    RET")
	g.emit("")
}

func (g *Z80Generator) generatePrintU8Decimal() {
	g.emit("print_u8_decimal:")
	g.emit("    ; Print u8 in A as decimal")
	g.emit("    LD H, 0")
	g.emit("    LD L, A            ; HL = value")
	g.emit("    ; Fall through to print_u16_decimal")
}

func (g *Z80Generator) generatePrintU16Decimal() {
	g.emit("print_u16_decimal:")
	g.emit("    ; Print u16 in HL as decimal")
	g.emit("    LD DE, 10000       ; Start with 10000s")
	g.emit("    CALL print_digit")
	g.emit("    LD DE, 1000        ; 1000s")
	g.emit("    CALL print_digit")
	g.emit("    LD DE, 100         ; 100s")
	g.emit("    CALL print_digit")
	g.emit("    LD DE, 10          ; 10s")
	g.emit("    CALL print_digit")
	g.emit("    LD A, L            ; 1s")
	g.emit("    ADD A, '0'")
	g.emit("    RST 16")
	g.emit("    RET")
	g.emit("")
}

func (g *Z80Generator) generatePrintDigit() {
	g.emit("print_digit:")
	g.emit("    ; Divide HL by DE, print quotient")
	g.emit("    XOR A              ; Clear counter")
	g.emit("print_digit_loop:")
	g.emit("    SBC HL, DE         ; Subtract")
	g.emit("    JR C, print_digit_done")
	g.emit("    INC A              ; Count")
	g.emit("    JR print_digit_loop")
	g.emit("print_digit_done:")
	g.emit("    ADD HL, DE         ; Restore")
	g.emit("    ADD A, '0'         ; Convert to ASCII")
	g.emit("    RST 16             ; Print")
	g.emit("    RET")
	g.emit("")
}

func (g *Z80Generator) generatePrintI8Decimal() {
	g.emit("print_i8_decimal:")
	g.emit("    ; Print i8 in A as decimal")
	g.emit("    BIT 7, A           ; Check sign")
	g.emit("    JR Z, print_u8_decimal")
	g.emit("    PUSH AF")
	g.emit("    LD A, '-'")
	g.emit("    RST 16             ; Print minus")
	g.emit("    POP AF")
	g.emit("    NEG                ; Make positive")
	g.emit("    JR print_u8_decimal")
	g.emit("")
}

func (g *Z80Generator) generatePrintI16Decimal() {
	g.emit("print_i16_decimal:")
	g.emit("    ; Print i16 in HL as decimal")
	g.emit("    BIT 7, H           ; Check sign")
	g.emit("    JR Z, print_u16_decimal")
	g.emit("    PUSH HL")
	g.emit("    LD A, '-'")
	g.emit("    RST 16             ; Print minus")
	g.emit("    POP HL")
	g.emit("    ; Negate HL")
	g.emit("    XOR A")
	g.emit("    SUB L")
	g.emit("    LD L, A")
	g.emit("    LD A, 0")
	g.emit("    SBC A, H")
	g.emit("    LD H, A")
	g.emit("    JR print_u16_decimal")
	g.emit("")
}

func (g *Z80Generator) generateZxSetBorder() {
	g.emit("zx_set_border:")
	g.emit("    POP HL             ; Return address")
	g.emit("    POP BC             ; Get color argument")
	g.emit("    PUSH HL            ; Restore return address")
	g.emit("    LD A, C            ; Color to A")
	g.emit("    AND 7              ; Mask to 0-7")
	g.emit("    OUT (254), A       ; Set border")
	g.emit("    RET")
	g.emit("")
}

func (g *Z80Generator) generateZxClearScreen() {
	g.emit("zx_clear_screen:")
	g.emit("    JP cls             ; Use standard cls")
	g.emit("")
}

func (g *Z80Generator) generateZxSetPixel() {
	g.emit("zx_set_pixel:")
	g.emit("    ; TODO: Implement pixel setting")
	g.emit("    ; For now, just return")
	g.emit("    RET")
	g.emit("")
}

func (g *Z80Generator) generateZxSetInk() {
	g.emit("zx_set_ink:")
	g.emit("    ; TODO: Implement ink color setting")
	g.emit("    RET")
	g.emit("")
}

func (g *Z80Generator) generateZxSetPaper() {
	g.emit("zx_set_paper:")
	g.emit("    ; TODO: Implement paper color setting")
	g.emit("    RET")
	g.emit("")
}

func (g *Z80Generator) generateZxReadKeyboard() {
	g.emit("; Input routines")
	g.emit("zx_read_keyboard:")
	g.emit("    ; Scan keyboard matrix")
	g.emit("    LD BC, $FEFE       ; First keyboard row")
	g.emit("    IN A, (C)          ; Read keyboard")
	g.emit("    CPL                ; Invert bits")
	g.emit("    AND $1F            ; Mask relevant bits")
	g.emit("    RET Z              ; Return 0 if no key")
	g.emit("    ; Simple mapping - just return raw value for now")
	g.emit("    RET")
	g.emit("")
}

func (g *Z80Generator) generateZxWaitKey() {
	g.emit("zx_wait_key:")
	g.emit("wait_key_loop:")
	g.emit("    CALL zx_read_keyboard")
	g.emit("    OR A               ; Test if zero")
	g.emit("    JR Z, wait_key_loop ; Loop if no key")
	g.emit("    RET                ; Return key code in A")
	g.emit("")
}

func (g *Z80Generator) generateZxIsKeyPressed() {
	g.emit("zx_is_key_pressed:")
	g.emit("    POP HL             ; Return address")
	g.emit("    POP BC             ; Get key code")
	g.emit("    PUSH HL            ; Restore return address")
	g.emit("    ; TODO: Implement specific key checking")
	g.emit("    LD A, 0            ; Return false for now")
	g.emit("    RET")
	g.emit("")
}

func (g *Z80Generator) generateZxBeep() {
	g.emit("; Sound routines")
	g.emit("zx_beep:")
	g.emit("    POP HL             ; Return address")
	g.emit("    POP DE             ; Duration")
	g.emit("    POP BC             ; Pitch")
	g.emit("    PUSH HL            ; Restore return address")
	g.emit("    ; Simple beep using OUT to speaker")
	g.emit("beep_loop:")
	g.emit("    LD A, 16           ; Speaker bit")
	g.emit("    OUT (254), A       ; Speaker on")
	g.emit("    PUSH BC")
	g.emit("beep_delay1:")
	g.emit("    DEC BC")
	g.emit("    LD A, B")
	g.emit("    OR C")
	g.emit("    JR NZ, beep_delay1")
	g.emit("    POP BC")
	g.emit("    XOR A              ; Speaker off")
	g.emit("    OUT (254), A")
	g.emit("    PUSH BC")
	g.emit("beep_delay2:")
	g.emit("    DEC BC")
	g.emit("    LD A, B")
	g.emit("    OR C")
	g.emit("    JR NZ, beep_delay2")
	g.emit("    POP BC")
	g.emit("    DEC DE")
	g.emit("    LD A, D")
	g.emit("    OR E")
	g.emit("    JR NZ, beep_loop")
	g.emit("    RET")
	g.emit("")
}

func (g *Z80Generator) generateZxClick() {
	g.emit("zx_click:")
	g.emit("    LD A, 16           ; Quick click")
	g.emit("    OUT (254), A")
	g.emit("    LD B, 10")
	g.emit("click_delay:")
	g.emit("    DJNZ click_delay")
	g.emit("    XOR A")
	g.emit("    OUT (254), A")
	g.emit("    RET")
	g.emit("")
}

func (g *Z80Generator) generateAbs() {
	g.emit("abs:")
	g.emit("    POP HL             ; Return address")
	g.emit("    POP BC             ; Get argument")
	g.emit("    PUSH HL            ; Restore return address")
	g.emit("    LD A, C            ; Value to A")
	g.emit("    OR A               ; Test sign")
	g.emit("    JP P, abs_done     ; If positive, done")
	g.emit("    NEG                ; Negate if negative")
	g.emit("abs_done:")
	g.emit("    RET")
	g.emit("")
}

func (g *Z80Generator) generateMin() {
	g.emit("min:")
	g.emit("    POP HL             ; Return address")
	g.emit("    POP BC             ; First argument")
	g.emit("    POP DE             ; Second argument")
	g.emit("    PUSH HL            ; Restore return address")
	g.emit("    LD A, C            ; First value")
	g.emit("    CP E               ; Compare with second")
	g.emit("    JR C, min_done     ; If first < second, keep first")
	g.emit("    LD A, E            ; Otherwise use second")
	g.emit("min_done:")
	g.emit("    RET")
	g.emit("")
}

func (g *Z80Generator) generateMax() {
	g.emit("max:")
	g.emit("    POP HL             ; Return address")
	g.emit("    POP BC             ; First argument")
	g.emit("    POP DE             ; Second argument")
	g.emit("    PUSH HL            ; Restore return address")
	g.emit("    LD A, C            ; First value")
	g.emit("    CP E               ; Compare with second")
	g.emit("    JR NC, max_done    ; If first >= second, keep first")
	g.emit("    LD A, E            ; Otherwise use second")
	g.emit("max_done:")
	g.emit("    RET")
	g.emit("")
}