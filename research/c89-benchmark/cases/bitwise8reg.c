/* Port of the SDCC upstream byte-bitwise register-pressure kernel. */
unsigned char c0, c1, c2, c3;
unsigned short result;
unsigned char and8(void) {
    unsigned char t0, t1, t2, t3, t4, t5;
    t0 = c0 + 1; t1 = c1 + 1; t2 = c2 + 1; t3 = c3 + 1;
    t4 = t0 & t1; t5 = t2 & t3;
    return t4 + t5;
}
unsigned char or8(void) {
    unsigned char t0, t1, t2, t3, t4, t5;
    t0 = c0 + 1; t1 = c1 + 1; t2 = c2 + 1; t3 = c3 + 1;
    t4 = t0 | t1; t5 = t2 | t3;
    return t4 + t5;
}
void execute(void) {
    unsigned char low = and8();
    unsigned char high = or8();
    result = low | ((unsigned short)high << 8);
}
