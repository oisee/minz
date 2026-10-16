#import "strings.c"
unsigned short library_string(unsigned char);
unsigned short check(unsigned char x) {
 const char *a = "other";
 const char *b = "packet";
 return library_string(x) + (unsigned char)a[x % 6] + (unsigned char)b[x % 7];
}
