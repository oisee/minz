/* MinZ ctype.h for Z80 targets */
/* Inline implementations — no lookup table, minimal code size */
#ifndef _MINZ_CTYPE_H
#define _MINZ_CTYPE_H

static int isdigit(int c) { return c >= '0' && c <= '9'; }
static int isxdigit(int c) { return isdigit(c) || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F'); }
static int islower(int c) { return c >= 'a' && c <= 'z'; }
static int isupper(int c) { return c >= 'A' && c <= 'Z'; }
static int isalpha(int c) { return islower(c) || isupper(c); }
static int isalnum(int c) { return isalpha(c) || isdigit(c); }
static int isspace(int c) { return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' || c == '\v'; }
static int isprint(int c) { return c >= 0x20 && c <= 0x7E; }
static int isgraph(int c) { return c > 0x20 && c <= 0x7E; }
static int ispunct(int c) { return isgraph(c) && !isalnum(c); }
static int iscntrl(int c) { return (c >= 0 && c < 0x20) || c == 0x7F; }
static int tolower(int c) { return isupper(c) ? c + 32 : c; }
static int toupper(int c) { return islower(c) ? c - 32 : c; }

#endif /* _MINZ_CTYPE_H */
