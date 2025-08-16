; Test file for MZA enhanced features
ORG $8000

; Test local labels
main:
    LD A, 0
.loop:
    INC A
    CP 10
    JR NZ, .loop
    
; Test multi-arg instructions
start:
    PUSH AF, BC, DE, HL    ; Multi-arg PUSH
    ; Some code here
    POP HL, DE, BC, AF     ; Multi-arg POP (reverse order)
    
; Test fake instructions
    LD HL, DE              ; Fake instruction
    LD BC, HL              ; Another fake instruction
    
; Test string escapes
message:
    DB "Hello\nWorld\0"    ; String with escapes
    DB 'Test\'s "quotes"'  ; Mixed quotes and escapes
    
; Test shift operations
shift_demo:
    SRL A, A, A           ; Shift right 3 times
    RLC B, B              ; Rotate left 2 times
    
END