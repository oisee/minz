unsigned char effects;
struct Pair { unsigned char hi, lo; };
struct Pair make_pair(unsigned char x) {
    struct Pair p;
    effects = effects + 1;
    p.hi = x; p.lo = x + 1;
    return p;
}
struct Pair forward(unsigned char x) { return make_pair(x); }
struct Pair transform(struct Pair p) { p.hi = p.hi + 2; return p; }
unsigned short consume(struct Pair p) { return (unsigned short)p.hi*256u + p.lo; }
unsigned short direct(unsigned char x) {
    effects = 0;
    return consume(transform(forward(x))) + x;
}
unsigned short copied(unsigned char x) {
    struct Pair p, q;
    unsigned short result;
    effects = 0;
    p = forward(x);
    q = transform(p);
    result = consume(q);
    return result + p.hi;
}
unsigned char count_check(unsigned char x) { copied(x); return effects; }
