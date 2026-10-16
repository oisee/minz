typedef struct { unsigned char a, b; } Pair;
unsigned char left, right, events;
unsigned short result;
void fill(unsigned char x, unsigned char y, Pair *out) {
    out->a = x + y;
    x = x + 3;
    events = events * 3 + 1;
    if (x >= 128) {
        out->b = x ^ y;
        return;
    }
    out->b = x + 7;
}
void execute(void) {
    Pair p = { 0, 0 };
    fill(left, right, &p);
    result = ((unsigned short)p.a << 8) | p.b;
}
