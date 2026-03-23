; Source: https://github.com/jacobly0/fasmg-ez80/blob/main/tests/ez80insts.src
; License: Unlicense (https://github.com/jacobly0/fasmg-ez80/blob/main/README.md)
; Description: Complete eZ80 instruction set encoding reference with hex opcodes.
;              Base Z80-compatible instructions (no suffix = default mode).
;
; Format: MNEMONIC ; expected_hex_encoding
; This is the gold standard for differential testing of instruction encoding.

	NOP  ; 00
	LD BC,$0000 ; 01 $0000
	LD (BC),A ; 02
	INC BC ; 03
	INC B ; 04
	DEC B ; 05
	LD B,$00 ; 06 $00
	RLCA  ; 07
	EX AF,AF' ; 08
	ADD HL,BC ; 09
	LD A,(BC) ; 0A
	DEC BC ; 0B
	INC C ; 0C
	DEC C ; 0D
	LD C,$00 ; 0E $00
	RRCA  ; 0F
	DJNZ $+$00 ; 10 $00
	LD DE,$0000 ; 11 $0000
	LD (DE),A ; 12
	INC DE ; 13
	INC D ; 14
	DEC D ; 15
	LD D,$00 ; 16 $00
	RLA  ; 17
	JR $+$00 ; 18 $00
	ADD HL,DE ; 19
	LD A,(DE) ; 1A
	DEC DE ; 1B
	INC E ; 1C
	DEC E ; 1D
	LD E,$00 ; 1E $00
	RRA  ; 1F
	JR NZ,$+$00 ; 20 $00
	LD HL,$0000 ; 21 $0000
	LD ($0000),HL ; 22 $0000
	INC HL ; 23
	INC H ; 24
	DEC H ; 25
	LD H,$00 ; 26 $00
	DAA  ; 27
	JR Z,$+$00 ; 28 $00
	ADD HL,HL ; 29
	LD HL,($0000) ; 2A $0000
	DEC HL ; 2B
	INC L ; 2C
	DEC L ; 2D
	LD L,$00 ; 2E $00
	CPL  ; 2F
	JR NC,$+$00 ; 30 $00
	LD SP,$0000 ; 31 $0000
	LD ($0000),A ; 32 $0000
	INC SP ; 33
	INC (HL) ; 34
	DEC (HL) ; 35
	LD (HL),$00 ; 36 $00
	SCF  ; 37
	JR C,$+$00 ; 38 $00
	ADD HL,SP ; 39
	LD A,($0000) ; 3A $0000
	DEC SP ; 3B
	INC A ; 3C
	DEC A ; 3D
	LD A,$00 ; 3E $00
	CCF  ; 3F
	LD B,C ; 41
	LD B,D ; 42
	LD B,E ; 43
	LD B,H ; 44
	LD B,L ; 45
	LD B,(HL) ; 46
	LD B,A ; 47
	LD C,B ; 48
	LD C,D ; 4A
	LD C,E ; 4B
	LD C,H ; 4C
	LD C,L ; 4D
	LD C,(HL) ; 4E
	LD C,A ; 4F
	LD D,B ; 50
	LD D,C ; 51
	LD D,E ; 53
	LD D,H ; 54
	LD D,L ; 55
	LD D,(HL) ; 56
	LD D,A ; 57
	LD E,B ; 58
	LD E,C ; 59
	LD E,D ; 5A
	LD E,H ; 5C
	LD E,L ; 5D
	LD E,(HL) ; 5E
	LD E,A ; 5F
	LD H,B ; 60
	LD H,C ; 61
	LD H,D ; 62
	LD H,E ; 63
	LD H,H ; 64
	LD H,L ; 65
	LD H,(HL) ; 66
	LD H,A ; 67
	LD L,B ; 68
	LD L,C ; 69
	LD L,D ; 6A
	LD L,E ; 6B
	LD L,H ; 6C
	LD L,L ; 6D
	LD L,(HL) ; 6E
	LD L,A ; 6F
	LD (HL),B ; 70
	LD (HL),C ; 71
	LD (HL),D ; 72
	LD (HL),E ; 73
	LD (HL),H ; 74
	LD (HL),L ; 75
	HALT  ; 76
	LD (HL),A ; 77
	LD A,B ; 78
	LD A,C ; 79
	LD A,D ; 7A
	LD A,E ; 7B
	LD A,H ; 7C
	LD A,L ; 7D
	LD A,(HL) ; 7E
	LD A,A ; 7F
	ADD A,B ; 80
	ADD A,C ; 81
	ADD A,D ; 82
	ADD A,E ; 83
	ADD A,H ; 84
	ADD A,L ; 85
	ADD A,(HL) ; 86
	ADD A,A ; 87
	ADC A,B ; 88
	ADC A,C ; 89
	ADC A,D ; 8A
	ADC A,E ; 8B
	ADC A,H ; 8C
	ADC A,L ; 8D
	ADC A,(HL) ; 8E
	ADC A,A ; 8F
	SUB A,B ; 90
	SUB A,C ; 91
	SUB A,D ; 92
	SUB A,E ; 93
	SUB A,H ; 94
	SUB A,L ; 95
	SUB A,(HL) ; 96
	SUB A,A ; 97
	SBC A,B ; 98
	SBC A,C ; 99
	SBC A,D ; 9A
	SBC A,E ; 9B
	SBC A,H ; 9C
	SBC A,L ; 9D
	SBC A,(HL) ; 9E
	SBC A,A ; 9F
	AND A,B ; A0
	AND A,C ; A1
	AND A,D ; A2
	AND A,E ; A3
	AND A,H ; A4
	AND A,L ; A5
	AND A,(HL) ; A6
	AND A,A ; A7
	XOR A,B ; A8
	XOR A,C ; A9
	XOR A,D ; AA
	XOR A,E ; AB
	XOR A,H ; AC
	XOR A,L ; AD
	XOR A,(HL) ; AE
	XOR A,A ; AF
	OR A,B ; B0
	OR A,C ; B1
	OR A,D ; B2
	OR A,E ; B3
	OR A,H ; B4
	OR A,L ; B5
	OR A,(HL) ; B6
	OR A,A ; B7
	CP A,B ; B8
	CP A,C ; B9
	CP A,D ; BA
	CP A,E ; BB
	CP A,H ; BC
	CP A,L ; BD
	CP A,(HL) ; BE
	CP A,A ; BF
	RET NZ ; C0
	POP BC ; C1
	JP NZ,$0000 ; C2 $0000
	JP $0000 ; C3 $0000
	CALL NZ,$0000 ; C4 $0000
	PUSH BC ; C5
	ADD A,$00 ; C6 $00
	RST 0H ; C7
	RET Z ; C8
	RET  ; C9
	JP Z,$0000 ; CA $0000
