typedef struct { unsigned char a, b; } Pair;
unsigned char left, right, events;
unsigned short result;
void fill(unsigned char x, unsigned char y, Pair *out) {
    out->b = y;
    y = y + 1;
    events = events * 3 + 5;
    out->a = x + y;
    out->b = y ^ 0xa5;
}
void execute(void) {
    Pair p;
    fill(left, right, &p);
    result = ((unsigned short)p.a << 8) | p.b;
}
