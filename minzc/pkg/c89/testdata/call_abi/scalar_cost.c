unsigned char a, b;
unsigned short result;
struct Pair { unsigned char hi, lo; };
struct Pair make_pair(unsigned char x) {
    struct Pair p = {0, 0}; p.hi = x; p.lo = x + 1; return p;
}
unsigned short consume(struct Pair p) {return (unsigned short)p.hi * 256u + p.lo;}
unsigned short caller(unsigned char x, unsigned char y) {
    struct Pair p = make_pair(x);
    return (consume(p) + x) ^ y;
}
void execute(void) { result = caller(a, b); }
