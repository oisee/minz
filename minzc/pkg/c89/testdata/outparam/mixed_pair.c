typedef struct { short w; unsigned char b; } Pair;
unsigned char left, right, events;
unsigned short result;
void fill(unsigned char x, unsigned char y, Pair *out) {
    out->w = (short)x - (short)y;
    events = events * 3 + 1;
    out->b = x ^ y;
}
void execute(void) {
    Pair p;
    fill(left, right, &p);
    result = (unsigned short)p.w ^ p.b;
}
