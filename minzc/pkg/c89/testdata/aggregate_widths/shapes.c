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
