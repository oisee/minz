struct Pair { unsigned char hi, lo; };
struct Pair make_pair(unsigned char x) {
    struct Pair p = {0, 0};
    p.hi = x; p.lo = (unsigned char)(x + 1);
    return p;
}
void touch(struct Pair *p) { p->hi = (unsigned char)(p->hi + 2); }
unsigned short consume(struct Pair p) {
    touch(&p);
    return (unsigned short)((unsigned short)p.hi * 256u + p.lo);
}
unsigned short check(unsigned char x) {
    struct Pair p = make_pair(x);
    unsigned short result = consume(p);
    return (unsigned short)(result + p.hi);
}
