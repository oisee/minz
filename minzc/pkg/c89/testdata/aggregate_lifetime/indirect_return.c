struct Pair { unsigned char a, b; };
struct Pair make_pair(unsigned char x) {
    struct Pair p = {0, 0}; p.a = x; p.b = x + 1; return p;
}
unsigned short consume(struct Pair p) {return (unsigned short)p.a * 256u + p.b;}
unsigned short direct(unsigned char x) {
    struct Pair (*f)(unsigned char) = make_pair;
    return consume(f(x));
}
unsigned short copied(unsigned char x) {
    struct Pair (*f)(unsigned char) = make_pair;
    struct Pair p = f(x);
    return consume(p);
}
