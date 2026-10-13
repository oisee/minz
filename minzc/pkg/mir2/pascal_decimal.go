package mir2

// Complete generated decimal runtime bodies, shared by the frontend and
// native runtime admission. Formatting oracles remain independent.
const PascalUnsignedDecimalASM = "PUSH IX / PUSH HL / POP IX / LD D, 0" +
	" / PUSH IX / POP HL / LD BC, 10000 / CALL _wr16_dig / PUSH HL / POP IX" +
	" / PUSH IX / POP HL / LD BC, 1000 / CALL _wr16_dig / PUSH HL / POP IX" +
	" / PUSH IX / POP HL / LD BC, 100 / CALL _wr16_dig / PUSH HL / POP IX" +
	" / PUSH IX / POP HL / LD BC, 10 / CALL _wr16_dig / PUSH HL / POP IX" +
	" / LD A, IXL / ADD A, 48 / LD E, A / LD C, 2 / CALL 5 / POP IX"

const PascalSignedDecimalPrefix = "PUSH IX / BIT 7, H / JR Z, _wr_i16_positive / PUSH HL / LD E, 45 / LD C, 2 / CALL 5 / POP HL / XOR A / SUB L / LD L, A / SBC A, A / SUB H / LD H, A / _wr_i16_positive:"

const PascalDecimalDigitASM = "LD A, 48 / _w16_sub: OR A / SBC HL, BC / JR NC, _w16_cont" +
	" / ADD HL, BC / CP 48 / JR NZ, _w16_pr / LD A, D / OR A / RET Z / LD A, 48" +
	" / _w16_pr: LD D, 1 / LD E, A / PUSH HL / PUSH DE / PUSH BC / LD C, 2 / CALL 5 / POP BC / POP DE / POP HL / RET" +
	" / _w16_cont: INC A / JR _w16_sub"
