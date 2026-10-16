struct Triple { unsigned char a,b,c; };
struct Quad { unsigned char a,b,c,d; };
struct Words { unsigned short a,b; };
struct Mixed { unsigned short a; unsigned char b,c; };
struct Triple make_triple(unsigned char x) {struct Triple p;p.a=x;p.b=x+1;p.c=x^90;return p;}
unsigned short consume_triple(struct Triple p) {return ((unsigned short)p.a<<8)^((unsigned short)p.b<<4)^p.c;}
unsigned short triple_check(unsigned char x) {struct Triple p=make_triple(x);return consume_triple(p)+x;}
struct Quad make_quad(unsigned char x) {struct Quad p;p.a=x;p.b=x+1;p.c=x^90;p.d=x+3;return p;}
unsigned short consume_quad(struct Quad p) {return ((unsigned short)p.a<<8)^((unsigned short)p.b<<4)^((unsigned short)p.c<<2)^p.d;}
unsigned short quad_check(unsigned char x) {struct Quad p=make_quad(x);return consume_quad(p)+x;}
struct Words make_words(unsigned char x) {struct Words p;p.a=(unsigned short)x*257;p.b=(unsigned short)x*257+3;return p;}
unsigned short consume_words(struct Words p) {return p.a^p.b;}
unsigned short words_check(unsigned char x) {struct Words p=make_words(x);return consume_words(p)+x;}
struct Mixed make_mixed(unsigned char x) {struct Mixed p;p.a=(unsigned short)x*257;p.b=x+1;p.c=x^90;return p;}
unsigned short consume_mixed(struct Mixed p) {return p.a^((unsigned short)p.b<<4)^p.c;}
unsigned short mixed_check(unsigned char x) {struct Mixed p=make_mixed(x);return consume_mixed(p)+x;}

struct ByteSingle { unsigned char a; };
struct WordSingle { unsigned short a; };
struct BytePair { unsigned char a, b; };
struct ByteWord { unsigned char a; unsigned short b; };
struct WordByte { unsigned short a; unsigned char b; };
struct MiddleWord { unsigned char a; unsigned short b; unsigned char c; };
struct TailWord { unsigned char a, b; unsigned short c; };

struct ByteSingle make_bytesingle(unsigned char x) {
    struct ByteSingle p; p.a=x^90; return p;
}
unsigned short consume_bytesingle(struct ByteSingle p) { return (unsigned short)p.a*257; }
unsigned short bytesingle_check(unsigned char x) {
    struct ByteSingle p=make_bytesingle(x); return consume_bytesingle(p)+x;
}
struct WordSingle make_wordsingle(unsigned char x) {
    struct WordSingle p; p.a=(unsigned short)x*257+0x1357; return p;
}
unsigned short consume_wordsingle(struct WordSingle p) { return p.a^0x5a5a; }
unsigned short wordsingle_check(unsigned char x) {
    struct WordSingle p=make_wordsingle(x); return consume_wordsingle(p)+x;
}
struct BytePair make_bytepair(unsigned char x) {
    struct BytePair p; p.a=x; p.b=x^90; return p;
}
unsigned short consume_bytepair(struct BytePair p) { return ((unsigned short)p.a<<4)^p.b; }
unsigned short bytepair_check(unsigned char x) {
    struct BytePair p=make_bytepair(x); return consume_bytepair(p)+x;
}
struct ByteWord make_byteword(unsigned char x) {
    struct ByteWord p; p.a=x; p.b=(unsigned short)x*257+0x1357; return p;
}
unsigned short consume_byteword(struct ByteWord p) { return ((unsigned short)p.a<<4)^p.b; }
unsigned short byteword_check(unsigned char x) {
    struct ByteWord p=make_byteword(x); return consume_byteword(p)+x;
}
struct WordByte make_wordbyte(unsigned char x) {
    struct WordByte p; p.a=(unsigned short)x*257+0x1357; p.b=x^90; return p;
}
unsigned short consume_wordbyte(struct WordByte p) { return p.a^((unsigned short)p.b<<4); }
unsigned short wordbyte_check(unsigned char x) {
    struct WordByte p=make_wordbyte(x); return consume_wordbyte(p)+x;
}
struct MiddleWord make_middleword(unsigned char x) {
    struct MiddleWord p; p.a=x; p.b=(unsigned short)x*257+0x1357; p.c=x^90; return p;
}
unsigned short consume_middleword(struct MiddleWord p) { return ((unsigned short)p.a<<4)^p.b^p.c; }
unsigned short middleword_check(unsigned char x) {
    struct MiddleWord p=make_middleword(x); return consume_middleword(p)+x;
}
struct TailWord make_tailword(unsigned char x) {
    struct TailWord p; p.a=x; p.b=x^90; p.c=(unsigned short)x*257+0x1357; return p;
}
unsigned short consume_tailword(struct TailWord p) { return ((unsigned short)p.a<<4)^p.b^p.c; }
unsigned short tailword_check(unsigned char x) {
    struct TailWord p=make_tailword(x); return consume_tailword(p)+x;
}

/* Signed value transport, with all intermediate arithmetic inside int16. */
struct SignedMixed { signed short a; signed char b, c; };
struct SignedTail { signed char a, b; signed short c; };
struct SignedMixed make_signedmixed(unsigned char x) {
    struct SignedMixed p;
    p.a=(int)x-128; p.b=(int)(x&127)-64; p.c=(int)(x>>1)-64;
    return p;
}
unsigned short consume_signedmixed(struct SignedMixed p) { return (unsigned short)(p.a*32+p.b*4+p.c); }
unsigned short signedmixed_check(unsigned char x) {
    struct SignedMixed p=make_signedmixed(x); return consume_signedmixed(p)+x;
}
struct SignedTail make_signedtail(unsigned char x) {
    struct SignedTail p;
    p.a=(int)(x&127)-64; p.b=(int)(x>>1)-64; p.c=(int)x-128;
    return p;
}
unsigned short consume_signedtail(struct SignedTail p) { return (unsigned short)(p.c*32+p.a*4+p.b); }
unsigned short signedtail_check(unsigned char x) {
    struct SignedTail p=make_signedtail(x); return consume_signedtail(p)+x;
}
