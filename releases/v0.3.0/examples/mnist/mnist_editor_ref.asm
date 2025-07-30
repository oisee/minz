; MNIST Editor for ZX Spectrum
; Attribute-based editor with XOR cursor
; Controls: Q/A=up/down, O/P=left/right, SPACE/M=toggle pixel

    DEVICE ZXSPECTRUM48
    ORG $8000

; Constants
SCREEN_START    EQU $4000
ATTR_START      EQU $5800
BORDER_PORT     EQU $FE

; Variables
cursor_x:       DB 0    ; 0-31 (attribute position)
cursor_y:       DB 0    ; 0-23 (attribute position)
old_attr:       DB 0    ; Store original attribute under cursor

; Entry point
start:
    DI                      ; Disable interrupts
    LD SP, $FFFE           ; Set stack
    
    ; Set border to black
    XOR A
    OUT (BORDER_PORT), A
    
    ; Fill screen with $FF pattern
    LD HL, SCREEN_START
    LD DE, SCREEN_START + 1
    LD BC, 6143             ; 6144 - 1
    LD (HL), $FF
    LDIR
    
    ; Fill attributes with paper/ink 00 (black on black)
    LD HL, ATTR_START
    LD DE, ATTR_START + 1
    LD BC, 767              ; 768 - 1
    LD (HL), $00
    LDIR
    
    ; Initialize cursor position (center)
    LD A, 16
    LD (cursor_x), A
    LD A, 12
    LD (cursor_y), A
    
    ; Main loop
main_loop:
    CALL update_cursor      ; Show cursor with XOR
    CALL scan_keys         ; Check keyboard
    JR main_loop

; Update cursor display
update_cursor:
    ; Calculate attribute address: ATTR_START + (y * 32) + x
    LD A, (cursor_y)
    LD L, A
    LD H, 0
    ADD HL, HL              ; y * 2
    ADD HL, HL              ; y * 4
    ADD HL, HL              ; y * 8
    ADD HL, HL              ; y * 16
    ADD HL, HL              ; y * 32
    
    LD A, (cursor_x)
    LD E, A
    LD D, 0
    ADD HL, DE              ; + x
    
    LD DE, ATTR_START
    ADD HL, DE              ; + ATTR_START
    
    ; XOR attribute with %100 (bit 2) to toggle BRIGHT
    LD A, (HL)
    XOR %00000100           ; Toggle bit 2
    LD (HL), A
    
    RET

; Scan keyboard
scan_keys:
    ; Small delay for key repeat
    LD BC, $0100
delay_loop:
    DEC BC
    LD A, B
    OR C
    JR NZ, delay_loop
    
    ; Check Q (up) - port $FB, bit 0
    LD A, $FB
    IN A, ($FE)
    BIT 0, A
    JR NZ, check_a
    
    ; Move cursor up
    LD A, (cursor_y)
    OR A
    JR Z, check_a           ; Already at top
    DEC A
    LD (cursor_y), A
    
check_a:
    ; Check A (down) - port $FD, bit 0
    LD A, $FD
    IN A, ($FE)
    BIT 0, A
    JR NZ, check_o
    
    ; Move cursor down
    LD A, (cursor_y)
    CP 23
    JR Z, check_o           ; Already at bottom
    INC A
    LD (cursor_y), A
    
check_o:
    ; Check O (left) - port $DF, bit 1
    LD A, $DF
    IN A, ($FE)
    BIT 1, A
    JR NZ, check_p
    
    ; Move cursor left
    LD A, (cursor_x)
    OR A
    JR Z, check_p           ; Already at left
    DEC A
    LD (cursor_x), A
    
check_p:
    ; Check P (right) - port $DF, bit 0
    LD A, $DF
    IN A, ($FE)
    BIT 0, A
    JR NZ, check_space
    
    ; Move cursor right
    LD A, (cursor_x)
    CP 31
    JR Z, check_space       ; Already at right
    INC A
    LD (cursor_x), A
    
check_space:
    ; Check SPACE - port $7F, bit 0
    LD A, $7F
    IN A, ($FE)
    BIT 0, A
    JR NZ, check_m
    
    ; Toggle pixel
    CALL toggle_pixel
    
check_m:
    ; Check M - port $7F, bit 2
    LD A, $7F
    IN A, ($FE)
    BIT 2, A
    JR NZ, check_exit
    
    ; Toggle pixel
    CALL toggle_pixel
    
check_exit:
    ; Check CTRL+SPACE (Symbol Shift + Space for exit)
    LD A, $7F
    IN A, ($FE)
    BIT 1, A                ; Symbol Shift
    JR NZ, scan_done
    BIT 0, A                ; Space
    JR NZ, scan_done
    
    ; Exit
    JP 0                    ; Reset
    
scan_done:
    RET

; Toggle pixel at cursor position
toggle_pixel:
    PUSH AF
    PUSH BC
    PUSH DE
    PUSH HL
    
    ; For 16x16 interpretation in top-left corner
    ; We'll use the first 2 bytes of each of the first 16 lines
    
    ; Calculate which bit in the 16x16 grid
    ; cursor_x (0-31) -> bit_x (0-15)
    ; cursor_y (0-23) -> bit_y (0-15)
    
    LD A, (cursor_x)
    AND $0F                 ; bit_x = cursor_x & 15
    LD B, A                 ; B = bit_x
    
    LD A, (cursor_y)
    AND $0F                 ; bit_y = cursor_y & 15
    LD C, A                 ; C = bit_y
    
    ; Calculate screen address for this pixel
    ; Using simplified addressing for first 16 lines
    LD A, C                 ; bit_y
    LD L, A
    LD H, 0
    ADD HL, HL              ; * 2
    ADD HL, HL              ; * 4
    ADD HL, HL              ; * 8
    ADD HL, HL              ; * 16
    ADD HL, HL              ; * 32 (bytes per line)
    
    ; Add byte offset (bit_x / 8)
    LD A, B
    SRL A
    SRL A
    SRL A                   ; bit_x / 8
    LD E, A
    LD D, 0
    ADD HL, DE
    
    LD DE, SCREEN_START
    ADD HL, DE              ; HL = screen address
    
    ; Calculate bit mask
    LD A, B
    AND 7                   ; bit_x & 7
    LD B, A
    LD A, $80               ; Start with bit 7
    
    ; Shift right B times
    INC B
shift_loop:
    DEC B
    JR Z, shift_done
    SRL A
    JR shift_loop
shift_done:
    
    ; Toggle the bit
    LD B, A                 ; Save mask
    LD A, (HL)
    XOR B                   ; Toggle bit
    LD (HL), A
    
    POP HL
    POP DE
    POP BC
    POP AF
    RET

