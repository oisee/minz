#ifndef MINZ_SYSTEM_FAT12_H
#define MINZ_SYSTEM_FAT12_H
/* Read-only FAT12, 512-byte sectors, root directory and one open file.
 * Errors: 0 OK, 1 I/O, 2 invalid volume, 3 not found, 4 corrupt chain.
 * The single reusable sector buffer makes this API non-reentrant. */
typedef unsigned char BYTE;
typedef unsigned int WORD;
typedef unsigned long DWORD;
BYTE block_read(WORD sector, BYTE *dst);
BYTE fat_mount(void);
BYTE fat_find(const BYTE *name11);
WORD fat_read(BYTE *dst);
#endif
