unsigned short library_string(unsigned char x) {
 const char *a = "packet";
 const char *b = "Z\n";
 return (unsigned char)a[x % 7] + (unsigned char)b[x % 3];
}
