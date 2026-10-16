unsigned long a, b, result;
unsigned long step(unsigned long x) {
    return (x << 7) ^ (x >> 3) ^ 0x12345678UL;
}
unsigned long joined(unsigned long x, unsigned long y) { return step(x) + y; }
unsigned long live(unsigned long x, unsigned long y) {
    unsigned long r = step(x);
    return (r + y) ^ x;
}
unsigned long forwarded(unsigned long x, unsigned long y) {
    return live(y, x) + joined(x, y);
}
void execute_joined(void) { result = joined(a, b); }
void execute_live(void) { result = live(a, b); }
void execute_forwarded(void) { result = forwarded(a, b); }
