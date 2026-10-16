struct Pair { unsigned char hi, lo; };
struct Pair make_pair(unsigned char x) {
    struct Pair p={0,0};
    p.hi=x; p.lo=(unsigned char)(x+1);
    return p;
}
unsigned short consume(struct Pair p) {
    return (unsigned short)((unsigned short)p.hi*256u+p.lo);
}
unsigned short own(unsigned char x) {
    struct Pair p=make_pair(x);
    return consume(p);
}
