#import "library.c"
unsigned short library(unsigned char);
const char *own = "yz";
unsigned short check(unsigned char x) {return library(x) + (unsigned char)own[x % 3];}
