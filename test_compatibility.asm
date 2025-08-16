; Compatibility test - features that both MZA and SjASMPlus support
	ORG $8000

; Test local labels
main:
	LD A, 0
.loop:
	INC A
	CP 10
	JR NZ, .loop
	
; Test multi-arg instructions (MZA feature - will expand for SjASMPlus)
start:
	PUSH AF, BC, DE, HL
	POP HL, DE, BC, AF
	
; Test fake instructions (MZA feature - will expand for SjASMPlus)
	LD HL, DE
	LD BC, HL
	
; Test basic string (no problematic escapes)
message:
	DB "Hello World", 0
	
; Test basic operations that both support
	LD A, $FF
	LD B, 255
	LD C, A
	
	END