unsigned char a, b;
unsigned short result;
unsigned char abs_diff(unsigned char x, unsigned char y) {
    if (x > y) return x - y;
    return y - x;
}
void execute(void) { result = abs_diff(a, b); }
