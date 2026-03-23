;--------------------------------------------------------
; File Created by SDCC : free open source ANSI-C Compiler
; Version 4.2.0 #13081 (Linux)
;--------------------------------------------------------
	.module ff
	.optsdcc -mz80
	
;--------------------------------------------------------
; Public variables in this module
;--------------------------------------------------------
	.globl _disk_ioctl
	.globl _disk_write
	.globl _disk_read
	.globl _disk_status
	.globl _disk_initialize
	.globl _get_fattime
	.globl _memcmp
	.globl _f_mount
	.globl _f_open
	.globl _f_read
	.globl _f_write
	.globl _f_sync
	.globl _f_close
	.globl _f_lseek
	.globl _f_opendir
	.globl _f_closedir
	.globl _f_readdir
	.globl _f_stat
	.globl _f_getfree
	.globl _f_truncate
	.globl _f_unlink
	.globl _f_mkdir
	.globl _f_rename
;--------------------------------------------------------
; special function registers
;--------------------------------------------------------
;--------------------------------------------------------
; ram data
;--------------------------------------------------------
	.area _DATA
_FatFs:
	.ds 2
_Fsid:
	.ds 2
;--------------------------------------------------------
; ram data
;--------------------------------------------------------
	.area _INITIALIZED
;--------------------------------------------------------
; absolute external ram data
;--------------------------------------------------------
	.area _DABS (ABS)
;--------------------------------------------------------
; global & static initialisations
;--------------------------------------------------------
	.area _HOME
	.area _GSINIT
	.area _GSFINAL
	.area _GSINIT
;--------------------------------------------------------
; Home
;--------------------------------------------------------
	.area _HOME
	.area _HOME
;--------------------------------------------------------
; code
;--------------------------------------------------------
	.area _CODE
;ff.c:619: static WORD ld_16 (const BYTE* ptr)	/*	 Load a 2-byte little-endian word */
;	---------------------------------
; Function ld_16
; ---------------------------------
_ld_16:
	ex	de, hl
;ff.c:623: rv = ptr[1];
	ld	c, e
	ld	b, d
	inc	bc
	ld	a, (bc)
;ff.c:624: rv = rv << 8 | ptr[0];
	ld	b, a
	ld	c, #0x00
	ld	a, (de)
	ld	e, a
	ld	d, #0x00
	ld	a, e
	or	a, c
	ld	e, a
	ld	a, d
	or	a, b
	ld	d, a
;ff.c:625: return rv;
;ff.c:626: }
	ret
_DbcTbl:
	.db #0x81	; 129
	.db #0x9f	; 159
	.db #0xe0	; 224
	.db #0xfc	; 252
	.db #0x40	; 64
	.db #0x7e	; 126
	.db #0x80	; 128
	.db #0xfc	; 252
	.db #0x00	; 0
	.db #0x00	; 0
;ff.c:628: static DWORD ld_32 (const BYTE* ptr)	/* Load a 4-byte little-endian word */
;	---------------------------------
; Function ld_32
; ---------------------------------
_ld_32:
	push	ix
	ld	ix,#0
	add	ix,sp
	push	af
	push	af
;ff.c:632: rv = ptr[3];
;	spillPairReg hl
;	spillPairReg hl
	ld	e, l
	ld	d, h
;	spillPairReg hl
;	spillPairReg hl
	inc	hl
	inc	hl
	inc	hl
	ld	c, (hl)
	ld	b, #0x00
	ld	hl, #0x0000
;ff.c:633: rv = rv << 8 | ptr[2];
	ld	-3 (ix), c
	ld	-2 (ix), b
	ld	-1 (ix), l
	ld	-4 (ix), #0x00
	ld	l, e
;	spillPairReg hl
;	spillPairReg hl
	ld	h, d
;	spillPairReg hl
;	spillPairReg hl
	inc	hl
	inc	hl
	ld	c, (hl)
	ld	b, #0x00
	ld	hl, #0x0000
	ld	a, -4 (ix)
	or	a, c
	ld	c, a
	ld	a, -3 (ix)
	or	a, b
	ld	b, a
	ld	a, -2 (ix)
	or	a, l
	ld	l, a
;	spillPairReg hl
;	spillPairReg hl
;ff.c:634: rv = rv << 8 | ptr[1];
	ld	-3 (ix), c
	ld	-2 (ix), b
	ld	-1 (ix), l
	ld	-4 (ix), #0x00
	ld	l, e
;	spillPairReg hl
;	spillPairReg hl
	ld	h, d
;	spillPairReg hl
;	spillPairReg hl
	inc	hl
	ld	c, (hl)
	ld	b, #0x00
	ld	hl, #0x0000
	ld	a, c
	or	a, -4 (ix)
	ld	c, a
	ld	a, b
	or	a, -3 (ix)
	ld	b, a
	ld	a, l
	or	a, -2 (ix)
	ld	l, a
;	spillPairReg hl
;	spillPairReg hl
	ld	a, h
	or	a, -1 (ix)
;	spillPairReg hl
;	spillPairReg hl
;ff.c:635: rv = rv << 8 | ptr[0];
	ld	h, c
;	spillPairReg hl
;	spillPairReg hl
	ld	c, b
	ld	b, l
	ld	l, #0x00
;	spillPairReg hl
;	spillPairReg hl
	ld	a, (de)
	push	iy
	ex	(sp), hl
	ld	h, #0x00
;	spillPairReg hl
;	spillPairReg hl
	ex	(sp), hl
	pop	iy
	ld	de, #0x0000
	or	a, l
	ld	l, a
;	spillPairReg hl
;	spillPairReg hl
	push	iy
	ld	a, -5 (ix)
	pop	iy
	or	a, h
	ld	h, a
;	spillPairReg hl
;	spillPairReg hl
	ld	a, e
	or	a, c
	ld	e, a
	ld	a, d
	or	a, b
	ld	d, a
;ff.c:636: return rv;
	ex	de, hl
;ff.c:637: }
	ld	sp, ix
	pop	ix
	ret
;ff.c:657: static void st_16 (BYTE* ptr, WORD val)	/* Store a 2-byte word in little-endian */
;	---------------------------------
; Function st_16
; ---------------------------------
_st_16:
;ff.c:659: *ptr++ = (BYTE)val; val >>= 8;
	ld	(hl), e
	inc	hl
;ff.c:660: *ptr++ = (BYTE)val;
	ld	(hl), d
;ff.c:661: }
	ret
;ff.c:663: static void st_32 (BYTE* ptr, DWORD val)	/* Store a 4-byte word in little-endian */
;	---------------------------------
; Function st_32
; ---------------------------------
_st_32:
	push	ix
	ld	ix,#0
	add	ix,sp
;ff.c:665: *ptr++ = (BYTE)val; val >>= 8;
	ld	a, 4 (ix)
	ld	(hl), a
	inc	hl
	ld	b, #0x08
00103$:
	srl	7 (ix)
	rr	6 (ix)
	rr	5 (ix)
	rr	4 (ix)
	djnz	00103$
;ff.c:666: *ptr++ = (BYTE)val; val >>= 8;
	ld	a, 4 (ix)
	ld	(hl), a
	inc	hl
	ld	b, #0x08
00105$:
	srl	7 (ix)
	rr	6 (ix)
	rr	5 (ix)
	rr	4 (ix)
	djnz	00105$
;ff.c:667: *ptr++ = (BYTE)val; val >>= 8;
	ld	a, 4 (ix)
	ld	(hl), a
	inc	hl
	ld	b, #0x08
00107$:
	srl	7 (ix)
	rr	6 (ix)
	rr	5 (ix)
	rr	4 (ix)
	djnz	00107$
;ff.c:668: *ptr++ = (BYTE)val;
	ld	a, 4 (ix)
	ld	(hl), a
;ff.c:669: }
	pop	ix
	pop	hl
	pop	af
	pop	af
	jp	(hl)
;ff.c:693: static int dbc_1st (BYTE c)
;	---------------------------------
; Function dbc_1st
; ---------------------------------
_dbc_1st:
	ld	c, a
;ff.c:701: if (c >= DbcTbl[0]) {
	ld	hl, #_DbcTbl
	ld	b, (hl)
	ld	a, c
	sub	a, b
	jr	C, 00107$
;ff.c:702: if (c <= DbcTbl[1]) return 1;
	ld	a, (#_DbcTbl + 1)
	sub	a, c
	jr	C, 00102$
	ld	de, #0x0001
	ret
00102$:
;ff.c:703: if (c >= DbcTbl[2] && c <= DbcTbl[3]) return 1;
	ld	hl, #_DbcTbl + 2
	ld	b, (hl)
	ld	a, c
	sub	a, b
	jr	C, 00107$
	ld	a, (#_DbcTbl + 3)
	sub	a, c
	jr	C, 00107$
	ld	de, #0x0001
	ret
00107$:
;ff.c:708: return 0;
	ld	de, #0x0000
;ff.c:709: }
	ret
;ff.c:713: static int dbc_2nd (BYTE c)
;	---------------------------------
; Function dbc_2nd
; ---------------------------------
_dbc_2nd:
	ld	c, a
;ff.c:722: if (c >= DbcTbl[4]) {
	ld	hl, #_DbcTbl + 4
	ld	b, (hl)
	ld	a, c
	sub	a, b
	jr	C, 00110$
;ff.c:723: if (c <= DbcTbl[5]) return 1;
	ld	a, (#_DbcTbl + 5)
	sub	a, c
	jr	C, 00102$
	ld	de, #0x0001
	ret
00102$:
;ff.c:724: if (c >= DbcTbl[6] && c <= DbcTbl[7]) return 1;
	ld	hl, #_DbcTbl + 6
	ld	b, (hl)
	ld	a, c
	sub	a, b
	jr	C, 00104$
	ld	a, (#_DbcTbl + 7)
	sub	a, c
	jr	C, 00104$
	ld	de, #0x0001
	ret
00104$:
;ff.c:725: if (c >= DbcTbl[8] && c <= DbcTbl[9]) return 1;
	ld	hl, #_DbcTbl + 8
	ld	b, (hl)
	ld	a, c
	sub	a, b
	jr	C, 00110$
	ld	a, (#_DbcTbl + 9)
	sub	a, c
	jr	C, 00110$
	ld	de, #0x0001
	ret
00110$:
;ff.c:730: return 0;
	ld	de, #0x0000
;ff.c:731: }
	ret
;ff.c:1057: static FRESULT sync_window (	/* Returns FR_OK or FR_DISK_ERR */
;	---------------------------------
; Function sync_window
; ---------------------------------
_sync_window:
	push	ix
	ld	ix,#0
	add	ix,sp
	ld	iy, #-21
	add	iy, sp
	ld	sp, iy
	ld	-2 (ix), l
	ld	-1 (ix), h
;ff.c:1061: FRESULT res = FR_OK;
	ld	-17 (ix), #0x00
;ff.c:1064: if (fs->wflag) {	/* Is the disk access window dirty? */
	ld	a, -2 (ix)
	add	a, #0x04
	ld	-6 (ix), a
	ld	a, -1 (ix)
	adc	a, #0x00
	ld	-5 (ix), a
	ld	l, -6 (ix)
	ld	h, -5 (ix)
	ld	a, (hl)
	ld	-3 (ix), a
	or	a, a
	jp	Z, 00109$
;ff.c:1065: if (disk_write(fs->pdrv, fs->win, fs->winsect, 1) == RES_OK) {	/* Write it back into the volume */
	ld	a, -2 (ix)
	add	a, #0x1c
	ld	-4 (ix), a
	ld	a, -1 (ix)
	adc	a, #0x00
	ld	-3 (ix), a
	ld	l, -4 (ix)
	ld	h, -3 (ix)
	ld	c, (hl)
	inc	hl
	ld	b, (hl)
	inc	hl
	ld	e, (hl)
	inc	hl
	ld	d, (hl)
	ld	a, -2 (ix)
	add	a, #0x30
	ld	-16 (ix), a
	ld	a, -1 (ix)
	adc	a, #0x00
	ld	-15 (ix), a
	ld	a, -2 (ix)
	add	a, #0x01
	ld	-14 (ix), a
	ld	a, -1 (ix)
	adc	a, #0x00
	ld	-13 (ix), a
	ld	l, -14 (ix)
	ld	h, -13 (ix)
	ld	l, (hl)
;	spillPairReg hl
	ld	iy, #0x0001
	push	iy
	push	de
	push	bc
	ld	e, -16 (ix)
	ld	d, -15 (ix)
	ld	a, l
	call	_disk_write
	or	a, a
	jp	NZ, 00106$
;ff.c:1066: fs->wflag = 0;	/* Clear window dirty flag */
	ld	l, -6 (ix)
	ld	h, -5 (ix)
	ld	(hl), #0x00
;ff.c:1067: if (fs->winsect - fs->fatbase < fs->fsize) {	/* Is it in the 1st FAT? */
	ld	e, -4 (ix)
	ld	d, -3 (ix)
	ld	hl, #9
	add	hl, sp
	ex	de, hl
	ld	bc, #0x0004
	ldir
	ld	c, -2 (ix)
	ld	b, -1 (ix)
	ld	hl, #36
	add	hl, bc
	ld	c, (hl)
	inc	hl
	ld	b, (hl)
	inc	hl
	ld	e, (hl)
	inc	hl
	ld	d, (hl)
	ld	a, -12 (ix)
	sub	a, c
	ld	c, a
	ld	a, -11 (ix)
	sbc	a, b
	ld	b, a
	ld	a, -10 (ix)
	sbc	a, e
	ld	e, a
	ld	a, -9 (ix)
	sbc	a, d
	ld	d, a
	ld	l, -2 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -1 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	de
	push	bc
	ex	de, hl
	ld	hl, #17
	add	hl, sp
	ex	de, hl
	ld	bc, #0x0018
	add	hl, bc
	ld	bc, #0x0004
	ldir
	pop	bc
	pop	de
	ld	a, c
	sub	a, -8 (ix)
	ld	a, b
	sbc	a, -7 (ix)
	ld	a, e
	sbc	a, -6 (ix)
	ld	a, d
	sbc	a, -5 (ix)
	jr	NC, 00109$
;ff.c:1068: if (fs->n_fats == 2) disk_write(fs->pdrv, fs->win, fs->winsect + fs->fsize, 1);	/* Reflect it to 2nd FAT if needed */
	ld	a, -2 (ix)
	ld	-4 (ix), a
	ld	a, -1 (ix)
	ld	-3 (ix), a
	ld	l, -4 (ix)
	ld	h, -3 (ix)
	inc	hl
	inc	hl
	inc	hl
	ld	a, (hl)
	ld	-3 (ix), a
	sub	a, #0x02
	jr	NZ, 00109$
	ld	a, -8 (ix)
	add	a, -12 (ix)
	ld	-21 (ix), a
	ld	a, -7 (ix)
	adc	a, -11 (ix)
	ld	-20 (ix), a
	ld	a, -6 (ix)
	adc	a, -10 (ix)
	ld	-19 (ix), a
	ld	a, -5 (ix)
	adc	a, -9 (ix)
	ld	-18 (ix), a
	ld	l, -14 (ix)
	ld	h, -13 (ix)
	ld	a, (hl)
	ld	-3 (ix), a
	ld	hl, #0x0001
	push	hl
	ld	l, -19 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -18 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	ld	l, -21 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -20 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	ld	e, -16 (ix)
	ld	d, -15 (ix)
	ld	a, -3 (ix)
	call	_disk_write
	jr	00109$
00106$:
;ff.c:1071: res = FR_DISK_ERR;
	ld	-17 (ix), #0x01
00109$:
;ff.c:1074: return res;
	ld	a, -17 (ix)
;ff.c:1075: }
	ld	sp, ix
	pop	ix
	ret
;ff.c:1079: static FRESULT move_window (	/* Returns FR_OK or FR_DISK_ERR */
;	---------------------------------
; Function move_window
; ---------------------------------
_move_window:
	push	ix
	ld	ix,#0
	add	ix,sp
	push	af
	dec	sp
	ld	c, l
	ld	b, h
;ff.c:1084: FRESULT res = FR_OK;
	ld	-3 (ix), #0x00
;ff.c:1087: if (sect != fs->winsect) {	/* Window offset changed? */
	ld	hl, #0x001c
	add	hl, bc
	ld	-2 (ix), l
	ld	-1 (ix), h
	ld	e, (hl)
	inc	hl
	ld	d, (hl)
	inc	hl
	inc	hl
	ld	a, (hl)
	dec	hl
	ld	l, (hl)
;	spillPairReg hl
	ld	h, a
;	spillPairReg hl
;	spillPairReg hl
	ld	a, 4 (ix)
	sub	a, e
	jr	NZ, 00124$
	ld	a, 5 (ix)
	sub	a, d
	jr	NZ, 00124$
	ld	a, l
	sub	a, 6 (ix)
	jr	NZ, 00124$
	ld	a, h
	sub	a, 7 (ix)
	jr	Z, 00106$
00124$:
;ff.c:1089: res = sync_window(fs);		/* Flush the window */
	push	bc
	ld	l, c
;	spillPairReg hl
;	spillPairReg hl
	ld	h, b
;	spillPairReg hl
;	spillPairReg hl
	call	_sync_window
	pop	bc
	ld	-3 (ix), a
;ff.c:1091: if (res == FR_OK) {			/* Fill sector window with new data */
	or	a, a
	jr	NZ, 00106$
;ff.c:1092: if (disk_read(fs->pdrv, fs->win, sect, 1) != RES_OK) {
	ld	hl, #0x0030
	add	hl, bc
	ex	de, hl
	inc	bc
	ld	a, (bc)
	ld	c, a
	ld	hl, #0x0001
	push	hl
	ld	l, 6 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, 7 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	ld	l, 4 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, 5 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	ld	a, c
	call	_disk_read
	or	a, a
	jr	Z, 00102$
;ff.c:1093: sect = (LBA_t)0 - 1;	/* Invalidate window if read data is not valid */
	ld	4 (ix), #0xff
	ld	5 (ix), #0xff
	ld	6 (ix), #0xff
	ld	7 (ix), #0xff
;ff.c:1094: res = FR_DISK_ERR;
	ld	-3 (ix), #0x01
00102$:
;ff.c:1096: fs->winsect = sect;
	ld	e, -2 (ix)
	ld	d, -1 (ix)
	ld	hl, #7
	add	hl, sp
	ld	bc, #0x0004
	ldir
00106$:
;ff.c:1099: return res;
	ld	a, -3 (ix)
;ff.c:1100: }
	ld	sp, ix
	pop	ix
	pop	hl
	pop	bc
	pop	bc
	jp	(hl)
;ff.c:1110: static FRESULT sync_fs (	/* Returns FR_OK or FR_DISK_ERR */
;	---------------------------------
; Function sync_fs
; ---------------------------------
_sync_fs:
	push	ix
	ld	ix,#0
	add	ix,sp
	ld	iy, #-9
	add	iy, sp
	ld	sp, iy
	ld	c, l
	ld	b, h
;ff.c:1117: res = sync_window(fs);
	push	bc
	ld	l, c
;	spillPairReg hl
;	spillPairReg hl
	ld	h, b
;	spillPairReg hl
;	spillPairReg hl
	call	_sync_window
	pop	bc
	ld	-9 (ix), a
;ff.c:1118: if (res == FR_OK) {
	or	a, a
	jp	NZ, 00108$
;ff.c:1119: if (fs->fsi_flag == 1) {	/* Allocation changed? */
	ld	hl, #0x0005
	add	hl, bc
	ld	e, (hl)
;ff.c:1129: disk_write(fs->pdrv, fs->win, fs->winsect = fs->volbase + 1, 1);	/* Write it into the FSInfo sector (Next to VBR) */
	ld	a, c
	add	a, #0x01
	ld	-8 (ix), a
	ld	a, b
	adc	a, #0x00
	ld	-7 (ix), a
;ff.c:1119: if (fs->fsi_flag == 1) {	/* Allocation changed? */
	dec	e
	jp	NZ,00104$
;ff.c:1120: fs->fsi_flag = 0;
	ld	(hl), #0x00
;ff.c:1121: if (fs->fs_type == FS_FAT32) {	/* FAT32: Update FSInfo sector */
	ld	a, (bc)
	sub	a, #0x03
	jp	NZ,00104$
;ff.c:1123: memset(fs->win, 0, sizeof fs->win);
	ld	hl, #0x0030
	add	hl, bc
;	spillPairReg hl
;	spillPairReg hl
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	push	bc
	ld	(hl), #0x00
	ld	e, l
	ld	d, h
	inc	de
	ld	bc, #0x01ff
	ldir
	pop	bc
	pop	de
;ff.c:1124: st_32(fs->win + FSI_LeadSig, 0x41615252);		/* Leading signature */
	push	bc
	push	de
	ld	hl, #0x4161
	push	hl
	ld	hl, #0x5252
;	spillPairReg hl
;	spillPairReg hl
	ex	de, hl
	push	de
;	spillPairReg hl
;	spillPairReg hl
	call	_st_32
	pop	de
	pop	bc
;ff.c:1125: st_32(fs->win + FSI_StrucSig, 0x61417272);		/* Structure signature */
	ld	hl, #0x0030
	add	hl, bc
	ld	-6 (ix), l
	ld	-5 (ix), h
	ld	a, -6 (ix)
	add	a, #0xe4
	ld	l, a
;	spillPairReg hl
;	spillPairReg hl
	ld	a, -5 (ix)
	adc	a, #0x01
	ld	h, a
;	spillPairReg hl
;	spillPairReg hl
	push	bc
	push	de
	ld	de, #0x6141
	push	de
	ld	de, #0x7272
	push	de
	call	_st_32
	pop	de
	pop	bc
;ff.c:1126: st_32(fs->win + FSI_Free_Count, fs->free_clst);	/* Number of free clusters */
	ld	l, c
;	spillPairReg hl
;	spillPairReg hl
	ld	h, b
;	spillPairReg hl
;	spillPairReg hl
	push	de
	push	bc
	ex	de, hl
	ld	hl, #9
	add	hl, sp
	ex	de, hl
	ld	bc, #0x0010
	add	hl, bc
	ld	bc, #0x0004
	ldir
	pop	bc
	pop	de
	ld	a, -6 (ix)
	add	a, #0xe8
	ld	l, a
;	spillPairReg hl
;	spillPairReg hl
	ld	a, -5 (ix)
	adc	a, #0x01
	ld	h, a
;	spillPairReg hl
;	spillPairReg hl
	push	bc
	push	de
	push	hl
	ld	l, -2 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -1 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ex	(sp), hl
	push	hl
	ld	l, -4 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -3 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ex	(sp), hl
	call	_st_32
	pop	de
	pop	bc
;ff.c:1127: st_32(fs->win + FSI_Nxt_Free, fs->last_clst);	/* Last allocated culuster */
	ld	l, c
;	spillPairReg hl
;	spillPairReg hl
	ld	h, b
;	spillPairReg hl
;	spillPairReg hl
	push	de
	push	bc
	ex	de, hl
	ld	hl, #9
	add	hl, sp
	ex	de, hl
	ld	bc, #0x000c
	add	hl, bc
	ld	bc, #0x0004
	ldir
	pop	bc
	pop	de
	ld	a, -6 (ix)
	add	a, #0xec
	ld	l, a
;	spillPairReg hl
;	spillPairReg hl
	ld	a, -5 (ix)
	adc	a, #0x01
	ld	h, a
;	spillPairReg hl
;	spillPairReg hl
	push	bc
	push	de
	push	hl
	ld	l, -2 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -1 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ex	(sp), hl
	push	hl
	ld	l, -4 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -3 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ex	(sp), hl
	call	_st_32
	pop	de
	pop	bc
;ff.c:1128: st_32(fs->win + FSI_TrailSig, 0xAA550000);		/* Trailing signature */
	ld	a, -6 (ix)
	add	a, #0xfc
	ld	l, a
;	spillPairReg hl
;	spillPairReg hl
	ld	a, -5 (ix)
	adc	a, #0x01
	ld	h, a
;	spillPairReg hl
;	spillPairReg hl
	push	bc
	push	de
	ld	de, #0xaa55
	push	de
	ld	de, #0x0000
	push	de
	call	_st_32
	pop	de
	pop	bc
;ff.c:1129: disk_write(fs->pdrv, fs->win, fs->winsect = fs->volbase + 1, 1);	/* Write it into the FSInfo sector (Next to VBR) */
	ld	hl, #0x001c
	add	hl, bc
	ld	-6 (ix), l
	ld	-5 (ix), h
	ld	hl, #32
	add	hl, bc
	ld	c, (hl)
	inc	hl
	ld	b, (hl)
	inc	hl
	inc	hl
	ld	a, (hl)
	dec	hl
	ld	l, (hl)
;	spillPairReg hl
	ld	h, a
;	spillPairReg hl
;	spillPairReg hl
	ld	a, c
	add	a, #0x01
	ld	-4 (ix), a
	ld	a, b
	adc	a, #0x00
	ld	-3 (ix), a
	ld	a, l
	adc	a, #0x00
	ld	-2 (ix), a
	ld	a, h
	adc	a, #0x00
	ld	-1 (ix), a
	push	de
	ld	e, -6 (ix)
	ld	d, -5 (ix)
	ld	hl, #7
	add	hl, sp
	ld	bc, #0x0004
	ldir
	pop	de
	ld	l, -8 (ix)
	ld	h, -7 (ix)
	ld	c, (hl)
	ld	hl, #0x0001
	push	hl
	ld	l, -2 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -1 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	ld	l, -4 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -3 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	ld	a, c
	call	_disk_write
00104$:
;ff.c:1145: if (disk_ioctl(fs->pdrv, CTRL_SYNC, 0) != RES_OK) res = FR_DISK_ERR;
	ld	l, -8 (ix)
	ld	h, -7 (ix)
	ld	c, (hl)
	ld	hl, #0x0000
	push	hl
	ld	l, #0x00
;	spillPairReg hl
;	spillPairReg hl
	ld	a, c
	call	_disk_ioctl
	or	a, a
	jr	Z, 00108$
	ld	-9 (ix), #0x01
00108$:
;ff.c:1148: return res;
	ld	a, -9 (ix)
;ff.c:1149: }
	ld	sp, ix
	pop	ix
	ret
;ff.c:1159: static LBA_t clst2sect (	/* !=0:Sector number, 0:Failed (invalid cluster#) */
;	---------------------------------
; Function clst2sect
; ---------------------------------
_clst2sect:
	push	ix
	ld	ix,#0
	add	ix,sp
	push	af
	push	af
	ex	de, hl
;ff.c:1164: clst -= 2;		/* Cluster number is origin from 2 */
	ld	a, 4 (ix)
	add	a, #0xfe
	ld	4 (ix), a
	ld	a, 5 (ix)
	adc	a, #0xff
	ld	5 (ix), a
	ld	a, 6 (ix)
	adc	a, #0xff
	ld	6 (ix), a
	ld	a, 7 (ix)
	adc	a, #0xff
	ld	7 (ix), a
;ff.c:1165: if (clst >= fs->n_fatent - 2) return 0;		/* Is it invalid cluster number? */
	ld	c, e
	ld	b, d
	ld	hl, #20
	add	hl, bc
	ld	c, (hl)
	inc	hl
	ld	b, (hl)
	inc	hl
	inc	hl
	ld	a, (hl)
	dec	hl
	ld	l, (hl)
;	spillPairReg hl
	ld	h, a
;	spillPairReg hl
;	spillPairReg hl
	ld	a, c
	add	a, #0xfe
	ld	c, a
	ld	a, b
	adc	a, #0xff
	ld	b, a
	ld	a, l
	adc	a, #0xff
	ld	l, a
;	spillPairReg hl
;	spillPairReg hl
	ld	a, h
	adc	a, #0xff
	ld	h, a
;	spillPairReg hl
;	spillPairReg hl
	ld	a, 4 (ix)
	sub	a, c
	ld	a, 5 (ix)
	sbc	a, b
	ld	a, 6 (ix)
	sbc	a, l
	ld	a, 7 (ix)
	sbc	a, h
	jr	C, 00102$
	ld	de, #0x0000
	ld	l, e
	ld	h, e
	jr	00103$
00102$:
;ff.c:1166: return fs->database + (LBA_t)fs->csize * clst;	/* Start sector number of the cluster */
	ld	l, e
;	spillPairReg hl
;	spillPairReg hl
	ld	h, d
;	spillPairReg hl
;	spillPairReg hl
	ex	de, hl
	push	hl
	ld	hl, #2
	add	hl, sp
	ex	de, hl
	ld	bc, #0x002c
	add	hl, bc
	ld	bc, #0x0004
	ldir
	pop	de
	ld	hl, #10
	add	hl, de
	ld	e, (hl)
	inc	hl
	ld	d, (hl)
	ld	hl, #0x0000
	ld	c, 6 (ix)
	ld	b, 7 (ix)
	push	bc
	ld	c, 4 (ix)
	ld	b, 5 (ix)
	push	bc
	call	__mullong
	pop	af
	pop	af
	ld	a, -4 (ix)
	add	a, e
	ld	e, a
	ld	a, -3 (ix)
	adc	a, d
	ld	d, a
	ld	a, -2 (ix)
	adc	a, l
	ld	l, a
;	spillPairReg hl
;	spillPairReg hl
	ld	a, -1 (ix)
	adc	a, h
	ld	h, a
;	spillPairReg hl
;	spillPairReg hl
00103$:
;ff.c:1167: }
	ld	sp, ix
	pop	ix
	ret
;ff.c:1176: static DWORD get_fat (		/* 0xFFFFFFFF:Disk error, 1:Internal error, 2..0x7FFFFFFF:Cluster status */
;	---------------------------------
; Function get_fat
; ---------------------------------
_get_fat:
	push	ix
	ld	ix,#0
	add	ix,sp
	ld	iy, #-14
	add	iy, sp
	ld	sp, iy
;ff.c:1183: FATFS *fs = obj->fs;
	ld	a, (hl)
	ld	-14 (ix), a
	inc	hl
	ld	a, (hl)
	ld	-13 (ix), a
;ff.c:1186: if (clst < 2 || clst >= fs->n_fatent) {	/* Check if in valid range */
	ld	a, 4 (ix)
	sub	a, #0x02
	ld	a, 5 (ix)
	sbc	a, #0x00
	ld	a, 6 (ix)
	sbc	a, #0x00
	ld	a, 7 (ix)
	sbc	a, #0x00
	jr	C, 00114$
	pop	bc
	push	bc
	ld	hl, #20
	add	hl, bc
	ld	c, (hl)
	inc	hl
	ld	b, (hl)
	inc	hl
	ld	e, (hl)
	inc	hl
	ld	d, (hl)
	ld	a, 4 (ix)
	sub	a, c
	ld	a, 5 (ix)
	sbc	a, b
	ld	a, 6 (ix)
	sbc	a, e
	ld	a, 7 (ix)
	sbc	a, d
	jr	C, 00115$
00114$:
;ff.c:1187: val = 1;	/* Internal error */
	ld	-12 (ix), #0x01
	xor	a, a
	ld	-11 (ix), a
	ld	-10 (ix), a
	ld	-9 (ix), a
	jp	00116$
00115$:
;ff.c:1190: val = 0xFFFFFFFF;	/* Default value falls on disk error */
	ld	-12 (ix), #0xff
	ld	-11 (ix), #0xff
	ld	-10 (ix), #0xff
	ld	-9 (ix), #0xff
;ff.c:1192: switch (fs->fs_type) {
	pop	hl
	push	hl
	ld	c, (hl)
;ff.c:1195: if (move_window(fs, fs->fatbase + (bc / SS(fs))) != FR_OK) break;
	ld	a, -14 (ix)
	add	a, #0x24
	ld	-2 (ix), a
	ld	a, -13 (ix)
	adc	a, #0x00
	ld	-1 (ix), a
;ff.c:1196: wc = fs->win[bc++ % SS(fs)];		/* Get 1st byte of the entry */
	ld	a, -14 (ix)
	add	a, #0x30
	ld	-8 (ix), a
	ld	a, -13 (ix)
	adc	a, #0x00
	ld	-7 (ix), a
;ff.c:1192: switch (fs->fs_type) {
	ld	a, c
	dec	a
	jr	Z, 00101$
	ld	a,c
	cp	a,#0x02
	jp	Z,00106$
	sub	a, #0x03
	jp	Z,00109$
	jp	00112$
;ff.c:1193: case FS_FAT12 :
00101$:
;ff.c:1194: bc = (UINT)clst; bc += bc / 2;
	ld	c, 4 (ix)
	ld	b, 5 (ix)
	ld	l, c
	ld	h, b
	srl	h
	rr	l
	add	hl, bc
;ff.c:1195: if (move_window(fs, fs->fatbase + (bc / SS(fs))) != FR_OK) break;
	push	hl
	ld	e, -2 (ix)
	ld	d, -1 (ix)
	ld	hl, #10
	add	hl, sp
	ex	de, hl
	ld	bc, #0x0004
	ldir
	pop	de
	ld	a, d
	srl	a
	ld	c, a
	ld	b, #0x00
	ld	hl, #0x0000
	ld	a, c
	add	a, -6 (ix)
	ld	c, a
	ld	a, b
	adc	a, -5 (ix)
	ld	b, a
	ld	a, l
	adc	a, -4 (ix)
	ld	l, a
;	spillPairReg hl
;	spillPairReg hl
	ld	a, h
	adc	a, -3 (ix)
	push	de
	ld	h, a
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	push	bc
	ld	l, -14 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -13 (ix)
;	spillPairReg hl
;	spillPairReg hl
	call	_move_window
	pop	de
	or	a, a
	jp	NZ, 00116$
;ff.c:1196: wc = fs->win[bc++ % SS(fs)];		/* Get 1st byte of the entry */
	ld	c, e
	ld	a, d
	inc	de
	and	a, #0x01
	ld	b, a
	ld	l, -8 (ix)
	ld	h, -7 (ix)
	add	hl, bc
	ld	a, (hl)
	ld	-6 (ix), a
	ld	-5 (ix), #0x00
;ff.c:1197: if (move_window(fs, fs->fatbase + (bc / SS(fs))) != FR_OK) break;
	push	de
	ld	e, -2 (ix)
	ld	d, -1 (ix)
	ld	hl, #12
	add	hl, sp
	ex	de, hl
	ld	bc, #0x0004
	ldir
	pop	de
	ld	a, d
	srl	a
	ld	c, a
	ld	b, #0x00
	ld	hl, #0x0000
	ld	a, c
	add	a, -4 (ix)
	ld	c, a
	ld	a, b
	adc	a, -3 (ix)
	ld	b, a
	ld	a, l
	adc	a, -2 (ix)
	ld	l, a
;	spillPairReg hl
;	spillPairReg hl
	ld	a, h
	adc	a, -1 (ix)
	push	de
	ld	h, a
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	push	bc
	ld	l, -14 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -13 (ix)
;	spillPairReg hl
;	spillPairReg hl
	call	_move_window
	pop	de
	or	a, a
	jp	NZ, 00116$
;ff.c:1198: wc |= fs->win[bc % SS(fs)] << 8;	/* Merge 2nd byte of the entry */
	ld	a, d
	and	a, #0x01
	ld	d, a
	ld	l, -8 (ix)
	ld	h, -7 (ix)
	add	hl, de
	ld	c, (hl)
	ld	-3 (ix), c
	ld	-4 (ix), #0x00
	ld	a, -6 (ix)
	or	a, -4 (ix)
	ld	-2 (ix), a
	ld	a, -5 (ix)
	or	a, -3 (ix)
	ld	-1 (ix), a
	ld	a, -2 (ix)
	ld	-4 (ix), a
	ld	a, -1 (ix)
	ld	-3 (ix), a
;ff.c:1199: val = (clst & 1) ? (wc >> 4) : (wc & 0xFFF);	/* Adjust bit position */
	bit	0, 4 (ix)
	jr	Z, 00120$
	ld	a, -4 (ix)
	ld	-2 (ix), a
	ld	a, -3 (ix)
	ld	-1 (ix), a
	srl	-1 (ix)
	rr	-2 (ix)
	srl	-1 (ix)
	rr	-2 (ix)
	srl	-1 (ix)
	rr	-2 (ix)
	srl	-1 (ix)
	rr	-2 (ix)
	jr	00121$
00120$:
	ld	a, -4 (ix)
	ld	-2 (ix), a
	ld	a, -3 (ix)
	and	a, #0x0f
	ld	-1 (ix), a
00121$:
	ld	a, -2 (ix)
	ld	-12 (ix), a
	ld	a, -1 (ix)
	ld	-11 (ix), a
	xor	a, a
	ld	-10 (ix), a
	ld	-9 (ix), a
;ff.c:1200: break;
	jp	00116$
;ff.c:1202: case FS_FAT16 :
00106$:
;ff.c:1203: if (move_window(fs, fs->fatbase + (clst / (SS(fs) / 2))) != FR_OK) break;
	ld	e, -2 (ix)
	ld	d, -1 (ix)
	ld	hl, #10
	add	hl, sp
	ex	de, hl
	ld	bc, #0x0004
	ldir
	ld	c, 5 (ix)
	ld	b, 6 (ix)
	ld	e, 7 (ix)
	ld	d, #0x00
	ld	a, -4 (ix)
	add	a, c
	ld	c, a
	ld	a, -3 (ix)
	adc	a, b
	ld	b, a
	ld	a, -2 (ix)
	adc	a, e
	ld	e, a
	ld	a, -1 (ix)
	adc	a, d
	ld	d, a
	push	de
	push	bc
	ld	l, -14 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -13 (ix)
;	spillPairReg hl
;	spillPairReg hl
	call	_move_window
	or	a, a
	jp	NZ, 00116$
;ff.c:1204: val = ld_16(fs->win + clst * 2 % SS(fs));		/* Simple WORD array */
	ld	c, 4 (ix)
	ld	a, 5 (ix)
;	spillPairReg hl
;	spillPairReg hl
;	spillPairReg hl
;	spillPairReg hl
	sla	c
	adc	a, a
	and	a, #0x01
	ld	b, a
	ld	l, -8 (ix)
	ld	h, -7 (ix)
	add	hl, bc
	call	_ld_16
	ld	-12 (ix), e
	ld	-11 (ix), d
	xor	a, a
	ld	-10 (ix), a
	ld	-9 (ix), a
;ff.c:1205: break;
	jp	00116$
;ff.c:1207: case FS_FAT32 :
00109$:
;ff.c:1208: if (move_window(fs, fs->fatbase + (clst / (SS(fs) / 4))) != FR_OK) break;
	ld	e, -2 (ix)
	ld	d, -1 (ix)
	ld	hl, #10
	add	hl, sp
	ex	de, hl
	ld	bc, #0x0004
	ldir
	ld	e, 4 (ix)
	ld	d, 5 (ix)
	ld	l, 6 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, 7 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	b, #0x07
00175$:
	srl	h
	rr	l
	rr	d
	rr	e
	djnz	00175$
	ld	a, -4 (ix)
	add	a, e
	ld	c, a
	ld	a, -3 (ix)
	adc	a, d
	ld	b, a
	ld	a, -2 (ix)
	adc	a, l
	ld	e, a
	ld	a, -1 (ix)
	adc	a, h
	ld	d, a
	push	de
	push	bc
	ld	l, -14 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -13 (ix)
;	spillPairReg hl
;	spillPairReg hl
	call	_move_window
	or	a, a
	jr	NZ, 00116$
;ff.c:1209: val = ld_32(fs->win + clst * 4 % SS(fs)) & 0x0FFFFFFF;	/* Simple DWORD array but mask out upper 4 bits */
	ld	c, 4 (ix)
	ld	a, 5 (ix)
	ld	l, 6 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, 7 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	b, #0x02
00177$:
	sla	c
	adc	a, a
	adc	hl, hl
	djnz	00177$
	and	a, #0x01
	ld	b, a
	ld	l, -8 (ix)
	ld	h, -7 (ix)
	add	hl, bc
	call	_ld_32
	ld	-12 (ix), e
	ld	-11 (ix), d
	ld	-10 (ix), l
	ld	a, h
	and	a, #0x0f
	ld	-9 (ix), a
;ff.c:1210: break;
	jr	00116$
;ff.c:1238: default:
00112$:
;ff.c:1239: val = 1;	/* Internal error */
	ld	-12 (ix), #0x01
	xor	a, a
	ld	-11 (ix), a
	ld	-10 (ix), a
	ld	-9 (ix), a
;ff.c:1240: }
00116$:
;ff.c:1243: return val;
	pop	hl
	pop	de
	push	de
	push	hl
	ld	l, -10 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -9 (ix)
;	spillPairReg hl
;	spillPairReg hl
;ff.c:1244: }
	ld	sp, ix
	pop	ix
	ret
;ff.c:1254: static FRESULT put_fat (	/* FR_OK(0):succeeded, !=0:error */
;	---------------------------------
; Function put_fat
; ---------------------------------
_put_fat:
	push	ix
	ld	ix,#0
	add	ix,sp
	ld	iy, #-16
	add	iy, sp
	ld	sp, iy
	ld	-2 (ix), l
	ld	-1 (ix), h
;ff.c:1262: FRESULT res = FR_INT_ERR;
	ld	-7 (ix), #0x02
;ff.c:1265: if (clst >= 2 && clst < fs->n_fatent) {	/* Check if in valid range */
	ld	a, 4 (ix)
	sub	a, #0x02
	ld	a, 5 (ix)
	sbc	a, #0x00
	ld	a, 6 (ix)
	sbc	a, #0x00
	ld	a, 7 (ix)
	sbc	a, #0x00
	jp	C, 00117$
	ld	c, -2 (ix)
	ld	b, -1 (ix)
	ld	hl, #20
	add	hl, bc
	ld	c, (hl)
	inc	hl
	ld	b, (hl)
	inc	hl
	ld	e, (hl)
	inc	hl
	ld	d, (hl)
	ld	a, 4 (ix)
	sub	a, c
	ld	a, 5 (ix)
	sbc	a, b
	ld	a, 6 (ix)
	sbc	a, e
	ld	a, 7 (ix)
	sbc	a, d
	jp	NC, 00117$
;ff.c:1266: switch (fs->fs_type) {
	ld	l, -2 (ix)
	ld	h, -1 (ix)
	ld	e, (hl)
;ff.c:1269: res = move_window(fs, fs->fatbase + (bc / SS(fs)));
	ld	a, -2 (ix)
	add	a, #0x24
	ld	-16 (ix), a
	ld	a, -1 (ix)
	adc	a, #0x00
	ld	-15 (ix), a
;ff.c:1271: p = fs->win + bc++ % SS(fs);
	ld	a, -2 (ix)
	add	a, #0x30
	ld	-14 (ix), a
	ld	a, -1 (ix)
	adc	a, #0x00
	ld	-13 (ix), a
;ff.c:1273: fs->wflag = 1;
	ld	a, -2 (ix)
	add	a, #0x04
	ld	c, a
	ld	a, -1 (ix)
	adc	a, #0x00
	ld	b, a
;ff.c:1266: switch (fs->fs_type) {
	ld	a, e
	dec	a
	jr	Z, 00101$
	ld	a,e
	cp	a,#0x02
	jp	Z,00106$
	sub	a, #0x03
	jp	Z,00109$
	jp	00117$
;ff.c:1267: case FS_FAT12:
00101$:
;ff.c:1268: bc = (UINT)clst; bc += bc / 2;	/* bc: byte offset of the entry */
	ld	e, 4 (ix)
	ld	d, 5 (ix)
	ld	l, e
	ld	h, d
	srl	h
	rr	l
	add	hl, de
	ld	-4 (ix), l
	ld	-3 (ix), h
;ff.c:1269: res = move_window(fs, fs->fatbase + (bc / SS(fs)));
	ld	l, c
	ld	h, b
	pop	de
	push	de
	push	hl
	ld	hl, #10
	add	hl, sp
	ex	de, hl
	ld	bc, #0x0004
	ldir
	pop	bc
	ld	a, -3 (ix)
	srl	a
	ld	e, a
	ld	d, #0x00
	ld	hl, #0x0000
	ld	a, e
	add	a, -8 (ix)
	ld	e, a
	ld	a, d
	adc	a, -7 (ix)
	ld	d, a
	ld	a, l
	adc	a, -6 (ix)
	ld	l, a
;	spillPairReg hl
;	spillPairReg hl
	ld	a, h
	adc	a, -5 (ix)
	push	bc
	ld	h, a
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	push	de
	ld	l, -2 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -1 (ix)
;	spillPairReg hl
;	spillPairReg hl
	call	_move_window
	pop	bc
;ff.c:1270: if (res != FR_OK) break;
	ld	-7 (ix), a
	or	a, a
	jp	NZ, 00117$
;ff.c:1271: p = fs->win + bc++ % SS(fs);
	ld	e, -4 (ix)
	ld	d, -3 (ix)
	ld	a, -4 (ix)
	add	a, #0x01
	ld	-12 (ix), a
	ld	a, -3 (ix)
	adc	a, #0x00
	ld	-11 (ix), a
	ld	a, d
	and	a, #0x01
	ld	d, a
	ld	a, -14 (ix)
	add	a, e
	ld	-10 (ix), a
	ld	a, -13 (ix)
	adc	a, d
	ld	-9 (ix), a
;ff.c:1272: *p = (clst & 1) ? ((*p & 0x0F) | ((BYTE)val << 4)) : (BYTE)val;	/* Update 1st byte */
	ld	a, -10 (ix)
	ld	-8 (ix), a
	ld	a, -9 (ix)
	ld	-7 (ix), a
	ld	a, 4 (ix)
	and	a, #0x01
	ld	-6 (ix), a
	xor	a, a
	ld	-5 (ix), a
	ld	-4 (ix), a
	ld	-3 (ix), a
	ld	e, 8 (ix)
	ld	a, -3 (ix)
	or	a, -4 (ix)
	or	a, -5 (ix)
	or	a, -6 (ix)
	jr	Z, 00121$
	ld	l, -10 (ix)
	ld	h, -9 (ix)
	ld	a, (hl)
	and	a, #0x0f
	ld	l, a
;	spillPairReg hl
;	spillPairReg hl
;	spillPairReg hl
;	spillPairReg hl
	ld	a, e
	add	a, a
	add	a, a
	add	a, a
	add	a, a
	push	af
	rlca
	pop	af
	or	a, l
	ld	e, a
00121$:
	ld	l, -8 (ix)
	ld	h, -7 (ix)
	ld	(hl), e
;ff.c:1273: fs->wflag = 1;
	ld	a, #0x01
	ld	(bc), a
;ff.c:1274: res = move_window(fs, fs->fatbase + (bc / SS(fs)));
	ld	l, c
	ld	h, b
	pop	de
	push	de
	push	hl
	ld	hl, #8
	add	hl, sp
	ex	de, hl
	ld	bc, #0x0004
	ldir
	pop	bc
	ld	a, -11 (ix)
	srl	a
	ld	e, a
	ld	d, #0x00
	ld	hl, #0x0000
	ld	a, e
	add	a, -10 (ix)
	ld	e, a
	ld	a, d
	adc	a, -9 (ix)
	ld	d, a
	ld	a, l
	adc	a, -8 (ix)
	ld	l, a
;	spillPairReg hl
;	spillPairReg hl
	ld	a, h
	adc	a, -7 (ix)
	ld	h, a
;	spillPairReg hl
;	spillPairReg hl
	push	bc
	push	hl
	push	de
	ld	l, -2 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -1 (ix)
;	spillPairReg hl
;	spillPairReg hl
	call	_move_window
	pop	bc
;ff.c:1275: if (res != FR_OK) break;
	ld	-7 (ix), a
	or	a, a
	jp	NZ, 00117$
;ff.c:1276: p = fs->win + bc % SS(fs);
	ld	e, -12 (ix)
	ld	a, -11 (ix)
	and	a, #0x01
	ld	d, a
	ld	l, -14 (ix)
	ld	h, -13 (ix)
	add	hl, de
	ex	de, hl
;ff.c:1277: *p = (clst & 1) ? (BYTE)(val >> 4) : ((*p & 0xF0) | ((BYTE)(val >> 8) & 0x0F));	/* Update 2nd byte */
	ld	-9 (ix), e
	ld	-8 (ix), d
	ld	a, -3 (ix)
	or	a, -4 (ix)
	or	a, -5 (ix)
	or	a, -6 (ix)
	jr	Z, 00123$
	ld	e, 8 (ix)
	ld	d, 9 (ix)
	srl	d
	rr	e
	srl	d
	rr	e
	srl	d
	rr	e
	srl	d
	rr	e
	ld	d, #0x00
	jr	00124$
00123$:
	ld	a, (de)
	and	a, #0xf0
	ld	e, a
	ld	d, #0x00
;	spillPairReg hl
;	spillPairReg hl
;	spillPairReg hl
;	spillPairReg hl
	ld	a, 9 (ix)
	and	a, #0x0f
	ld	l, a
;	spillPairReg hl
;	spillPairReg hl
	ld	h, #0x00
;	spillPairReg hl
;	spillPairReg hl
	ld	a, e
	or	a, l
	ld	e, a
	ld	a, d
	or	a, h
	ld	d, a
00124$:
	ld	l, -9 (ix)
	ld	h, -8 (ix)
	ld	(hl), e
;ff.c:1278: fs->wflag = 1;
	ld	a, #0x01
	ld	(bc), a
;ff.c:1279: break;
	jp	00117$
;ff.c:1281: case FS_FAT16:
00106$:
;ff.c:1282: res = move_window(fs, fs->fatbase + (clst / (SS(fs) / 2)));
	ld	l, c
	ld	h, b
	pop	de
	push	de
	push	hl
	ld	hl, #12
	add	hl, sp
	ex	de, hl
	ld	bc, #0x0004
	ldir
	pop	bc
	ld	e, 5 (ix)
	ld	d, 6 (ix)
	ld	l, 7 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, #0x00
;	spillPairReg hl
;	spillPairReg hl
	ld	a, e
	add	a, -6 (ix)
	ld	e, a
	ld	a, d
	adc	a, -5 (ix)
	ld	d, a
	ld	a, l
	adc	a, -4 (ix)
	ld	l, a
;	spillPairReg hl
;	spillPairReg hl
	ld	a, h
	adc	a, -3 (ix)
	ld	h, a
;	spillPairReg hl
;	spillPairReg hl
	push	bc
	push	hl
	push	de
	ld	l, -2 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -1 (ix)
;	spillPairReg hl
;	spillPairReg hl
	call	_move_window
	pop	bc
;ff.c:1283: if (res != FR_OK) break;
	ld	-7 (ix), a
	or	a, a
	jp	NZ, 00117$
;ff.c:1284: st_16(fs->win + clst * 2 % SS(fs), (WORD)val);	/* Simple WORD array */
	ld	a, 8 (ix)
	ld	-4 (ix), a
	ld	a, 9 (ix)
	ld	-3 (ix), a
	ld	l, 4 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, 5 (ix)
;	spillPairReg hl
;	spillPairReg hl
	add	hl, hl
	ld	a, h
	and	a, #0x01
	ld	h, a
;	spillPairReg hl
;	spillPairReg hl
	ld	a, l
	add	a, -14 (ix)
	ld	l, a
;	spillPairReg hl
;	spillPairReg hl
	ld	a, h
	adc	a, -13 (ix)
	ld	h, a
;	spillPairReg hl
;	spillPairReg hl
	push	bc
	ld	e, -4 (ix)
	ld	d, -3 (ix)
	call	_st_16
	pop	bc
;ff.c:1285: fs->wflag = 1;
	ld	a, #0x01
	ld	(bc), a
;ff.c:1286: break;
	jp	00117$
;ff.c:1288: case FS_FAT32:
00109$:
;ff.c:1292: res = move_window(fs, fs->fatbase + (clst / (SS(fs) / 4)));
	ld	l, c
	ld	h, b
	pop	de
	push	de
	push	hl
	ld	hl, #12
	add	hl, sp
	ex	de, hl
	ld	bc, #0x0004
	ldir
	pop	bc
	ld	e, 4 (ix)
	ld	d, 5 (ix)
	ld	l, 6 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, 7 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	a, #0x07
00187$:
	srl	h
	rr	l
	rr	d
	rr	e
	dec	a
	jr	NZ, 00187$
	ld	a, e
	add	a, -6 (ix)
	ld	e, a
	ld	a, d
	adc	a, -5 (ix)
	ld	d, a
	ld	a, l
	adc	a, -4 (ix)
	ld	l, a
;	spillPairReg hl
;	spillPairReg hl
	ld	a, h
	adc	a, -3 (ix)
	ld	h, a
;	spillPairReg hl
;	spillPairReg hl
	push	bc
	push	hl
	push	de
	ld	l, -2 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -1 (ix)
;	spillPairReg hl
;	spillPairReg hl
	call	_move_window
	pop	bc
;ff.c:1293: if (res != FR_OK) break;
	ld	-7 (ix), a
	or	a, a
	jp	NZ, 00117$
;ff.c:1295: val = (val & 0x0FFFFFFF) | (ld_32(fs->win + clst * 4 % SS(fs)) & 0xF0000000);
	ld	a, 8 (ix)
	ld	-6 (ix), a
	ld	a, 9 (ix)
	ld	-5 (ix), a
	ld	a, 10 (ix)
	ld	-4 (ix), a
	ld	a, 11 (ix)
	and	a, #0x0f
	ld	-3 (ix), a
	ld	l, 4 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, 5 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	e, 6 (ix)
	ld	d, 7 (ix)
	ld	a, #0x02
00189$:
	add	hl, hl
	rl	e
	rl	d
	dec	a
	jr	NZ,00189$
	ld	a, h
	and	a, #0x01
	ld	h, a
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	pop	iy
	ld	e, -14 (ix)
	ld	d, -13 (ix)
	add	iy, de
	push	bc
	push	iy
	push	iy
	pop	hl
	call	_ld_32
	pop	iy
	pop	bc
	ld	de, #0x0000
	ld	l, #0x00
;	spillPairReg hl
;	spillPairReg hl
	ld	a, h
	and	a, #0xf0
	ld	h, a
;	spillPairReg hl
;	spillPairReg hl
	ld	a, e
	or	a, -6 (ix)
	ld	e, a
	ld	a, d
	or	a, -5 (ix)
	ld	d, a
	ld	a, l
	or	a, -4 (ix)
	ld	l, a
;	spillPairReg hl
;	spillPairReg hl
	ld	a, h
	or	a, -3 (ix)
	ld	h, a
;	spillPairReg hl
;	spillPairReg hl
	ld	8 (ix), e
	ld	9 (ix), d
	ld	10 (ix), l
	ld	11 (ix), h
;ff.c:1297: st_32(fs->win + clst * 4 % SS(fs), val);
	push	bc
	ld	l, 10 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, 11 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	ld	l, 8 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, 9 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	push	iy
	pop	hl
	call	_st_32
	pop	bc
;ff.c:1298: fs->wflag = 1;
	ld	a, #0x01
	ld	(bc), a
;ff.c:1300: }
00117$:
;ff.c:1302: return res;
	ld	a, -7 (ix)
;ff.c:1303: }
	ld	sp, ix
	pop	ix
	pop	hl
	pop	bc
	pop	bc
	pop	bc
	pop	bc
	jp	(hl)
;ff.c:1444: static FRESULT remove_chain (	/* FR_OK(0):succeeded, !=0:error */
;	---------------------------------
; Function remove_chain
; ---------------------------------
_remove_chain:
	push	ix
	ld	ix,#0
	add	ix,sp
	ld	iy, #-16
	add	iy, sp
	ld	sp, iy
;ff.c:1452: FATFS *fs = obj->fs;
	ld	-2 (ix), l
	ld	-1 (ix), h
	ld	a, (hl)
	ld	-16 (ix), a
	inc	hl
	ld	a, (hl)
	ld	-15 (ix), a
;ff.c:1460: if (clst < 2 || clst >= fs->n_fatent) return FR_INT_ERR;	/* Check if in valid range */
	ld	a, 4 (ix)
	sub	a, #0x02
	ld	a, 5 (ix)
	sbc	a, #0x00
	ld	a, 6 (ix)
	sbc	a, #0x00
	ld	a, 7 (ix)
	sbc	a, #0x00
	jr	C, 00101$
	pop	hl
	push	hl
	ld	de, #0x0014
	add	hl, de
	push	hl
	ld	c, (hl)
	inc	hl
	ld	b, (hl)
	inc	hl
	ld	e, (hl)
	inc	hl
	ld	d, (hl)
	pop	hl
	ld	a, 4 (ix)
	sub	a, c
	ld	a, 5 (ix)
	sbc	a, b
	ld	a, 6 (ix)
	sbc	a, e
	ld	a, 7 (ix)
	sbc	a, d
	jr	C, 00102$
00101$:
	ld	a, #0x02
	jp	00127$
00102$:
;ff.c:1463: if (pclst != 0 && (!FF_FS_EXFAT || fs->fs_type != FS_EXFAT || obj->stat != 2)) {
	ld	a, 11 (ix)
	or	a, 10 (ix)
	or	a, 9 (ix)
	or	a, 8 (ix)
	jr	Z, 00137$
;ff.c:1464: res = put_fat(fs, pclst, 0xFFFFFFFF);
	push	hl
	ld	de, #0xffff
	push	de
	push	de
	ld	e, 10 (ix)
	ld	d, 11 (ix)
	push	de
	ld	e, 8 (ix)
	ld	d, 9 (ix)
	push	de
	ld	l, -16 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -15 (ix)
;	spillPairReg hl
;	spillPairReg hl
	call	_put_fat
	pop	hl
;ff.c:1465: if (res != FR_OK) return res;
	or	a, a
;ff.c:1469: do {
	jp	NZ,00127$
00137$:
	ld	-14 (ix), l
	ld	-13 (ix), h
00124$:
;ff.c:1470: nxt = get_fat(obj, clst);			/* Get cluster status */
	ld	l, 6 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, 7 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	ld	l, 4 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, 5 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	ld	l, -2 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -1 (ix)
;	spillPairReg hl
;	spillPairReg hl
	call	_get_fat
	pop	af
	pop	af
	ld	-12 (ix), e
	ld	-11 (ix), d
	ld	-10 (ix), l
;ff.c:1471: if (nxt == 0) break;				/* Empty cluster? */
	ld	-9 (ix), h
	ld	a, h
	or	a, -10 (ix)
	or	a, -11 (ix)
	or	a, -12 (ix)
	jp	Z, 00126$
;ff.c:1472: if (nxt == 1) return FR_INT_ERR;	/* Internal error? */
	ld	a, -12 (ix)
	dec	a
	or	a, -11 (ix)
	or	a, -10 (ix)
	or	a, -9 (ix)
	jr	NZ, 00114$
	ld	a, #0x02
	jp	00127$
00114$:
;ff.c:1473: if (nxt == 0xFFFFFFFF) return FR_DISK_ERR;	/* Disk error? */
	ld	a, -12 (ix)
	and	a, -11 (ix)
	and	a, -10 (ix)
	and	a, -9 (ix)
	inc	a
	jr	NZ, 00119$
	ld	a, #0x01
	jp	00127$
;ff.c:1474: if (!FF_FS_EXFAT || fs->fs_type != FS_EXFAT) {
00119$:
;ff.c:1475: res = put_fat(fs, clst, 0);		/* Mark the cluster 'free' on the FAT */
	ld	hl, #0x0000
	push	hl
	push	hl
	ld	l, 6 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, 7 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	ld	l, 4 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, 5 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	ld	l, -16 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -15 (ix)
;	spillPairReg hl
;	spillPairReg hl
	call	_put_fat
;ff.c:1476: if (res != FR_OK) return res;
	or	a, a
	jp	NZ,00127$
;ff.c:1478: if (fs->free_clst < fs->n_fatent - 2) {	/* Update allocation information if it is valid */
	ld	a, -16 (ix)
	add	a, #0x10
	ld	-8 (ix), a
	ld	a, -15 (ix)
	adc	a, #0x00
	ld	-7 (ix), a
	ld	e, -8 (ix)
	ld	d, -7 (ix)
	ld	hl, #10
	add	hl, sp
	ex	de, hl
	ld	bc, #0x0004
	ldir
	ld	l, -14 (ix)
	ld	h, -13 (ix)
	ld	c, (hl)
	inc	hl
	ld	b, (hl)
	inc	hl
	ld	e, (hl)
	inc	hl
	ld	d, (hl)
	ld	a, c
	add	a, #0xfe
	ld	c, a
	ld	a, b
	adc	a, #0xff
	ld	b, a
	ld	a, e
	adc	a, #0xff
	ld	e, a
	ld	a, d
	adc	a, #0xff
	ld	d, a
	ld	a, -6 (ix)
	sub	a, c
	ld	a, -5 (ix)
	sbc	a, b
	ld	a, -4 (ix)
	sbc	a, e
	ld	a, -3 (ix)
	sbc	a, d
	jr	NC, 00123$
;ff.c:1479: fs->free_clst++;
	ld	a, -6 (ix)
	add	a, #0x01
	ld	c, a
	ld	a, -5 (ix)
	adc	a, #0x00
	ld	b, a
	ld	a, -4 (ix)
	adc	a, #0x00
	ld	e, a
	ld	a, -3 (ix)
	adc	a, #0x00
	ld	d, a
	ld	l, -8 (ix)
	ld	h, -7 (ix)
	ld	(hl), c
	inc	hl
	ld	(hl), b
	inc	hl
	ld	(hl), e
	inc	hl
	ld	(hl), d
;ff.c:1480: fs->fsi_flag |= 1;
	pop	hl
	push	hl
	ld	de, #0x0005
	add	hl, de
	set	0, (hl)
00123$:
;ff.c:1500: clst = nxt;					/* Next cluster */
	ld	hl, #20
	add	hl, sp
	ex	de, hl
	ld	hl, #4
	add	hl, sp
	ld	bc, #4
	ldir
;ff.c:1501: } while (clst < fs->n_fatent);	/* Repeat until the last link */
	pop	hl
	pop	de
	push	de
	push	hl
	ld	hl, #10
	add	hl, sp
	ex	de, hl
	ld	bc, #0x0004
	ldir
	ld	a, -12 (ix)
	sub	a, -6 (ix)
	ld	a, -11 (ix)
	sbc	a, -5 (ix)
	ld	a, -10 (ix)
	sbc	a, -4 (ix)
	ld	a, -9 (ix)
	sbc	a, -3 (ix)
	jp	C, 00124$
00126$:
;ff.c:1529: return FR_OK;
	xor	a, a
00127$:
;ff.c:1530: }
	ld	sp, ix
	pop	ix
	pop	hl
	pop	bc
	pop	bc
	pop	bc
	pop	bc
	jp	(hl)
;ff.c:1539: static DWORD create_chain (	/* 0:No free cluster, 1:Internal error, 0xFFFFFFFF:Disk error, >=2:New cluster# */
;	---------------------------------
; Function create_chain
; ---------------------------------
_create_chain:
	push	ix
	ld	ix,#0
	add	ix,sp
	ld	iy, #-19
	add	iy, sp
	ld	sp, iy
;ff.c:1546: FATFS *fs = obj->fs;
	ld	-6 (ix), l
	ld	-5 (ix), h
	ld	a, (hl)
	ld	-19 (ix), a
	inc	hl
	ld	a, (hl)
	ld	-18 (ix), a
;ff.c:1550: scl = fs->last_clst;				/* Suggested cluster to start to find */
	ld	a, -19 (ix)
	add	a, #0x0c
	ld	-17 (ix), a
	ld	a, -18 (ix)
	adc	a, #0x00
	ld	-16 (ix), a
;ff.c:1551: if (scl == 0 || scl >= fs->n_fatent) scl = 1;
	ld	a, -19 (ix)
	add	a, #0x14
	ld	-15 (ix), a
	ld	a, -18 (ix)
	adc	a, #0x00
	ld	-14 (ix), a
;ff.c:1549: if (clst == 0) {	/* Create a new chain */
	ld	a, 7 (ix)
	or	a, 6 (ix)
	or	a, 5 (ix)
	or	a, 4 (ix)
	jr	NZ, 00111$
;ff.c:1550: scl = fs->last_clst;				/* Suggested cluster to start to find */
	pop	hl
	pop	de
	push	de
	push	hl
	ld	hl, #6
	add	hl, sp
	ex	de, hl
	ld	bc, #0x0004
	ldir
;ff.c:1551: if (scl == 0 || scl >= fs->n_fatent) scl = 1;
	ld	a, -10 (ix)
	or	a, -11 (ix)
	or	a, -12 (ix)
	or	a, -13 (ix)
	jr	Z, 00101$
	ld	e, -15 (ix)
	ld	d, -14 (ix)
	ld	hl, #15
	add	hl, sp
	ex	de, hl
	ld	bc, #0x0004
	ldir
	ld	a, -13 (ix)
	sub	a, -4 (ix)
	ld	a, -12 (ix)
	sbc	a, -3 (ix)
	ld	a, -11 (ix)
	sbc	a, -2 (ix)
	ld	a, -10 (ix)
	sbc	a, -1 (ix)
	jp	C, 00112$
00101$:
	ld	-13 (ix), #0x01
	xor	a, a
	ld	-12 (ix), a
	ld	-11 (ix), a
	ld	-10 (ix), a
	jp	00112$
00111$:
;ff.c:1554: cs = get_fat(obj, clst);			/* Check the cluster status */
	ld	l, 6 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, 7 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	ld	l, 4 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, 5 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	ld	l, -6 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -5 (ix)
;	spillPairReg hl
;	spillPairReg hl
	call	_get_fat
	pop	af
	pop	af
	ld	-4 (ix), e
	ld	-3 (ix), d
	ld	-2 (ix), l
	ld	-1 (ix), h
;ff.c:1555: if (cs < 2) return 1;				/* Test for insanity */
	ld	a, -4 (ix)
	sub	a, #0x02
	ld	a, -3 (ix)
	sbc	a, #0x00
	ld	a, -2 (ix)
	sbc	a, #0x00
	ld	a, -1 (ix)
	sbc	a, #0x00
	jr	NC, 00105$
	ld	de, #0x0001
	ld	l, d
	ld	h, d
	jp	00152$
00105$:
;ff.c:1556: if (cs == 0xFFFFFFFF) return cs;	/* Test for disk error */
	ld	a, -4 (ix)
	and	a, -3 (ix)
	and	a, -2 (ix)
	and	a, -1 (ix)
	inc	a
	jr	NZ, 00107$
	ld	e, -4 (ix)
	ld	d, -3 (ix)
	ld	l, -2 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -1 (ix)
;	spillPairReg hl
;	spillPairReg hl
	jp	00152$
00107$:
;ff.c:1557: if (cs < fs->n_fatent) return cs;	/* It is already followed by next cluster */
	ld	l, -15 (ix)
	ld	h, -14 (ix)
	ld	c, (hl)
	inc	hl
	ld	b, (hl)
	inc	hl
	ld	e, (hl)
	inc	hl
	ld	d, (hl)
	ld	a, -4 (ix)
	sub	a, c
	ld	a, -3 (ix)
	sbc	a, b
	ld	a, -2 (ix)
	sbc	a, e
	ld	a, -1 (ix)
	sbc	a, d
	jr	NC, 00109$
	ld	e, -4 (ix)
	ld	d, -3 (ix)
	ld	l, -2 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -1 (ix)
;	spillPairReg hl
;	spillPairReg hl
	jp	00152$
00109$:
;ff.c:1558: scl = clst;							/* Cluster to start to find */
	ld	hl, #6
	add	hl, sp
	ex	de, hl
	ld	hl, #23
	add	hl, sp
	ld	bc, #4
	ldir
00112$:
;ff.c:1560: if (fs->free_clst == 0) return 0;		/* No free cluster */
	ld	a, -19 (ix)
	add	a, #0x10
	ld	-9 (ix), a
	ld	a, -18 (ix)
	adc	a, #0x00
	ld	-8 (ix), a
	ld	l, -9 (ix)
	ld	h, -8 (ix)
	ld	c, (hl)
	inc	hl
	ld	b, (hl)
	inc	hl
	ld	e, (hl)
	inc	hl
	ld	d, (hl)
	ld	a, d
	or	a, e
	or	a, b
	or	a, c
	jr	NZ, 00114$
	ld	de, #0x0000
	ld	l, e
	ld	h, e
	jp	00152$
00114$:
;ff.c:1589: ncl = 0;
	xor	a, a
	ld	-4 (ix), a
	ld	-3 (ix), a
	ld	-2 (ix), a
	ld	-1 (ix), a
;ff.c:1590: if (scl == clst) {						/* Stretching an existing chain? */
	ld	a, -13 (ix)
	sub	a, 4 (ix)
	jp	NZ,00126$
	ld	a, -12 (ix)
	sub	a, 5 (ix)
	jp	NZ,00126$
	ld	a, -11 (ix)
	sub	a, 6 (ix)
	jp	NZ,00126$
	ld	a, -10 (ix)
	sub	a, 7 (ix)
	jp	NZ,00126$
;ff.c:1591: ncl = scl + 1;						/* Test if next cluster is free */
	ld	a, -13 (ix)
	add	a, #0x01
	ld	-4 (ix), a
	ld	a, -12 (ix)
	adc	a, #0x00
	ld	-3 (ix), a
	ld	a, -11 (ix)
	adc	a, #0x00
	ld	-2 (ix), a
	ld	a, -10 (ix)
	adc	a, #0x00
	ld	-1 (ix), a
;ff.c:1592: if (ncl >= fs->n_fatent) ncl = 2;
	ld	l, -15 (ix)
	ld	h, -14 (ix)
	ld	c, (hl)
	inc	hl
	ld	b, (hl)
	inc	hl
	ld	e, (hl)
	inc	hl
	ld	d, (hl)
	ld	a, -4 (ix)
	sub	a, c
	ld	a, -3 (ix)
	sbc	a, b
	ld	a, -2 (ix)
	sbc	a, e
	ld	a, -1 (ix)
	sbc	a, d
	jr	C, 00116$
	ld	-4 (ix), #0x02
	xor	a, a
	ld	-3 (ix), a
	ld	-2 (ix), a
	ld	-1 (ix), a
00116$:
;ff.c:1593: cs = get_fat(obj, ncl);				/* Get next cluster status */
	ld	l, -2 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -1 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	ld	l, -4 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -3 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	ld	l, -6 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -5 (ix)
;	spillPairReg hl
;	spillPairReg hl
	call	_get_fat
	pop	af
	pop	af
;ff.c:1594: if (cs == 1 || cs == 0xFFFFFFFF) return cs;	/* Test for error */
	ld	a, e
	dec	a
	or	a, d
	or	a, l
	or	a, h
	jp	Z,00152$
	ld	a, e
	and	a, d
	and	a, l
	and	a, h
	inc	a
	jp	Z,00152$
;ff.c:1595: if (cs != 0) {						/* Not free? */
	ld	a, h
	or	a, l
	or	a, d
	or	a, e
	jr	Z, 00126$
;ff.c:1596: cs = fs->last_clst;				/* Start at suggested cluster if it is valid */
	pop	hl
	pop	de
	push	de
	push	hl
	ld	hl, #15
	add	hl, sp
	ex	de, hl
	ld	bc, #0x0004
	ldir
;ff.c:1597: if (cs >= 2 && cs < fs->n_fatent) scl = cs;
	ld	a, -4 (ix)
	sub	a, #0x02
	ld	a, -3 (ix)
	sbc	a, #0x00
	ld	a, -2 (ix)
	sbc	a, #0x00
	ld	a, -1 (ix)
	sbc	a, #0x00
	jr	C, 00121$
	ld	l, -15 (ix)
	ld	h, -14 (ix)
	ld	c, (hl)
	inc	hl
	ld	b, (hl)
	inc	hl
	ld	e, (hl)
	inc	hl
	ld	d, (hl)
	ld	a, -4 (ix)
	sub	a, c
	ld	a, -3 (ix)
	sbc	a, b
	ld	a, -2 (ix)
	sbc	a, e
	ld	a, -1 (ix)
	sbc	a, d
	jr	NC, 00121$
	ld	hl, #6
	add	hl, sp
	ex	de, hl
	ld	hl, #15
	add	hl, sp
	ld	bc, #4
	ldir
00121$:
;ff.c:1598: ncl = 0;
	xor	a, a
	ld	-4 (ix), a
	ld	-3 (ix), a
	ld	-2 (ix), a
	ld	-1 (ix), a
00126$:
;ff.c:1601: if (ncl == 0) {	/* The new cluster cannot be contiguous and find another fragment */
	ld	a, -1 (ix)
	or	a, -2 (ix)
	or	a, -3 (ix)
	or	a, -4 (ix)
	jp	NZ, 00140$
;ff.c:1602: ncl = scl;	/* Start cluster */
	ld	hl, #15
	add	hl, sp
	ex	de, hl
	ld	hl, #6
	add	hl, sp
	ld	bc, #4
	ldir
	ld	a, -13 (ix)
	sub	a, #0x02
	ld	a, -12 (ix)
	sbc	a, #0x00
	ld	a, -11 (ix)
	sbc	a, #0x00
	ld	a, -10 (ix)
	sbc	a, #0x00
	ld	a, #0x00
	rla
	ld	-7 (ix), a
	ld	c, -15 (ix)
	ld	b, -14 (ix)
00150$:
;ff.c:1604: ncl++;							/* Next cluster */
	inc	-4 (ix)
	jr	NZ, 00288$
	inc	-3 (ix)
	jr	NZ, 00288$
	inc	-2 (ix)
	jr	NZ, 00288$
	inc	-1 (ix)
00288$:
;ff.c:1605: if (ncl >= fs->n_fatent) {		/* Check wrap-around */
	ld	l, c
	ld	h, b
	ld	e, (hl)
	inc	hl
	ld	d, (hl)
	inc	hl
	inc	hl
	ld	a, (hl)
	dec	hl
	ld	l, (hl)
;	spillPairReg hl
	ld	h, a
;	spillPairReg hl
;	spillPairReg hl
	ld	a, -4 (ix)
	sub	a, e
	ld	a, -3 (ix)
	sbc	a, d
	ld	a, -2 (ix)
	sbc	a, l
	ld	a, -1 (ix)
	sbc	a, h
	jr	C, 00130$
;ff.c:1606: ncl = 2;
	ld	-4 (ix), #0x02
	xor	a, a
	ld	-3 (ix), a
	ld	-2 (ix), a
	ld	-1 (ix), a
;ff.c:1607: if (ncl > scl) return 0;	/* No free cluster found? */
	ld	a, -7 (ix)
	or	a, a
	jr	Z, 00130$
	ld	de, #0x0000
	ld	l, e
	ld	h, e
	jp	00152$
00130$:
;ff.c:1609: cs = get_fat(obj, ncl);			/* Get the cluster status */
	push	bc
	ld	l, -2 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -1 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	ld	l, -4 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -3 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	ld	l, -6 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -5 (ix)
;	spillPairReg hl
;	spillPairReg hl
	call	_get_fat
	pop	af
	pop	af
	pop	bc
;ff.c:1610: if (cs == 0) break;				/* Found a free cluster? */
	ld	a, h
	or	a, l
	or	a, d
	or	a, e
	jr	Z, 00140$
;ff.c:1611: if (cs == 1 || cs == 0xFFFFFFFF) return cs;	/* Test for error */
	ld	a, e
	dec	a
	or	a, d
	or	a, l
	or	a, h
	jp	Z,00152$
	ld	a, e
	and	a, d
	and	a, l
	and	a, h
	inc	a
	jp	Z,00152$
;ff.c:1612: if (ncl == scl) return 0;		/* No free cluster found? */
	ld	a, -4 (ix)
	sub	a, -13 (ix)
	jp	NZ,00150$
	ld	a, -3 (ix)
	sub	a, -12 (ix)
	jp	NZ,00150$
	ld	a, -2 (ix)
	sub	a, -11 (ix)
	jp	NZ,00150$
	ld	a, -1 (ix)
	sub	a, -10 (ix)
	jp	NZ,00150$
	ld	de, #0x0000
	ld	l, e
	ld	h, e
	jp	00152$
00140$:
;ff.c:1615: res = put_fat(fs, ncl, 0xFFFFFFFF);		/* Mark the new cluster 'EOC' */
	ld	hl, #0xffff
	push	hl
	push	hl
	ld	l, -2 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -1 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	ld	l, -4 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -3 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	ld	l, -19 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -18 (ix)
;	spillPairReg hl
;	spillPairReg hl
	call	_put_fat
;ff.c:1616: if (res == FR_OK && clst != 0) {
	ld	c, a
	or	a, a
	jr	NZ, 00142$
	ld	a, 7 (ix)
	or	a, 6 (ix)
	or	a, 5 (ix)
	or	a, 4 (ix)
	jr	Z, 00142$
;ff.c:1617: res = put_fat(fs, clst, ncl);		/* Link it from the previous one if needed */
	ld	l, -2 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -1 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	ld	l, -4 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -3 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	ld	l, 6 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, 7 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	ld	l, 4 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, 5 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	ld	l, -19 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -18 (ix)
;	spillPairReg hl
;	spillPairReg hl
	call	_put_fat
	ld	c, a
00142$:
;ff.c:1621: if (res == FR_OK) {			/* Update allocation information if the function succeeded */
	ld	a, c
	or	a, a
	jp	NZ, 00148$
;ff.c:1622: fs->last_clst = ncl;
	ld	e, -17 (ix)
	ld	d, -16 (ix)
	ld	hl, #15
	add	hl, sp
	ld	bc, #0x0004
	ldir
;ff.c:1623: if (fs->free_clst > 0 && fs->free_clst <= fs->n_fatent - 2) {
	ld	e, -9 (ix)
	ld	d, -8 (ix)
	ld	hl, #6
	add	hl, sp
	ex	de, hl
	ld	bc, #0x0004
	ldir
	ld	a, -10 (ix)
	or	a, -11 (ix)
	or	a, -12 (ix)
	or	a, -13 (ix)
	jr	Z, 00149$
	ld	l, -15 (ix)
	ld	h, -14 (ix)
	ld	c, (hl)
	inc	hl
	ld	b, (hl)
	inc	hl
	ld	e, (hl)
	inc	hl
	ld	d, (hl)
	ld	a, c
	add	a, #0xfe
	ld	c, a
	ld	a, b
	adc	a, #0xff
	ld	b, a
	ld	a, e
	adc	a, #0xff
	ld	e, a
	ld	a, d
	adc	a, #0xff
	ld	d, a
	ld	a, c
	sub	a, -13 (ix)
	ld	a, b
	sbc	a, -12 (ix)
	ld	a, e
	sbc	a, -11 (ix)
	ld	a, d
	sbc	a, -10 (ix)
	jr	C, 00149$
;ff.c:1624: fs->free_clst--;
	ld	a, -13 (ix)
	add	a, #0xff
	ld	c, a
	ld	a, -12 (ix)
	adc	a, #0xff
	ld	b, a
	ld	a, -11 (ix)
	adc	a, #0xff
	ld	e, a
	ld	a, -10 (ix)
	adc	a, #0xff
	ld	d, a
	ld	l, -9 (ix)
	ld	h, -8 (ix)
	ld	(hl), c
	inc	hl
	ld	(hl), b
	inc	hl
	ld	(hl), e
	inc	hl
	ld	(hl), d
;ff.c:1625: fs->fsi_flag |= 1;
	pop	hl
	push	hl
	ld	de, #0x0005
	add	hl, de
	set	0, (hl)
	jr	00149$
00148$:
;ff.c:1628: ncl = (res == FR_DISK_ERR) ? 0xFFFFFFFF : 1;	/* Failed. Generate error status */
	dec	c
	jr	NZ, 00154$
	ld	bc, #0xffff
	ld	de, #0xffff
	jr	00155$
00154$:
	ld	bc, #0x0001
	ld	de, #0x0000
00155$:
	ld	-4 (ix), c
	ld	-3 (ix), b
	ld	-2 (ix), e
	ld	-1 (ix), d
00149$:
;ff.c:1631: return ncl;		/* Return new cluster number or error status */
	ld	e, -4 (ix)
	ld	d, -3 (ix)
	ld	l, -2 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -1 (ix)
;	spillPairReg hl
;	spillPairReg hl
00152$:
;ff.c:1632: }
	ld	sp, ix
	pop	ix
	ret
;ff.c:1675: static FRESULT dir_clear (	/* Returns FR_OK or FR_DISK_ERR */
;	---------------------------------
; Function dir_clear
; ---------------------------------
_dir_clear:
	push	ix
	ld	ix,#0
	add	ix,sp
	ld	iy, #-18
	add	iy, sp
	ld	sp, iy
;ff.c:1685: if (sync_window(fs) != FR_OK) return FR_DISK_ERR;	/* Flush disk access window */
;	spillPairReg hl
;	spillPairReg hl
	ld	-4 (ix), l
	ld	-3 (ix), h
;	spillPairReg hl
;	spillPairReg hl
	call	_sync_window
	ld	-1 (ix), a
	or	a, a
	jr	Z, 00102$
	ld	a, #0x01
	jp	00108$
00102$:
;ff.c:1686: sect = clst2sect(fs, clst);		/* Top of the cluster */
	ld	l, 6 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, 7 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	ld	l, 4 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, 5 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	ld	l, -4 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -3 (ix)
;	spillPairReg hl
;	spillPairReg hl
	call	_clst2sect
	pop	af
	pop	af
	ld	-8 (ix), e
	ld	-7 (ix), d
	ld	-6 (ix), l
	ld	-5 (ix), h
	ld	hl, #0
	add	hl, sp
	ex	de, hl
	ld	hl, #10
	add	hl, sp
	ld	bc, #4
	ldir
;ff.c:1687: fs->winsect = sect;				/* Set window to top of the cluster */
	ld	a, -4 (ix)
	add	a, #0x1c
	ld	-2 (ix), a
	ld	a, -3 (ix)
	adc	a, #0x00
	ld	-1 (ix), a
	ld	e, -2 (ix)
	ld	d, -1 (ix)
	ld	hl, #0
	add	hl, sp
	ld	bc, #0x0004
	ldir
;ff.c:1688: memset(fs->win, 0, sizeof fs->win);	/* Clear window buffer */
	ld	a, -4 (ix)
	add	a, #0x30
	ld	-2 (ix), a
	ld	a, -3 (ix)
	adc	a, #0x00
	ld	-1 (ix), a
	ld	l, -2 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -1 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	(hl), #0x00
	ld	e, l
	ld	d, h
	inc	de
	ld	bc, #0x01ff
	ldir
;ff.c:1700: ibuf = fs->win; szb = 1;	/* Use window buffer (many single-sector writes may take a time) */
	ld	a, -2 (ix)
	ld	-14 (ix), a
	ld	a, -1 (ix)
	ld	-13 (ix), a
;ff.c:1701: for (n = 0; n < fs->csize && disk_write(fs->pdrv, ibuf, sect + n, szb) == RES_OK; n += szb) ;	/* Fill the cluster with 0 */
	ld	a, -4 (ix)
	ld	-12 (ix), a
	ld	a, -3 (ix)
	ld	-11 (ix), a
	ld	a, -4 (ix)
	add	a, #0x0a
	ld	-10 (ix), a
	ld	a, -3 (ix)
	adc	a, #0x00
	ld	-9 (ix), a
	xor	a, a
	ld	-2 (ix), a
	ld	-1 (ix), a
00106$:
	ld	l, -10 (ix)
	ld	h, -9 (ix)
	ld	a, (hl)
	ld	-8 (ix), a
	inc	hl
	ld	a, (hl)
	ld	-7 (ix), a
	ld	a, -2 (ix)
	ld	-6 (ix), a
	ld	a, -1 (ix)
	ld	-5 (ix), a
	ld	a, -6 (ix)
	sub	a, -8 (ix)
	ld	a, -5 (ix)
	sbc	a, -7 (ix)
	jr	NC, 00103$
	ld	c, -2 (ix)
	ld	b, -1 (ix)
	ld	de, #0x0000
	ld	a, c
	add	a, -18 (ix)
	ld	c, a
	ld	a, b
	adc	a, -17 (ix)
	ld	b, a
	ld	a, e
	adc	a, -16 (ix)
	ld	e, a
	ld	a, d
	adc	a, -15 (ix)
	ld	d, a
	ld	l, -12 (ix)
	ld	h, -11 (ix)
	inc	hl
	ld	l, (hl)
;	spillPairReg hl
	ld	iy, #0x0001
	push	iy
	push	de
	push	bc
	ld	e, -14 (ix)
	ld	d, -13 (ix)
	ld	a, l
	call	_disk_write
	or	a, a
	jr	NZ, 00103$
	inc	-2 (ix)
	jr	NZ, 00106$
	inc	-1 (ix)
	jr	00106$
00103$:
;ff.c:1703: return (n == fs->csize) ? FR_OK : FR_DISK_ERR;
	ld	l, -10 (ix)
	ld	h, -9 (ix)
	ld	a, (hl)
	ld	-2 (ix), a
	inc	hl
	ld	a, (hl)
	ld	-1 (ix), a
	ld	a, -6 (ix)
	sub	a, -2 (ix)
	jr	NZ, 00110$
	ld	a, -5 (ix)
	sub	a, -1 (ix)
	jr	NZ, 00110$
	xor	a, a
	jr	00111$
00110$:
	ld	a, #0x01
00111$:
00108$:
;ff.c:1704: }
	ld	sp, ix
	pop	ix
	pop	hl
	pop	bc
	pop	bc
	jp	(hl)
;ff.c:1714: static FRESULT dir_sdi (	/* FR_OK(0):succeeded, !=0:error */
;	---------------------------------
; Function dir_sdi
; ---------------------------------
_dir_sdi:
	push	ix
	ld	ix,#0
	add	ix,sp
	ld	iy, #-18
	add	iy, sp
	ld	sp, iy
;ff.c:1720: FATFS *fs = dp->obj.fs;
	ld	-2 (ix), l
	ld	-1 (ix), h
	ld	a, (hl)
	ld	-18 (ix), a
	inc	hl
	ld	a, (hl)
	ld	-17 (ix), a
;ff.c:1723: if (ofs >= (DWORD)((FF_FS_EXFAT && fs->fs_type == FS_EXFAT) ? MAX_DIR_EX : MAX_DIR) || ofs % SZDIRE) {	/* Check range of offset and alignment */
	ld	a, 6 (ix)
	sub	a, #0x20
	ld	a, 7 (ix)
	sbc	a, #0x00
	jr	NC, 00101$
	ld	a, 4 (ix)
	and	a, #0x1f
	jr	Z, 00102$
00101$:
;ff.c:1724: return FR_INT_ERR;
	ld	a, #0x02
	jp	00124$
00102$:
;ff.c:1726: dp->dptr = ofs;				/* Set current offset */
	ld	a, -2 (ix)
	add	a, #0x0e
	ld	e, a
	ld	a, -1 (ix)
	adc	a, #0x00
	ld	d, a
	ld	hl, #22
	add	hl, sp
	ld	bc, #0x0004
	ldir
;ff.c:1727: clst = dp->obj.sclust;		/* Table start cluster (0:root) */
	ld	a, -2 (ix)
	ld	-4 (ix), a
	ld	a, -1 (ix)
	ld	-3 (ix), a
	ld	e, -4 (ix)
	ld	d, -3 (ix)
	ld	hl, #6
	add	hl, sp
	ex	de, hl
	ld	bc, #0x0006
	add	hl, bc
	ld	bc, #0x0004
	ldir
;ff.c:1729: clst = (DWORD)fs->dirbase;
	ld	a, -18 (ix)
	add	a, #0x28
	ld	-8 (ix), a
	ld	a, -17 (ix)
	adc	a, #0x00
	ld	-7 (ix), a
;ff.c:1728: if (clst == 0 && fs->fs_type >= FS_FAT32) {	/* Replace cluster# 0 with root cluster# */
	ld	a, -9 (ix)
	or	a, -10 (ix)
	or	a, -11 (ix)
	or	a, -12 (ix)
	jr	NZ, 00107$
	pop	hl
	push	hl
	ld	a, (hl)
	sub	a, #0x03
	jr	C, 00107$
;ff.c:1729: clst = (DWORD)fs->dirbase;
	ld	e, -8 (ix)
	ld	d, -7 (ix)
	ld	hl, #6
	add	hl, sp
	ex	de, hl
	ld	bc, #0x0004
	ldir
;ff.c:1730: if (FF_FS_EXFAT) dp->obj.stat = 0;	/* exFAT: Root dir has an FAT chain */
00107$:
;ff.c:1735: dp->sect = fs->dirbase;
	ld	a, -2 (ix)
	add	a, #0x16
	ld	-16 (ix), a
	ld	a, -1 (ix)
	adc	a, #0x00
	ld	-15 (ix), a
;ff.c:1733: if (clst == 0) {	/* Static table (root-directory on the FAT volume) */
	ld	a, -9 (ix)
	or	a, -10 (ix)
	or	a, -11 (ix)
	or	a, -12 (ix)
	jr	NZ, 00120$
;ff.c:1734: if (ofs / SZDIRE >= fs->n_rootdir) return FR_INT_ERR;	/* Is index out of range? */
	ld	c, 4 (ix)
	ld	b, 5 (ix)
	ld	e, 6 (ix)
	ld	d, 7 (ix)
	ld	a, #0x05
00183$:
	srl	d
	rr	e
	rr	b
	rr	c
	dec	a
	jr	NZ, 00183$
	pop	hl
	push	hl
	push	bc
	ld	bc, #0x0008
	add	hl, bc
	pop	bc
	ld	a, (hl)
	inc	hl
	ld	h, (hl)
;	spillPairReg hl
;	spillPairReg hl
;	spillPairReg hl
	ld	-6 (ix), a
	ld	-5 (ix), h
	xor	a, a
	ld	-4 (ix), a
	ld	-3 (ix), a
	ld	a, c
	sub	a, -6 (ix)
	ld	a, b
	sbc	a, -5 (ix)
	ld	a, e
	sbc	a, -4 (ix)
	ld	a, d
	sbc	a, -3 (ix)
	jr	C, 00110$
	ld	a, #0x02
	jp	00124$
00110$:
;ff.c:1735: dp->sect = fs->dirbase;
	ld	l, -8 (ix)
	ld	h, -7 (ix)
	ld	c, (hl)
	inc	hl
	ld	b, (hl)
	inc	hl
	ld	e, (hl)
	inc	hl
	ld	d, (hl)
	ld	l, -16 (ix)
	ld	h, -15 (ix)
	ld	(hl), c
	inc	hl
	ld	(hl), b
	inc	hl
	ld	(hl), e
	inc	hl
	ld	(hl), d
	jp	00121$
00120$:
;ff.c:1738: csz = (DWORD)fs->csize * SS(fs);	/* Bytes per cluster */
	pop	bc
	push	bc
	ld	hl, #10
	add	hl, bc
	ld	c, (hl)
	inc	hl
	ld	a, (hl)
	ld	hl, #0x0000
	ld	h, l
;	spillPairReg hl
;	spillPairReg hl
	ld	l, a
;	spillPairReg hl
;	spillPairReg hl
	ld	a, c
	ld	c, #0x00
	add	a, a
	adc	hl, hl
	ld	-8 (ix), c
	ld	-7 (ix), a
	ld	-6 (ix), l
	ld	-5 (ix), h
;ff.c:1739: while (ofs >= csz) {				/* Follow cluster chain */
	ld	a, -18 (ix)
	ld	-4 (ix), a
	ld	a, -17 (ix)
	ld	-3 (ix), a
00116$:
	ld	a, 4 (ix)
	sub	a, -8 (ix)
	ld	a, 5 (ix)
	sbc	a, -7 (ix)
	ld	a, 6 (ix)
	sbc	a, -6 (ix)
	ld	a, 7 (ix)
	sbc	a, -5 (ix)
	jp	C, 00118$
;ff.c:1740: clst = get_fat(&dp->obj, clst);				/* Get next cluster */
	ld	l, -10 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -9 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	ld	l, -12 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -11 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	ld	l, -2 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -1 (ix)
;	spillPairReg hl
;	spillPairReg hl
	call	_get_fat
	pop	af
	pop	af
	ld	-12 (ix), e
	ld	-11 (ix), d
	ld	-10 (ix), l
	ld	-9 (ix), h
;ff.c:1741: if (clst == 0xFFFFFFFF) return FR_DISK_ERR;	/* Disk error */
	ld	a, -12 (ix)
	and	a, -11 (ix)
	and	a, -10 (ix)
	and	a, -9 (ix)
	inc	a
	jr	NZ, 00112$
	ld	a, #0x01
	jp	00124$
00112$:
;ff.c:1742: if (clst < 2 || clst >= fs->n_fatent) return FR_INT_ERR;	/* Reached to end of table or internal error */
	ld	a, -12 (ix)
	sub	a, #0x02
	ld	a, -11 (ix)
	sbc	a, #0x00
	ld	a, -10 (ix)
	sbc	a, #0x00
	ld	a, -9 (ix)
	sbc	a, #0x00
	jr	C, 00113$
	ld	l, -4 (ix)
	ld	h, -3 (ix)
	ld	de, #0x0014
	add	hl, de
	ld	c, (hl)
	inc	hl
	ld	b, (hl)
	inc	hl
	ld	e, (hl)
	inc	hl
	ld	d, (hl)
	ld	a, -12 (ix)
	sub	a, c
	ld	a, -11 (ix)
	sbc	a, b
	ld	a, -10 (ix)
	sbc	a, e
	ld	a, -9 (ix)
	sbc	a, d
	jr	C, 00114$
00113$:
	ld	a, #0x02
	jp	00124$
00114$:
;ff.c:1743: ofs -= csz;
	ld	a, 4 (ix)
	sub	a, -8 (ix)
	ld	4 (ix), a
	ld	a, 5 (ix)
	sbc	a, -7 (ix)
	ld	5 (ix), a
	ld	a, 6 (ix)
	sbc	a, -6 (ix)
	ld	6 (ix), a
	ld	a, 7 (ix)
	sbc	a, -5 (ix)
	ld	7 (ix), a
	jp	00116$
00118$:
;ff.c:1745: dp->sect = clst2sect(fs, clst);
	ld	l, -10 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -9 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	ld	l, -12 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -11 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	ld	l, -18 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -17 (ix)
;	spillPairReg hl
;	spillPairReg hl
	call	_clst2sect
	pop	af
	pop	af
	ld	c, l
	ld	b, h
	ld	l, -16 (ix)
	ld	h, -15 (ix)
	ld	(hl), e
	inc	hl
	ld	(hl), d
	inc	hl
	ld	(hl), c
	inc	hl
	ld	(hl), b
00121$:
;ff.c:1747: dp->clust = clst;					/* Current cluster# */
	ld	a, -2 (ix)
	add	a, #0x12
	ld	e, a
	ld	a, -1 (ix)
	adc	a, #0x00
	ld	d, a
	ld	hl, #6
	add	hl, sp
	ld	bc, #0x0004
	ldir
;ff.c:1748: if (dp->sect == 0) return FR_INT_ERR;
	pop	hl
	pop	de
	push	de
	push	hl
	ld	hl, #4
	add	hl, sp
	ex	de, hl
	ld	bc, #0x0004
	ldir
	ld	a, -11 (ix)
	or	a, -12 (ix)
	or	a, -13 (ix)
	or	a, -14 (ix)
	jr	NZ, 00123$
	ld	a, #0x02
	jp	00124$
00123$:
;ff.c:1749: dp->sect += ofs / SS(fs);			/* Sector# of the directory entry */
	ld	a, 5 (ix)
	ld	-10 (ix), a
	ld	a, 6 (ix)
	ld	-9 (ix), a
	ld	a, 7 (ix)
	ld	-8 (ix), a
	ld	-7 (ix), #0x00
	srl	-8 (ix)
	rr	-9 (ix)
	rr	-10 (ix)
	ld	a, -10 (ix)
	add	a, -14 (ix)
	ld	-6 (ix), a
	ld	a, -9 (ix)
	adc	a, -13 (ix)
	ld	-5 (ix), a
	ld	a, -8 (ix)
	adc	a, -12 (ix)
	ld	-4 (ix), a
	ld	a, -7 (ix)
	adc	a, -11 (ix)
	ld	-3 (ix), a
	ld	e, -16 (ix)
	ld	d, -15 (ix)
	ld	hl, #12
	add	hl, sp
	ld	bc, #0x0004
	ldir
;ff.c:1750: dp->dir = fs->win + (ofs % SS(fs));	/* Pointer to the entry in the win[] */
	ld	a, -2 (ix)
	add	a, #0x1a
	ld	-6 (ix), a
	ld	a, -1 (ix)
	adc	a, #0x00
	ld	-5 (ix), a
	ld	a, -18 (ix)
	add	a, #0x30
	ld	-4 (ix), a
	ld	a, -17 (ix)
	adc	a, #0x00
	ld	-3 (ix), a
	ld	a, 4 (ix)
	ld	-12 (ix), a
	ld	a, 5 (ix)
	and	a, #0x01
	ld	-11 (ix), a
	xor	a, a
	ld	-10 (ix), a
	ld	-9 (ix), a
	ld	a, -4 (ix)
	add	a, -12 (ix)
	ld	-8 (ix), a
	ld	a, -3 (ix)
	adc	a, -11 (ix)
	ld	-7 (ix), a
	ld	l, -6 (ix)
	ld	h, -5 (ix)
	ld	a, -8 (ix)
	ld	(hl), a
	inc	hl
	ld	a, -7 (ix)
	ld	(hl), a
;ff.c:1752: return FR_OK;
	xor	a, a
00124$:
;ff.c:1753: }
	ld	sp, ix
	pop	ix
	pop	hl
	pop	bc
	pop	bc
	jp	(hl)
;ff.c:1762: static FRESULT dir_next (	/* FR_OK(0):succeeded, FR_NO_FILE:End of table, FR_DENIED:Could not stretch */
;	---------------------------------
; Function dir_next
; ---------------------------------
_dir_next:
	push	ix
	ld	ix,#0
	add	ix,sp
	ld	iy, #-28
	add	iy, sp
	ld	sp, iy
	ld	-2 (ix), l
	ld	-1 (ix), h
	ld	-4 (ix), e
	ld	-3 (ix), d
;ff.c:1768: FATFS *fs = dp->obj.fs;
	ld	l, -2 (ix)
	ld	h, -1 (ix)
	ld	a, (hl)
	ld	-28 (ix), a
	inc	hl
	ld	a, (hl)
	ld	-27 (ix), a
;ff.c:1771: ofs = dp->dptr + SZDIRE;	/* Next entry */
	ld	a, -2 (ix)
	add	a, #0x0e
	ld	-26 (ix), a
	ld	a, -1 (ix)
	adc	a, #0x00
	ld	-25 (ix), a
	pop	hl
	pop	de
	push	de
	push	hl
	ld	hl, #20
	add	hl, sp
	ex	de, hl
	ld	bc, #0x0004
	ldir
	ld	a, -8 (ix)
	add	a, #0x20
	ld	-24 (ix), a
	ld	a, -7 (ix)
	adc	a, #0x00
	ld	-23 (ix), a
	ld	a, -6 (ix)
	adc	a, #0x00
	ld	-22 (ix), a
	ld	a, -5 (ix)
	adc	a, #0x00
	ld	-21 (ix), a
;ff.c:1772: if (ofs >= (DWORD)((FF_FS_EXFAT && fs->fs_type == FS_EXFAT) ? MAX_DIR_EX : MAX_DIR)) dp->sect = 0;	/* Disable it if the offset reached the max value */
	ld	a, -2 (ix)
	add	a, #0x16
	ld	-20 (ix), a
	ld	a, -1 (ix)
	adc	a, #0x00
	ld	-19 (ix), a
	ld	a, -22 (ix)
	sub	a, #0x20
	ld	a, -21 (ix)
	sbc	a, #0x00
	jr	C, 00102$
	ld	l, -20 (ix)
	ld	h, -19 (ix)
	xor	a, a
	ld	(hl), a
	inc	hl
	ld	(hl), a
	inc	hl
	ld	(hl), a
	inc	hl
	ld	(hl), a
00102$:
;ff.c:1773: if (dp->sect == 0) return FR_NO_FILE;	/* Report EOT if it has been disabled */
	ld	e, -20 (ix)
	ld	d, -19 (ix)
	ld	hl, #20
	add	hl, sp
	ex	de, hl
	ld	bc, #0x0004
	ldir
	ld	a, -5 (ix)
	or	a, -6 (ix)
	or	a, -7 (ix)
	or	a, -8 (ix)
	jr	NZ, 00104$
	ld	a, #0x04
	jp	00132$
00104$:
;ff.c:1775: if (ofs % SS(fs) == 0) {	/* Sector changed? */
	ld	a, -24 (ix)
	ld	-18 (ix), a
	ld	a, -23 (ix)
	and	a, #0x01
	ld	-17 (ix), a
	xor	a, a
	ld	-16 (ix), a
	ld	-15 (ix), a
	or	a, -16 (ix)
	or	a, -17 (ix)
	or	a, -18 (ix)
	jp	NZ, 00131$
;ff.c:1776: dp->sect++;				/* Next sector */
	ld	a, -8 (ix)
	add	a, #0x01
	ld	c, a
	ld	a, -7 (ix)
	adc	a, #0x00
	ld	b, a
	ld	a, -6 (ix)
	adc	a, #0x00
	ld	e, a
	ld	a, -5 (ix)
	adc	a, #0x00
	ld	d, a
	ld	l, -20 (ix)
	ld	h, -19 (ix)
	ld	(hl), c
	inc	hl
	ld	(hl), b
	inc	hl
	ld	(hl), e
	inc	hl
	ld	(hl), d
;ff.c:1778: if (dp->clust == 0) {	/* Static table */
	ld	a, -2 (ix)
	add	a, #0x12
	ld	-14 (ix), a
	ld	a, -1 (ix)
	adc	a, #0x00
	ld	-13 (ix), a
	ld	e, -14 (ix)
	ld	d, -13 (ix)
	ld	hl, #16
	add	hl, sp
	ex	de, hl
	ld	bc, #0x0004
	ldir
	ld	a, -9 (ix)
	or	a, -10 (ix)
	or	a, -11 (ix)
	or	a, -12 (ix)
	jr	NZ, 00128$
;ff.c:1779: if (ofs / SZDIRE >= fs->n_rootdir) {	/* Report EOT if it reached end of static table */
	ld	c, -24 (ix)
	ld	b, -23 (ix)
	ld	e, -22 (ix)
	ld	d, -21 (ix)
	ld	a, #0x05
00210$:
	srl	d
	rr	e
	rr	b
	rr	c
	dec	a
	jr	NZ, 00210$
	pop	hl
	push	hl
	push	bc
	ld	bc, #0x0008
	add	hl, bc
	pop	bc
	ld	a, (hl)
	inc	hl
	ld	h, (hl)
;	spillPairReg hl
;	spillPairReg hl
;	spillPairReg hl
	ld	-8 (ix), a
	ld	-7 (ix), h
	xor	a, a
	ld	-6 (ix), a
	ld	-5 (ix), a
	ld	a, c
	sub	a, -8 (ix)
	ld	a, b
	sbc	a, -7 (ix)
	ld	a, e
	sbc	a, -6 (ix)
	ld	a, d
	sbc	a, -5 (ix)
	jp	C, 00131$
;ff.c:1780: dp->sect = 0; return FR_NO_FILE;
	ld	l, -20 (ix)
	ld	h, -19 (ix)
	xor	a, a
	ld	(hl), a
	inc	hl
	ld	(hl), a
	inc	hl
	ld	(hl), a
	inc	hl
	ld	(hl), a
	ld	a, #0x04
	jp	00132$
00128$:
;ff.c:1784: if ((ofs / SS(fs) & (fs->csize - 1)) == 0) {	/* Cluster changed? */
	ld	c, -23 (ix)
	ld	b, -22 (ix)
	ld	e, -21 (ix)
	ld	d, #0x00
	srl	e
	rr	b
	rr	c
	pop	hl
	push	hl
	push	bc
	ld	bc, #0x000a
	add	hl, bc
	pop	bc
	ld	a, (hl)
	inc	hl
	ld	h, (hl)
;	spillPairReg hl
	ld	l, a
;	spillPairReg hl
;	spillPairReg hl
	dec	hl
	ld	-8 (ix), l
	ld	-7 (ix), h
	xor	a, a
	ld	-6 (ix), a
	ld	-5 (ix), a
	ld	a, c
	and	a, -8 (ix)
	ld	c, a
	ld	a, b
	and	a, -7 (ix)
	ld	b, a
	ld	a, e
	and	a, -6 (ix)
	ld	e, a
	ld	a, d
	and	a, -5 (ix)
	or	a, e
	or	a, b
	or	a, c
	jp	NZ, 00131$
;ff.c:1785: clst = get_fat(&dp->obj, dp->clust);		/* Get next cluster */
	ld	l, -10 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -9 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	ld	l, -12 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -11 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	ld	l, -2 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -1 (ix)
;	spillPairReg hl
;	spillPairReg hl
	call	_get_fat
	pop	af
	pop	af
	ld	c, l
	ld	b, h
;ff.c:1786: if (clst <= 1) return FR_INT_ERR;			/* Internal error */
	ld	a, #0x01
	cp	a, e
	ld	a, #0x00
	sbc	a, d
	ld	hl, #0x0000
	sbc	hl, bc
	jr	C, 00108$
	ld	a, #0x02
	jp	00132$
00108$:
;ff.c:1787: if (clst == 0xFFFFFFFF) return FR_DISK_ERR;	/* Disk error */
	ld	a, e
	and	a, d
	and	a, c
	and	a, b
	inc	a
	jr	NZ, 00110$
	ld	a, #0x01
	jp	00132$
00110$:
;ff.c:1788: if (clst >= fs->n_fatent) {					/* It reached end of dynamic table */
	pop	hl
	push	hl
	push	de
	push	bc
	ex	de, hl
	ld	hl, #24
	add	hl, sp
	ex	de, hl
	ld	bc, #0x0014
	add	hl, bc
	ld	bc, #0x0004
	ldir
	pop	bc
	pop	de
	ld	a, e
	sub	a, -8 (ix)
	ld	a, d
	sbc	a, -7 (ix)
	ld	a, c
	sbc	a, -6 (ix)
	ld	a, b
	sbc	a, -5 (ix)
	jr	C, 00124$
;ff.c:1790: if (!stretch) {								/* If no stretch, report EOT */
	ld	a, -3 (ix)
	or	a, -4 (ix)
	jr	NZ, 00112$
;ff.c:1791: dp->sect = 0; return FR_NO_FILE;
	ld	l, -20 (ix)
	ld	h, -19 (ix)
	xor	a, a
	ld	(hl), a
	inc	hl
	ld	(hl), a
	inc	hl
	ld	(hl), a
	inc	hl
	ld	(hl), a
	ld	a, #0x04
	jp	00132$
00112$:
;ff.c:1793: clst = create_chain(&dp->obj, dp->clust);	/* Allocate a cluster */
	ld	l, -14 (ix)
	ld	h, -13 (ix)
	ld	c, (hl)
	inc	hl
	ld	b, (hl)
	inc	hl
	ld	e, (hl)
	inc	hl
	ld	d, (hl)
	push	de
	push	bc
	ld	l, -2 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -1 (ix)
;	spillPairReg hl
;	spillPairReg hl
	call	_create_chain
	pop	af
	pop	af
	ld	c, l
	ld	b, h
;ff.c:1794: if (clst == 0) return FR_DENIED;			/* No free cluster */
	ld	a, b
	or	a, c
	or	a, d
	or	a, e
	jr	NZ, 00114$
	ld	a, #0x07
	jp	00132$
00114$:
;ff.c:1795: if (clst == 1) return FR_INT_ERR;			/* Internal error */
	ld	a, e
	dec	a
	or	a, d
	or	a, c
	or	a, b
	jr	NZ, 00116$
	ld	a, #0x02
	jp	00132$
00116$:
;ff.c:1796: if (clst == 0xFFFFFFFF) return FR_DISK_ERR;	/* Disk error */
	ld	a, e
	and	a, d
	and	a, c
	and	a, b
	inc	a
	jr	NZ, 00118$
	ld	a, #0x01
	jp	00132$
00118$:
;ff.c:1797: if (dir_clear(fs, clst) != FR_OK) return FR_DISK_ERR;	/* Clean up the stretched table */
	push	bc
	push	de
	push	bc
	push	de
	ld	l, -28 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -27 (ix)
;	spillPairReg hl
;	spillPairReg hl
	call	_dir_clear
	pop	de
	pop	bc
	or	a, a
	jr	Z, 00124$
	ld	a, #0x01
	jr	00132$
;ff.c:1798: if (FF_FS_EXFAT) dp->obj.stat |= 4;			/* exFAT: The directory has been stretched */
00124$:
;ff.c:1804: dp->clust = clst;		/* Initialize data for new cluster */
	ld	l, -14 (ix)
	ld	h, -13 (ix)
	ld	(hl), e
	inc	hl
	ld	(hl), d
	inc	hl
	ld	(hl), c
	inc	hl
	ld	(hl), b
;ff.c:1805: dp->sect = clst2sect(fs, clst);
	push	bc
	push	de
	ld	l, -28 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -27 (ix)
;	spillPairReg hl
;	spillPairReg hl
	call	_clst2sect
	pop	af
	pop	af
	ld	c, l
	ld	b, h
	ld	l, -20 (ix)
	ld	h, -19 (ix)
	ld	(hl), e
	inc	hl
	ld	(hl), d
	inc	hl
	ld	(hl), c
	inc	hl
	ld	(hl), b
00131$:
;ff.c:1809: dp->dptr = ofs;						/* Current entry */
	ld	e, -26 (ix)
	ld	d, -25 (ix)
	ld	hl, #4
	add	hl, sp
	ld	bc, #0x0004
	ldir
;ff.c:1810: dp->dir = fs->win + ofs % SS(fs);	/* Pointer to the entry in the win[] */
	ld	a, -2 (ix)
	add	a, #0x1a
	ld	-8 (ix), a
	ld	a, -1 (ix)
	adc	a, #0x00
	ld	-7 (ix), a
	ld	a, -28 (ix)
	add	a, #0x30
	ld	-6 (ix), a
	ld	a, -27 (ix)
	adc	a, #0x00
	ld	-5 (ix), a
	ld	a, -6 (ix)
	add	a, -18 (ix)
	ld	-10 (ix), a
	ld	a, -5 (ix)
	adc	a, -17 (ix)
	ld	-9 (ix), a
	ld	l, -8 (ix)
	ld	h, -7 (ix)
	ld	a, -10 (ix)
	ld	(hl), a
	inc	hl
	ld	a, -9 (ix)
	ld	(hl), a
;ff.c:1812: return FR_OK;
	xor	a, a
00132$:
;ff.c:1813: }
	ld	sp, ix
	pop	ix
	ret
;ff.c:1823: static FRESULT dir_alloc (	/* FR_OK(0):succeeded, !=0:error */
;	---------------------------------
; Function dir_alloc
; ---------------------------------
_dir_alloc:
	push	ix
	ld	ix,#0
	add	ix,sp
	ld	iy, #-13
	add	iy, sp
	ld	sp, iy
	ld	-4 (ix), l
	ld	-3 (ix), h
	ld	-6 (ix), e
	ld	-5 (ix), d
;ff.c:1830: FATFS *fs = dp->obj.fs;
	ld	l, -4 (ix)
	ld	h, -3 (ix)
	ld	a, (hl)
	ld	-13 (ix), a
	inc	hl
	ld	a, (hl)
	ld	-12 (ix), a
;ff.c:1833: res = dir_sdi(dp, 0);
	ld	hl, #0x0000
	push	hl
	push	hl
	ld	l, -4 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -3 (ix)
;	spillPairReg hl
;	spillPairReg hl
	call	_dir_sdi
;ff.c:1834: if (res == FR_OK) {
	ld	-11 (ix), a
;ff.c:1835: n = 0;
	or	a,a
	jp	NZ,00113$
	ld	-2 (ix), a
	ld	-1 (ix), a
;ff.c:1836: do {
	ld	a, -4 (ix)
	ld	-10 (ix), a
	ld	a, -3 (ix)
	ld	-9 (ix), a
	ld	a, -4 (ix)
	ld	-8 (ix), a
	ld	a, -3 (ix)
	ld	-7 (ix), a
00109$:
;ff.c:1837: res = move_window(fs, dp->sect);
	ld	l, -8 (ix)
	ld	h, -7 (ix)
	ld	de, #0x0016
	add	hl, de
	ld	c, (hl)
	inc	hl
	ld	b, (hl)
	inc	hl
	ld	e, (hl)
	inc	hl
	ld	d, (hl)
	push	de
	push	bc
	ld	l, -13 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -12 (ix)
;	spillPairReg hl
;	spillPairReg hl
	call	_move_window
;ff.c:1838: if (res != FR_OK) break;
	ld	-11 (ix), a
	or	a, a
	jr	NZ, 00113$
;ff.c:1842: if (dp->dir[DIR_Name] == DDEM || dp->dir[DIR_Name] == 0) {	/* Is the entry free? */
	ld	l, -10 (ix)
	ld	h, -9 (ix)
	ld	de, #0x001a
	add	hl, de
	ld	c, (hl)
	inc	hl
	ld	b, (hl)
	ld	a, (bc)
	cp	a, #0xe5
	jr	Z, 00105$
	or	a, a
	jr	NZ, 00106$
00105$:
;ff.c:1844: if (++n == n_ent) break;	/* Is a block of contiguous free entries found? */
	inc	-2 (ix)
	jr	NZ, 00149$
	inc	-1 (ix)
00149$:
	ld	a, -2 (ix)
	sub	a, -6 (ix)
	jr	NZ, 00107$
	ld	a, -1 (ix)
	sub	a, -5 (ix)
	jr	Z, 00113$
	jr	00107$
00106$:
;ff.c:1846: n = 0;				/* Not a free entry, restart to search */
	xor	a, a
	ld	-2 (ix), a
	ld	-1 (ix), a
00107$:
;ff.c:1848: res = dir_next(dp, 1);	/* Next entry with table stretch enabled */
	ld	de, #0x0001
	ld	l, -4 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -3 (ix)
;	spillPairReg hl
;	spillPairReg hl
	call	_dir_next
;ff.c:1849: } while (res == FR_OK);
	ld	-11 (ix), a
	or	a, a
	jr	Z, 00109$
00113$:
;ff.c:1852: if (res == FR_NO_FILE) res = FR_DENIED;	/* No directory entry to allocate */
	ld	a, -11 (ix)
	sub	a, #0x04
	jr	NZ, 00115$
	ld	-11 (ix), #0x07
00115$:
;ff.c:1853: return res;
	ld	a, -11 (ix)
;ff.c:1854: }
	ld	sp, ix
	pop	ix
	ret
;ff.c:1865: static DWORD ld_clust (	/* Returns the top cluster value of the SFN entry */
;	---------------------------------
; Function ld_clust
; ---------------------------------
_ld_clust:
	push	ix
	ld	ix,#0
	add	ix,sp
	push	af
	push	af
	ld	c, l
	ld	b, h
;ff.c:1872: cl = ld_16(dir + DIR_FstClusLO);
	ld	hl, #0x001a
	add	hl, de
	push	bc
	push	de
	call	_ld_16
	ex	de, hl
	pop	de
	pop	bc
	ex	(sp), hl
	xor	a, a
	ld	-2 (ix), a
	ld	-1 (ix), a
;ff.c:1873: if (fs->fs_type == FS_FAT32) {
	ld	a, (bc)
	sub	a, #0x03
	jr	NZ, 00102$
;ff.c:1874: cl |= (DWORD)ld_16(dir + DIR_FstClusHI) << 16;
	ld	hl, #0x0014
	add	hl, de
	call	_ld_16
	ld	bc, #0x0000
	ld	a, -4 (ix)
	or	a, c
	ld	-4 (ix), a
	ld	a, -3 (ix)
	or	a, b
	ld	-3 (ix), a
	ld	a, -2 (ix)
	or	a, e
	ld	-2 (ix), a
	ld	a, -1 (ix)
	or	a, d
	ld	-1 (ix), a
00102$:
;ff.c:1877: return cl;
	pop	de
	push	de
	ld	l, -2 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -1 (ix)
;	spillPairReg hl
;	spillPairReg hl
;ff.c:1878: }
	ld	sp, ix
	pop	ix
	ret
;ff.c:1882: static void st_clust (
;	---------------------------------
; Function st_clust
; ---------------------------------
_st_clust:
	push	ix
	ld	ix,#0
	add	ix,sp
	ld	c, e
	ld	b, d
;ff.c:1888: st_16(dir + DIR_FstClusLO, (WORD)cl);
	ld	e, 4 (ix)
	ld	d, 5 (ix)
	ld	iy, #0x001a
	add	iy, bc
	push	hl
	push	bc
	push	iy
	pop	hl
	call	_st_16
	pop	bc
	pop	hl
;ff.c:1889: if (fs->fs_type == FS_FAT32) {
	ld	a, (hl)
	sub	a, #0x03
	jr	NZ, 00103$
;ff.c:1890: st_16(dir + DIR_FstClusHI, (WORD)(cl >> 16));
	ld	e, 6 (ix)
	ld	d, 7 (ix)
	ld	hl, #0x0014
	add	hl, bc
	call	_st_16
00103$:
;ff.c:1892: }
	pop	ix
	pop	hl
	pop	af
	pop	af
	jp	(hl)
;ff.c:2334: static FRESULT dir_read (
;	---------------------------------
; Function dir_read
; ---------------------------------
_dir_read:
	push	ix
	ld	ix,#0
	add	ix,sp
	ld	iy, #-15
	add	iy, sp
	ld	sp, iy
	ld	-2 (ix), l
	ld	-1 (ix), h
	ld	-4 (ix), e
	ld	-3 (ix), d
;ff.c:2339: FRESULT res = FR_NO_FILE;
	ld	-15 (ix), #0x04
;ff.c:2340: FATFS *fs = dp->obj.fs;
	ld	l, -2 (ix)
	ld	h, -1 (ix)
	ld	a, (hl)
	ld	-14 (ix), a
	inc	hl
	ld	a, (hl)
	ld	-13 (ix), a
;ff.c:2346: while (dp->sect) {
	ld	a, -2 (ix)
	add	a, #0x16
	ld	-12 (ix), a
	ld	a, -1 (ix)
	adc	a, #0x00
	ld	-11 (ix), a
	ld	a, -12 (ix)
	ld	-10 (ix), a
	ld	a, -11 (ix)
	ld	-9 (ix), a
00112$:
	ld	e, -10 (ix)
	ld	d, -9 (ix)
	ld	hl, #7
	add	hl, sp
	ex	de, hl
	ld	bc, #0x0004
	ldir
	ld	a, -5 (ix)
	or	a, -6 (ix)
	or	a, -7 (ix)
	or	a, -8 (ix)
	jp	Z, 00114$
;ff.c:2347: res = move_window(fs, dp->sect);
	ld	l, -6 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -5 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	ld	l, -8 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -7 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	ld	l, -14 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -13 (ix)
;	spillPairReg hl
;	spillPairReg hl
	call	_move_window
;ff.c:2348: if (res != FR_OK) break;
	ld	-15 (ix), a
	or	a, a
	jr	NZ, 00114$
;ff.c:2349: et = dp->dir[DIR_Name];	/* Test for the entry type */
	ld	c, -2 (ix)
	ld	b, -1 (ix)
	ld	hl, #26
	add	hl, bc
	ld	a, (hl)
	inc	hl
	ld	h, (hl)
;	spillPairReg hl
	ld	l, a
;	spillPairReg hl
;	spillPairReg hl
	ld	b, (hl)
;ff.c:2350: if (et == 0) {
	ld	a, b
	or	a, a
	jr	NZ, 00104$
;ff.c:2351: res = FR_NO_FILE; break; /* Reached to end of the directory */
	ld	-15 (ix), #0x04
	jr	00114$
00104$:
;ff.c:2370: dp->obj.attr = attr = dp->dir[DIR_Attr] & AM_MASK;	/* Get attribute */
	ld	a, -2 (ix)
	add	a, #0x04
	ld	e, a
	ld	a, -1 (ix)
	adc	a, #0x00
	ld	d, a
	push	bc
	ld	bc, #0x000b
	add	hl, bc
	pop	bc
	ld	a, (hl)
	and	a, #0x3f
	ld	c, a
	ld	(de), a
;ff.c:2391: if (et != DDEM && et != '.' && attr != AM_LFN && (int)((attr & ~AM_ARC) == AM_VOL) == vol) {	/* Is it a valid entry? */
	ld	a, b
	sub	a, #0xe5
	jr	Z, 00106$
	ld	a, b
	sub	a, #0x2e
	jr	Z, 00106$
	ld	a, c
	sub	a, #0x0f
	jr	Z, 00106$
	ld	b, #0x00
	res	5, c
	ld	a, c
	sub	a, #0x08
	or	a, b
	ld	a, #0x01
	jr	Z, 00163$
	xor	a, a
00163$:
	ld	c, a
	ld	b, #0x00
	ld	l, -4 (ix)
	ld	h, -3 (ix)
	cp	a, a
	sbc	hl, bc
	jr	Z, 00114$
;ff.c:2392: break;
00106$:
;ff.c:2396: res = dir_next(dp, 0);		/* Next entry */
	ld	de, #0x0000
	ld	l, -2 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -1 (ix)
;	spillPairReg hl
;	spillPairReg hl
	call	_dir_next
;ff.c:2397: if (res != FR_OK) break;
	ld	-15 (ix), a
	or	a, a
	jp	Z, 00112$
00114$:
;ff.c:2400: if (res != FR_OK) dp->sect = 0;		/* Terminate the read operation on error or EOT */
	ld	a, -15 (ix)
	or	a, a
	jr	Z, 00116$
	ld	l, -12 (ix)
	ld	h, -11 (ix)
	xor	a, a
	ld	(hl), a
	inc	hl
	ld	(hl), a
	inc	hl
	ld	(hl), a
	inc	hl
	ld	(hl), a
00116$:
;ff.c:2401: return res;
	ld	a, -15 (ix)
;ff.c:2402: }
	ld	sp, ix
	pop	ix
	ret
;ff.c:2412: static FRESULT dir_find (	/* FR_OK(0):succeeded, !=0:error */
;	---------------------------------
; Function dir_find
; ---------------------------------
_dir_find:
	push	ix
	ld	ix,#0
	add	ix,sp
	ld	iy, #-9
	add	iy, sp
	ld	sp, iy
;ff.c:2417: FATFS *fs = dp->obj.fs;
	ld	-2 (ix), l
	ld	-1 (ix), h
	ld	a, (hl)
	ld	-9 (ix), a
	inc	hl
	ld	a, (hl)
	ld	-8 (ix), a
;ff.c:2423: res = dir_sdi(dp, 0);			/* Rewind directory object */
	ld	hl, #0x0000
	push	hl
	push	hl
	ld	l, -2 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -1 (ix)
;	spillPairReg hl
;	spillPairReg hl
	call	_dir_sdi
;ff.c:2424: if (res != FR_OK) return res;
	ld	-3 (ix), a
	or	a, a
	jr	Z, 00119$
	ld	a, -3 (ix)
	jp	00113$
;ff.c:2449: do {
00119$:
	ld	a, -2 (ix)
	add	a, #0x1c
	ld	-7 (ix), a
	ld	a, -1 (ix)
	adc	a, #0x00
	ld	-6 (ix), a
	ld	a, -2 (ix)
	ld	-5 (ix), a
	ld	a, -1 (ix)
	ld	-4 (ix), a
00110$:
;ff.c:2450: res = move_window(fs, dp->sect);
	ld	l, -5 (ix)
	ld	h, -4 (ix)
	ld	de, #0x0016
	add	hl, de
	ld	c, (hl)
	inc	hl
	ld	b, (hl)
	inc	hl
	ld	e, (hl)
	inc	hl
	ld	d, (hl)
	push	de
	push	bc
	ld	l, -9 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -8 (ix)
;	spillPairReg hl
;	spillPairReg hl
	call	_move_window
;ff.c:2451: if (res != FR_OK) break;
	ld	-3 (ix), a
	or	a, a
	jr	NZ, 00112$
;ff.c:2452: et = dp->dir[DIR_Name];		/* Entry type */
	ld	a, -2 (ix)
	add	a, #0x1a
	ld	c, a
	ld	a, -1 (ix)
	adc	a, #0x00
	ld	b, a
	ld	l, c
	ld	h, b
	ld	a, (hl)
	inc	hl
	ld	h, (hl)
;	spillPairReg hl
	ld	l, a
;	spillPairReg hl
;	spillPairReg hl
	ld	a, (hl)
;ff.c:2453: if (et == 0) { res = FR_NO_FILE; break; }	/* Reached end of directory table */
	or	a, a
	jr	NZ, 00106$
	ld	-3 (ix), #0x04
	jr	00112$
00106$:
;ff.c:2477: dp->obj.attr = dp->dir[DIR_Attr] & AM_MASK;
	ld	a, -2 (ix)
	add	a, #0x04
	ld	e, a
	ld	a, -1 (ix)
	adc	a, #0x00
	ld	d, a
	push	bc
	ld	bc, #0x000b
	add	hl, bc
	pop	bc
	ld	a, (hl)
	and	a, #0x3f
	ld	(de), a
;ff.c:2478: if (!(dp->dir[DIR_Attr] & AM_VOL) && !memcmp(dp->dir, dp->fn, 11)) break;	/* Is it a valid entry? */
	ld	l, c
	ld	h, b
	ld	c, (hl)
	inc	hl
	ld	a, (hl)
	ld	e, c
	ld	d, a
	ld	hl, #11
	add	hl, de
	bit	3, (hl)
	jr	NZ, 00108$
	pop	hl
	pop	de
	push	de
	push	hl
	ld	l, c
;	spillPairReg hl
;	spillPairReg hl
	ld	h, a
;	spillPairReg hl
;	spillPairReg hl
	ld	bc, #0x000b
	push	bc
	call	_memcmp
	ld	a, d
	or	a, e
	jr	Z, 00112$
00108$:
;ff.c:2480: res = dir_next(dp, 0);	/* Next entry */
	ld	de, #0x0000
	ld	l, -2 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -1 (ix)
;	spillPairReg hl
;	spillPairReg hl
	call	_dir_next
;ff.c:2481: } while (res == FR_OK);
	ld	-3 (ix), a
	or	a, a
	jp	Z, 00110$
00112$:
;ff.c:2483: return res;
	ld	a, -3 (ix)
00113$:
;ff.c:2484: }
	ld	sp, ix
	pop	ix
	ret
;ff.c:2494: static FRESULT dir_register (	/* FR_OK:succeeded, FR_DENIED:no free entry or too many SFN collision, FR_DISK_ERR:disk error */
;	---------------------------------
; Function dir_register
; ---------------------------------
_dir_register:
	push	ix
	ld	ix,#0
	add	ix,sp
	push	af
	push	af
	ld	c, l
	ld	b, h
;ff.c:2499: FATFS *fs = dp->obj.fs;
	ld	a, (bc)
	ld	-4 (ix), a
	inc	bc
	ld	a, (bc)
	ld	-3 (ix), a
	dec	bc
;ff.c:2578: res = dir_alloc(dp, 1);		/* Allocate an entry for SFN */
	push	bc
	ld	de, #0x0001
	ld	l, c
;	spillPairReg hl
;	spillPairReg hl
	ld	h, b
;	spillPairReg hl
;	spillPairReg hl
	call	_dir_alloc
	pop	bc
;ff.c:2583: if (res == FR_OK) {
	ld	e, a
	or	a, a
	jr	NZ, 00104$
;ff.c:2584: res = move_window(fs, dp->sect);
	ld	e, c
	ld	d, b
	ld	hl, #22
	add	hl, de
	ld	e, (hl)
	inc	hl
	ld	d, (hl)
	inc	hl
	inc	hl
	ld	a, (hl)
	dec	hl
	ld	l, (hl)
;	spillPairReg hl
	ld	h, a
;	spillPairReg hl
;	spillPairReg hl
	push	bc
	push	hl
	push	de
	ld	l, -4 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -3 (ix)
;	spillPairReg hl
;	spillPairReg hl
	call	_move_window
	pop	bc
;ff.c:2585: if (res == FR_OK) {
	ld	e, a
	or	a, a
	jr	NZ, 00104$
;ff.c:2586: memset(dp->dir, 0, SZDIRE);	/* Clean the entry */
	ld	hl, #0x001a
	add	hl, bc
	ld	a, (hl)
	inc	hl
	ld	d, (hl)
	dec	hl
	ld	-2 (ix), a
	ld	-1 (ix), d
	push	hl
	push	bc
	ld	l, -2 (ix)
	ld	h, -1 (ix)
	ld	b, #0x20
00117$:
	ld	(hl), #0x00
	inc	hl
	djnz	00117$
	pop	bc
	pop	hl
;ff.c:2587: memcpy(dp->dir + DIR_Name, dp->fn, 11);	/* Put SFN */
	ld	a, (hl)
	inc	hl
	ld	h, (hl)
;	spillPairReg hl
	ld	l, a
;	spillPairReg hl
;	spillPairReg hl
	ld	a, c
	add	a, #0x1c
	ld	c, a
	jr	NC, 00119$
	inc	b
00119$:
	ex	de, hl
	push	hl
	ld	l, c
;	spillPairReg hl
;	spillPairReg hl
	ld	h, b
;	spillPairReg hl
;	spillPairReg hl
	ld	bc, #0x000b
	ldir
	pop	de
;ff.c:2591: fs->wflag = 1;
	pop	hl
	push	hl
	ld	bc, #0x0004
	add	hl, bc
	ld	(hl), #0x01
00104$:
;ff.c:2595: return res;
	ld	a, e
;ff.c:2596: }
	ld	sp, ix
	pop	ix
	ret
;ff.c:2607: static FRESULT dir_remove (	/* FR_OK:Succeeded, FR_DISK_ERR:A disk error */
;	---------------------------------
; Function dir_remove
; ---------------------------------
_dir_remove:
	push	ix
	ld	ix,#0
	add	ix,sp
	push	af
	ex	de, hl
;ff.c:2612: FATFS *fs = dp->obj.fs;
	ld	a, (de)
	ld	-2 (ix), a
	inc	de
	ld	a, (de)
	ld	-1 (ix), a
	dec	de
;ff.c:2634: res = move_window(fs, dp->sect);
	ld	c, e
	ld	b, d
	ld	hl, #22
	add	hl, bc
	ld	c, (hl)
	inc	hl
	ld	b, (hl)
	inc	hl
	inc	hl
	ld	a, (hl)
	dec	hl
	ld	l, (hl)
;	spillPairReg hl
	ld	h, a
;	spillPairReg hl
;	spillPairReg hl
	push	de
	push	hl
	push	bc
	ld	l, -2 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -1 (ix)
;	spillPairReg hl
;	spillPairReg hl
	call	_move_window
	pop	de
	ld	c, a
;ff.c:2635: if (res == FR_OK) {
	or	a, a
	jr	NZ, 00102$
;ff.c:2636: dp->dir[DIR_Name] = DDEM;	/* Mark the entry 'deleted'.*/
	ld	hl, #26
	add	hl, de
	ld	e, (hl)
	inc	hl
	ld	d, (hl)
	ld	a, #0xe5
	ld	(de), a
;ff.c:2637: fs->wflag = 1;
	pop	hl
	push	hl
	ld	de, #0x0004
	add	hl, de
	ld	(hl), #0x01
00102$:
;ff.c:2641: return res;
	ld	a, c
;ff.c:2642: }
	ld	sp, ix
	pop	ix
	ret
;ff.c:2653: static void get_fileinfo (
;	---------------------------------
; Function get_fileinfo
; ---------------------------------
_get_fileinfo:
	push	ix
	ld	ix,#0
	add	ix,sp
	ld	iy, #-9
	add	iy, sp
	ld	sp, iy
	ld	c, l
	ld	b, h
;ff.c:2668: fno->fname[0] = 0;
	ld	hl, #0x0009
	add	hl, de
	ld	(hl), #0x00
;ff.c:2669: if (dp->sect == 0) return;	/* Exit if read pointer has reached end of directory */
	ld	l, c
;	spillPairReg hl
;	spillPairReg hl
	ld	h, b
;	spillPairReg hl
;	spillPairReg hl
	push	de
	push	bc
	ex	de, hl
	ld	hl, #9
	add	hl, sp
	ex	de, hl
	ld	bc, #0x0016
	add	hl, bc
	ld	bc, #0x0004
	ldir
	pop	bc
	pop	de
	ld	a, -1 (ix)
	or	a, -2 (ix)
	or	a, -3 (ix)
	or	a, -4 (ix)
	jp	Z,00112$
;ff.c:2774: si = di = 0;
	xor	a, a
	ld	-4 (ix), a
	ld	-3 (ix), a
;ff.c:2775: while (si < 11) {		/* Copy name body and extension */
	ld	hl, #0x001a
	add	hl, bc
	ex	(sp), hl
	ld	hl, #0x0009
	add	hl, de
	ld	c, l
	ld	b, h
	ld	-7 (ix), c
	ld	-6 (ix), b
	xor	a, a
	ld	-2 (ix), a
	ld	-1 (ix), a
00109$:
	ld	a, -2 (ix)
	sub	a, #0x0b
	ld	a, -1 (ix)
	sbc	a, #0x00
	jr	NC, 00111$
;ff.c:2776: c = (TCHAR)dp->dir[si++];
	pop	hl
	push	hl
	ld	a, (hl)
	inc	hl
	ld	h, (hl)
;	spillPairReg hl
;	spillPairReg hl
;	spillPairReg hl
	add	a, -2 (ix)
	ld	l, a
;	spillPairReg hl
;	spillPairReg hl
	ld	a, h
	adc	a, -1 (ix)
	inc	-2 (ix)
	jr	NZ, 00145$
	inc	-1 (ix)
00145$:
	ld	h, a
	ld	a, (hl)
;ff.c:2777: if (c == ' ') continue;		/* Skip padding spaces */
	ld	-5 (ix), a
	sub	a, #0x20
	jr	Z, 00109$
;ff.c:2778: if (c == RDDEM) c = DDEM;	/* Restore replaced DDEM character */
	ld	a, -5 (ix)
	sub	a, #0x05
	jr	NZ, 00106$
	ld	-5 (ix), #0xe5
00106$:
;ff.c:2779: if (si == 9) fno->fname[di++] = '.';/* Insert a . if extension is exist */
	ld	a, -2 (ix)
	sub	a, #0x09
	or	a, -1 (ix)
	jr	NZ, 00108$
	ld	l, -4 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -3 (ix)
;	spillPairReg hl
;	spillPairReg hl
	inc	-4 (ix)
	jr	NZ, 00151$
	inc	-3 (ix)
00151$:
	add	hl, bc
	ld	(hl), #0x2e
00108$:
;ff.c:2780: fno->fname[di++] = c;
	ld	a, -4 (ix)
	ld	h, -3 (ix)
;	spillPairReg hl
;	spillPairReg hl
	inc	-4 (ix)
	jr	NZ, 00152$
	inc	-3 (ix)
00152$:
	add	a, -7 (ix)
	ld	l, a
;	spillPairReg hl
;	spillPairReg hl
	ld	a, h
	adc	a, -6 (ix)
	ld	h, a
	ld	a, -5 (ix)
	ld	(hl), a
	jr	00109$
00111$:
;ff.c:2782: fno->fname[di] = 0;		/* Terminate the SFN */
	ld	l, -4 (ix)
	ld	h, -3 (ix)
	add	hl, bc
	ld	(hl), #0x00
;ff.c:2785: fno->fattrib = dp->dir[DIR_Attr] & AM_MASK;		/* Attribute */
	ld	hl, #0x0008
	add	hl, de
	ld	c, l
	ld	b, h
	pop	hl
	push	hl
	ld	a, (hl)
	inc	hl
	ld	h, (hl)
;	spillPairReg hl
	ld	l, a
;	spillPairReg hl
;	spillPairReg hl
	push	bc
	ld	bc, #0x000b
	add	hl, bc
	pop	bc
	ld	a, (hl)
	and	a, #0x3f
	ld	(bc), a
;ff.c:2786: fno->fsize = ld_32(dp->dir + DIR_FileSize);		/* Size */
	pop	hl
	push	hl
	ld	c, (hl)
	inc	hl
	ld	b, (hl)
	ld	hl, #0x001c
	add	hl, bc
	push	de
	call	_ld_32
	ld	-4 (ix), e
	ld	-3 (ix), d
	ld	-2 (ix), l
	ld	-1 (ix), h
	pop	de
	push	de
	ld	hl, #7
	add	hl, sp
	ld	bc, #0x0004
	ldir
	pop	de
;ff.c:2787: fno->ftime = ld_16(dp->dir + DIR_ModTime + 0);	/* Last modified time */
	ld	hl, #0x0006
	add	hl, de
	ld	-2 (ix), l
	ld	-1 (ix), h
	pop	hl
	push	hl
	ld	c, (hl)
	inc	hl
	ld	b, (hl)
	ld	hl, #0x0016
	add	hl, bc
	push	de
	call	_ld_16
	ld	c, e
	ld	b, d
	pop	de
	ld	l, -2 (ix)
	ld	h, -1 (ix)
	ld	(hl), c
	inc	hl
	ld	(hl), b
;ff.c:2788: fno->fdate = ld_16(dp->dir + DIR_ModTime + 2);	/* Last Modified date */
	ld	hl, #0x0004
	add	hl, de
	ld	c, l
	ld	b, h
	pop	hl
	push	hl
	ld	e, (hl)
	inc	hl
	ld	d, (hl)
	ld	hl, #0x0018
	add	hl, de
	push	bc
	call	_ld_16
	pop	bc
	ld	a, e
	ld	(bc), a
	inc	bc
	ld	a, d
	ld	(bc), a
00112$:
;ff.c:2793: }
	ld	sp, ix
	pop	ix
	ret
;ff.c:2892: static FRESULT create_name (	/* FR_OK: successful, FR_INVALID_NAME: could not create */
;	---------------------------------
; Function create_name
; ---------------------------------
_create_name:
	push	ix
	ld	ix,#0
	add	ix,sp
	ld	iy, #-13
	add	iy, sp
	ld	sp, iy
	ld	c, l
	ld	b, h
;ff.c:3031: p = *path; sfn = dp->fn;
	ld	-4 (ix), e
	ld	-3 (ix), d
	ex	de,hl
	ld	a, (hl)
	ld	-13 (ix), a
	inc	hl
	ld	a, (hl)
	ld	-12 (ix), a
	ld	hl, #0x001c
	add	hl, bc
	ld	-11 (ix), l
	ld	-10 (ix), h
;ff.c:3032: memset(sfn, ' ', 11);
	pop	de
	pop	hl
	push	hl
	push	de
	ld	b, #0x0b
00218$:
	ld	(hl), #0x20
	inc	hl
	djnz	00218$
;ff.c:3033: si = i = 0; ni = 8;
	xor	a, a
	ld	-9 (ix), a
	ld	-8 (ix), a
	xor	a, a
	ld	-2 (ix), a
	ld	-1 (ix), a
	ld	-7 (ix), #0x08
	ld	-6 (ix), #0
00133$:
;ff.c:3053: c = (BYTE)p[si++];				/* Get a byte */
	ld	c, -2 (ix)
	ld	b, -1 (ix)
	inc	-2 (ix)
	jr	NZ, 00220$
	inc	-1 (ix)
00220$:
	pop	hl
	push	hl
	add	hl, bc
	ld	a, (hl)
	ld	-5 (ix), a
;ff.c:3054: if (c <= ' ') break; 			/* Break if end of the path name */
	ld	a, #0x20
	sub	a, -5 (ix)
	ld	a, #0x00
	rla
	ld	c, a
	bit	0, c
	jp	Z, 00128$
;ff.c:3055: if (IsSeparator(c)) {			/* Break if a separator is found */
	ld	a, -5 (ix)
	sub	a, #0x2f
	jr	Z, 00144$
	ld	a, -5 (ix)
	sub	a, #0x5c
	jr	NZ, 00108$
;ff.c:3056: while (IsSeparator(p[si])) si++;	/* Skip duplicated separators */
00144$:
	ld	e, -2 (ix)
	ld	d, -1 (ix)
00104$:
	pop	hl
	push	hl
	add	hl, de
	ld	a, (hl)
	cp	a, #0x2f
	jr	Z, 00105$
	sub	a, #0x5c
	jp	NZ,00157$
00105$:
	inc	de
	jr	00104$
;ff.c:3057: break;
00108$:
;ff.c:3059: if (c == '.' || i >= ni) {		/* End of body or field overflow? */
	ld	a, -5 (ix)
	sub	a, #0x2e
	ld	a, #0x01
	jr	Z, 00228$
	xor	a, a
00228$:
	ld	c, a
	or	a, a
	jr	NZ, 00113$
	ld	a, -9 (ix)
	sub	a, -7 (ix)
	ld	a, -8 (ix)
	sbc	a, -6 (ix)
	jr	C, 00114$
00113$:
;ff.c:3060: if (ni == 11 || c != '.') return FR_INVALID_NAME;	/* Field overflow or invalid dot? */
	ld	a, -7 (ix)
	sub	a, #0x0b
	or	a, -6 (ix)
	jr	Z, 00110$
	bit	0, c
	jr	NZ, 00111$
00110$:
	ld	a, #0x06
	jp	00134$
00111$:
;ff.c:3061: i = 8; ni = 11;				/* Enter file extension field */
	ld	-9 (ix), #0x08
	ld	-8 (ix), #0
	ld	-7 (ix), #0x0b
	ld	-6 (ix), #0
;ff.c:3062: continue;
	jp	00133$
00114$:
;ff.c:3073: if (dbc_1st(c)) {				/* Check if it is a DBC 1st byte */
	ld	a, -5 (ix)
	call	_dbc_1st
;ff.c:3076: sfn[i++] = c;
	ld	c, -9 (ix)
	ld	b, -8 (ix)
	inc	bc
;ff.c:3073: if (dbc_1st(c)) {				/* Check if it is a DBC 1st byte */
	ld	a, d
	or	a, e
	jr	Z, 00125$
;ff.c:3074: d = (BYTE)p[si++];			/* Get 2nd byte */
	ld	e, -2 (ix)
	ld	d, -1 (ix)
	inc	-2 (ix)
	jr	NZ, 00230$
	inc	-1 (ix)
00230$:
	pop	hl
	push	hl
	add	hl, de
	ld	e, (hl)
;ff.c:3075: if (!dbc_2nd(d) || i >= ni - 1) return FR_INVALID_NAME;	/* Reject invalid DBC */
	push	bc
	push	de
	ld	a, e
	call	_dbc_2nd
	ex	de, hl
	pop	de
	pop	bc
	ld	a, h
	or	a, l
	jr	Z, 00116$
	ld	l, -7 (ix)
	ld	h, -6 (ix)
	dec	hl
	ld	a, -9 (ix)
	sub	a, l
	ld	a, -8 (ix)
	sbc	a, h
	jr	C, 00117$
00116$:
	ld	a, #0x06
	jp	00134$
00117$:
;ff.c:3076: sfn[i++] = c;
	ld	l, -9 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -8 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	a, -11 (ix)
	add	a, l
	ld	l, a
;	spillPairReg hl
;	spillPairReg hl
	ld	a, -10 (ix)
	adc	a, h
	ld	h, a
	ld	a, -5 (ix)
	ld	(hl), a
;ff.c:3077: sfn[i++] = d;
	ld	l, c
;	spillPairReg hl
;	spillPairReg hl
	ld	h, b
;	spillPairReg hl
;	spillPairReg hl
	inc	bc
	ld	-9 (ix), c
	ld	-8 (ix), b
	ld	c, -11 (ix)
	ld	b, -10 (ix)
	add	hl, bc
	ld	(hl), e
	jp	00133$
00125$:
;ff.c:3079: if (strchr("*+,:;<=>[]|\"\?\x7F", (int)c)) return FR_INVALID_NAME;	/* Reject illegal chrs for SFN */
	ld	de, #___str_0
	ld	l, -5 (ix)
;	spillPairReg hl
;	spillPairReg hl
;	spillPairReg hl
;	spillPairReg hl
00232$:
	ld	a, (de)
	cp	a, l
	jr	Z, 00231$
	or	a, a
	inc	de
	jr	NZ, 00232$
	ld	e, a
	ld	d, a
00231$:
	ld	a, e
	ld	e, d
	or	a, e
	jr	Z, 00120$
	ld	a, #0x06
	jp	00134$
00120$:
;ff.c:3080: if (IsLower(c)) c -= 0x20;	/* To upper */
	ld	a, -5 (ix)
	sub	a, #0x61
	jr	C, 00122$
	ld	a, #0x7a
	sub	a, -5 (ix)
	jr	C, 00122$
	ld	a, -5 (ix)
	add	a, #0xe0
	ld	-5 (ix), a
00122$:
;ff.c:3081: sfn[i++] = c;
	ld	e, -9 (ix)
	ld	d, -8 (ix)
	ld	-9 (ix), c
	ld	-8 (ix), b
	ld	l, -11 (ix)
	ld	h, -10 (ix)
	add	hl, de
	ld	a, -5 (ix)
	ld	(hl), a
	jp	00133$
00157$:
	ld	-2 (ix), e
	ld	-1 (ix), d
00128$:
;ff.c:3084: *path = &p[si];						/* Return pointer to the next segment */
	ld	a, -13 (ix)
	add	a, -2 (ix)
	ld	e, a
	ld	a, -12 (ix)
	adc	a, -1 (ix)
	ld	d, a
	ld	l, -4 (ix)
	ld	h, -3 (ix)
	ld	(hl), e
	inc	hl
	ld	(hl), d
;ff.c:3085: if (i == 0) return FR_INVALID_NAME;	/* Reject nul string */
	ld	a, -8 (ix)
	or	a, -9 (ix)
	jr	NZ, 00130$
	ld	a, #0x06
	jr	00134$
00130$:
;ff.c:3087: if (sfn[0] == DDEM) sfn[0] = RDDEM;	/* If the first character collides with DDEM, replace it with RDDEM */
	ld	l, -11 (ix)
	ld	h, -10 (ix)
	ld	a, (hl)
	sub	a, #0xe5
	jr	NZ, 00132$
	ld	l, -11 (ix)
	ld	h, -10 (ix)
	ld	(hl), #0x05
00132$:
;ff.c:3088: sfn[NSFLAG] = (c <= ' ' || p[si] <= ' ') ? NS_LAST : 0;	/* Set last segment flag if end of the path */
	ld	a, -11 (ix)
	add	a, #0x0b
	ld	-2 (ix), a
	ld	a, -10 (ix)
	adc	a, #0x00
	ld	-1 (ix), a
	bit	0, c
	jr	Z, 00139$
	ld	a, (de)
	ld	c, a
	ld	a, #0x20
	sub	a, c
	jr	C, 00136$
00139$:
	ld	bc, #0x0004
	jr	00137$
00136$:
	ld	bc, #0x0000
00137$:
	ld	l, -2 (ix)
	ld	h, -1 (ix)
	ld	(hl), c
;ff.c:3090: return FR_OK;
	xor	a, a
00134$:
;ff.c:3092: }
	ld	sp, ix
	pop	ix
	ret
___str_0:
	.ascii "*+,:;<=>[]|"
	.db 0x22
	.ascii "?"
	.db 0x7f
	.db 0x00
;ff.c:3101: static FRESULT follow_path (	/* FR_OK(0): successful, !=0: error code */
;	---------------------------------
; Function follow_path
; ---------------------------------
_follow_path:
	push	ix
	ld	ix,#0
	add	ix,sp
	ld	iy, #-22
	add	iy, sp
	ld	sp, iy
	ld	-2 (ix), l
	ld	-1 (ix), h
	ld	-4 (ix), e
	ld	-3 (ix), d
;ff.c:3108: FATFS *fs = dp->obj.fs;
	ld	l, -2 (ix)
	ld	h, -1 (ix)
	ld	a, (hl)
	ld	-22 (ix), a
	inc	hl
	ld	a, (hl)
	ld	-21 (ix), a
;ff.c:3118: while (IsSeparator(*path)) path++;	/* Strip heading separators */
	ld	c, -4 (ix)
	ld	b, -3 (ix)
00102$:
	ld	a, (bc)
	cp	a, #0x2f
	jr	Z, 00103$
	sub	a, #0x5c
	jr	NZ, 00141$
00103$:
	inc	bc
	ld	-4 (ix), c
	ld	-3 (ix), b
	jr	00102$
00141$:
	ld	-4 (ix), c
	ld	-3 (ix), b
;ff.c:3119: dp->obj.sclust = 0;					/* Start at the root directory */
	ld	a, -2 (ix)
	add	a, #0x06
	ld	-20 (ix), a
	ld	a, -1 (ix)
	adc	a, #0x00
	ld	-19 (ix), a
	pop	bc
	pop	hl
	push	hl
	push	bc
	xor	a, a
	ld	(hl), a
	inc	hl
	ld	(hl), a
	inc	hl
	ld	(hl), a
	inc	hl
	ld	(hl), a
;ff.c:3142: if ((UINT)*path < ' ') {				/* Null path name is the origin directory itself */
	ld	a, -4 (ix)
	ld	-6 (ix), a
	ld	a, -3 (ix)
	ld	-5 (ix), a
	ld	l, -6 (ix)
	ld	h, -5 (ix)
	ld	a, (hl)
	ld	-5 (ix), a
	ld	-8 (ix), a
	ld	-7 (ix), #0x00
;ff.c:3143: dp->fn[NSFLAG] = NS_NONAME;
	ld	a, -2 (ix)
	add	a, #0x1c
	ld	-6 (ix), a
	ld	a, -1 (ix)
	adc	a, #0x00
	ld	-5 (ix), a
	ld	a, -6 (ix)
	add	a, #0x0b
	ld	-18 (ix), a
	ld	a, -5 (ix)
	adc	a, #0x00
	ld	-17 (ix), a
;ff.c:3142: if ((UINT)*path < ' ') {				/* Null path name is the origin directory itself */
	ld	a, -8 (ix)
	sub	a, #0x20
	ld	a, -7 (ix)
	sbc	a, #0x00
	jr	NC, 00140$
;ff.c:3143: dp->fn[NSFLAG] = NS_NONAME;
	ld	l, -18 (ix)
	ld	h, -17 (ix)
	ld	(hl), #0x80
;ff.c:3144: res = dir_sdi(dp, 0);
	ld	hl, #0x0000
	push	hl
	push	hl
	ld	l, -2 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -1 (ix)
;	spillPairReg hl
;	spillPairReg hl
	call	_dir_sdi
	ld	-5 (ix), a
	jp	00127$
00140$:
	ld	a, -2 (ix)
	ld	-16 (ix), a
	ld	a, -1 (ix)
	ld	-15 (ix), a
	ld	a, -2 (ix)
	ld	-14 (ix), a
	ld	a, -1 (ix)
	ld	-13 (ix), a
00128$:
;ff.c:3148: res = create_name(dp, &path);	/* Get a segment name of the path */
	ld	hl, #18
	add	hl, sp
	ex	de, hl
	ld	l, -2 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -1 (ix)
;	spillPairReg hl
;	spillPairReg hl
	call	_create_name
;ff.c:3149: if (res != FR_OK) break;
	ld	-5 (ix), a
	or	a, a
	jp	NZ, 00127$
;ff.c:3150: ns = dp->fn[NSFLAG];
	ld	l, -18 (ix)
	ld	h, -17 (ix)
	ld	c, (hl)
;ff.c:3171: res = dir_find(dp);				/* Find an object with the segment name */
	push	bc
	ld	l, -2 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -1 (ix)
;	spillPairReg hl
;	spillPairReg hl
	call	_dir_find
	pop	bc
	ld	-5 (ix), a
;ff.c:3179: if (!(ns & NS_LAST)) res = FR_NO_PATH;	/* Adjust error code if not last segment */
	ld	a, c
	and	a, #0x04
	ld	c, a
	ld	b, #0x00
;ff.c:3172: if (res != FR_OK) {				/* Failed to find the object */
	ld	a, -5 (ix)
	or	a, a
	jr	Z, 00118$
;ff.c:3173: if (res == FR_NO_FILE) {	/* Object is not found */
	ld	a, -5 (ix)
	sub	a, #0x04
	jp	NZ,00127$
;ff.c:3179: if (!(ns & NS_LAST)) res = FR_NO_PATH;	/* Adjust error code if not last segment */
	ld	a, b
	or	a, c
	jp	NZ, 00127$
	ld	-5 (ix), #0x05
;ff.c:3182: break;
	jp	00127$
00118$:
;ff.c:3194: if (ns & NS_LAST) break;		/* If last segment matched, the function completed */
	ld	a, b
	or	a, c
	jr	NZ, 00127$
;ff.c:3196: if (!(dp->obj.attr & AM_DIR)) {
	ld	l, -16 (ix)
	ld	h, -15 (ix)
	ld	de, #0x0004
	add	hl, de
	bit	4, (hl)
	jr	NZ, 00122$
;ff.c:3197: res = FR_NO_PATH; break;	/* It is not a sub-directory and cannot follow the path */
	ld	-5 (ix), #0x05
	jr	00127$
00122$:
;ff.c:3205: dp->obj.sclust = ld_clust(fs, fs->win + dp->dptr % SS(fs));	/* Open next directory */
	ld	a, -22 (ix)
	add	a, #0x30
	ld	-12 (ix), a
	ld	a, -21 (ix)
	adc	a, #0x00
	ld	-11 (ix), a
	ld	l, -14 (ix)
	ld	h, -13 (ix)
	ld	de, #0x000e
	add	hl, de
	ld	c, (hl)
	inc	hl
	ld	b, (hl)
	ld	-10 (ix), c
	ld	a, b
	and	a, #0x01
	ld	-9 (ix), a
	xor	a, a
	ld	-8 (ix), a
	ld	-7 (ix), a
	ld	a, -10 (ix)
	add	a, -12 (ix)
	ld	-6 (ix), a
	ld	a, -9 (ix)
	adc	a, -11 (ix)
	ld	-5 (ix), a
	ld	e, -6 (ix)
	ld	d, -5 (ix)
	pop	hl
	push	hl
	call	_ld_clust
	ld	-8 (ix), e
	ld	-7 (ix), d
	ld	-6 (ix), l
	ld	-5 (ix), h
	ld	e, -20 (ix)
	ld	d, -19 (ix)
	ld	hl, #14
	add	hl, sp
	ld	bc, #0x0004
	ldir
	jp	00128$
00127$:
;ff.c:3210: return res;
	ld	a, -5 (ix)
;ff.c:3211: }
	ld	sp, ix
	pop	ix
	ret
;ff.c:3220: static int get_ldnumber (	/* Returns logical drive number (-1:invalid drive number or null pointer) */
;	---------------------------------
; Function get_ldnumber
; ---------------------------------
_get_ldnumber:
	push	ix
	ld	ix,#0
	add	ix,sp
	ld	iy, #-8
	add	iy, sp
	ld	sp, iy
;ff.c:3233: tt = tp = *path;
	ld	-2 (ix), l
	ld	-1 (ix), h
	ld	a, (hl)
	ld	-4 (ix), a
	inc	hl
	ld	a, (hl)
	ld	-3 (ix), a
	ld	a, -4 (ix)
	ld	-8 (ix), a
	ld	a, -3 (ix)
;ff.c:3234: if (!tp) return -1;		/* Invalid path name? */
	ld	-7 (ix), a
	or	a, -8 (ix)
	jr	NZ, 00119$
	ld	de, #0xffff
	jp	00115$
;ff.c:3235: do {					/* Find a colon in the path */
00119$:
	ld	e, -4 (ix)
	ld	d, -3 (ix)
00104$:
;ff.c:3236: chr = *tt++;
	ld	a, (de)
	inc	de
;ff.c:3237: } while (!IsTerminator(chr) && chr != ':');
	ld	l, a
;	spillPairReg hl
;	spillPairReg hl
	ld	h, #0x00
;	spillPairReg hl
;	spillPairReg hl
	sub	a, #0x3a
	ld	a, #0x01
	jr	Z, 00159$
	xor	a, a
00159$:
	ld	c, a
	ld	a, l
	sub	a, #0x21
	ld	a, h
	sbc	a, #0x00
	jr	C, 00125$
	bit	0, c
	jr	Z, 00104$
00125$:
	ld	-6 (ix), e
	ld	-5 (ix), d
;ff.c:3239: if (chr == ':') {	/* Is there a DOS/Windows style volume ID? */
	ld	a, c
	or	a, a
	jr	Z, 00114$
;ff.c:3240: i = FF_VOLUMES;
	ld	-4 (ix), #0x01
	ld	-3 (ix), #0
;ff.c:3241: if (IsDigit(*tp) && tp + 2 == tt) {	/* Is it a numeric volume ID + colon? */
	pop	hl
	push	hl
	ld	c, (hl)
	ld	a, c
	sub	a, #0x30
	jr	C, 00108$
	ld	a, #0x39
	sub	a, c
	jr	C, 00108$
	pop	hl
	push	hl
	inc	hl
	inc	hl
	cp	a, a
	sbc	hl, de
	jr	NZ, 00108$
;ff.c:3242: i = (int)*tp - '0';	/* Get the logical drive number */
	ld	a, c
	ld	c, #0x00
	add	a, #0xd0
	ld	-4 (ix), a
	ld	a, c
	adc	a, #0xff
	ld	-3 (ix), a
00108$:
;ff.c:3257: if (i >= FF_VOLUMES) return -1;	/* Not found or invalid volume ID */
	ld	a, -4 (ix)
	sub	a, #0x01
	ld	a, -3 (ix)
	rla
	ccf
	rra
	sbc	a, #0x80
	jr	C, 00112$
	ld	de, #0xffff
	jr	00115$
00112$:
;ff.c:3258: *path = tt;		/* Snip the drive prefix off */
	ld	l, -2 (ix)
	ld	h, -1 (ix)
	ld	a, -6 (ix)
	ld	(hl), a
	inc	hl
	ld	a, -5 (ix)
	ld	(hl), a
;ff.c:3259: return i;		/* Return the found drive number */
	ld	e, -4 (ix)
	ld	d, -3 (ix)
	jr	00115$
00114$:
;ff.c:3282: return 0;				/* Default drive is 0 */
	ld	de, #0x0000
00115$:
;ff.c:3284: }
	ld	sp, ix
	pop	ix
	ret
;ff.c:3367: static UINT check_fs (	/* 0:FAT/FAT32 VBR, 1:exFAT VBR, 2:Not FAT and valid BS, 3:Not FAT and invalid BS, 4:Disk error */
;	---------------------------------
; Function check_fs
; ---------------------------------
_check_fs:
	push	ix
	ld	ix,#0
	add	ix,sp
	push	af
	dec	sp
;ff.c:3376: fs->wflag = 0; fs->winsect = (LBA_t)0 - 1;		/* Invaidate window */
	ld	-2 (ix), l
	ld	-1 (ix), h
	ld	de, #0x0004
	add	hl, de
	ld	(hl), #0x00
	ld	l, -2 (ix)
	ld	h, -1 (ix)
	ld	de, #0x001c
	add	hl, de
	ld	(hl), #0xff
	inc	hl
	ld	(hl), #0xff
	inc	hl
	ld	(hl), #0xff
	inc	hl
	ld	(hl), #0xff
;ff.c:3377: if (move_window(fs, sect) != FR_OK) return 4;	/* Load the boot sector */
	ld	l, 6 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, 7 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	ld	l, 4 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, 5 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	ld	l, -2 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -1 (ix)
;	spillPairReg hl
;	spillPairReg hl
	call	_move_window
	or	a, a
	jr	Z, 00102$
	ld	de, #0x0004
	jp	00122$
00102$:
;ff.c:3378: sign = ld_16(fs->win + BS_55AA);
	ld	a, -2 (ix)
	add	a, #0x30
	ld	c, a
	ld	a, -1 (ix)
	adc	a, #0x00
	ld	b, a
	ld	hl, #0x01fe
	add	hl, bc
	push	bc
	call	_ld_16
	pop	bc
;ff.c:3382: b = fs->win[BS_JmpBoot];
	ld	a, (bc)
	ld	c, a
;ff.c:3384: if (sign == 0xAA55 && !memcmp(fs->win + BS_FilSysType32, "FAT32   ", 8)) {
	ld	a, e
	sub	a, #0x55
	jr	NZ, 00206$
	ld	a, d
	sub	a, #0xaa
	ld	a, #0x01
	jr	Z, 00207$
00206$:
	xor	a, a
00207$:
	ld	-3 (ix), a
;ff.c:3383: if (b == 0xEB || b == 0xE9 || b == 0xE8) {	/* Valid JumpBoot code? (short jump, near jump or near call) */
	ld	a,c
	cp	a,#0xeb
	jr	Z, 00118$
	cp	a,#0xe9
	jr	Z, 00118$
	sub	a, #0xe8
	jp	NZ,00119$
00118$:
;ff.c:3384: if (sign == 0xAA55 && !memcmp(fs->win + BS_FilSysType32, "FAT32   ", 8)) {
	ld	a, -2 (ix)
	add	a, #0x30
	ld	c, a
	ld	a, -1 (ix)
	adc	a, #0x00
	ld	b, a
	ld	a, -3 (ix)
	or	a, a
	jr	Z, 00104$
	ld	hl, #0x0052
	add	hl, bc
	push	bc
	ld	de, #0x0008
	push	de
	ld	de, #___str_1
	call	_memcmp
	pop	bc
	ld	a, d
	or	a, e
	jr	NZ, 00104$
;ff.c:3385: return 0;	/* It is an FAT32 VBR */
	ld	de, #0x0000
	jp	00122$
00104$:
;ff.c:3388: w = ld_16(fs->win + BPB_BytsPerSec);
	ld	hl, #0x000b
	add	hl, bc
	push	bc
	call	_ld_16
	pop	bc
;ff.c:3389: b = fs->win[BPB_SecPerClus];
	ld	l, c
;	spillPairReg hl
;	spillPairReg hl
	ld	h, b
;	spillPairReg hl
;	spillPairReg hl
	push	bc
	ld	bc, #0x000d
	add	hl, bc
	pop	bc
	ld	h, (hl)
;	spillPairReg hl
;ff.c:3390: if ((w & (w - 1)) == 0 && w >= FF_MIN_SS && w <= FF_MAX_SS	/* Properness of sector size (512-4096 and 2^n) */
	ld	a, e
	add	a, #0xff
	ld	l, a
;	spillPairReg hl
;	spillPairReg hl
	ld	a, d
	adc	a, #0xff
	push	af
	ld	a, l
	and	a, e
	ld	l, a
;	spillPairReg hl
;	spillPairReg hl
	pop	af
	and	a, d
	or	a, l
	jr	NZ, 00119$
	ld	a, d
	sub	a, #0x02
	jr	C, 00119$
;ff.c:3391: && b != 0 && (b & (b - 1)) == 0				/* Properness of cluster size (2^n) */
	xor	a, a
	cp	a, e
	ld	a, #0x02
	sbc	a, d
	jr	C, 00119$
	ld	a, h
	or	a, a
	jr	Z, 00119$
	ld	e, h
	ld	d, #0x00
	ld	l, e
	ld	h, d
	dec	hl
	ld	a, l
	and	a, e
	ld	e, a
	ld	a, h
	and	a, d
	or	a, e
	jr	NZ, 00119$
;ff.c:3392: && ld_16(fs->win + BPB_RsvdSecCnt) != 0		/* Properness of number of reserved sectors (MNBZ) */
	ld	hl, #0x000e
	add	hl, bc
	push	bc
	call	_ld_16
	pop	bc
	ld	a, d
	or	a, e
	jr	Z, 00119$
;ff.c:3393: && (UINT)fs->win[BPB_NumFATs] - 1 <= 1		/* Properness of number of FATs (1 or 2) */
	ld	e, c
	ld	d, b
	ld	hl, #16
	add	hl, de
	ld	e, (hl)
	ld	d, #0x00
	dec	de
	ld	a, #0x01
	cp	a, e
	ld	a, #0x00
	sbc	a, d
	jr	C, 00119$
;ff.c:3394: && ld_16(fs->win + BPB_RootEntCnt) != 0		/* Properness of root dir size (MNBZ) */
	ld	hl, #0x0011
	add	hl, bc
	push	bc
	call	_ld_16
	pop	bc
	ld	a, d
	or	a, e
	jr	Z, 00119$
;ff.c:3395: && (ld_16(fs->win + BPB_TotSec16) >= 128 || ld_32(fs->win + BPB_TotSec32) >= 0x10000)	/* Properness of volume size (>=128) */
	ld	hl, #0x0013
	add	hl, bc
	push	bc
	call	_ld_16
	pop	bc
	ld	a, e
	sub	a, #0x80
	ld	a, d
	sbc	a, #0x00
	jr	NC, 00117$
	ld	hl, #0x0020
	add	hl, bc
	push	bc
	call	_ld_32
	pop	bc
	ld	de, #0x0001
	cp	a, a
	sbc	hl, de
	jr	C, 00119$
00117$:
;ff.c:3396: && ld_16(fs->win + BPB_FATSz16) != 0) {		/* Properness of FAT size (MNBZ) */
	ld	hl, #0x0016
	add	hl, bc
	call	_ld_16
	ld	a, d
	or	a, e
	jr	Z, 00119$
;ff.c:3397: return 0;	/* It can be presumed an FAT VBR */
	ld	de, #0x0000
	jr	00122$
00119$:
;ff.c:3400: return sign == 0xAA55 ? 2 : 3;	/* Not an FAT VBR (with valid or invalid BS) */
	ld	a, -3 (ix)
	or	a, a
	jr	Z, 00124$
	ld	de, #0x0002
	jr	00125$
00124$:
	ld	de, #0x0003
00125$:
00122$:
;ff.c:3401: }
	ld	sp, ix
	pop	ix
	pop	hl
	pop	af
	pop	af
	jp	(hl)
___str_1:
	.ascii "FAT32   "
	.db 0x00
;ff.c:3407: static UINT find_volume (	/* Returns BS status found in the hosting drive */
;	---------------------------------
; Function find_volume
; ---------------------------------
_find_volume:
	push	ix
	ld	ix,#0
	add	ix,sp
	ld	iy, #-28
	add	iy, sp
	ld	sp, iy
	ld	-4 (ix), l
	ld	-3 (ix), h
	ld	-6 (ix), e
	ld	-5 (ix), d
;ff.c:3416: fmt = check_fs(fs, 0);				/* Load sector 0 and check if it is an FAT VBR as SFD format */
	ld	hl, #0x0000
	push	hl
	push	hl
	ld	l, -4 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -3 (ix)
;	spillPairReg hl
;	spillPairReg hl
	call	_check_fs
;ff.c:3417: if (fmt != 2 && (fmt >= 3 || part == 0)) return fmt;	/* Returns if it is an FAT VBR as auto scan, not a BS or disk error */
	ld	a, e
	sub	a, #0x02
	or	a, d
	jr	Z, 00124$
	ld	a, e
	sub	a, #0x03
	ld	a, d
	sbc	a, #0x00
	jp	NC,00116$
	ld	a, -5 (ix)
	or	a, -6 (ix)
;ff.c:3444: for (i = 0; i < 4; i++) {		/* Load partition offset in the MBR */
	jp	Z,00116$
00124$:
	ld	a, -4 (ix)
	add	a, #0x30
	ld	-2 (ix), a
	ld	a, -3 (ix)
	adc	a, #0x00
	ld	-1 (ix), a
	ld	bc, #0x0000
00114$:
;ff.c:3445: mbr_pt[i] = ld_32(fs->win + MBR_Table + i * SZ_PTE + PTE_StLba);
	ld	l, c
;	spillPairReg hl
;	spillPairReg hl
	ld	h, b
;	spillPairReg hl
;	spillPairReg hl
	add	hl, hl
	add	hl, hl
	ex	de,hl
	push	de
	ld	hl, #6
	add	hl, sp
	add	hl, de
	pop	de
	ex	de, hl
	ld	l, c
	ld	h, b
	add	hl, hl
	add	hl, hl
	add	hl, hl
	add	hl, hl
	push	de
	ld	de, #0x01c6
	add	hl, de
	pop	de
	ld	a, l
	add	a, -2 (ix)
	ld	l, a
;	spillPairReg hl
;	spillPairReg hl
	ld	a, h
	adc	a, -1 (ix)
	ld	h, a
;	spillPairReg hl
;	spillPairReg hl
	push	bc
	push	de
	call	_ld_32
	ld	-28 (ix), e
	ld	-27 (ix), d
	ld	-26 (ix), l
	ld	-25 (ix), h
	pop	de
	ld	hl, #2
	add	hl, sp
	ld	bc, #0x0004
	ldir
	pop	bc
;ff.c:3444: for (i = 0; i < 4; i++) {		/* Load partition offset in the MBR */
	inc	bc
	ld	a, c
	sub	a, #0x04
	ld	a, b
	sbc	a, #0x00
	jr	C, 00114$
;ff.c:3447: i = part ? part - 1 : 0;		/* Table index to find first */
	ld	a, -5 (ix)
	or	a, -6 (ix)
	jr	Z, 00118$
	ld	a, -6 (ix)
	add	a, #0xff
	ld	-2 (ix), a
	ld	a, -5 (ix)
	adc	a, #0xff
	ld	-1 (ix), a
	jr	00119$
00118$:
	xor	a, a
	ld	-2 (ix), a
	ld	-1 (ix), a
00119$:
;ff.c:3448: do {							/* Find an FAT volume */
00111$:
;ff.c:3449: fmt = mbr_pt[i] ? check_fs(fs, mbr_pt[i]) : 3;	/* Check if the partition is FAT */
	ld	l, -2 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -1 (ix)
;	spillPairReg hl
;	spillPairReg hl
	add	hl, hl
	add	hl, hl
	ex	de,hl
	ld	hl, #4
	add	hl, sp
	add	hl, de
	ex	de, hl
	ld	hl, #0
	add	hl, sp
	ex	de, hl
	ld	bc, #0x0004
	ldir
	ld	a, -25 (ix)
	or	a, -26 (ix)
	or	a, -27 (ix)
	or	a, -28 (ix)
	jr	Z, 00120$
	pop	de
	pop	hl
	ex	de, hl
	push	de
	push	hl
	push	de
	push	hl
	ld	l, -4 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -3 (ix)
;	spillPairReg hl
;	spillPairReg hl
	call	_check_fs
	ld	-8 (ix), e
	ld	-7 (ix), d
	jr	00121$
00120$:
	ld	-8 (ix), #0x03
	ld	-7 (ix), #0
00121$:
	ld	c, -8 (ix)
	ld	b, -7 (ix)
;ff.c:3450: } while (part == 0 && fmt >= 2 && ++i < 4);
	ld	a, -5 (ix)
	or	a, -6 (ix)
	jr	NZ, 00113$
	ld	a, c
	sub	a, #0x02
	ld	a, b
	sbc	a, #0x00
	jr	C, 00113$
	inc	-2 (ix)
	jr	NZ, 00170$
	inc	-1 (ix)
00170$:
	ld	a, -2 (ix)
	sub	a, #0x04
	ld	a, -1 (ix)
	sbc	a, #0x00
	jr	C, 00111$
00113$:
;ff.c:3451: return fmt;
	ld	e, c
	ld	d, b
00116$:
;ff.c:3452: }
	ld	sp, ix
	pop	ix
	ret
;ff.c:3461: static FRESULT mount_volume (	/* FR_OK(0): successful, !=0: an error occurred */
;	---------------------------------
; Function mount_volume
; ---------------------------------
_mount_volume:
	push	ix
	ld	ix,#0
	add	ix,sp
	ld	iy, #-38
	add	iy, sp
	ld	sp, iy
	ld	c, l
	ld	b, h
;ff.c:3475: *rfs = 0;
	ld	-2 (ix), e
	ld	-1 (ix), d
	ex	de,hl
	xor	a, a
	ld	(hl), a
	inc	hl
	ld	(hl), a
;ff.c:3476: vol = get_ldnumber(path);
	ld	l, c
;	spillPairReg hl
;	spillPairReg hl
	ld	h, b
;	spillPairReg hl
;	spillPairReg hl
	call	_get_ldnumber
	ld	-4 (ix), e
	ld	-3 (ix), d
;ff.c:3477: if (vol < 0) return FR_INVALID_DRIVE;
	bit	7, -3 (ix)
	jr	Z, 00102$
	ld	a, #0x0b
	jp	00170$
00102$:
;ff.c:3480: fs = FatFs[vol];					/* Get pointer to the filesystem object */
	ld	bc, #_FatFs+0
	ld	l, -4 (ix)
	ld	h, -3 (ix)
	add	hl, hl
	add	hl, bc
	ld	a, (hl)
	ld	-34 (ix), a
	inc	hl
	ld	a, (hl)
;ff.c:3481: if (!fs) return FR_NOT_ENABLED;		/* Is the filesystem object available? */
	ld	-33 (ix), a
	or	a, -34 (ix)
	jr	NZ, 00104$
	ld	a, #0x0c
	jp	00170$
00104$:
;ff.c:3485: *rfs = fs;							/* Return pointer to the filesystem object */
	ld	l, -2 (ix)
	ld	h, -1 (ix)
	ld	a, -34 (ix)
	ld	(hl), a
	inc	hl
	ld	a, -33 (ix)
	ld	(hl), a
;ff.c:3487: mode &= (BYTE)~FA_READ;				/* Desired access mode, write access or not */
	ld	a, 4 (ix)
	and	a, #0xfe
	ld	4 (ix), a
;ff.c:3488: if (fs->fs_type != 0) {				/* If the volume has been mounted */
	ld	l, -34 (ix)
	ld	h, -33 (ix)
	ld	a, (hl)
;ff.c:3489: stat = disk_status(fs->pdrv);
	ld	c, -34 (ix)
	ld	b, -33 (ix)
	inc	bc
;ff.c:3488: if (fs->fs_type != 0) {				/* If the volume has been mounted */
	or	a, a
	jr	Z, 00112$
;ff.c:3489: stat = disk_status(fs->pdrv);
	ld	a, (bc)
	ld	e, a
	push	bc
	ld	a, e
	call	_disk_status
	pop	bc
	ld	e, a
;ff.c:3490: if (!(stat & STA_NOINIT)) {		/* and the physical drive is kept initialized */
	bit	0, e
	jr	NZ, 00112$
;ff.c:3491: if (!FF_FS_READONLY && mode && (stat & STA_PROTECT)) {	/* Check write protection if needed */
	ld	a, 4 (ix)
	or	a, a
	jr	Z, 00106$
	bit	2, e
	jr	Z, 00106$
;ff.c:3492: return FR_WRITE_PROTECTED;
	ld	a, #0x0a
	jp	00170$
00106$:
;ff.c:3494: return FR_OK;				/* The filesystem object is already valid */
	xor	a, a
	jp	00170$
00112$:
;ff.c:3501: fs->fs_type = 0;					/* Invalidate the filesystem object */
	ld	l, -34 (ix)
	ld	h, -33 (ix)
	ld	(hl), #0x00
;ff.c:3502: stat = disk_initialize(fs->pdrv);	/* Initialize the volume hosting physical drive */
	ld	a, (bc)
	call	_disk_initialize
	ld	c, a
;ff.c:3503: if (stat & STA_NOINIT) { 			/* Check if the initialization succeeded */
	bit	0, c
	jr	Z, 00117$
;ff.c:3504: return FR_NOT_READY;			/* Failed to initialize due to no medium or hard error */
	ld	a, #0x03
	jp	00170$
;ff.c:3506: if (!FF_FS_READONLY && mode && (stat & STA_PROTECT)) { /* Check disk write protection if needed */
00117$:
	ld	a, 4 (ix)
	or	a, a
	jr	Z, 00116$
	bit	2, c
	jr	Z, 00116$
;ff.c:3507: return FR_WRITE_PROTECTED;
	ld	a, #0x0a
	jp	00170$
00116$:
;ff.c:3515: fmt = find_volume(fs, LD2PT(vol));
	ld	de, #0x0000
	ld	l, -34 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -33 (ix)
;	spillPairReg hl
;	spillPairReg hl
	call	_find_volume
;ff.c:3516: if (fmt == 4) return FR_DISK_ERR;		/* An error occurred in the disk I/O layer */
	ld	a, e
	sub	a, #0x04
	or	a, d
	jr	NZ, 00120$
	ld	a, #0x01
	jp	00170$
00120$:
;ff.c:3517: if (fmt >= 2) return FR_NO_FILESYSTEM;	/* No FAT volume is found */
	ld	a, e
	sub	a, #0x02
	ld	a, d
	sbc	a, #0x00
	jr	C, 00122$
	ld	a, #0x0d
	jp	00170$
00122$:
;ff.c:3518: bsect = fs->winsect;					/* Volume offset in the hosting physical drive */
	ld	e, -34 (ix)
	ld	d, -33 (ix)
	ld	hl, #6
	add	hl, sp
	ex	de, hl
	ld	bc, #0x001c
	add	hl, bc
	ld	bc, #0x0004
	ldir
;ff.c:3589: if (ld_16(fs->win + BPB_BytsPerSec) != SS(fs)) return FR_NO_FILESYSTEM;	/* (BPB_BytsPerSec must be equal to the physical sector size) */
	ld	a, -34 (ix)
	add	a, #0x30
	ld	-28 (ix), a
	ld	a, -33 (ix)
	adc	a, #0x00
	ld	-27 (ix), a
	ld	l, -28 (ix)
	ld	h, -27 (ix)
	ld	de, #0x000b
	add	hl, de
	call	_ld_16
	ld	a, e
	or	a, a
	jr	NZ, 00366$
	ld	a, d
	sub	a, #0x02
	jr	Z, 00124$
00366$:
	ld	a, #0x0d
	jp	00170$
00124$:
;ff.c:3591: fasize = ld_16(fs->win + BPB_FATSz16);		/* Number of sectors per FAT */
	ld	l, -28 (ix)
	ld	h, -27 (ix)
	ld	de, #0x0016
	add	hl, de
	call	_ld_16
	ld	bc, #0x0000
;ff.c:3592: if (fasize == 0) fasize = ld_32(fs->win + BPB_FATSz32);
	ld	a, b
	or	a, c
	or	a, d
	or	a, e
	jr	NZ, 00126$
	ld	l, -28 (ix)
	ld	h, -27 (ix)
	ld	de, #0x0024
	add	hl, de
	call	_ld_32
	ld	c, l
	ld	b, h
00126$:
;ff.c:3593: fs->fsize = fasize;
	ld	a, -34 (ix)
	add	a, #0x18
	ld	-26 (ix), a
	ld	a, -33 (ix)
	adc	a, #0x00
	ld	-25 (ix), a
	ld	l, -26 (ix)
	ld	h, -25 (ix)
	ld	(hl), e
	inc	hl
	ld	(hl), d
	inc	hl
	ld	(hl), c
	inc	hl
	ld	(hl), b
;ff.c:3595: fs->n_fats = fs->win[BPB_NumFATs];				/* Number of FATs */
	ld	a, -34 (ix)
	add	a, #0x03
	ld	-4 (ix), a
	ld	a, -33 (ix)
	adc	a, #0x00
	ld	-3 (ix), a
	ld	l, -28 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -27 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	bc
	ld	bc, #0x0010
	add	hl, bc
	pop	bc
	ld	a, (hl)
	ld	l, -4 (ix)
	ld	h, -3 (ix)
	ld	(hl), a
;ff.c:3596: if (fs->n_fats != 1 && fs->n_fats != 2) return FR_NO_FILESYSTEM;	/* (Must be 1 or 2) */
	ld	l, -4 (ix)
	ld	h, -3 (ix)
	ld	l, (hl)
;	spillPairReg hl
	dec	a
	jr	Z, 00128$
	ld	a, l
	sub	a, #0x02
	jr	Z, 00128$
	ld	a, #0x0d
	jp	00170$
00128$:
;ff.c:3597: fasize *= fs->n_fats;							/* Number of sectors for FAT area */
	ld	h, #0x00
;	spillPairReg hl
;	spillPairReg hl
	ld	iy, #0x0000
	push	iy
	push	hl
	ld	l, c
;	spillPairReg hl
;	spillPairReg hl
	ld	h, b
;	spillPairReg hl
;	spillPairReg hl
	call	__mullong
	pop	af
	pop	af
	ld	-24 (ix), e
	ld	-23 (ix), d
	ld	-22 (ix), l
	ld	-21 (ix), h
;ff.c:3599: fs->csize = fs->win[BPB_SecPerClus];			/* Cluster size */
	ld	a, -34 (ix)
	add	a, #0x0a
	ld	-4 (ix), a
	ld	a, -33 (ix)
	adc	a, #0x00
	ld	-3 (ix), a
	ld	c, -28 (ix)
	ld	b, -27 (ix)
	ld	hl, #13
	add	hl, bc
	ld	e, (hl)
	ld	d, #0x00
	ld	l, -4 (ix)
	ld	h, -3 (ix)
	ld	(hl), e
	inc	hl
	ld	(hl), d
;ff.c:3600: if (fs->csize == 0 || (fs->csize & (fs->csize - 1))) return FR_NO_FILESYSTEM;	/* (Must be power of 2) */
	ld	l, -4 (ix)
	ld	h, -3 (ix)
	ld	c, (hl)
	inc	hl
	ld	b, (hl)
	ld	a, d
	or	a, e
	jr	Z, 00130$
	ld	a, c
	ld	e, a
	ld	d, b
	dec	de
	and	a, e
	ld	c, a
	ld	a, b
	and	a, d
	or	a, c
	jr	Z, 00131$
00130$:
	ld	a, #0x0d
	jp	00170$
00131$:
;ff.c:3602: fs->n_rootdir = ld_16(fs->win + BPB_RootEntCnt);	/* Number of root directory entries */
	ld	a, -34 (ix)
	add	a, #0x08
	ld	-20 (ix), a
	ld	a, -33 (ix)
	adc	a, #0x00
	ld	-19 (ix), a
	ld	l, -28 (ix)
	ld	h, -27 (ix)
	ld	de, #0x0011
	add	hl, de
	call	_ld_16
	ld	l, -20 (ix)
	ld	h, -19 (ix)
	ld	(hl), e
	inc	hl
	ld	(hl), d
;ff.c:3603: if (fs->n_rootdir % (SS(fs) / SZDIRE)) return FR_NO_FILESYSTEM;	/* (Must be sector aligned) */
	ld	a, e
	and	a, #0x0f
	jr	Z, 00134$
	ld	a, #0x0d
	jp	00170$
00134$:
;ff.c:3605: tsect = ld_16(fs->win + BPB_TotSec16);			/* Number of sectors on the volume */
	ld	l, -28 (ix)
	ld	h, -27 (ix)
	ld	de, #0x0013
	add	hl, de
	call	_ld_16
	ld	bc, #0x0000
;ff.c:3606: if (tsect == 0) tsect = ld_32(fs->win + BPB_TotSec32);
	ld	a, b
	or	a, c
	or	a, d
	or	a, e
	jr	NZ, 00136$
	ld	l, -28 (ix)
	ld	h, -27 (ix)
	ld	de, #0x0020
	add	hl, de
	call	_ld_32
	ld	c, l
	ld	b, h
00136$:
;ff.c:3608: nrsv = ld_16(fs->win + BPB_RsvdSecCnt);			/* Number of reserved sectors */
	ld	a, -28 (ix)
	add	a, #0x0e
	ld	l, a
;	spillPairReg hl
;	spillPairReg hl
	ld	a, -27 (ix)
	adc	a, #0x00
	ld	h, a
;	spillPairReg hl
;	spillPairReg hl
	push	bc
	push	de
	call	_ld_16
	ex	de, hl
	pop	de
	pop	bc
	ld	-6 (ix), l
	ld	-5 (ix), h
;ff.c:3609: if (nrsv == 0) return FR_NO_FILESYSTEM;			/* (Must not be 0) */
	ld	a, h
	or	a, l
	jr	NZ, 00138$
	ld	a, #0x0d
	jp	00170$
00138$:
;ff.c:3612: sysect = nrsv + fasize + fs->n_rootdir / (SS(fs) / SZDIRE);	/* RSV + FAT + DIR */
	ld	a, -6 (ix)
	ld	-18 (ix), a
	ld	a, -5 (ix)
	ld	-17 (ix), a
	xor	a, a
	ld	-16 (ix), a
	ld	-15 (ix), a
	ld	a, -18 (ix)
	add	a, -24 (ix)
	ld	-8 (ix), a
	ld	a, -17 (ix)
	adc	a, -23 (ix)
	ld	-7 (ix), a
	ld	a, -16 (ix)
	adc	a, -22 (ix)
	ld	-6 (ix), a
	ld	a, -15 (ix)
	adc	a, -21 (ix)
	ld	-5 (ix), a
	ld	l, -20 (ix)
	ld	h, -19 (ix)
	ld	a, (hl)
	inc	hl
	ld	h, (hl)
;	spillPairReg hl
	ld	l, a
;	spillPairReg hl
;	spillPairReg hl
	srl	h
	rr	l
	srl	h
	rr	l
	srl	h
	rr	l
	srl	h
	rr	l
	ld	a, l
	ld	iy, #0x0000
	add	a, -8 (ix)
	ld	l, a
;	spillPairReg hl
;	spillPairReg hl
	ld	a, h
	adc	a, -7 (ix)
	ld	h, a
;	spillPairReg hl
;	spillPairReg hl
	push	iy
	ld	a, -40 (ix)
	pop	iy
	adc	a, -6 (ix)
	push	iy
	ld	-40 (ix), a
	ld	a, -39 (ix)
	pop	iy
	adc	a, -5 (ix)
	ld	-14 (ix), l
	ld	-13 (ix), h
	push	iy
	ex	(sp), hl
	ld	-12 (ix), l
	ex	(sp), hl
	pop	iy
	ld	-11 (ix), a
;ff.c:3613: if (tsect < sysect) return FR_NO_FILESYSTEM;	/* (Invalid volume size) */
	ld	a, e
	sub	a, -14 (ix)
	ld	a, d
	sbc	a, -13 (ix)
	ld	a, c
	sbc	a, -12 (ix)
	ld	a, b
	sbc	a, -11 (ix)
	jr	NC, 00140$
	ld	a, #0x0d
	jp	00170$
00140$:
;ff.c:3614: nclst = (tsect - sysect) / fs->csize;			/* Number of clusters */
	ld	a, e
	sub	a, -14 (ix)
	ld	e, a
	ld	a, d
	sbc	a, -13 (ix)
	ld	d, a
	ld	a, c
	sbc	a, -12 (ix)
	ld	c, a
	ld	a, b
	sbc	a, -11 (ix)
	ld	b, a
	ld	l, -4 (ix)
	ld	h, -3 (ix)
	ld	a, (hl)
	inc	hl
	ld	h, (hl)
;	spillPairReg hl
	ld	l, a
;	spillPairReg hl
;	spillPairReg hl
	ld	iy, #0x0000
	push	iy
	push	hl
	ld	l, c
;	spillPairReg hl
;	spillPairReg hl
	ld	h, b
;	spillPairReg hl
;	spillPairReg hl
	call	__divulong
	pop	af
	pop	af
	inc	sp
	inc	sp
	push	de
	ld	-36 (ix), l
;ff.c:3615: if (nclst == 0) return FR_NO_FILESYSTEM;		/* (Invalid volume size) */
	ld	-35 (ix), h
	ld	a, h
	or	a, -36 (ix)
	or	a, -37 (ix)
	or	a, -38 (ix)
	jr	NZ, 00142$
	ld	a, #0x0d
	jp	00170$
00142$:
;ff.c:3616: fmt = 0;
	xor	a, a
	ld	-10 (ix), a
	ld	-9 (ix), a
;ff.c:3617: if (nclst <= MAX_FAT32) fmt = FS_FAT32;
	ld	a, #0xf5
	cp	a, -38 (ix)
	ld	a, #0xff
	sbc	a, -37 (ix)
	ld	a, #0xff
	sbc	a, -36 (ix)
	ld	a, #0x0f
	sbc	a, -35 (ix)
	jr	C, 00144$
	ld	-10 (ix), #0x03
	ld	-9 (ix), #0
00144$:
;ff.c:3618: if (nclst <= MAX_FAT16) fmt = FS_FAT16;
	ld	a, #0xf5
	cp	a, -38 (ix)
	ld	a, #0xff
	sbc	a, -37 (ix)
	ld	a, #0x00
	sbc	a, -36 (ix)
	ld	a, #0x00
	sbc	a, -35 (ix)
	jr	C, 00146$
	ld	-10 (ix), #0x02
	ld	-9 (ix), #0
00146$:
;ff.c:3619: if (nclst <= MAX_FAT12) fmt = FS_FAT12;
	ld	a, #0xf5
	cp	a, -38 (ix)
	ld	a, #0x0f
	sbc	a, -37 (ix)
	ld	a, #0x00
	sbc	a, -36 (ix)
	ld	a, #0x00
	sbc	a, -35 (ix)
	jr	C, 00148$
	ld	-10 (ix), #0x01
	ld	-9 (ix), #0
00148$:
;ff.c:3620: if (fmt == 0) return FR_NO_FILESYSTEM;
	ld	a, -9 (ix)
	or	a, -10 (ix)
	jr	NZ, 00150$
	ld	a, #0x0d
	jp	00170$
00150$:
;ff.c:3623: fs->n_fatent = nclst + 2;						/* Number of FAT entries */
	ld	a, -34 (ix)
	add	a, #0x14
	ld	c, a
	ld	a, -33 (ix)
	adc	a, #0x00
	ld	b, a
	ld	a, -38 (ix)
	add	a, #0x02
	ld	-6 (ix), a
	ld	a, -37 (ix)
	adc	a, #0x00
	ld	-5 (ix), a
	ld	a, -36 (ix)
	adc	a, #0x00
	ld	-4 (ix), a
	ld	a, -35 (ix)
	adc	a, #0x00
	ld	-3 (ix), a
	push	bc
	ld	e, c
	ld	d, b
	ld	hl, #34
	add	hl, sp
	ld	bc, #0x0004
	ldir
	pop	bc
;ff.c:3624: fs->volbase = bsect;							/* Volume start sector */
	ld	a, -34 (ix)
	add	a, #0x20
	ld	e, a
	ld	a, -33 (ix)
	adc	a, #0x00
	ld	d, a
	push	bc
	ld	hl, #8
	add	hl, sp
	ld	bc, #0x0004
	ldir
	pop	bc
;ff.c:3625: fs->fatbase = bsect + nrsv; 					/* FAT start sector */
	ld	a, -34 (ix)
	add	a, #0x24
	ld	-8 (ix), a
	ld	a, -33 (ix)
	adc	a, #0x00
	ld	-7 (ix), a
	ld	a, -18 (ix)
	add	a, -32 (ix)
	ld	-6 (ix), a
	ld	a, -17 (ix)
	adc	a, -31 (ix)
	ld	-5 (ix), a
	ld	a, -16 (ix)
	adc	a, -30 (ix)
	ld	-4 (ix), a
	ld	a, -15 (ix)
	adc	a, -29 (ix)
	ld	-3 (ix), a
	push	bc
	ld	e, -8 (ix)
	ld	d, -7 (ix)
	ld	hl, #34
	add	hl, sp
	ld	bc, #0x0004
	ldir
	pop	bc
;ff.c:3626: fs->database = bsect + sysect;					/* Data start sector */
	ld	a, -34 (ix)
	add	a, #0x2c
	ld	e, a
	ld	a, -33 (ix)
	adc	a, #0x00
	ld	d, a
	ld	a, -14 (ix)
	add	a, -32 (ix)
	ld	-6 (ix), a
	ld	a, -13 (ix)
	adc	a, -31 (ix)
	ld	-5 (ix), a
	ld	a, -12 (ix)
	adc	a, -30 (ix)
	ld	-4 (ix), a
	ld	a, -11 (ix)
	adc	a, -29 (ix)
	ld	-3 (ix), a
	push	bc
	ld	hl, #34
	add	hl, sp
	ld	bc, #0x0004
	ldir
	pop	bc
;ff.c:3627: if (fmt == FS_FAT32) {
	ld	a, -10 (ix)
	sub	a, #0x03
	or	a, -9 (ix)
	ld	a, #0x01
	jr	Z, 00371$
	xor	a, a
00371$:
	ld	-3 (ix), a
;ff.c:3630: fs->dirbase = ld_32(fs->win + BPB_RootClus32);	/* Root directory start cluster */
	ld	a, -34 (ix)
	add	a, #0x28
	ld	e, a
	ld	a, -33 (ix)
	adc	a, #0x00
	ld	d, a
;ff.c:3627: if (fmt == FS_FAT32) {
	ld	a, -3 (ix)
	or	a, a
	jr	Z, 00158$
;ff.c:3628: if (ld_16(fs->win + BPB_FSVer32) != 0) return FR_NO_FILESYSTEM;	/* (Must be FAT32 revision 0.0) */
	ld	a, -28 (ix)
	add	a, #0x2a
	ld	l, a
;	spillPairReg hl
;	spillPairReg hl
	ld	a, -27 (ix)
	adc	a, #0x00
	ld	h, a
;	spillPairReg hl
;	spillPairReg hl
	push	bc
	push	de
	call	_ld_16
	ex	de, hl
	pop	de
	pop	bc
	ld	a, h
	or	a, l
	jr	Z, 00152$
	ld	a, #0x0d
	jp	00170$
00152$:
;ff.c:3629: if (fs->n_rootdir != 0) return FR_NO_FILESYSTEM;	/* (BPB_RootEntCnt must be 0) */
	ld	l, -20 (ix)
	ld	h, -19 (ix)
	inc	hl
	ld	a, (hl)
	dec	hl
	ld	l, (hl)
;	spillPairReg hl
	or	a, l
	jr	Z, 00154$
	ld	a, #0x0d
	jp	00170$
00154$:
;ff.c:3630: fs->dirbase = ld_32(fs->win + BPB_RootClus32);	/* Root directory start cluster */
	ld	a, -28 (ix)
	add	a, #0x2c
	ld	l, a
;	spillPairReg hl
;	spillPairReg hl
	ld	a, -27 (ix)
	adc	a, #0x00
	ld	h, a
;	spillPairReg hl
;	spillPairReg hl
	push	bc
	push	de
	call	_ld_32
	ld	-7 (ix), e
	ld	-6 (ix), d
	ld	-5 (ix), l
	ld	-4 (ix), h
	pop	de
	ld	hl, #33
	add	hl, sp
	ld	bc, #0x0004
	ldir
;ff.c:3631: szbfat = fs->n_fatent * 4;					/* (Needed FAT size) */
	pop	hl
	ld	c, (hl)
	inc	hl
	ld	b, (hl)
	inc	hl
	ld	e, (hl)
	inc	hl
	ld	d, (hl)
	ld	a, #0x02
00372$:
	sla	c
	rl	b
	rl	e
	rl	d
	dec	a
	jr	NZ,00372$
	jp	00159$
00158$:
;ff.c:3633: if (fs->n_rootdir == 0)	return FR_NO_FILESYSTEM;	/* (BPB_RootEntCnt must not be 0) */
	ld	l, -20 (ix)
	ld	h, -19 (ix)
	inc	hl
	ld	a, (hl)
	dec	hl
	ld	l, (hl)
;	spillPairReg hl
	or	a, l
	jr	NZ, 00156$
	ld	a, #0x0d
	jp	00170$
00156$:
;ff.c:3634: fs->dirbase = fs->fatbase + fasize;			/* Root directory start sector */
	push	de
	push	bc
	ld	e, -8 (ix)
	ld	d, -7 (ix)
	ld	hl, #28
	add	hl, sp
	ex	de, hl
	ld	bc, #0x0004
	ldir
	pop	bc
	pop	de
	ld	a, -14 (ix)
	add	a, -24 (ix)
	ld	-7 (ix), a
	ld	a, -13 (ix)
	adc	a, -23 (ix)
	ld	-6 (ix), a
	ld	a, -12 (ix)
	adc	a, -22 (ix)
	ld	-5 (ix), a
	ld	a, -11 (ix)
	adc	a, -21 (ix)
	ld	-4 (ix), a
	push	bc
	ld	hl, #33
	add	hl, sp
	ld	bc, #0x0004
	ldir
;ff.c:3631: szbfat = fs->n_fatent * 4;					/* (Needed FAT size) */
	pop	de
	ld	hl, #24
	add	hl, sp
	ex	de, hl
	ld	bc, #0x0004
	ldir
;ff.c:3635: szbfat = (fmt == FS_FAT16) ?				/* (Needed FAT size) */
	ld	a, -10 (ix)
	sub	a, #0x02
	or	a, -9 (ix)
	jr	NZ, 00172$
;ff.c:3636: fs->n_fatent * 2 : fs->n_fatent * 3 / 2 + (fs->n_fatent & 1);
	ld	l, -14 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -13 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	c, -12 (ix)
	ld	b, -11 (ix)
	add	hl, hl
	rl	c
	rl	b
	jr	00173$
00172$:
	ld	l, -12 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -11 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	ld	l, -14 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -13 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	ld	de, #0x0003
	ld	hl, #0x0000
	call	__mullong
	pop	af
	pop	af
	ld	c, l
	ld	b, h
	srl	b
	rr	c
	rr	d
	rr	e
	ld	a, -14 (ix)
	and	a, #0x01
	ld	-7 (ix), a
	xor	a, a
	ld	-6 (ix), a
	ld	-5 (ix), a
	ld	-4 (ix), a
	ld	a, e
	add	a, -7 (ix)
	ld	l, a
;	spillPairReg hl
;	spillPairReg hl
	ld	a, d
	adc	a, -6 (ix)
	ld	h, a
;	spillPairReg hl
;	spillPairReg hl
	ld	a, c
	adc	a, -5 (ix)
	ld	c, a
	ld	a, b
	adc	a, -4 (ix)
	ld	b, a
00173$:
	ld	e, c
	ld	c, l
	ld	d, b
	ld	b, h
00159$:
;ff.c:3638: if (fs->fsize < (szbfat + (SS(fs) - 1)) / SS(fs)) return FR_NO_FILESYSTEM;	/* (BPB_FATSz must not be less than the size needed) */
	push	de
	push	bc
	ld	e, -26 (ix)
	ld	d, -25 (ix)
	ld	hl, #35
	add	hl, sp
	ex	de, hl
	ld	bc, #0x0004
	ldir
	pop	bc
	pop	de
	ld	a, c
	add	a, #0xff
	ld	a, b
	adc	a, #0x01
	ld	b, a
	jr	NC, 00380$
	inc	de
00380$:
	ld	c, b
	ld	b, e
	ld	e, d
	ld	d, #0x00
	srl	e
	rr	b
	rr	c
	ld	a, -7 (ix)
	sub	a, c
	ld	a, -6 (ix)
	sbc	a, b
	ld	a, -5 (ix)
	sbc	a, e
	ld	a, -4 (ix)
	sbc	a, d
	jr	NC, 00161$
	ld	a, #0x0d
	jp	00170$
00161$:
;ff.c:3642: fs->last_clst = fs->free_clst = 0xFFFFFFFF;		/* Invalidate cluster allocation information */
	ld	a, -34 (ix)
	add	a, #0x0c
	ld	-12 (ix), a
	ld	a, -33 (ix)
	adc	a, #0x00
	ld	-11 (ix), a
	ld	a, -34 (ix)
	add	a, #0x10
	ld	-7 (ix), a
	ld	a, -33 (ix)
	adc	a, #0x00
	ld	-6 (ix), a
	ld	l, -7 (ix)
	ld	h, -6 (ix)
	ld	(hl), #0xff
	inc	hl
	ld	(hl), #0xff
	inc	hl
	ld	(hl), #0xff
	inc	hl
	ld	(hl), #0xff
	ld	l, -12 (ix)
	ld	h, -11 (ix)
	ld	(hl), #0xff
	inc	hl
	ld	(hl), #0xff
	inc	hl
	ld	(hl), #0xff
	inc	hl
	ld	(hl), #0xff
;ff.c:3643: fs->fsi_flag = 0x80;	/* Disable FSInfo by default */
	ld	a, -34 (ix)
	add	a, #0x05
	ld	-5 (ix), a
	ld	a, -33 (ix)
	adc	a, #0x00
	ld	-4 (ix), a
	ld	l, -5 (ix)
	ld	h, -4 (ix)
	ld	(hl), #0x80
;ff.c:3645: && ld_16(fs->win + BPB_FSInfo32) == 1	/* FAT32: Enable FSInfo feature only if FSInfo sector is next to VBR */
	ld	a, -3 (ix)
	or	a, a
	jp	Z, 00167$
	ld	a, -28 (ix)
	add	a, #0x30
	ld	-14 (ix), a
	ld	a, -27 (ix)
	adc	a, #0x00
	ld	-13 (ix), a
	ld	l, -14 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -13 (ix)
;	spillPairReg hl
;	spillPairReg hl
	call	_ld_16
	ld	-14 (ix), e
	ld	-13 (ix), d
	ld	a, -14 (ix)
	dec	a
	or	a, -13 (ix)
	jp	NZ,00167$
;ff.c:3646: && move_window(fs, bsect + 1) == FR_OK)
	ld	a, -32 (ix)
	add	a, #0x01
	ld	-16 (ix), a
	ld	a, -31 (ix)
	adc	a, #0x00
	ld	-15 (ix), a
	ld	a, -30 (ix)
	adc	a, #0x00
	ld	-14 (ix), a
	ld	a, -29 (ix)
	adc	a, #0x00
	ld	-13 (ix), a
	ld	l, -14 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -13 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	ld	l, -16 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -15 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	ld	l, -34 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -33 (ix)
;	spillPairReg hl
;	spillPairReg hl
	call	_move_window
	or	a, a
	jp	NZ, 00167$
;ff.c:3648: fs->fsi_flag = 0;
	ld	l, -5 (ix)
	ld	h, -4 (ix)
	ld	(hl), #0x00
;ff.c:3649: if (   ld_32(fs->win + FSI_LeadSig) == 0x41615252	/* Load FSInfo data if available */
	ld	a, -34 (ix)
	add	a, #0x30
	ld	-4 (ix), a
	ld	a, -33 (ix)
	adc	a, #0x00
	ld	-3 (ix), a
	ld	l, -4 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -3 (ix)
;	spillPairReg hl
;	spillPairReg hl
	call	_ld_32
	ld	-16 (ix), e
	ld	-15 (ix), d
	ld	-14 (ix), l
	ld	-13 (ix), h
	ld	a, -16 (ix)
	sub	a, #0x52
	jp	NZ,00167$
	ld	a, -15 (ix)
	sub	a, #0x52
	jp	NZ,00167$
	ld	a, -14 (ix)
	sub	a, #0x61
	jp	NZ,00167$
	ld	a, -13 (ix)
	sub	a, #0x41
	jp	NZ,00167$
;ff.c:3650: && ld_32(fs->win + FSI_StrucSig) == 0x61417272
	ld	a, -28 (ix)
	add	a, #0xe4
	ld	-4 (ix), a
	ld	a, -27 (ix)
	adc	a, #0x01
	ld	-3 (ix), a
	ld	l, -4 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -3 (ix)
;	spillPairReg hl
;	spillPairReg hl
	call	_ld_32
	ld	-16 (ix), e
	ld	-15 (ix), d
	ld	-14 (ix), l
	ld	-13 (ix), h
	ld	a, -16 (ix)
	sub	a, #0x72
	jp	NZ,00167$
	ld	a, -15 (ix)
	sub	a, #0x72
	jp	NZ,00167$
	ld	a, -14 (ix)
	sub	a, #0x41
	jp	NZ,00167$
	ld	a, -13 (ix)
	sub	a, #0x61
	jp	NZ,00167$
;ff.c:3651: && ld_32(fs->win + FSI_TrailSig) == 0xAA550000)
	ld	a, -28 (ix)
	add	a, #0xfc
	ld	-4 (ix), a
	ld	a, -27 (ix)
	adc	a, #0x01
	ld	-3 (ix), a
	ld	l, -4 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -3 (ix)
;	spillPairReg hl
;	spillPairReg hl
	call	_ld_32
	ld	-16 (ix), e
	ld	-15 (ix), d
	ld	-14 (ix), l
	ld	-13 (ix), h
	ld	a, -16 (ix)
	or	a, a
	or	a, -15 (ix)
	jr	NZ, 00167$
	ld	a, -14 (ix)
	sub	a, #0x55
	jr	NZ, 00167$
	ld	a, -13 (ix)
	sub	a, #0xaa
	jr	NZ, 00167$
;ff.c:3654: fs->free_clst = ld_32(fs->win + FSI_Free_Count);
	ld	a, -28 (ix)
	add	a, #0xe8
	ld	-4 (ix), a
	ld	a, -27 (ix)
	adc	a, #0x01
	ld	-3 (ix), a
	ld	l, -4 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -3 (ix)
;	spillPairReg hl
;	spillPairReg hl
	call	_ld_32
	ld	-16 (ix), e
	ld	-15 (ix), d
	ld	-14 (ix), l
	ld	-13 (ix), h
	ld	e, -7 (ix)
	ld	d, -6 (ix)
	ld	hl, #22
	add	hl, sp
	ld	bc, #0x0004
	ldir
;ff.c:3657: fs->last_clst = ld_32(fs->win + FSI_Nxt_Free);
	ld	a, -28 (ix)
	add	a, #0xec
	ld	-4 (ix), a
	ld	a, -27 (ix)
	adc	a, #0x01
	ld	-3 (ix), a
	ld	l, -4 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -3 (ix)
;	spillPairReg hl
;	spillPairReg hl
	call	_ld_32
	ld	-6 (ix), e
	ld	-5 (ix), d
	ld	-4 (ix), l
	ld	-3 (ix), h
	ld	e, -12 (ix)
	ld	d, -11 (ix)
	ld	hl, #32
	add	hl, sp
	ld	bc, #0x0004
	ldir
00167$:
;ff.c:3664: fs->fs_type = (BYTE)fmt;/* FAT sub-type (the filesystem object gets valid) */
	ld	a, -10 (ix)
	ld	l, -34 (ix)
	ld	h, -33 (ix)
	ld	(hl), a
;ff.c:3665: fs->id = ++Fsid;		/* Volume mount ID */
	ld	a, -34 (ix)
	add	a, #0x06
	ld	c, a
	ld	a, -33 (ix)
	adc	a, #0x00
	ld	b, a
	ld	hl, (_Fsid)
	inc	hl
	ld	(_Fsid), hl
	ld	a, (_Fsid+0)
	ld	(bc), a
	inc	bc
	ld	a, (_Fsid+1)
	ld	(bc), a
;ff.c:3685: return FR_OK;
	xor	a, a
00170$:
;ff.c:3686: }
	ld	sp, ix
	pop	ix
	pop	hl
	inc	sp
	jp	(hl)
;ff.c:3695: static FRESULT validate (	/* Returns FR_OK or FR_INVALID_OBJECT */
;	---------------------------------
; Function validate
; ---------------------------------
_validate:
	push	ix
	ld	ix,#0
	add	ix,sp
	push	af
	push	af
	dec	sp
	ld	c, l
	ld	b, h
	ld	-2 (ix), e
	ld	-1 (ix), d
;ff.c:3700: FRESULT res = FR_INVALID_OBJECT;
	ld	-5 (ix), #0x09
;ff.c:3703: if (obj && obj->fs && obj->fs->fs_type && obj->id == obj->fs->id) {	/* Test if the object is valid */
	ld	a, b
	or	a, c
	jr	Z, 00104$
	ld	l, c
	ld	h, b
	ld	e, (hl)
	inc	hl
	ld	d, (hl)
	ld	a, d
	or	a, e
	jr	Z, 00104$
	ld	a, (de)
	or	a, a
	jr	Z, 00104$
	ld	l, c
;	spillPairReg hl
;	spillPairReg hl
	ld	h, b
;	spillPairReg hl
;	spillPairReg hl
	inc	hl
	inc	hl
	ld	a, (hl)
	ld	-4 (ix), a
	inc	hl
	ld	a, (hl)
	ld	-3 (ix), a
	push	de
	pop	iy
;	spillPairReg hl
;	spillPairReg hl
	ld	a, 6 (iy)
	ld	h, 7 (iy)
	sub	a, -4 (ix)
	jr	NZ, 00104$
	ld	a, h
	sub	a, -3 (ix)
	jr	NZ, 00104$
;ff.c:3715: if (!(disk_status(obj->fs->pdrv) & STA_NOINIT)) { /* Test if the hosting physical drive is kept initialized */
	inc	de
	ld	a, (de)
	ld	e, a
	push	bc
	ld	a, e
	call	_disk_status
	pop	bc
	rrca
	jr	C, 00104$
;ff.c:3716: res = FR_OK;
	ld	-5 (ix), #0x00
00104$:
;ff.c:3720: *rfs = (res == FR_OK) ? obj->fs : 0;	/* Return corresponding filesystem object if it is valid */
	ld	e, -2 (ix)
	ld	d, -1 (ix)
	ld	a, -5 (ix)
	or	a, a
	jr	NZ, 00110$
	ld	l, c
	ld	h, b
	ld	c, (hl)
	inc	hl
	ld	b, (hl)
	jr	00111$
00110$:
	ld	bc, #0x0000
00111$:
	ld	a, c
	ld	(de), a
	inc	de
	ld	a, b
	ld	(de), a
;ff.c:3721: return res;
	ld	a, -5 (ix)
;ff.c:3722: }
	ld	sp, ix
	pop	ix
	ret
;ff.c:3739: FRESULT f_mount (
;	---------------------------------
; Function f_mount
; ---------------------------------
_f_mount::
	push	ix
	ld	ix,#0
	add	ix,sp
	ld	iy, #-9
	add	iy, sp
	ld	sp, iy
	ld	-2 (ix), l
	ld	-1 (ix), h
	ld	-4 (ix), e
	ld	-3 (ix), d
;ff.c:3748: const TCHAR *rp = path;
	ld	a, -4 (ix)
	ld	-7 (ix), a
	ld	a, -3 (ix)
	ld	-6 (ix), a
;ff.c:3752: vol = get_ldnumber(&rp);
	ld	hl, #2
	add	hl, sp
	call	_get_ldnumber
	inc	sp
	inc	sp
	push	de
;ff.c:3753: if (vol < 0) return FR_INVALID_DRIVE;
	bit	7, -8 (ix)
	jr	Z, 00102$
	ld	a, #0x0b
	jr	00109$
00102$:
;ff.c:3755: cfs = FatFs[vol];			/* Pointer to the filesystem object of the volume */
	ld	bc, #_FatFs+0
	pop	hl
	push	hl
	add	hl, hl
	add	hl, bc
	ld	c,l
	ld	b,h
	inc	hl
	ld	a, (hl)
	dec	hl
	ld	l, (hl)
;	spillPairReg hl
	ld	e, l
	ld	d, a
;ff.c:3756: if (cfs) {					/* Unregister current filesystem object */
	or	a, l
	jr	Z, 00104$
;ff.c:3757: FatFs[vol] = 0;
	ld	l, c
	ld	h, b
	xor	a, a
	ld	(hl), a
	inc	hl
	ld	(hl), a
;ff.c:3764: cfs->fs_type = 0;		/* Invalidate the filesystem object to be unregistered */
	xor	a, a
	ld	(de), a
00104$:
;ff.c:3767: if (fs) {					/* Register new filesystem object */
	ld	a, -1 (ix)
	or	a, -2 (ix)
	jr	Z, 00106$
;ff.c:3768: fs->pdrv = LD2PD(vol);	/* Volume hosting physical drive */
	ld	e, -2 (ix)
	ld	d, -1 (ix)
	inc	de
	ld	a, -9 (ix)
	ld	(de), a
;ff.c:3782: fs->fs_type = 0;		/* Invalidate the new filesystem object */
	ld	e, -2 (ix)
	ld	d, -1 (ix)
	xor	a, a
	ld	(de), a
;ff.c:3783: FatFs[vol] = fs;		/* Register it */
	ld	a, -2 (ix)
	ld	(bc), a
	inc	bc
	ld	a, -1 (ix)
	ld	(bc), a
00106$:
;ff.c:3786: if (opt == 0) return FR_OK;	/* Do not mount now, it will be mounted in subsequent file functions */
	ld	a, 4 (ix)
	or	a, a
	jr	NZ, 00108$
	xor	a, a
	jr	00109$
00108$:
;ff.c:3788: res = mount_volume(&path, &fs, 0);	/* Force mounted the volume in this function */
	xor	a, a
	push	af
	inc	sp
	ld	hl, #8
	add	hl, sp
	ex	de, hl
	ld	hl, #6
	add	hl, sp
	call	_mount_volume
	ld	-5 (ix), a
;ff.c:3789: LEAVE_FF(fs, res);
00109$:
;ff.c:3790: }
	ld	sp, ix
	pop	ix
	pop	hl
	inc	sp
	jp	(hl)
;ff.c:3799: FRESULT f_open (
;	---------------------------------
; Function f_open
; ---------------------------------
_f_open::
	push	ix
	ld	ix,#0
	add	ix,sp
	ld	iy, #-63
	add	iy, sp
	ld	sp, iy
	ld	-6 (ix), l
	ld	-5 (ix), h
	ld	-8 (ix), e
	ld	-7 (ix), d
;ff.c:3811: if (!fp) return FR_INVALID_OBJECT;	/* Reject null pointer */
	ld	a, -5 (ix)
	or	a, -6 (ix)
	jr	NZ, 00102$
	ld	a, #0x09
	jp	00165$
00102$:
;ff.c:3814: mode &= FF_FS_READONLY ? FA_READ : FA_READ | FA_WRITE | FA_CREATE_ALWAYS | FA_CREATE_NEW | FA_OPEN_ALWAYS | FA_OPEN_APPEND;
	ld	a, 4 (ix)
	ld	-1 (ix), a
	and	a, #0x3f
;ff.c:3815: res = mount_volume(&path, &fs, mode);
	ld	4 (ix), a
	push	af
	inc	sp
	ld	hl, #41
	add	hl, sp
	ex	de, hl
	ld	hl, #56
	add	hl, sp
	call	_mount_volume
;ff.c:3817: if (res == FR_OK) {
	ld	-21 (ix), a
	or	a, a
	jp	NZ, 00158$
;ff.c:3818: fp->obj.fs = fs;
	ld	l, -6 (ix)
	ld	h, -5 (ix)
	ld	a, -23 (ix)
	ld	(hl), a
	inc	hl
	ld	a, -22 (ix)
	ld	(hl), a
;ff.c:3819: dj.obj.fs = fs;
	ld	a, -23 (ix)
	ld	-63 (ix), a
	ld	a, -22 (ix)
	ld	-62 (ix), a
;ff.c:3821: res = follow_path(&dj, path);	/* Follow the file path */
	ld	e, -8 (ix)
	ld	d, -7 (ix)
	ld	hl, #0
	add	hl, sp
	call	_follow_path
;ff.c:3823: if (res == FR_OK) {
	ld	-21 (ix), a
	or	a, a
	jr	NZ, 00106$
;ff.c:3824: if (dj.fn[NSFLAG] & NS_NONAME) {	/* Origin directory itself? */
	ld	a, -24 (ix)
	rlca
	jr	NC, 00106$
;ff.c:3825: res = FR_INVALID_NAME;
	ld	-21 (ix), #0x06
00106$:
;ff.c:3814: mode &= FF_FS_READONLY ? FA_READ : FA_READ | FA_WRITE | FA_CREATE_ALWAYS | FA_CREATE_NEW | FA_OPEN_ALWAYS | FA_OPEN_APPEND;
	ld	a, 4 (ix)
	ld	-15 (ix), a
;ff.c:3849: if (dj.obj.attr & (AM_RDO | AM_DIR)) res = FR_DENIED;	/* Cannot overwrite it (R/O or DIR) */
;ff.c:3875: st_32(dj.dir + DIR_CrtTime, tm);	/* Set created time */
;ff.c:3834: if (mode & (FA_CREATE_ALWAYS | FA_OPEN_ALWAYS | FA_CREATE_NEW)) {
	ld	a, 4 (ix)
	and	a, #0x1c
	jp	Z,00133$
;ff.c:3835: if (res != FR_OK) {					/* No file, create new */
	ld	a, -21 (ix)
	or	a, a
	jr	Z, 00115$
;ff.c:3836: if (res == FR_NO_FILE) {		/* There is no file to open, create a new entry */
	ld	a, -21 (ix)
	sub	a, #0x04
	jr	NZ, 00108$
;ff.c:3840: res = dir_register(&dj);
	ld	hl, #0
	add	hl, sp
	call	_dir_register
	ld	-21 (ix), a
00108$:
;ff.c:3843: mode |= FA_CREATE_ALWAYS;		/* File is created */
	ld	a, 4 (ix)
	or	a, #0x08
	ld	4 (ix), a
	jr	00116$
00115$:
;ff.c:3846: if (mode & FA_CREATE_NEW) {
	bit	2, -15 (ix)
	jr	Z, 00112$
;ff.c:3847: res = FR_EXIST;				/* Cannot create as new file */
	ld	-21 (ix), #0x08
	jr	00116$
00112$:
;ff.c:3849: if (dj.obj.attr & (AM_RDO | AM_DIR)) res = FR_DENIED;	/* Cannot overwrite it (R/O or DIR) */
	ld	a, -59 (ix)
	and	a, #0x11
	jr	Z, 00116$
	ld	-21 (ix), #0x07
00116$:
;ff.c:3814: mode &= FF_FS_READONLY ? FA_READ : FA_READ | FA_WRITE | FA_CREATE_ALWAYS | FA_CREATE_NEW | FA_OPEN_ALWAYS | FA_OPEN_APPEND;
	ld	a, 4 (ix)
	ld	-15 (ix), a
;ff.c:3852: if (res == FR_OK && (mode & FA_CREATE_ALWAYS)) {	/* Truncate the file if overwrite mode */
	ld	a, -21 (ix)
	or	a, a
	jp	NZ, 00134$
	bit	3, -15 (ix)
	jp	Z,00134$
;ff.c:3853: DWORD tm = GET_FATTIME();
	call	_get_fattime
	ld	-4 (ix), e
	ld	-3 (ix), d
	ld	-2 (ix), l
	ld	-1 (ix), h
	ld	hl, #51
	add	hl, sp
	ex	de, hl
	ld	hl, #59
	add	hl, sp
	ld	bc, #4
	ldir
;ff.c:3875: st_32(dj.dir + DIR_CrtTime, tm);	/* Set created time */
	ld	a, -37 (ix)
	ld	-4 (ix), a
	ld	a, -36 (ix)
	ld	-3 (ix), a
	ld	a, -4 (ix)
	add	a, #0x0e
	ld	-2 (ix), a
	ld	a, -3 (ix)
	adc	a, #0x00
	ld	-1 (ix), a
	ld	l, -10 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -9 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	ld	l, -12 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -11 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	ld	l, -2 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -1 (ix)
;	spillPairReg hl
;	spillPairReg hl
	call	_st_32
;ff.c:3876: st_32(dj.dir + DIR_ModTime, tm);	/* Set modified time (tmp setting) */
	ld	a, -37 (ix)
	ld	-4 (ix), a
	ld	a, -36 (ix)
	ld	-3 (ix), a
	ld	a, -4 (ix)
	add	a, #0x16
	ld	-2 (ix), a
	ld	a, -3 (ix)
	adc	a, #0x00
	ld	-1 (ix), a
	ld	l, -10 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -9 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	ld	l, -12 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -11 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	ld	l, -2 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -1 (ix)
;	spillPairReg hl
;	spillPairReg hl
	call	_st_32
;ff.c:3877: cl = ld_clust(fs, dj.dir);			/* Get current cluster chain */
	ld	e, -37 (ix)
	ld	d, -36 (ix)
	ld	l, -23 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -22 (ix)
;	spillPairReg hl
;	spillPairReg hl
	call	_ld_clust
	ld	-4 (ix), e
	ld	-3 (ix), d
	ld	-2 (ix), l
	ld	-1 (ix), h
	ld	hl, #49
	add	hl, sp
	ex	de, hl
	ld	hl, #59
	add	hl, sp
	ld	bc, #4
	ldir
;ff.c:3878: dj.dir[DIR_Attr] = AM_ARC;			/* Reset attribute */
	ld	a, -37 (ix)
	ld	-4 (ix), a
	ld	a, -36 (ix)
	ld	-3 (ix), a
	ld	a, -4 (ix)
	add	a, #0x0b
	ld	-2 (ix), a
	ld	a, -3 (ix)
	adc	a, #0x00
	ld	-1 (ix), a
	ld	l, -2 (ix)
	ld	h, -1 (ix)
	ld	(hl), #0x20
;ff.c:3879: st_clust(fs, dj.dir, 0);			/* Reset file allocation info */
	ld	a, -37 (ix)
	ld	-2 (ix), a
	ld	a, -36 (ix)
	ld	-1 (ix), a
	ld	hl, #0x0000
	push	hl
	push	hl
	ld	e, -2 (ix)
	ld	d, -1 (ix)
	ld	l, -23 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -22 (ix)
;	spillPairReg hl
;	spillPairReg hl
	call	_st_clust
;ff.c:3880: st_32(dj.dir + DIR_FileSize, 0);
	ld	a, -37 (ix)
	ld	-4 (ix), a
	ld	a, -36 (ix)
	ld	-3 (ix), a
	ld	a, -4 (ix)
	add	a, #0x1c
	ld	-2 (ix), a
	ld	a, -3 (ix)
	adc	a, #0x00
	ld	-1 (ix), a
	ld	hl, #0x0000
	push	hl
	push	hl
	ld	l, -2 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -1 (ix)
;	spillPairReg hl
;	spillPairReg hl
	call	_st_32
;ff.c:3881: fs->wflag = 1;
	ld	a, -23 (ix)
	ld	-4 (ix), a
	ld	a, -22 (ix)
	ld	-3 (ix), a
	ld	a, -4 (ix)
	add	a, #0x04
	ld	-2 (ix), a
	ld	a, -3 (ix)
	adc	a, #0x00
	ld	-1 (ix), a
	ld	l, -2 (ix)
	ld	h, -1 (ix)
	ld	(hl), #0x01
;ff.c:3882: if (cl != 0) {						/* Remove the cluster chain if exist */
	ld	a, -11 (ix)
	or	a, -12 (ix)
	or	a, -13 (ix)
	or	a, -14 (ix)
	jp	Z, 00134$
;ff.c:3883: LBA_t sc = fs->winsect;
	ld	a, -23 (ix)
	ld	-2 (ix), a
	ld	a, -22 (ix)
	ld	-1 (ix), a
	ld	e, -2 (ix)
	ld	d, -1 (ix)
	ld	hl, #59
	add	hl, sp
	ex	de, hl
	ld	bc, #0x001c
	add	hl, bc
	ld	bc, #0x0004
	ldir
;ff.c:3885: res = remove_chain(&dj.obj, cl, 0);
	ld	hl, #0x0000
	push	hl
	push	hl
	ld	l, -12 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -11 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	ld	l, -14 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -13 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	ld	hl, #8
	add	hl, sp
	call	_remove_chain
;ff.c:3886: if (res == FR_OK) {
	ld	-21 (ix), a
	or	a, a
	jp	NZ, 00134$
;ff.c:3887: res = move_window(fs, sc);
	ld	l, -2 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -1 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	ld	l, -4 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -3 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	ld	l, -23 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -22 (ix)
;	spillPairReg hl
;	spillPairReg hl
	call	_move_window
	ld	-1 (ix), a
	ld	-21 (ix), a
;ff.c:3888: fs->last_clst = cl - 1;		/* Reuse the cluster hole */
	ld	a, -23 (ix)
	ld	-2 (ix), a
	ld	a, -22 (ix)
	ld	-1 (ix), a
	ld	a, -2 (ix)
	add	a, #0x0c
	ld	-10 (ix), a
	ld	a, -1 (ix)
	adc	a, #0x00
	ld	-9 (ix), a
	ld	a, -14 (ix)
	add	a, #0xff
	ld	-4 (ix), a
	ld	a, -13 (ix)
	adc	a, #0xff
	ld	-3 (ix), a
	ld	a, -12 (ix)
	adc	a, #0xff
	ld	-2 (ix), a
	ld	a, -11 (ix)
	adc	a, #0xff
	ld	-1 (ix), a
	ld	e, -10 (ix)
	ld	d, -9 (ix)
	ld	hl, #59
	add	hl, sp
	ld	bc, #0x0004
	ldir
	jr	00134$
00133$:
;ff.c:3895: if (res == FR_OK) {					/* Is the object exsiting? */
	ld	a, -21 (ix)
	or	a, a
	jr	NZ, 00134$
;ff.c:3896: if (dj.obj.attr & AM_DIR) {		/* File open against a directory */
	ld	a, -59 (ix)
	bit	4, a
	jr	Z, 00128$
;ff.c:3897: res = FR_NO_FILE;
	ld	-21 (ix), #0x04
	jr	00134$
00128$:
;ff.c:3899: if ((mode & FA_WRITE) && (dj.obj.attr & AM_RDO)) { /* Write mode open against R/O file */
	bit	1, -15 (ix)
	jr	Z, 00134$
	rrca
	jr	NC, 00134$
;ff.c:3900: res = FR_DENIED;
	ld	-21 (ix), #0x07
00134$:
;ff.c:3905: if (res == FR_OK) {
	ld	a, -21 (ix)
	or	a, a
	jr	NZ, 00138$
;ff.c:3906: if (mode & FA_CREATE_ALWAYS) mode |= FA_MODIFIED;	/* Set file change flag if created or overwritten */
	bit	3, -15 (ix)
	jr	Z, 00136$
	ld	a, -15 (ix)
	or	a, #0x40
	ld	4 (ix), a
00136$:
;ff.c:3907: fp->dir_sect = fs->winsect;			/* Pointer to the directory entry */
	ld	a, -6 (ix)
	add	a, #0x1c
	ld	e, a
	ld	a, -5 (ix)
	adc	a, #0x00
	ld	d, a
	ld	l, -23 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -22 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ex	de, hl
	push	hl
	ld	hl, #61
	add	hl, sp
	ex	de, hl
	ld	bc, #0x001c
	add	hl, bc
	ld	bc, #0x0004
	ldir
	pop	de
	ld	hl, #59
	add	hl, sp
	ld	bc, #0x0004
	ldir
;ff.c:3908: fp->dir_ptr = dj.dir;
	ld	a, -6 (ix)
	add	a, #0x20
	ld	c, a
	ld	a, -5 (ix)
	adc	a, #0x00
	ld	b, a
	ld	e, -37 (ix)
	ld	d, -36 (ix)
	ld	a, e
	ld	(bc), a
	inc	bc
	ld	a, d
	ld	(bc), a
00138$:
;ff.c:3927: if (res == FR_OK) {
	ld	a, -21 (ix)
	or	a, a
	jp	NZ, 00158$
;ff.c:3934: fp->obj.sclust = ld_clust(fs, dj.dir);					/* Get object allocation info */
	ld	a, -6 (ix)
	add	a, #0x06
	ld	-4 (ix), a
	ld	a, -5 (ix)
	adc	a, #0x00
	ld	-3 (ix), a
	ld	e, -37 (ix)
	ld	d, -36 (ix)
	ld	l, -23 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -22 (ix)
;	spillPairReg hl
;	spillPairReg hl
	call	_ld_clust
	ld	c, l
	ld	b, h
	ld	l, -4 (ix)
	ld	h, -3 (ix)
	ld	(hl), e
	inc	hl
	ld	(hl), d
	inc	hl
	ld	(hl), c
	inc	hl
	ld	(hl), b
;ff.c:3935: fp->obj.objsize = ld_32(dj.dir + DIR_FileSize);
	ld	a, -6 (ix)
	add	a, #0x0a
	ld	-2 (ix), a
	ld	a, -5 (ix)
	adc	a, #0x00
	ld	-1 (ix), a
	ld	c, -37 (ix)
	ld	b, -36 (ix)
	ld	hl, #0x001c
	add	hl, bc
	call	_ld_32
	ld	c, l
	ld	b, h
	ld	l, -2 (ix)
	ld	h, -1 (ix)
	ld	(hl), e
	inc	hl
	ld	(hl), d
	inc	hl
	ld	(hl), c
	inc	hl
	ld	(hl), b
;ff.c:3940: fp->obj.id = fs->id;	/* Set current volume mount ID */
	ld	c, -6 (ix)
	ld	b, -5 (ix)
	inc	bc
	inc	bc
	ld	e, -23 (ix)
	ld	d, -22 (ix)
	ld	hl, #6
	add	hl, de
	ld	e, (hl)
	inc	hl
	ld	d, (hl)
	ld	a, e
	ld	(bc), a
	inc	bc
	ld	a, d
	ld	(bc), a
;ff.c:3941: fp->flag = mode;	/* Set file access mode */
	ld	l, -6 (ix)
	ld	h, -5 (ix)
	ld	de, #0x000e
	add	hl, de
	ld	a, 4 (ix)
	ld	(hl), a
;ff.c:3942: fp->err = 0;		/* Clear error flag */
	ld	l, -6 (ix)
	ld	h, -5 (ix)
	ld	de, #0x000f
	add	hl, de
	ld	(hl), #0x00
;ff.c:3943: fp->sect = 0;		/* Invalidate current data sector */
	ld	a, -6 (ix)
	add	a, #0x18
	ld	-20 (ix), a
	ld	a, -5 (ix)
	adc	a, #0x00
	ld	-19 (ix), a
	ld	l, -20 (ix)
	ld	h, -19 (ix)
	xor	a, a
	ld	(hl), a
	inc	hl
	ld	(hl), a
	inc	hl
	ld	(hl), a
	inc	hl
	ld	(hl), a
;ff.c:3944: fp->fptr = 0;		/* Set file pointer top of the file */
	ld	a, -6 (ix)
	add	a, #0x10
	ld	-10 (ix), a
	ld	a, -5 (ix)
	adc	a, #0x00
	ld	-9 (ix), a
	ld	l, -10 (ix)
	ld	h, -9 (ix)
	xor	a, a
	ld	(hl), a
	inc	hl
	ld	(hl), a
	inc	hl
	ld	(hl), a
	inc	hl
	ld	(hl), a
;ff.c:3947: memset(fp->buf, 0, sizeof fp->buf);	/* Clear sector buffer */
	ld	a, -6 (ix)
	add	a, #0x22
	ld	-18 (ix), a
	ld	a, -5 (ix)
	adc	a, #0x00
	ld	-17 (ix), a
	ld	l, -18 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -17 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	(hl), #0x00
	ld	e, l
	ld	d, h
	inc	de
	ld	bc, #0x01ff
	ldir
;ff.c:3949: if ((mode & FA_SEEKEND) && fp->obj.objsize > 0) {	/* Seek to end of file if FA_OPEN_APPEND is specified */
	bit	5, 4 (ix)
	jp	Z,00158$
	ld	l, -2 (ix)
	ld	h, -1 (ix)
	ld	c, (hl)
	inc	hl
	ld	b, (hl)
	inc	hl
	ld	e, (hl)
	inc	hl
	ld	d, (hl)
	ld	a, d
	or	a, e
	or	a, b
	or	a, c
	jp	Z, 00158$
;ff.c:3953: fp->fptr = fp->obj.objsize;			/* Offset to seek */
	ld	l, -10 (ix)
	ld	h, -9 (ix)
	ld	(hl), c
	inc	hl
	ld	(hl), b
	inc	hl
	ld	(hl), e
	inc	hl
	ld	(hl), d
;ff.c:3954: bcs = (DWORD)fs->csize * SS(fs);	/* Cluster size in byte */
	ld	a, -23 (ix)
	ld	-10 (ix), a
	ld	a, -22 (ix)
	ld	-9 (ix), a
	ld	l, -10 (ix)
	ld	h, -9 (ix)
	ld	de, #0x000a
	add	hl, de
	ld	a, (hl)
	ld	-10 (ix), a
	inc	hl
	ld	a, (hl)
	ld	-9 (ix), a
	ld	a, -10 (ix)
	ld	-12 (ix), a
	ld	a, -9 (ix)
	ld	-11 (ix), a
	xor	a, a
	ld	-10 (ix), a
	ld	-9 (ix), a
	ld	a, -12 (ix)
	ld	-15 (ix), a
	ld	a, -11 (ix)
	ld	-14 (ix), a
	ld	a, -10 (ix)
	ld	-13 (ix), a
	ld	-16 (ix), #0x00
	sla	-15 (ix)
	rl	-14 (ix)
	rl	-13 (ix)
;ff.c:3955: clst = fp->obj.sclust;				/* Follow the cluster chain */
	ld	e, -4 (ix)
	ld	d, -3 (ix)
	ld	hl, #51
	add	hl, sp
	ex	de, hl
	ld	bc, #0x0004
	ldir
;ff.c:3956: for (ofs = fp->obj.objsize; res == FR_OK && ofs > bcs; ofs -= bcs) {
	ld	e, -2 (ix)
	ld	d, -1 (ix)
	ld	hl, #59
	add	hl, sp
	ex	de, hl
	ld	bc, #0x0004
	ldir
00163$:
	ld	a, -21 (ix)
	or	a, a
	jp	NZ, 00143$
	ld	a, -16 (ix)
	sub	a, -4 (ix)
	ld	a, -15 (ix)
	sbc	a, -3 (ix)
	ld	a, -14 (ix)
	sbc	a, -2 (ix)
	ld	a, -13 (ix)
	sbc	a, -1 (ix)
	jr	NC, 00143$
;ff.c:3957: clst = get_fat(&fp->obj, clst);
	ld	l, -10 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -9 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	ld	l, -12 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -11 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	ld	l, -6 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -5 (ix)
;	spillPairReg hl
;	spillPairReg hl
	call	_get_fat
	pop	af
	pop	af
	ld	-12 (ix), e
	ld	-11 (ix), d
	ld	-10 (ix), l
	ld	-9 (ix), h
;ff.c:3958: if (clst <= 1) res = FR_INT_ERR;
	ld	a, #0x01
	cp	a, -12 (ix)
	ld	a, #0x00
	sbc	a, -11 (ix)
	ld	a, #0x00
	sbc	a, -10 (ix)
	ld	a, #0x00
	sbc	a, -9 (ix)
	jr	C, 00140$
	ld	-21 (ix), #0x02
00140$:
;ff.c:3959: if (clst == 0xFFFFFFFF) res = FR_DISK_ERR;
	ld	a, -12 (ix)
	and	a, -11 (ix)
	and	a, -10 (ix)
	and	a, -9 (ix)
	inc	a
	jr	NZ, 00164$
	ld	-21 (ix), #0x01
00164$:
;ff.c:3956: for (ofs = fp->obj.objsize; res == FR_OK && ofs > bcs; ofs -= bcs) {
	ld	a, -4 (ix)
	sub	a, -16 (ix)
	ld	-4 (ix), a
	ld	a, -3 (ix)
	sbc	a, -15 (ix)
	ld	-3 (ix), a
	ld	a, -2 (ix)
	sbc	a, -14 (ix)
	ld	-2 (ix), a
	ld	a, -1 (ix)
	sbc	a, -13 (ix)
	ld	-1 (ix), a
	jp	00163$
00143$:
;ff.c:3961: fp->clust = clst;
	ld	a, -6 (ix)
	add	a, #0x14
	ld	e, a
	ld	a, -5 (ix)
	adc	a, #0x00
	ld	d, a
	ld	hl, #51
	add	hl, sp
	ld	bc, #0x0004
	ldir
;ff.c:3962: if (res == FR_OK && ofs % SS(fs)) {	/* Fill sector buffer if not on the sector boundary */
	ld	a, -21 (ix)
	or	a, a
	jp	NZ, 00158$
	ld	a, -4 (ix)
	or	a, a
	jr	NZ, 00343$
	bit	0, -3 (ix)
	jp	Z,00158$
00343$:
;ff.c:3963: LBA_t sec = clst2sect(fs, clst);
	ld	l, -10 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -9 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	ld	l, -12 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -11 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	ld	l, -23 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -22 (ix)
;	spillPairReg hl
;	spillPairReg hl
	call	_clst2sect
	pop	af
	pop	af
	ld	-16 (ix), e
	ld	-15 (ix), d
	ld	-14 (ix), l
	ld	-13 (ix), h
	ld	hl, #51
	add	hl, sp
	ex	de, hl
	ld	hl, #47
	add	hl, sp
	ld	bc, #4
	ldir
;ff.c:3965: if (sec == 0) {
	ld	a, -13 (ix)
	or	a, -14 (ix)
	or	a, -15 (ix)
	or	a, -16 (ix)
	jr	NZ, 00147$
;ff.c:3966: res = FR_INT_ERR;
	ld	-21 (ix), #0x02
	jr	00158$
00147$:
;ff.c:3968: fp->sect = sec + (DWORD)(ofs / SS(fs));
	ld	b, #0x09
00344$:
	srl	-1 (ix)
	rr	-2 (ix)
	rr	-3 (ix)
	rr	-4 (ix)
	djnz	00344$
	ld	a, -4 (ix)
	add	a, -12 (ix)
	ld	-16 (ix), a
	ld	a, -3 (ix)
	adc	a, -11 (ix)
	ld	-15 (ix), a
	ld	a, -2 (ix)
	adc	a, -10 (ix)
	ld	-14 (ix), a
	ld	a, -1 (ix)
	adc	a, -9 (ix)
	ld	-13 (ix), a
	ld	e, -20 (ix)
	ld	d, -19 (ix)
	ld	hl, #47
	add	hl, sp
	ld	bc, #0x0004
	ldir
;ff.c:3970: if (disk_read(fs->pdrv, fp->buf, fp->sect, 1) != RES_OK) res = FR_DISK_ERR;
	ld	l, -23 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -22 (ix)
;	spillPairReg hl
;	spillPairReg hl
	inc	hl
	ld	c, (hl)
	ld	hl, #0x0001
	push	hl
	ld	l, -14 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -13 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	ld	l, -16 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -15 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	ld	e, -18 (ix)
	ld	d, -17 (ix)
	ld	a, c
	call	_disk_read
	or	a, a
	jr	Z, 00158$
	ld	-21 (ix), #0x01
00158$:
;ff.c:3984: if (res != FR_OK) fp->obj.fs = 0;	/* Invalidate file object on error */
	ld	a, -21 (ix)
	or	a, a
	jr	Z, 00160$
	ld	l, -6 (ix)
	ld	h, -5 (ix)
	xor	a, a
	ld	(hl), a
	inc	hl
	ld	(hl), a
00160$:
;ff.c:3986: LEAVE_FF(fs, res);
	ld	a, -21 (ix)
00165$:
;ff.c:3987: }
	ld	sp, ix
	pop	ix
	pop	hl
	inc	sp
	jp	(hl)
;ff.c:3996: FRESULT f_read (
;	---------------------------------
; Function f_read
; ---------------------------------
_f_read::
	push	ix
	ld	ix,#0
	add	ix,sp
	ld	iy, #-50
	add	iy, sp
	ld	sp, iy
	ld	-4 (ix), l
	ld	-3 (ix), h
;ff.c:4008: BYTE *rbuff = (BYTE*)buff;
	ld	-2 (ix), e
	ld	-1 (ix), d
;ff.c:4011: *br = 0;	/* Clear read byte counter */
	ld	a, 6 (ix)
	ld	-44 (ix), a
	ld	a, 7 (ix)
	ld	-43 (ix), a
	ld	l, -44 (ix)
	ld	h, -43 (ix)
	xor	a, a
	ld	(hl), a
	inc	hl
	ld	(hl), a
;ff.c:4012: res = validate(&fp->obj, &fs);				/* Check validity of the file object */
	ld	hl, #4
	add	hl, sp
	ex	de, hl
	ld	l, -4 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -3 (ix)
;	spillPairReg hl
;	spillPairReg hl
	call	_validate
	ld	c, a
	ld	-5 (ix), c
;ff.c:4013: if (res != FR_OK || (res = (FRESULT)fp->err) != FR_OK) LEAVE_FF(fs, res);	/* Check validity */
	ld	a, c
	or	a, a
	jr	NZ, 00101$
	ld	a, -4 (ix)
	add	a, #0x0f
	ld	-42 (ix), a
	ld	a, -3 (ix)
	adc	a, #0x00
	ld	-41 (ix), a
	ld	l, -42 (ix)
	ld	h, -41 (ix)
	ld	a, (hl)
	ld	-5 (ix), a
	or	a, a
	jr	Z, 00102$
00101$:
	ld	a, -5 (ix)
	jp	00144$
00102$:
;ff.c:4014: if (!(fp->flag & FA_READ)) LEAVE_FF(fs, FR_DENIED); /* Check access mode */
	ld	a, -4 (ix)
	add	a, #0x0e
	ld	-40 (ix), a
	ld	a, -3 (ix)
	adc	a, #0x00
	ld	-39 (ix), a
	ld	l, -40 (ix)
	ld	h, -39 (ix)
	ld	a, (hl)
	rrca
	jr	C, 00105$
	ld	a, #0x07
	jp	00144$
00105$:
;ff.c:4015: remain = fp->obj.objsize - fp->fptr;
	ld	e, -4 (ix)
	ld	d, -3 (ix)
	ld	hl, #42
	add	hl, sp
	ex	de, hl
	ld	bc, #0x000a
	add	hl, bc
	ld	bc, #0x0004
	ldir
	ld	a, -4 (ix)
	add	a, #0x10
	ld	-38 (ix), a
	ld	a, -3 (ix)
	adc	a, #0x00
	ld	-37 (ix), a
	ld	a, -38 (ix)
	ld	-36 (ix), a
	ld	a, -37 (ix)
	ld	-35 (ix), a
	ld	l, -38 (ix)
	ld	h, -37 (ix)
	ld	c, (hl)
	inc	hl
	ld	b, (hl)
	inc	hl
	ld	e, (hl)
	inc	hl
	ld	d, (hl)
	ld	a, -8 (ix)
	sub	a, c
	ld	c, a
	ld	a, -7 (ix)
	sbc	a, b
	ld	b, a
	ld	a, -6 (ix)
	sbc	a, e
	ld	e, a
	ld	a, -5 (ix)
	sbc	a, d
	ld	d, a
;ff.c:4016: if (btr > remain) btr = (UINT)remain;		/* Truncate btr by remaining bytes */
	ld	a, 4 (ix)
	ld	-8 (ix), a
	ld	a, 5 (ix)
	ld	-7 (ix), a
	xor	a, a
	ld	-6 (ix), a
	ld	-5 (ix), a
	ld	a, c
	sub	a, -8 (ix)
	ld	a, b
	sbc	a, -7 (ix)
	ld	a, e
	sbc	a, -6 (ix)
	ld	a, d
	sbc	a, -5 (ix)
	jr	NC, 00168$
	ld	4 (ix), c
	ld	5 (ix), b
00168$:
	ld	a, -4 (ix)
	add	a, #0x22
	ld	-34 (ix), a
	ld	a, -3 (ix)
	adc	a, #0x00
	ld	-33 (ix), a
	ld	a, -34 (ix)
	ld	-32 (ix), a
	ld	a, -33 (ix)
	ld	-31 (ix), a
	ld	a, -4 (ix)
	add	a, #0x18
	ld	-30 (ix), a
	ld	a, -3 (ix)
	adc	a, #0x00
	ld	-29 (ix), a
	ld	a, -30 (ix)
	ld	-28 (ix), a
	ld	a, -29 (ix)
	ld	-27 (ix), a
	ld	a, -40 (ix)
	ld	-26 (ix), a
	ld	a, -39 (ix)
	ld	-25 (ix), a
	ld	a, -4 (ix)
	ld	-24 (ix), a
	ld	a, -3 (ix)
	ld	-23 (ix), a
;ff.c:4033: clst = get_fat(&fp->obj, fp->clust);	/* Follow cluster chain on the FAT */
	ld	a, -4 (ix)
	add	a, #0x14
	ld	-22 (ix), a
	ld	a, -3 (ix)
	adc	a, #0x00
	ld	-21 (ix), a
;ff.c:4016: if (btr > remain) btr = (UINT)remain;		/* Truncate btr by remaining bytes */
	ld	a, -22 (ix)
	ld	-20 (ix), a
	ld	a, -21 (ix)
	ld	-19 (ix), a
	ld	a, -38 (ix)
	ld	-18 (ix), a
	ld	a, -37 (ix)
	ld	-17 (ix), a
	ld	a, -34 (ix)
	ld	-16 (ix), a
	ld	a, -33 (ix)
	ld	-15 (ix), a
00143$:
;ff.c:4018: for ( ; btr > 0; btr -= rcnt, *br += rcnt, rbuff += rcnt, fp->fptr += rcnt) {	/* Repeat until btr bytes read */
	ld	a, 5 (ix)
	or	a, 4 (ix)
	jp	Z, 00141$
;ff.c:4019: if (fp->fptr % SS(fs) == 0) {			/* On the sector boundary? */
	ld	e, -36 (ix)
	ld	d, -35 (ix)
	ld	hl, #42
	add	hl, sp
	ex	de, hl
	ld	bc, #0x0004
	ldir
	ld	a, -8 (ix)
	or	a, a
	jp	NZ,00137$
	bit	0, -7 (ix)
	jp	NZ,00137$
;ff.c:4020: csect = (UINT)(fp->fptr / SS(fs) & (fs->csize - 1));	/* Sector offset in the cluster */
	ld	c, -7 (ix)
	ld	b, -6 (ix)
	ld	e, -5 (ix)
	srl	e
	rr	b
	rr	c
	ld	l, -46 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -45 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	bc
	ld	bc, #0x000b
	add	hl, bc
	pop	bc
	ld	a, (hl)
	dec	hl
	ld	l, (hl)
;	spillPairReg hl
	ld	h, a
	dec	hl
	ld	a, c
	and	a, l
	ld	c, a
	ld	a, b
	and	a, h
	ld	-14 (ix), c
;ff.c:4021: if (csect == 0) {					/* On the cluster boundary? */
	ld	-13 (ix), a
	or	a, -14 (ix)
	jp	NZ, 00116$
;ff.c:4024: if (fp->fptr == 0) {			/* On the top of the file? */
	ld	a, -5 (ix)
	or	a, -6 (ix)
	or	a, -7 (ix)
	or	a, -8 (ix)
	jr	NZ, 00109$
;ff.c:4025: clst = fp->obj.sclust;		/* Follow cluster chain from the origin */
	ld	e, -24 (ix)
	ld	d, -23 (ix)
	ld	hl, #42
	add	hl, sp
	ex	de, hl
	ld	bc, #0x0006
	add	hl, bc
	ld	bc, #0x0004
	ldir
	jr	00110$
00109$:
;ff.c:4033: clst = get_fat(&fp->obj, fp->clust);	/* Follow cluster chain on the FAT */
	ld	e, -22 (ix)
	ld	d, -21 (ix)
	ld	hl, #42
	add	hl, sp
	ex	de, hl
	ld	bc, #0x0004
	ldir
	ld	l, -6 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -5 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	ld	l, -8 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -7 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	ld	l, -4 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -3 (ix)
;	spillPairReg hl
;	spillPairReg hl
	call	_get_fat
	pop	af
	pop	af
	ld	-8 (ix), e
	ld	-7 (ix), d
	ld	-6 (ix), l
	ld	-5 (ix), h
00110$:
;ff.c:4036: if (clst < 2) ABORT(fs, FR_INT_ERR);
	ld	a, -8 (ix)
	sub	a, #0x02
	ld	a, -7 (ix)
	sbc	a, #0x00
	ld	a, -6 (ix)
	sbc	a, #0x00
	ld	a, -5 (ix)
	sbc	a, #0x00
	jr	NC, 00112$
	ld	l, -42 (ix)
	ld	h, -41 (ix)
	ld	a,#0x02
	ld	(hl),a
	jp	00144$
00112$:
;ff.c:4037: if (clst == 0xFFFFFFFF) ABORT(fs, FR_DISK_ERR);
	ld	a, -8 (ix)
	and	a, -7 (ix)
	and	a, -6 (ix)
	and	a, -5 (ix)
	inc	a
	jr	NZ, 00114$
	ld	l, -42 (ix)
	ld	h, -41 (ix)
	ld	a,#0x01
	ld	(hl),a
	jp	00144$
00114$:
;ff.c:4038: fp->clust = clst;				/* Update current cluster */
	ld	e, -22 (ix)
	ld	d, -21 (ix)
	ld	hl, #42
	add	hl, sp
	ld	bc, #0x0004
	ldir
00116$:
;ff.c:4040: sect = clst2sect(fs, fp->clust);	/* Get current sector */
	ld	l, -20 (ix)
	ld	h, -19 (ix)
	ld	c, (hl)
	inc	hl
	ld	b, (hl)
	inc	hl
	ld	e, (hl)
	inc	hl
	ld	d, (hl)
	push	de
	push	bc
	ld	l, -46 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -45 (ix)
;	spillPairReg hl
;	spillPairReg hl
	call	_clst2sect
	pop	af
	pop	af
;ff.c:4041: if (sect == 0) ABORT(fs, FR_INT_ERR);
	ld	a, h
	or	a, l
	or	a, d
	or	a, e
	jr	NZ, 00118$
	ld	l, -42 (ix)
	ld	h, -41 (ix)
	ld	a,#0x02
	ld	(hl),a
	jp	00144$
00118$:
;ff.c:4042: sect += csect;
	ld	a, -14 (ix)
	push	iy
	ex	(sp), hl
	ld	h, -13 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ex	(sp), hl
	pop	iy
	ld	bc, #0x0000
	add	a, e
	ld	e, a
	push	iy
	ld	a, -51 (ix)
	pop	iy
	adc	a, d
	ld	d, a
	ld	a, c
	adc	a, l
	ld	c, a
	ld	a, b
	adc	a, h
	ld	b, a
	ld	-12 (ix), e
	ld	-11 (ix), d
	ld	-10 (ix), c
	ld	-9 (ix), b
;ff.c:4043: cc = btr / SS(fs);					/* When remaining bytes >= sector size, */
	ld	a, 5 (ix)
	srl	a
	ld	c, a
	ld	b, #0x00
;ff.c:4020: csect = (UINT)(fp->fptr / SS(fs) & (fs->csize - 1));	/* Sector offset in the cluster */
	ld	e, -46 (ix)
	ld	d, -45 (ix)
;ff.c:4048: if (disk_read(fs->pdrv, rbuff, sect, cc) != RES_OK) ABORT(fs, FR_DISK_ERR);
	ld	l, e
	ld	h, d
	inc	hl
	ld	-8 (ix), l
	ld	-7 (ix), h
;ff.c:4044: if (cc > 0) {						/* Read maximum contiguous sectors directly */
	ld	a, b
	or	a, c
	jp	Z, 00127$
;ff.c:4045: if (csect + cc > fs->csize) {	/* Clip at cluster boundary */
	ld	a, -14 (ix)
	add	a, c
	ld	-6 (ix), a
	ld	a, -13 (ix)
	adc	a, b
	ld	-5 (ix), a
	ld	hl, #10
	add	hl, de
	ld	e, (hl)
	inc	hl
	ld	d, (hl)
	ld	l, -6 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -5 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	a, e
	sub	a, l
	ld	a, d
	sbc	a, h
	jr	NC, 00120$
;ff.c:4046: cc = fs->csize - csect;
	ld	a, e
	sub	a, -14 (ix)
	ld	c, a
	ld	a, d
	sbc	a, -13 (ix)
	ld	b, a
00120$:
;ff.c:4048: if (disk_read(fs->pdrv, rbuff, sect, cc) != RES_OK) ABORT(fs, FR_DISK_ERR);
	ld	l, -8 (ix)
	ld	h, -7 (ix)
	ld	l, (hl)
;	spillPairReg hl
	push	bc
	push	bc
	ld	e, -10 (ix)
	ld	d, -9 (ix)
	push	de
	ld	e, -12 (ix)
	ld	d, -11 (ix)
	push	de
	ld	e, -2 (ix)
	ld	d, -1 (ix)
	ld	a, l
	call	_disk_read
	pop	bc
	or	a, a
	jr	Z, 00122$
	ld	l, -42 (ix)
	ld	h, -41 (ix)
	ld	a,#0x01
	ld	(hl),a
	jp	00144$
00122$:
;ff.c:4055: if ((fp->flag & FA_DIRTY) && fp->sect - sect < cc) {
	ld	l, -26 (ix)
	ld	h, -25 (ix)
	ld	a, (hl)
	rlca
	jr	NC, 00124$
	push	bc
	ld	e, -28 (ix)
	ld	d, -27 (ix)
	ld	hl, #2
	add	hl, sp
	ex	de, hl
	ld	bc, #0x0004
	ldir
	pop	bc
	ld	a, -50 (ix)
	sub	a, -12 (ix)
	ld	-8 (ix), a
	ld	a, -49 (ix)
	sbc	a, -11 (ix)
	ld	-7 (ix), a
	ld	a, -48 (ix)
	sbc	a, -10 (ix)
	ld	-6 (ix), a
	ld	a, -47 (ix)
	sbc	a, -9 (ix)
	ld	-5 (ix), a
	ld	e, c
	ld	d, b
	ld	hl, #0x0000
	ld	a, -8 (ix)
	sub	a, e
	ld	a, -7 (ix)
	sbc	a, d
	ld	a, -6 (ix)
	sbc	a, l
	ld	a, -5 (ix)
	sbc	a, h
	jr	NC, 00124$
;ff.c:4056: memcpy(rbuff + ((fp->sect - sect) * SS(fs)), fp->buf, SS(fs));
	pop	hl
	push	hl
	ld	e, -12 (ix)
	ld	d, -11 (ix)
	cp	a, a
	sbc	hl, de
	ld	a, l
	add	a, a
	ld	d, a
	ld	e, #0x00
	ld	l, -2 (ix)
	ld	h, -1 (ix)
	add	hl, de
	ex	de, hl
	ld	l, -32 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -31 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	bc
	ld	bc, #0x0200
	ldir
	pop	bc
00124$:
;ff.c:4060: rcnt = SS(fs) * cc;				/* Number of bytes transferred */
	ld	a, c
	add	a, a
	ld	b, a
	ld	c, #0x00
;ff.c:4061: continue;
	jp	00140$
00127$:
;ff.c:4064: if (fp->sect != sect) {			/* Load data sector if not in cache */
	ld	l, -30 (ix)
	ld	h, -29 (ix)
	ld	c, (hl)
	inc	hl
	ld	b, (hl)
	inc	hl
	ld	e, (hl)
	inc	hl
	ld	d, (hl)
	ld	a, -12 (ix)
	sub	a, c
	jr	NZ, 00262$
	ld	a, -11 (ix)
	sub	a, b
	jr	NZ, 00262$
	ld	l, -10 (ix)
	ld	h, -9 (ix)
	cp	a, a
	sbc	hl, de
	jr	Z, 00135$
00262$:
;ff.c:4066: if (fp->flag & FA_DIRTY) {		/* Write-back dirty sector cache */
	ld	l, -40 (ix)
	ld	h, -39 (ix)
	ld	a, (hl)
	rlca
	jr	NC, 00131$
;ff.c:4067: if (disk_write(fs->pdrv, fp->buf, fp->sect, 1) != RES_OK) ABORT(fs, FR_DISK_ERR);
	ld	l, -8 (ix)
	ld	h, -7 (ix)
	ld	l, (hl)
;	spillPairReg hl
	ld	iy, #0x0001
	push	iy
	push	de
	push	bc
	ld	e, -34 (ix)
	ld	d, -33 (ix)
	ld	a, l
	call	_disk_write
	or	a, a
	jr	Z, 00129$
	ld	l, -42 (ix)
	ld	h, -41 (ix)
	ld	a,#0x01
	ld	(hl),a
	jp	00144$
00129$:
;ff.c:4068: fp->flag &= (BYTE)~FA_DIRTY;
	ld	l, -40 (ix)
	ld	h, -39 (ix)
	ld	a, (hl)
	res	7, a
	ld	l, -40 (ix)
	ld	h, -39 (ix)
	ld	(hl), a
00131$:
;ff.c:4071: if (disk_read(fs->pdrv, fp->buf, sect, 1) != RES_OK) ABORT(fs, FR_DISK_ERR);	/* Fill sector cache */
	ld	l, -46 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -45 (ix)
;	spillPairReg hl
;	spillPairReg hl
	inc	hl
	ld	c, (hl)
	ld	hl, #0x0001
	push	hl
	ld	l, -10 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -9 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	ld	l, -12 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -11 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	ld	e, -34 (ix)
	ld	d, -33 (ix)
	ld	a, c
	call	_disk_read
	or	a, a
	jr	Z, 00135$
	ld	l, -42 (ix)
	ld	h, -41 (ix)
	ld	a,#0x01
	ld	(hl),a
	jp	00144$
00135$:
;ff.c:4074: fp->sect = sect;
	ld	e, -30 (ix)
	ld	d, -29 (ix)
	ld	hl, #38
	add	hl, sp
	ld	bc, #0x0004
	ldir
00137$:
;ff.c:4076: rcnt = SS(fs) - (UINT)fp->fptr % SS(fs);	/* Number of bytes remains in the sector */
	ld	e, -18 (ix)
	ld	d, -17 (ix)
	ld	hl, #40
	add	hl, sp
	ex	de, hl
	ld	bc, #0x0004
	ldir
	ld	c, -10 (ix)
	ld	a, -9 (ix)
	and	a, #0x01
	ld	b, a
	xor	a, a
	sub	a, c
	ld	c, a
	ld	a, #0x02
	sbc	a, b
	ld	b, a
;ff.c:4077: if (rcnt > btr) rcnt = btr;					/* Clip it by btr if needed */
	ld	a, 4 (ix)
	sub	a, c
	ld	a, 5 (ix)
	sbc	a, b
	jr	NC, 00139$
	ld	c, 4 (ix)
	ld	b, 5 (ix)
00139$:
;ff.c:4082: memcpy(rbuff, fp->buf + fp->fptr % SS(fs), rcnt);	/* Extract partial sector */
	ld	a, -2 (ix)
	ld	-6 (ix), a
	ld	a, -1 (ix)
	ld	-5 (ix), a
	ld	l, -10 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	a, -9 (ix)
	and	a, #0x01
	ld	h, a
;	spillPairReg hl
;	spillPairReg hl
	ld	a, l
	add	a, -16 (ix)
	ld	l, a
;	spillPairReg hl
;	spillPairReg hl
	ld	a, h
	adc	a, -15 (ix)
	ld	h, a
;	spillPairReg hl
;	spillPairReg hl
	push	bc
	ld	e, -6 (ix)
	ld	d, -5 (ix)
	ld	a, b
	or	a, c
	jr	Z, 00264$
	ldir
00264$:
	pop	bc
00140$:
;ff.c:4018: for ( ; btr > 0; btr -= rcnt, *br += rcnt, rbuff += rcnt, fp->fptr += rcnt) {	/* Repeat until btr bytes read */
	ld	a, 4 (ix)
	sub	a, c
	ld	4 (ix), a
	ld	a, 5 (ix)
	sbc	a, b
	ld	5 (ix), a
	ld	l, -44 (ix)
	ld	h, -43 (ix)
	ld	a, (hl)
	inc	hl
	ld	h, (hl)
;	spillPairReg hl
	ld	l, a
;	spillPairReg hl
;	spillPairReg hl
	add	hl, bc
	ex	de, hl
	ld	l, -44 (ix)
	ld	h, -43 (ix)
	ld	(hl), e
	inc	hl
	ld	(hl), d
	ld	a, c
	add	a, -2 (ix)
	ld	-2 (ix), a
	ld	a, b
	adc	a, -1 (ix)
	ld	-1 (ix), a
	push	bc
	ld	e, -38 (ix)
	ld	d, -37 (ix)
	ld	hl, #44
	add	hl, sp
	ex	de, hl
	ld	bc, #0x0004
	ldir
	pop	bc
	ld	de, #0x0000
	ld	a, c
	add	a, -8 (ix)
	ld	c, a
	ld	a, b
	adc	a, -7 (ix)
	ld	b, a
	ld	a, e
	adc	a, -6 (ix)
	ld	e, a
	ld	a, d
	adc	a, -5 (ix)
	ld	d, a
	ld	l, -38 (ix)
	ld	h, -37 (ix)
	ld	(hl), c
	inc	hl
	ld	(hl), b
	inc	hl
	ld	(hl), e
	inc	hl
	ld	(hl), d
	jp	00143$
00141$:
;ff.c:4086: LEAVE_FF(fs, FR_OK);
	xor	a, a
00144$:
;ff.c:4087: }
	ld	sp, ix
	pop	ix
	pop	hl
	pop	bc
	pop	bc
	jp	(hl)
;ff.c:4097: FRESULT f_write (
;	---------------------------------
; Function f_write
; ---------------------------------
_f_write::
	push	ix
	ld	ix,#0
	add	ix,sp
	ld	iy, #-58
	add	iy, sp
	ld	sp, iy
	ld	-4 (ix), l
	ld	-3 (ix), h
;ff.c:4109: const BYTE *wbuff = (const BYTE*)buff;
	ld	-2 (ix), e
	ld	-1 (ix), d
;ff.c:4112: *bw = 0;	/* Clear write byte counter */
	ld	a, 6 (ix)
	ld	-52 (ix), a
	ld	a, 7 (ix)
	ld	-51 (ix), a
	ld	l, -52 (ix)
	ld	h, -51 (ix)
	xor	a, a
	ld	(hl), a
	inc	hl
	ld	(hl), a
;ff.c:4113: res = validate(&fp->obj, &fs);			/* Check validity of the file object */
	ld	hl, #4
	add	hl, sp
	ex	de, hl
	ld	l, -4 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -3 (ix)
;	spillPairReg hl
;	spillPairReg hl
	call	_validate
	ld	c, a
	ld	-5 (ix), c
;ff.c:4114: if (res != FR_OK || (res = (FRESULT)fp->err) != FR_OK) LEAVE_FF(fs, res);	/* Check validity */
	ld	a, c
	or	a, a
	jr	NZ, 00101$
	ld	a, -4 (ix)
	add	a, #0x0f
	ld	-50 (ix), a
	ld	a, -3 (ix)
	adc	a, #0x00
	ld	-49 (ix), a
	ld	l, -50 (ix)
	ld	h, -49 (ix)
	ld	a, (hl)
	ld	-5 (ix), a
	or	a, a
	jr	Z, 00102$
00101$:
	ld	a, -5 (ix)
	jp	00151$
00102$:
;ff.c:4115: if (!(fp->flag & FA_WRITE)) LEAVE_FF(fs, FR_DENIED);	/* Check access mode */
	ld	a, -4 (ix)
	add	a, #0x0e
	ld	-48 (ix), a
	ld	a, -3 (ix)
	adc	a, #0x00
	ld	-47 (ix), a
	ld	l, -48 (ix)
	ld	h, -47 (ix)
	bit	1, (hl)
	jr	NZ, 00109$
	ld	a, #0x07
	jp	00151$
;ff.c:4118: if ((!FF_FS_EXFAT || fs->fs_type != FS_EXFAT) && (DWORD)(fp->fptr + btw) < (DWORD)fp->fptr) {
00109$:
	ld	a, -4 (ix)
	add	a, #0x10
	ld	-46 (ix), a
	ld	a, -3 (ix)
	adc	a, #0x00
	ld	-45 (ix), a
	ld	a, -46 (ix)
	ld	-44 (ix), a
	ld	a, -45 (ix)
	ld	-43 (ix), a
	ld	l, -46 (ix)
	ld	h, -45 (ix)
	ld	c, (hl)
	inc	hl
	ld	b, (hl)
	inc	hl
	ld	e, (hl)
	inc	hl
	ld	d, (hl)
	ld	a, 4 (ix)
	push	iy
	ex	(sp), hl
	ld	l, 5 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ex	(sp), hl
	pop	iy
	ld	hl, #0x0000
	add	a, c
	ld	-8 (ix), a
	push	iy
	ld	a, -60 (ix)
	pop	iy
	adc	a, b
	ld	-7 (ix), a
	ld	a, l
	adc	a, e
	ld	-6 (ix), a
	ld	a, h
	adc	a, d
	ld	-5 (ix), a
	ld	a, -8 (ix)
	sub	a, c
	ld	a, -7 (ix)
	sbc	a, b
	ld	a, -6 (ix)
	sbc	a, e
	ld	a, -5 (ix)
	sbc	a, d
	jr	NC, 00181$
;ff.c:4119: btw = (UINT)(0xFFFFFFFF - (DWORD)fp->fptr);
	ld	a, #0xff
	sub	a, c
	ld	4 (ix), a
	ld	a, #0xff
	sbc	a, b
	ld	5 (ix), a
00181$:
;ff.c:4151: if (disk_write(fs->pdrv, fp->buf, fp->sect, 1) != RES_OK) ABORT(fs, FR_DISK_ERR);
	ld	a, -4 (ix)
	add	a, #0x22
	ld	-42 (ix), a
	ld	a, -3 (ix)
	adc	a, #0x00
	ld	-41 (ix), a
;ff.c:4119: btw = (UINT)(0xFFFFFFFF - (DWORD)fp->fptr);
	ld	a, -42 (ix)
	ld	-40 (ix), a
	ld	a, -41 (ix)
	ld	-39 (ix), a
	ld	a, -4 (ix)
	add	a, #0x18
	ld	-38 (ix), a
	ld	a, -3 (ix)
	adc	a, #0x00
	ld	-37 (ix), a
	ld	a, -38 (ix)
	ld	-36 (ix), a
	ld	a, -37 (ix)
	ld	-35 (ix), a
	ld	a, -4 (ix)
	add	a, #0x06
	ld	-34 (ix), a
	ld	a, -3 (ix)
	adc	a, #0x00
	ld	-33 (ix), a
	ld	a, -34 (ix)
	ld	-32 (ix), a
	ld	a, -33 (ix)
	ld	-31 (ix), a
	ld	a, -38 (ix)
	ld	-30 (ix), a
	ld	a, -37 (ix)
	ld	-29 (ix), a
;ff.c:4138: clst = create_chain(&fp->obj, fp->clust);	/* Follow or stretch cluster chain on the FAT */
	ld	a, -4 (ix)
	add	a, #0x14
	ld	-28 (ix), a
	ld	a, -3 (ix)
	adc	a, #0x00
	ld	-27 (ix), a
;ff.c:4119: btw = (UINT)(0xFFFFFFFF - (DWORD)fp->fptr);
	ld	a, -28 (ix)
	ld	-26 (ix), a
	ld	a, -27 (ix)
	ld	-25 (ix), a
	ld	a, -46 (ix)
	ld	-24 (ix), a
	ld	a, -45 (ix)
	ld	-23 (ix), a
	ld	a, -4 (ix)
	add	a, #0x0a
	ld	-22 (ix), a
	ld	a, -3 (ix)
	adc	a, #0x00
	ld	-21 (ix), a
	ld	a, -22 (ix)
	ld	-20 (ix), a
	ld	a, -21 (ix)
	ld	-19 (ix), a
	ld	a, -46 (ix)
	ld	-18 (ix), a
	ld	a, -45 (ix)
	ld	-17 (ix), a
	ld	a, -42 (ix)
	ld	-16 (ix), a
	ld	a, -41 (ix)
	ld	-15 (ix), a
00150$:
;ff.c:4122: for ( ; btw > 0; btw -= wcnt, *bw += wcnt, wbuff += wcnt, fp->fptr += wcnt, fp->obj.objsize = (fp->fptr > fp->obj.objsize) ? fp->fptr : fp->obj.objsize) {	/* Repeat until all data written */
	ld	a, 5 (ix)
	or	a, 4 (ix)
	jp	Z, 00148$
;ff.c:4123: if (fp->fptr % SS(fs) == 0) {		/* On the sector boundary? */
	ld	e, -44 (ix)
	ld	d, -43 (ix)
	ld	hl, #50
	add	hl, sp
	ex	de, hl
	ld	bc, #0x0004
	ldir
	ld	a, -8 (ix)
	or	a, a
	jp	NZ,00144$
	bit	0, -7 (ix)
	jp	NZ,00144$
;ff.c:4124: csect = (UINT)(fp->fptr / SS(fs)) & (fs->csize - 1);	/* Sector offset in the cluster */
	ld	c, -7 (ix)
	ld	b, -6 (ix)
	ld	e, -5 (ix)
	srl	e
	rr	b
	rr	c
	ld	e, -54 (ix)
	ld	d, -53 (ix)
	ld	hl, #10
	add	hl, de
	ld	e, (hl)
	inc	hl
	ld	d, (hl)
	dec	de
	ld	a, c
	and	a, e
	ld	c, a
	ld	a, b
	and	a, d
	ld	-14 (ix), c
;ff.c:4125: if (csect == 0) {				/* On the cluster boundary? */
	ld	-13 (ix), a
	or	a, -14 (ix)
	jp	NZ, 00124$
;ff.c:4126: if (fp->fptr == 0) {		/* On the top of the file? */
	ld	a, -5 (ix)
	or	a, -6 (ix)
	or	a, -7 (ix)
	or	a, -8 (ix)
	jr	NZ, 00113$
;ff.c:4127: clst = fp->obj.sclust;	/* Follow from the origin */
	ld	e, -32 (ix)
	ld	d, -31 (ix)
	ld	hl, #50
	add	hl, sp
	ex	de, hl
	ld	bc, #0x0004
	ldir
;ff.c:4128: if (clst == 0) {		/* If no cluster is allocated, */
	ld	a, -5 (ix)
	or	a, -6 (ix)
	or	a, -7 (ix)
	or	a, -8 (ix)
	jr	NZ, 00114$
;ff.c:4129: clst = create_chain(&fp->obj, 0);	/* create a new cluster chain */
	ld	hl, #0x0000
	push	hl
	push	hl
	ld	l, -4 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -3 (ix)
;	spillPairReg hl
;	spillPairReg hl
	call	_create_chain
	pop	af
	pop	af
	ld	-8 (ix), e
	ld	-7 (ix), d
	ld	-6 (ix), l
	ld	-5 (ix), h
	jr	00114$
00113$:
;ff.c:4138: clst = create_chain(&fp->obj, fp->clust);	/* Follow or stretch cluster chain on the FAT */
	ld	e, -28 (ix)
	ld	d, -27 (ix)
	ld	hl, #50
	add	hl, sp
	ex	de, hl
	ld	bc, #0x0004
	ldir
	ld	l, -6 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -5 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	ld	l, -8 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -7 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	ld	l, -4 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -3 (ix)
;	spillPairReg hl
;	spillPairReg hl
	call	_create_chain
	pop	af
	pop	af
	ld	-8 (ix), e
	ld	-7 (ix), d
	ld	-6 (ix), l
	ld	-5 (ix), h
00114$:
;ff.c:4141: if (clst == 0) break;		/* Could not allocate a new cluster (disk full) */
	ld	a, -5 (ix)
	or	a, -6 (ix)
	or	a, -7 (ix)
	or	a, -8 (ix)
	jp	Z, 00148$
;ff.c:4142: if (clst == 1) ABORT(fs, FR_INT_ERR);
	ld	a, -8 (ix)
	dec	a
	or	a, -7 (ix)
	or	a, -6 (ix)
	or	a, -5 (ix)
	jr	NZ, 00118$
	ld	l, -50 (ix)
	ld	h, -49 (ix)
	ld	a,#0x02
	ld	(hl),a
	jp	00151$
00118$:
;ff.c:4143: if (clst == 0xFFFFFFFF) ABORT(fs, FR_DISK_ERR);
	ld	a, -8 (ix)
	and	a, -7 (ix)
	and	a, -6 (ix)
	and	a, -5 (ix)
	inc	a
	jr	NZ, 00120$
	ld	l, -50 (ix)
	ld	h, -49 (ix)
	ld	a,#0x01
	ld	(hl),a
	jp	00151$
00120$:
;ff.c:4144: fp->clust = clst;			/* Update current cluster */
	ld	e, -28 (ix)
	ld	d, -27 (ix)
	ld	hl, #50
	add	hl, sp
	ld	bc, #0x0004
	ldir
;ff.c:4145: if (fp->obj.sclust == 0) fp->obj.sclust = clst;	/* Set start cluster if the first write */
	ld	l, -34 (ix)
	ld	h, -33 (ix)
	ld	c, (hl)
	inc	hl
	ld	b, (hl)
	inc	hl
	ld	a, (hl)
	inc	hl
	or	a, (hl)
	or	a, b
	or	a, c
	jr	NZ, 00124$
	ld	e, -34 (ix)
	ld	d, -33 (ix)
	ld	hl, #50
	add	hl, sp
	ld	bc, #0x0004
	ldir
00124$:
;ff.c:4150: if (fp->flag & FA_DIRTY) {		/* Write-back sector cache */
	ld	l, -48 (ix)
	ld	h, -47 (ix)
	ld	a, (hl)
	rlca
	jr	NC, 00128$
;ff.c:4151: if (disk_write(fs->pdrv, fp->buf, fp->sect, 1) != RES_OK) ABORT(fs, FR_DISK_ERR);
	ld	l, -30 (ix)
	ld	h, -29 (ix)
	ld	e, (hl)
	inc	hl
	ld	d, (hl)
	inc	hl
	ld	b, (hl)
	inc	hl
	ld	a, (hl)
	ld	l, -54 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -53 (ix)
;	spillPairReg hl
;	spillPairReg hl
	inc	hl
	ld	c, (hl)
	ld	hl, #0x0001
	push	hl
	ld	l, b
;	spillPairReg hl
;	spillPairReg hl
	ld	h, a
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	push	de
	ld	e, -42 (ix)
	ld	d, -41 (ix)
	ld	a, c
	call	_disk_write
	or	a, a
	jr	Z, 00126$
	ld	l, -50 (ix)
	ld	h, -49 (ix)
	ld	a,#0x01
	ld	(hl),a
	jp	00151$
00126$:
;ff.c:4152: fp->flag &= (BYTE)~FA_DIRTY;
	ld	l, -48 (ix)
	ld	h, -47 (ix)
	ld	a, (hl)
	res	7, a
	ld	l, -48 (ix)
	ld	h, -47 (ix)
	ld	(hl), a
00128$:
;ff.c:4155: sect = clst2sect(fs, fp->clust);	/* Get current sector */
	ld	l, -26 (ix)
	ld	h, -25 (ix)
	ld	c, (hl)
	inc	hl
	ld	b, (hl)
	inc	hl
	ld	e, (hl)
	inc	hl
	ld	d, (hl)
	push	de
	push	bc
	ld	l, -54 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -53 (ix)
;	spillPairReg hl
;	spillPairReg hl
	call	_clst2sect
	pop	af
	pop	af
;ff.c:4156: if (sect == 0) ABORT(fs, FR_INT_ERR);
	ld	a, h
	or	a, l
	or	a, d
	or	a, e
	jr	NZ, 00130$
	ld	l, -50 (ix)
	ld	h, -49 (ix)
	ld	a,#0x02
	ld	(hl),a
	jp	00151$
00130$:
;ff.c:4157: sect += csect;
	ld	a, -14 (ix)
	push	iy
	ex	(sp), hl
	ld	h, -13 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ex	(sp), hl
	pop	iy
	ld	bc, #0x0000
	add	a, e
	ld	e, a
	push	iy
	ld	a, -59 (ix)
	pop	iy
	adc	a, d
	ld	d, a
	ld	a, c
	adc	a, l
	ld	c, a
	ld	a, b
	adc	a, h
	ld	b, a
	ld	-12 (ix), e
	ld	-11 (ix), d
	ld	-10 (ix), c
	ld	-9 (ix), b
;ff.c:4158: cc = btw / SS(fs);				/* When remaining bytes >= sector size, */
	ld	a, 5 (ix)
	srl	a
	ld	c, a
	ld	b, #0x00
;ff.c:4124: csect = (UINT)(fp->fptr / SS(fs)) & (fs->csize - 1);	/* Sector offset in the cluster */
	ld	e, -54 (ix)
	ld	d, -53 (ix)
;ff.c:4163: if (disk_write(fs->pdrv, wbuff, sect, cc) != RES_OK) ABORT(fs, FR_DISK_ERR);
	ld	l, e
	ld	h, d
	inc	hl
	ld	-8 (ix), l
	ld	-7 (ix), h
;ff.c:4159: if (cc > 0) {					/* Write maximum contiguous sectors directly */
	ld	a, b
	or	a, c
	jp	Z, 00138$
;ff.c:4160: if (csect + cc > fs->csize) {	/* Clip at cluster boundary */
	ld	a, -14 (ix)
	add	a, c
	ld	-6 (ix), a
	ld	a, -13 (ix)
	adc	a, b
	ld	-5 (ix), a
	ld	hl, #10
	add	hl, de
	ld	e, (hl)
	inc	hl
	ld	d, (hl)
	ld	l, -6 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -5 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	a, e
	sub	a, l
	ld	a, d
	sbc	a, h
	jr	NC, 00132$
;ff.c:4161: cc = fs->csize - csect;
	ld	a, e
	sub	a, -14 (ix)
	ld	c, a
	ld	a, d
	sbc	a, -13 (ix)
	ld	b, a
00132$:
;ff.c:4163: if (disk_write(fs->pdrv, wbuff, sect, cc) != RES_OK) ABORT(fs, FR_DISK_ERR);
	ld	l, -8 (ix)
	ld	h, -7 (ix)
	ld	l, (hl)
;	spillPairReg hl
	push	bc
	push	bc
	ld	e, -10 (ix)
	ld	d, -9 (ix)
	push	de
	ld	e, -12 (ix)
	ld	d, -11 (ix)
	push	de
	ld	e, -2 (ix)
	ld	d, -1 (ix)
	ld	a, l
	call	_disk_write
	pop	bc
	or	a, a
	jr	Z, 00134$
	ld	l, -50 (ix)
	ld	h, -49 (ix)
	ld	a,#0x01
	ld	(hl),a
	jp	00151$
00134$:
;ff.c:4171: if (fp->sect - sect < cc) { /* Refill sector cache if it gets invalidated by the direct write */
	push	bc
	ld	e, -36 (ix)
	ld	d, -35 (ix)
	ld	hl, #2
	add	hl, sp
	ex	de, hl
	ld	bc, #0x0004
	ldir
	pop	bc
	ld	a, -58 (ix)
	sub	a, -12 (ix)
	ld	-8 (ix), a
	ld	a, -57 (ix)
	sbc	a, -11 (ix)
	ld	-7 (ix), a
	ld	a, -56 (ix)
	sbc	a, -10 (ix)
	ld	-6 (ix), a
	ld	a, -55 (ix)
	sbc	a, -9 (ix)
	ld	-5 (ix), a
	ld	e, c
	ld	d, b
	ld	hl, #0x0000
	ld	a, -8 (ix)
	sub	a, e
	ld	a, -7 (ix)
	sbc	a, d
	ld	a, -6 (ix)
	sbc	a, l
	ld	a, -5 (ix)
	sbc	a, h
	jr	NC, 00136$
;ff.c:4172: memcpy(fp->buf, wbuff + ((fp->sect - sect) * SS(fs)), SS(fs));
	ld	a, -40 (ix)
	ld	-6 (ix), a
	ld	a, -39 (ix)
	ld	-5 (ix), a
	pop	hl
	push	hl
	ld	e, -12 (ix)
	ld	d, -11 (ix)
	cp	a, a
	sbc	hl, de
	ld	a, l
	add	a, a
	ld	d, a
	ld	e, #0x00
	ld	l, -2 (ix)
	ld	h, -1 (ix)
	add	hl, de
	push	bc
	ld	e, -6 (ix)
	ld	d, -5 (ix)
	ld	bc, #0x0200
	ldir
	pop	bc
;ff.c:4173: fp->flag &= (BYTE)~FA_DIRTY;
	ld	l, -48 (ix)
	ld	h, -47 (ix)
	ld	a, (hl)
	res	7, a
	ld	l, -48 (ix)
	ld	h, -47 (ix)
	ld	(hl), a
00136$:
;ff.c:4177: wcnt = SS(fs) * cc;		/* Number of bytes transferred */
	ld	a, c
	add	a, a
	ld	b, a
	ld	c, #0x00
;ff.c:4178: continue;
	jp	00147$
00138$:
;ff.c:4186: if (fp->sect != sect && 		/* Fill sector cache with file data */
	ld	l, -38 (ix)
	ld	h, -37 (ix)
	ld	c, (hl)
	inc	hl
	ld	b, (hl)
	inc	hl
	ld	e, (hl)
	inc	hl
	ld	d, (hl)
	ld	a, -12 (ix)
	sub	a, c
	jr	NZ, 00293$
	ld	a, -11 (ix)
	sub	a, b
	jr	NZ, 00293$
	ld	l, -10 (ix)
	ld	h, -9 (ix)
	cp	a, a
	sbc	hl, de
	jr	Z, 00140$
00293$:
;ff.c:4187: fp->fptr < fp->obj.objsize &&
	ld	e, -24 (ix)
	ld	d, -23 (ix)
	ld	hl, #0
	add	hl, sp
	ex	de, hl
	ld	bc, #0x0004
	ldir
	ld	l, -20 (ix)
	ld	h, -19 (ix)
	ld	c, (hl)
	inc	hl
	ld	b, (hl)
	inc	hl
	ld	e, (hl)
	inc	hl
	ld	d, (hl)
	ld	a, -58 (ix)
	sub	a, c
	ld	a, -57 (ix)
	sbc	a, b
	ld	a, -56 (ix)
	sbc	a, e
	ld	a, -55 (ix)
	sbc	a, d
	jr	NC, 00140$
;ff.c:4188: disk_read(fs->pdrv, fp->buf, sect, 1) != RES_OK) {
	ld	l, -8 (ix)
	ld	h, -7 (ix)
	ld	c, (hl)
	ld	hl, #0x0001
	push	hl
	ld	l, -10 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -9 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	ld	l, -12 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -11 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	ld	e, -42 (ix)
	ld	d, -41 (ix)
	ld	a, c
	call	_disk_read
	or	a, a
	jr	Z, 00140$
;ff.c:4189: ABORT(fs, FR_DISK_ERR);
	ld	l, -50 (ix)
	ld	h, -49 (ix)
	ld	a,#0x01
	ld	(hl),a
	jp	00151$
00140$:
;ff.c:4192: fp->sect = sect;
	ld	e, -38 (ix)
	ld	d, -37 (ix)
	ld	hl, #46
	add	hl, sp
	ld	bc, #0x0004
	ldir
00144$:
;ff.c:4194: wcnt = SS(fs) - (UINT)fp->fptr % SS(fs);	/* Number of bytes remains in the sector */
	ld	e, -18 (ix)
	ld	d, -17 (ix)
	ld	hl, #50
	add	hl, sp
	ex	de, hl
	ld	bc, #0x0004
	ldir
	ld	c, -8 (ix)
	ld	a, -7 (ix)
	and	a, #0x01
	ld	b, a
	xor	a, a
	sub	a, c
	ld	c, a
	ld	a, #0x02
	sbc	a, b
	ld	b, a
;ff.c:4195: if (wcnt > btw) wcnt = btw;					/* Clip it by btw if needed */
	ld	a, 4 (ix)
	sub	a, c
	ld	a, 5 (ix)
	sbc	a, b
	jr	NC, 00146$
	ld	c, 4 (ix)
	ld	b, 5 (ix)
00146$:
;ff.c:4201: memcpy(fp->buf + fp->fptr % SS(fs), wbuff, wcnt);	/* Fit data to the sector */
	ld	e, -8 (ix)
	ld	a, -7 (ix)
	and	a, #0x01
	ld	d, a
	ld	a, e
	add	a, -16 (ix)
	ld	e, a
	ld	a, d
	adc	a, -15 (ix)
	ld	d, a
	ld	l, -2 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -1 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	bc
	ld	a, b
	or	a, c
	jr	Z, 00294$
	ldir
00294$:
	pop	bc
;ff.c:4202: fp->flag |= FA_DIRTY;
	ld	l, -48 (ix)
	ld	h, -47 (ix)
	ld	a, (hl)
	set	7, a
	ld	l, -48 (ix)
	ld	h, -47 (ix)
	ld	(hl), a
00147$:
;ff.c:4122: for ( ; btw > 0; btw -= wcnt, *bw += wcnt, wbuff += wcnt, fp->fptr += wcnt, fp->obj.objsize = (fp->fptr > fp->obj.objsize) ? fp->fptr : fp->obj.objsize) {	/* Repeat until all data written */
	ld	a, 4 (ix)
	sub	a, c
	ld	4 (ix), a
	ld	a, 5 (ix)
	sbc	a, b
	ld	5 (ix), a
	ld	l, -52 (ix)
	ld	h, -51 (ix)
	ld	a, (hl)
	inc	hl
	ld	h, (hl)
;	spillPairReg hl
	ld	l, a
;	spillPairReg hl
;	spillPairReg hl
	add	hl, bc
	ex	de, hl
	ld	l, -52 (ix)
	ld	h, -51 (ix)
	ld	(hl), e
	inc	hl
	ld	(hl), d
	ld	a, c
	add	a, -2 (ix)
	ld	-2 (ix), a
	ld	a, b
	adc	a, -1 (ix)
	ld	-1 (ix), a
	push	bc
	ld	e, -46 (ix)
	ld	d, -45 (ix)
	ld	hl, #48
	add	hl, sp
	ex	de, hl
	ld	bc, #0x0004
	ldir
	pop	bc
	ld	de, #0x0000
	ld	a, -12 (ix)
	add	a, c
	ld	-8 (ix), a
	ld	a, -11 (ix)
	adc	a, b
	ld	-7 (ix), a
	ld	a, -10 (ix)
	adc	a, e
	ld	-6 (ix), a
	ld	a, -9 (ix)
	adc	a, d
	ld	-5 (ix), a
	ld	e, -46 (ix)
	ld	d, -45 (ix)
	ld	hl, #50
	add	hl, sp
	ld	bc, #0x0004
	ldir
	ld	l, -22 (ix)
	ld	h, -21 (ix)
	ld	c, (hl)
	inc	hl
	ld	b, (hl)
	inc	hl
	ld	e, (hl)
	inc	hl
	ld	d, (hl)
	ld	a, c
	sub	a, -8 (ix)
	ld	a, b
	sbc	a, -7 (ix)
	ld	a, e
	sbc	a, -6 (ix)
	ld	a, d
	sbc	a, -5 (ix)
	jr	NC, 00155$
	ld	e, -46 (ix)
	ld	d, -45 (ix)
	ld	hl, #50
	add	hl, sp
	ex	de, hl
	ld	bc, #0x0004
	ldir
	jr	00156$
00155$:
	ld	-8 (ix), c
	ld	-7 (ix), b
	ld	-6 (ix), e
	ld	-5 (ix), d
00156$:
	ld	e, -22 (ix)
	ld	d, -21 (ix)
	ld	hl, #50
	add	hl, sp
	ld	bc, #0x0004
	ldir
	jp	00150$
00148$:
;ff.c:4206: fp->flag |= FA_MODIFIED;				/* Set file change flag */
	ld	l, -48 (ix)
	ld	h, -47 (ix)
	ld	a, (hl)
	set	6, a
	ld	l, -48 (ix)
	ld	h, -47 (ix)
	ld	(hl), a
;ff.c:4208: LEAVE_FF(fs, FR_OK);
	xor	a, a
00151$:
;ff.c:4209: }
	ld	sp, ix
	pop	ix
	pop	hl
	pop	bc
	pop	bc
	jp	(hl)
;ff.c:4218: FRESULT f_sync (
;	---------------------------------
; Function f_sync
; ---------------------------------
_f_sync::
	push	ix
	ld	ix,#0
	add	ix,sp
	ld	iy, #-10
	add	iy, sp
	ld	sp, iy
	ld	c, l
	ld	b, h
;ff.c:4226: res = validate(&fp->obj, &fs);	/* Check validity of the file object */
	push	bc
	ld	hl, #2
	add	hl, sp
	ex	de, hl
	ld	l, c
;	spillPairReg hl
;	spillPairReg hl
	ld	h, b
;	spillPairReg hl
;	spillPairReg hl
	call	_validate
	pop	bc
;ff.c:4227: if (res == FR_OK) {
	ld	e, a
	or	a, a
	jp	NZ, 00110$
;ff.c:4228: if (fp->flag & FA_MODIFIED) {	/* Is there any change to the file? */
	ld	hl, #0x000e
	add	hl, bc
	ld	-8 (ix), l
	ld	-7 (ix), h
	ld	a, (hl)
	bit	6, a
	jp	Z,00110$
;ff.c:4230: if (fp->flag & FA_DIRTY) {	/* Write-back cached data if needed */
	rlca
	jr	NC, 00104$
;ff.c:4231: if (disk_write(fs->pdrv, fp->buf, fp->sect, 1) != RES_OK) LEAVE_FF(fs, FR_DISK_ERR);
	ld	e, c
	ld	d, b
	push	bc
	ld	hl, #8
	add	hl, sp
	ex	de, hl
	ld	bc, #0x0018
	add	hl, bc
	ld	bc, #0x0004
	ldir
	pop	bc
	ld	iy, #0x0022
	add	iy, bc
	pop	hl
	push	hl
	inc	hl
	ld	l, (hl)
;	spillPairReg hl
	push	bc
	ld	de, #0x0001
	push	de
	ld	e, -2 (ix)
	ld	d, -1 (ix)
	push	de
	ld	e, -4 (ix)
	ld	d, -3 (ix)
	push	de
	push	iy
	pop	de
	ld	a, l
	call	_disk_write
	pop	bc
	or	a, a
	jr	Z, 00102$
	ld	a, #0x01
	jp	00111$
00102$:
;ff.c:4232: fp->flag &= (BYTE)~FA_DIRTY;
	ld	l, -8 (ix)
	ld	h, -7 (ix)
	ld	a, (hl)
	res	7, a
	pop	de
	pop	hl
	push	hl
	push	de
	ld	(hl), a
00104$:
;ff.c:4270: res = move_window(fs, fp->dir_sect);
	ld	e, c
	ld	d, b
	ld	hl, #28
	add	hl, de
	ld	e, (hl)
	inc	hl
	ld	d, (hl)
	inc	hl
	inc	hl
	ld	a, (hl)
	dec	hl
	ld	l, (hl)
;	spillPairReg hl
	ld	h, a
;	spillPairReg hl
;	spillPairReg hl
	push	bc
	push	hl
	push	de
	ld	l, -10 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -9 (ix)
;	spillPairReg hl
;	spillPairReg hl
	call	_move_window
	pop	bc
;ff.c:4271: if (res == FR_OK) {
	ld	e, a
	or	a, a
	jp	NZ, 00110$
;ff.c:4272: BYTE *dir = fp->dir_ptr;
	ld	e, c
	ld	d, b
	ld	hl, #32
	add	hl, de
	ld	a, (hl)
	ld	-6 (ix), a
	inc	hl
	ld	a, (hl)
	ld	-5 (ix), a
;ff.c:4274: dir[DIR_Attr] |= AM_ARC;					/* Set archive attribute to indicate that the file has been changed */
	ld	l, -6 (ix)
	ld	h, -5 (ix)
	ld	de, #0x000b
	add	hl, de
	set	5, (hl)
;ff.c:4275: st_clust(fp->obj.fs, dir, fp->obj.sclust);	/* Update file allocation information  */
	ld	e, c
	ld	d, b
	push	bc
	ld	hl, #8
	add	hl, sp
	ex	de, hl
	ld	bc, #0x0006
	add	hl, bc
	ld	bc, #0x0004
	ldir
	pop	bc
	ld	l, c
	ld	h, b
	ld	a, (hl)
	inc	hl
	ld	h, (hl)
;	spillPairReg hl
	ld	l, a
;	spillPairReg hl
;	spillPairReg hl
	push	bc
	ld	e, -2 (ix)
	ld	d, -1 (ix)
	push	de
	ld	e, -4 (ix)
	ld	d, -3 (ix)
	push	de
	ld	e, -6 (ix)
	ld	d, -5 (ix)
	call	_st_clust
	pop	bc
;ff.c:4276: st_32(dir + DIR_FileSize, (DWORD)fp->obj.objsize);	/* Update file size */
	ld	hl, #10
	add	hl, bc
	ld	c, (hl)
	inc	hl
	ld	b, (hl)
	inc	hl
	ld	e, (hl)
	inc	hl
	ld	d, (hl)
	ld	a, -6 (ix)
	add	a, #0x1c
	ld	l, a
;	spillPairReg hl
;	spillPairReg hl
	ld	a, -5 (ix)
	adc	a, #0x00
	ld	h, a
;	spillPairReg hl
;	spillPairReg hl
	push	de
	push	bc
	call	_st_32
;ff.c:4277: st_32(dir + DIR_ModTime, GET_FATTIME());	/* Update modified time */
	call	_get_fattime
	ld	c, l
	ld	b, h
	ld	a, -6 (ix)
	add	a, #0x16
	ld	l, a
;	spillPairReg hl
;	spillPairReg hl
	ld	a, -5 (ix)
	adc	a, #0x00
	ld	h, a
;	spillPairReg hl
;	spillPairReg hl
	push	bc
	push	de
	call	_st_32
;ff.c:4278: st_16(dir + DIR_LstAccDate, 0);				/* Invalidate last access date */
	ld	l, -6 (ix)
	ld	h, -5 (ix)
	ld	de, #0x0012
	add	hl, de
	ld	de, #0x0000
	call	_st_16
;ff.c:4279: fs->wflag = 1;
	pop	bc
	push	bc
	ld	hl, #0x0004
	add	hl, bc
	ld	(hl), #0x01
;ff.c:4280: res = sync_fs(fs);							/* Restore it to the directory */
	pop	hl
	push	hl
	call	_sync_fs
	ld	e, a
;ff.c:4281: fp->flag &= (BYTE)~FA_MODIFIED;
	ld	l, -8 (ix)
	ld	h, -7 (ix)
	ld	a, (hl)
	res	6, a
	pop	bc
	pop	hl
	push	hl
	push	bc
	ld	(hl), a
00110$:
;ff.c:4287: LEAVE_FF(fs, res);
	ld	a, e
00111$:
;ff.c:4288: }
	ld	sp, ix
	pop	ix
	ret
;ff.c:4299: FRESULT f_close (
;	---------------------------------
; Function f_close
; ---------------------------------
_f_close::
	push	ix
	ld	ix,#0
	add	ix,sp
	push	af
	ld	c, l
	ld	b, h
;ff.c:4307: res = f_sync(fp);					/* Flush cached data */
	push	bc
	ld	l, c
;	spillPairReg hl
;	spillPairReg hl
	ld	h, b
;	spillPairReg hl
;	spillPairReg hl
	call	_f_sync
	pop	bc
;ff.c:4308: if (res == FR_OK)
	or	a, a
	jr	NZ, 00104$
;ff.c:4311: res = validate(&fp->obj, &fs);	/* Lock volume */
	push	bc
	ld	hl, #2
	add	hl, sp
	ex	de, hl
	ld	l, c
;	spillPairReg hl
;	spillPairReg hl
	ld	h, b
;	spillPairReg hl
;	spillPairReg hl
	call	_validate
	pop	bc
;ff.c:4312: if (res == FR_OK) {
	or	a, a
	jr	NZ, 00104$
;ff.c:4317: fp->obj.fs = 0;	/* Invalidate file object */
	push	af
	xor	a, a
	ld	(bc), a
	inc	bc
	xor	a, a
	ld	(bc), a
	pop	af
00104$:
;ff.c:4324: return res;
;ff.c:4325: }
	ld	sp, ix
	pop	ix
	ret
;ff.c:4555: FRESULT f_lseek (
;	---------------------------------
; Function f_lseek
; ---------------------------------
_f_lseek::
	push	ix
	ld	ix,#0
	add	ix,sp
	ld	iy, #-35
	add	iy, sp
	ld	sp, iy
	ld	-2 (ix), l
	ld	-1 (ix), h
;ff.c:4567: res = validate(&fp->obj, &fs);		/* Check validity of the file object */
	ld	hl, #4
	add	hl, sp
	ex	de, hl
	ld	l, -2 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -1 (ix)
;	spillPairReg hl
;	spillPairReg hl
	call	_validate
	ld	-29 (ix), a
;ff.c:4568: if (res == FR_OK) res = (FRESULT)fp->err;
	ld	a, -2 (ix)
	add	a, #0x0f
	ld	-28 (ix), a
	ld	a, -1 (ix)
	adc	a, #0x00
	ld	-27 (ix), a
	ld	a, -29 (ix)
	or	a, a
	jr	NZ, 00102$
	ld	l, -28 (ix)
	ld	h, -27 (ix)
	ld	a, (hl)
	ld	-29 (ix), a
00102$:
;ff.c:4574: if (res != FR_OK) LEAVE_FF(fs, res);
	ld	a, -29 (ix)
	or	a, a
	jr	Z, 00104$
	ld	a, -29 (ix)
	jp	00155$
00104$:
;ff.c:4637: if (ofs > fp->obj.objsize && (FF_FS_READONLY || !(fp->flag & FA_WRITE))) {	/* In read-only mode, clip offset with the file size */
	ld	a, -2 (ix)
	add	a, #0x0a
	ld	-26 (ix), a
	ld	a, -1 (ix)
	adc	a, #0x00
	ld	-25 (ix), a
	ld	e, -26 (ix)
	ld	d, -25 (ix)
	ld	hl, #29
	add	hl, sp
	ex	de, hl
	ld	bc, #0x0004
	ldir
	ld	a, -2 (ix)
	add	a, #0x0e
	ld	-24 (ix), a
	ld	a, -1 (ix)
	adc	a, #0x00
	ld	-23 (ix), a
	ld	a, -6 (ix)
	sub	a, 4 (ix)
	ld	a, -5 (ix)
	sbc	a, 5 (ix)
	ld	a, -4 (ix)
	sbc	a, 6 (ix)
	ld	a, -3 (ix)
	sbc	a, 7 (ix)
	jr	NC, 00106$
	ld	l, -24 (ix)
	ld	h, -23 (ix)
	bit	1, (hl)
	jr	NZ, 00106$
;ff.c:4638: ofs = fp->obj.objsize;
	ld	hl, #39
	add	hl, sp
	ex	de, hl
	ld	hl, #29
	add	hl, sp
	ld	bc, #4
	ldir
00106$:
;ff.c:4640: ifptr = fp->fptr;
	ld	a, -2 (ix)
	add	a, #0x10
	ld	c, a
	ld	a, -1 (ix)
	adc	a, #0x00
	ld	b, a
	push	bc
	ld	e, c
	ld	d, b
	ld	hl, #2
	add	hl, sp
	ex	de, hl
	ld	bc, #0x0004
	ldir
	pop	bc
;ff.c:4641: fp->fptr = nsect = 0;
	xor	a, a
	ld	-22 (ix), a
	ld	-21 (ix), a
	ld	-20 (ix), a
	ld	-19 (ix), a
	ld	l, c
	ld	h, b
	xor	a, a
	ld	(hl), a
	inc	hl
	ld	(hl), a
	inc	hl
	ld	(hl), a
	inc	hl
	ld	(hl), a
;ff.c:4642: if (ofs > 0) {
	ld	a, 7 (ix)
	or	a, 6 (ix)
	or	a, 5 (ix)
	or	a, 4 (ix)
	jp	Z, 00145$
;ff.c:4643: bcs = (DWORD)fs->csize * SS(fs);	/* Cluster size (byte) */
	ld	e, -31 (ix)
	ld	d, -30 (ix)
	ld	hl, #10
	add	hl, de
	ld	e, (hl)
	inc	hl
	ld	d, (hl)
	ld	hl, #0x0000
	ld	h, l
;	spillPairReg hl
;	spillPairReg hl
	ld	l, d
;	spillPairReg hl
;	spillPairReg hl
	ld	d, e
	ld	e, #0x00
	sla	d
	adc	hl, hl
	ld	-18 (ix), e
	ld	-17 (ix), d
	ld	-16 (ix), l
	ld	-15 (ix), h
;ff.c:4648: clst = fp->clust;
	ld	a, -2 (ix)
	add	a, #0x14
	ld	-14 (ix), a
	ld	a, -1 (ix)
	adc	a, #0x00
	ld	-13 (ix), a
;ff.c:4644: if (ifptr > 0 &&
	ld	a, -32 (ix)
	or	a, -33 (ix)
	or	a, -34 (ix)
	or	a, -35 (ix)
	jp	Z, 00116$
;ff.c:4645: (ofs - 1) / bcs >= (ifptr - 1) / bcs) {	/* When seek to same or following cluster, */
	ld	a, 4 (ix)
	add	a, #0xff
	ld	e, a
	ld	a, 5 (ix)
	adc	a, #0xff
	ld	d, a
	ld	a, 6 (ix)
	adc	a, #0xff
	ld	l, a
;	spillPairReg hl
;	spillPairReg hl
	ld	a, 7 (ix)
	adc	a, #0xff
	ld	h, a
;	spillPairReg hl
;	spillPairReg hl
	push	bc
	push	hl
	ld	l, -16 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -15 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ex	(sp), hl
	push	hl
	ld	l, -18 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -17 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ex	(sp), hl
	call	__divulong
	pop	af
	pop	af
	ld	-10 (ix), e
	ld	-9 (ix), d
	ld	-8 (ix), l
	ld	-7 (ix), h
	pop	bc
	ld	a, -35 (ix)
	add	a, #0xff
	ld	-6 (ix), a
	ld	a, -34 (ix)
	adc	a, #0xff
	ld	-5 (ix), a
	ld	a, -33 (ix)
	adc	a, #0xff
	ld	-4 (ix), a
	ld	a, -32 (ix)
	adc	a, #0xff
	ld	-3 (ix), a
	push	bc
	ld	l, -16 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -15 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	ld	l, -18 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -17 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	ld	e, -6 (ix)
	ld	d, -5 (ix)
	ld	l, -4 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -3 (ix)
;	spillPairReg hl
;	spillPairReg hl
	call	__divulong
	pop	af
	pop	af
	pop	bc
	ld	a, -10 (ix)
	sub	a, e
	ld	a, -9 (ix)
	sbc	a, d
	ld	a, -8 (ix)
	sbc	a, l
	ld	a, -7 (ix)
	sbc	a, h
	jp	C, 00116$
;ff.c:4646: fp->fptr = (ifptr - 1) & ~(FSIZE_t)(bcs - 1);	/* start from the current cluster */
	ld	a, -18 (ix)
	add	a, #0xff
	ld	e, a
	ld	a, -17 (ix)
	adc	a, #0xff
	ld	d, a
	ld	a, -16 (ix)
	adc	a, #0xff
	ld	l, a
;	spillPairReg hl
;	spillPairReg hl
	ld	a, -15 (ix)
	adc	a, #0xff
	ld	h, a
;	spillPairReg hl
;	spillPairReg hl
	ld	a, e
	cpl
	push	af
	ld	a, d
	cpl
	ld	e, a
	ld	a, l
	cpl
	ld	l, a
;	spillPairReg hl
;	spillPairReg hl
	ld	a, h
	cpl
	ld	h, a
;	spillPairReg hl
;	spillPairReg hl
	pop	af
	and	a, -6 (ix)
	ld	-10 (ix), a
	ld	a, e
	and	a, -5 (ix)
	ld	-9 (ix), a
	ld	a, l
	and	a, -4 (ix)
	ld	-8 (ix), a
	ld	a, h
	and	a, -3 (ix)
	ld	-7 (ix), a
	push	bc
	ld	e, c
	ld	d, b
	ld	hl, #27
	add	hl, sp
	ld	bc, #0x0004
	ldir
	pop	bc
;ff.c:4647: ofs -= fp->fptr;
	ld	a, 4 (ix)
	sub	a, -10 (ix)
	ld	4 (ix), a
	ld	a, 5 (ix)
	sbc	a, -9 (ix)
	ld	5 (ix), a
	ld	a, 6 (ix)
	sbc	a, -8 (ix)
	ld	6 (ix), a
	ld	a, 7 (ix)
	sbc	a, -7 (ix)
	ld	7 (ix), a
;ff.c:4648: clst = fp->clust;
	push	bc
	ld	e, -14 (ix)
	ld	d, -13 (ix)
	ld	hl, #25
	add	hl, sp
	ex	de, hl
	ld	bc, #0x0004
	ldir
	pop	bc
	jp	00117$
00116$:
;ff.c:4650: clst = fp->obj.sclust;					/* start from the first cluster */
	ld	a, -2 (ix)
	add	a, #0x06
	ld	-4 (ix), a
	ld	a, -1 (ix)
	adc	a, #0x00
	ld	-3 (ix), a
	push	bc
	ld	e, -4 (ix)
	ld	d, -3 (ix)
	ld	hl, #25
	add	hl, sp
	ex	de, hl
	ld	bc, #0x0004
	ldir
	pop	bc
;ff.c:4652: if (clst == 0) {						/* If no cluster chain, create a new chain */
	ld	a, -9 (ix)
	or	a, -10 (ix)
	or	a, -11 (ix)
	or	a, -12 (ix)
	jr	NZ, 00114$
;ff.c:4653: clst = create_chain(&fp->obj, 0);
	push	bc
	ld	hl, #0x0000
	push	hl
	push	hl
	ld	l, -2 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -1 (ix)
;	spillPairReg hl
;	spillPairReg hl
	call	_create_chain
	pop	af
	pop	af
	pop	bc
	ld	-12 (ix), e
	ld	-11 (ix), d
	ld	-10 (ix), l
	ld	-9 (ix), h
;ff.c:4654: if (clst == 1) ABORT(fs, FR_INT_ERR);
	ld	a, -12 (ix)
	dec	a
	or	a, -11 (ix)
	or	a, -10 (ix)
	or	a, -9 (ix)
	jr	NZ, 00110$
	ld	l, -28 (ix)
	ld	h, -27 (ix)
	ld	a,#0x02
	ld	(hl),a
	jp	00155$
00110$:
;ff.c:4655: if (clst == 0xFFFFFFFF) ABORT(fs, FR_DISK_ERR);
	ld	a, -12 (ix)
	and	a, -11 (ix)
	and	a, -10 (ix)
	and	a, -9 (ix)
	inc	a
	jr	NZ, 00112$
	ld	l, -28 (ix)
	ld	h, -27 (ix)
	ld	a,#0x01
	ld	(hl),a
	jp	00155$
00112$:
;ff.c:4656: fp->obj.sclust = clst;
	push	bc
	ld	e, -4 (ix)
	ld	d, -3 (ix)
	ld	hl, #25
	add	hl, sp
	ld	bc, #0x0004
	ldir
	pop	bc
00114$:
;ff.c:4659: fp->clust = clst;
	push	bc
	ld	e, -14 (ix)
	ld	d, -13 (ix)
	ld	hl, #25
	add	hl, sp
	ld	bc, #0x0004
	ldir
	pop	bc
00117$:
;ff.c:4661: if (clst != 0) {
	ld	a, -9 (ix)
	or	a, -10 (ix)
	or	a, -11 (ix)
	or	a, -12 (ix)
	jp	Z, 00145$
;ff.c:4662: while (ofs > bcs) {						/* Cluster following loop */
	ld	a, -24 (ix)
	ld	-8 (ix), a
	ld	a, -23 (ix)
	ld	-7 (ix), a
00132$:
	ld	a, -18 (ix)
	sub	a, 4 (ix)
	ld	a, -17 (ix)
	sbc	a, 5 (ix)
	ld	a, -16 (ix)
	sbc	a, 6 (ix)
	ld	a, -15 (ix)
	sbc	a, 7 (ix)
	jp	NC, 00134$
;ff.c:4663: ofs -= bcs; fp->fptr += bcs;
	ld	a, 4 (ix)
	sub	a, -18 (ix)
	ld	4 (ix), a
	ld	a, 5 (ix)
	sbc	a, -17 (ix)
	ld	5 (ix), a
	ld	a, 6 (ix)
	sbc	a, -16 (ix)
	ld	6 (ix), a
	ld	a, 7 (ix)
	sbc	a, -15 (ix)
	ld	7 (ix), a
	ld	l, c
	ld	h, b
	ld	e, (hl)
	inc	hl
	ld	d, (hl)
	inc	hl
	inc	hl
	ld	a, (hl)
	dec	hl
	ld	l, (hl)
;	spillPairReg hl
	ld	h, a
;	spillPairReg hl
;	spillPairReg hl
	ld	a, e
	add	a, -18 (ix)
	ld	-6 (ix), a
	ld	a, d
	adc	a, -17 (ix)
	ld	-5 (ix), a
	ld	a, l
	adc	a, -16 (ix)
	ld	-4 (ix), a
	ld	a, h
	adc	a, -15 (ix)
	ld	-3 (ix), a
	push	bc
	ld	e, c
	ld	d, b
	ld	hl, #31
	add	hl, sp
	ld	bc, #0x0004
	ldir
	pop	bc
;ff.c:4665: if (fp->flag & FA_WRITE) {			/* Check if in write mode or not */
	ld	l, -8 (ix)
	ld	h, -7 (ix)
	bit	1, (hl)
	jr	Z, 00125$
;ff.c:4670: clst = create_chain(&fp->obj, clst);	/* Follow chain with forceed stretch */
	push	bc
	ld	l, -10 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -9 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	ld	l, -12 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -11 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	ld	l, -2 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -1 (ix)
;	spillPairReg hl
;	spillPairReg hl
	call	_create_chain
	pop	af
	pop	af
	pop	bc
	ld	-12 (ix), e
	ld	-11 (ix), d
	ld	-10 (ix), l
;ff.c:4671: if (clst == 0) {				/* Clip file size in case of disk full */
	ld	-9 (ix), h
	ld	a, h
	or	a, -10 (ix)
	or	a, -11 (ix)
	or	a, -12 (ix)
	jr	NZ, 00126$
;ff.c:4672: ofs = 0; break;
	xor	a, a
	ld	4 (ix), a
	ld	5 (ix), a
	ld	6 (ix), a
	ld	7 (ix), a
	jp	00134$
00125$:
;ff.c:4677: clst = get_fat(&fp->obj, clst);	/* Follow cluster chain if not in write mode */
	push	bc
	ld	l, -10 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -9 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	ld	l, -12 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -11 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	ld	l, -2 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -1 (ix)
;	spillPairReg hl
;	spillPairReg hl
	call	_get_fat
	pop	af
	pop	af
	pop	bc
	ld	-12 (ix), e
	ld	-11 (ix), d
	ld	-10 (ix), l
	ld	-9 (ix), h
00126$:
;ff.c:4679: if (clst == 0xFFFFFFFF) ABORT(fs, FR_DISK_ERR);
	ld	a, -12 (ix)
	and	a, -11 (ix)
	and	a, -10 (ix)
	and	a, -9 (ix)
	inc	a
	jr	NZ, 00128$
	ld	l, -28 (ix)
	ld	h, -27 (ix)
	ld	a,#0x01
	ld	(hl),a
	jp	00155$
00128$:
;ff.c:4680: if (clst <= 1 || clst >= fs->n_fatent) ABORT(fs, FR_INT_ERR);
	ld	a, #0x01
	cp	a, -12 (ix)
	ld	a, #0x00
	sbc	a, -11 (ix)
	ld	a, #0x00
	sbc	a, -10 (ix)
	ld	a, #0x00
	sbc	a, -9 (ix)
	jr	NC, 00129$
	ld	e, -31 (ix)
	ld	d, -30 (ix)
	ld	hl, #20
	add	hl, de
	ld	e, (hl)
	inc	hl
	ld	d, (hl)
	inc	hl
	inc	hl
	ld	a, (hl)
	dec	hl
	ld	l, (hl)
;	spillPairReg hl
	ld	h, a
;	spillPairReg hl
;	spillPairReg hl
	ld	a, -12 (ix)
	sub	a, e
	ld	a, -11 (ix)
	sbc	a, d
	ld	a, -10 (ix)
	sbc	a, l
	ld	a, -9 (ix)
	sbc	a, h
	jr	C, 00130$
00129$:
	ld	l, -28 (ix)
	ld	h, -27 (ix)
	ld	a,#0x02
	ld	(hl),a
	jp	00155$
00130$:
;ff.c:4681: fp->clust = clst;
	push	bc
	ld	e, -14 (ix)
	ld	d, -13 (ix)
	ld	hl, #25
	add	hl, sp
	ld	bc, #0x0004
	ldir
	pop	bc
	jp	00132$
00134$:
;ff.c:4683: fp->fptr += ofs;
	push	bc
	ld	e, c
	ld	d, b
	ld	hl, #21
	add	hl, sp
	ex	de, hl
	ld	bc, #0x0004
	ldir
	pop	bc
	ld	a, -16 (ix)
	add	a, 4 (ix)
	ld	-6 (ix), a
	ld	a, -15 (ix)
	adc	a, 5 (ix)
	ld	-5 (ix), a
	ld	a, -14 (ix)
	adc	a, 6 (ix)
	ld	-4 (ix), a
	ld	a, -13 (ix)
	adc	a, 7 (ix)
	ld	-3 (ix), a
	push	bc
	ld	e, c
	ld	d, b
	ld	hl, #31
	add	hl, sp
	ld	bc, #0x0004
	ldir
	pop	bc
;ff.c:4684: if (ofs % SS(fs)) {
	ld	a, 4 (ix)
	or	a, a
	jr	NZ, 00292$
	bit	0, 5 (ix)
	jr	Z, 00145$
00292$:
;ff.c:4685: nsect = clst2sect(fs, clst);	/* Current sector */
	push	bc
	ld	l, -10 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -9 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	ld	l, -12 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -11 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	ld	l, -31 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -30 (ix)
;	spillPairReg hl
;	spillPairReg hl
	call	_clst2sect
	pop	af
	pop	af
	pop	bc
;ff.c:4686: if (nsect == 0) ABORT(fs, FR_INT_ERR);
	ld	a, h
	or	a, l
	or	a, d
	or	a, e
	jr	NZ, 00136$
	ld	l, -28 (ix)
	ld	h, -27 (ix)
	ld	a,#0x02
	ld	(hl),a
	jp	00155$
00136$:
;ff.c:4687: nsect += (DWORD)(ofs / SS(fs));
	ld	a, 5 (ix)
	ld	-6 (ix), a
	ld	a, 6 (ix)
	ld	-5 (ix), a
	ld	a, 7 (ix)
	ld	-4 (ix), a
	ld	-3 (ix), #0x00
	srl	-4 (ix)
	rr	-5 (ix)
	rr	-6 (ix)
	ld	a, e
	add	a, -6 (ix)
	ld	-22 (ix), a
	ld	a, d
	adc	a, -5 (ix)
	ld	-21 (ix), a
	ld	a, l
	adc	a, -4 (ix)
	ld	-20 (ix), a
	ld	a, h
	adc	a, -3 (ix)
	ld	-19 (ix), a
;ff.c:4691: if (!FF_FS_READONLY && fp->fptr > fp->obj.objsize) {	/* Set file change flag if the file size is extended */
00145$:
	push	bc
	ld	e, c
	ld	d, b
	ld	hl, #31
	add	hl, sp
	ex	de, hl
	ld	bc, #0x0004
	ldir
	pop	bc
	ld	l, -26 (ix)
	ld	h, -25 (ix)
	ld	e, (hl)
	inc	hl
	ld	d, (hl)
	inc	hl
	inc	hl
	ld	a, (hl)
	dec	hl
	ld	l, (hl)
;	spillPairReg hl
	ld	h, a
;	spillPairReg hl
;	spillPairReg hl
	ld	a, e
	sub	a, -6 (ix)
	ld	a, d
	sbc	a, -5 (ix)
	ld	a, l
	sbc	a, -4 (ix)
	ld	a, h
	sbc	a, -3 (ix)
	jr	NC, 00144$
;ff.c:4692: fp->obj.objsize = fp->fptr;
	push	bc
	ld	e, -26 (ix)
	ld	d, -25 (ix)
	ld	hl, #31
	add	hl, sp
	ld	bc, #0x0004
	ldir
	pop	bc
;ff.c:4693: fp->flag |= FA_MODIFIED;
	ld	l, -24 (ix)
	ld	h, -23 (ix)
	ld	a, (hl)
	set	6, a
	ld	l, -24 (ix)
	ld	h, -23 (ix)
	ld	(hl), a
00144$:
;ff.c:4695: if (fp->fptr % SS(fs) && nsect != fp->sect) {	/* Fill sector cache if needed */
	ld	l, c
	ld	h, b
	ld	c, (hl)
	inc	hl
	ld	b, (hl)
	inc	hl
	ld	e, (hl)
	inc	hl
	ld	d, (hl)
	ld	a, c
	or	a, a
	jr	NZ, 00295$
	bit	0, b
	jp	Z,00153$
00295$:
	ld	a, -2 (ix)
	add	a, #0x18
	ld	-4 (ix), a
	ld	a, -1 (ix)
	adc	a, #0x00
	ld	-3 (ix), a
	ld	l, -4 (ix)
	ld	h, -3 (ix)
	ld	c, (hl)
	inc	hl
	ld	b, (hl)
	inc	hl
	ld	e, (hl)
	inc	hl
	ld	d, (hl)
	ld	a, -22 (ix)
	sub	a, c
	jr	NZ, 00296$
	ld	a, -21 (ix)
	sub	a, b
	jr	NZ, 00296$
	ld	l, -20 (ix)
	ld	h, -19 (ix)
	cp	a, a
	sbc	hl, de
	jp	Z,00153$
00296$:
;ff.c:4698: if (fp->flag & FA_DIRTY) {			/* Write-back dirty sector cache */
	ld	l, -24 (ix)
	ld	h, -23 (ix)
	ld	h, (hl)
;	spillPairReg hl
;ff.c:4699: if (disk_write(fs->pdrv, fp->buf, fp->sect, 1) != RES_OK) ABORT(fs, FR_DISK_ERR);
	ld	iy, #0x0022
	push	bc
	ld	c, -2 (ix)
	ld	b, -1 (ix)
	add	iy, bc
	pop	bc
;ff.c:4698: if (fp->flag & FA_DIRTY) {			/* Write-back dirty sector cache */
	add	hl, hl
	jr	NC, 00149$
;ff.c:4699: if (disk_write(fs->pdrv, fp->buf, fp->sect, 1) != RES_OK) ABORT(fs, FR_DISK_ERR);
	ld	l, -31 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -30 (ix)
;	spillPairReg hl
;	spillPairReg hl
	inc	hl
	ld	l, (hl)
;	spillPairReg hl
	push	iy
	push	hl
	ld	hl, #0x0001
	ex	(sp), hl
	push	de
	push	bc
	push	iy
	pop	de
	ld	a, l
	call	_disk_write
	pop	iy
	or	a, a
	jr	Z, 00147$
	ld	l, -28 (ix)
	ld	h, -27 (ix)
	ld	a,#0x01
	ld	(hl),a
	jr	00155$
00147$:
;ff.c:4700: fp->flag &= (BYTE)~FA_DIRTY;
	ld	l, -24 (ix)
	ld	h, -23 (ix)
	ld	a, (hl)
	res	7, a
	ld	l, -24 (ix)
	ld	h, -23 (ix)
	ld	(hl), a
00149$:
;ff.c:4703: if (disk_read(fs->pdrv, fp->buf, nsect, 1) != RES_OK) ABORT(fs, FR_DISK_ERR);	/* Fill sector cache */
	ld	l, -31 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -30 (ix)
;	spillPairReg hl
;	spillPairReg hl
	inc	hl
	ld	c, (hl)
	ld	hl, #0x0001
	push	hl
	ld	l, -20 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -19 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	ld	l, -22 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -21 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	push	iy
	pop	de
	ld	a, c
	call	_disk_read
	or	a, a
	jr	Z, 00151$
	ld	l, -28 (ix)
	ld	h, -27 (ix)
	ld	a,#0x01
	ld	(hl),a
	jr	00155$
00151$:
;ff.c:4705: fp->sect = nsect;
	ld	e, -4 (ix)
	ld	d, -3 (ix)
	ld	hl, #13
	add	hl, sp
	ld	bc, #0x0004
	ldir
00153$:
;ff.c:4709: LEAVE_FF(fs, res);
	ld	a, -29 (ix)
00155$:
;ff.c:4710: }
	ld	sp, ix
	pop	ix
	pop	hl
	pop	bc
	pop	bc
	jp	(hl)
;ff.c:4719: FRESULT f_opendir (
;	---------------------------------
; Function f_opendir
; ---------------------------------
_f_opendir::
	push	ix
	ld	ix,#0
	add	ix,sp
	ld	iy, #-11
	add	iy, sp
	ld	sp, iy
	ld	c, l
	ld	b, h
	ld	-2 (ix), e
	ld	-1 (ix), d
;ff.c:4729: if (!dp) return FR_INVALID_OBJECT;	/* Reject null pointer */
	ld	a, b
	or	a, c
	jr	NZ, 00102$
	ld	a, #0x09
	jp	00118$
00102$:
;ff.c:4731: res = mount_volume(&path, &fs, 0);	/* Get logical drive and mount the volume if needed */
	push	bc
	xor	a, a
	push	af
	inc	sp
	ld	hl, #3
	add	hl, sp
	ex	de, hl
	ld	hl, #12
	add	hl, sp
	call	_mount_volume
	pop	bc
;ff.c:4732: if (res == FR_OK) {
	ld	-9 (ix), a
	or	a, a
	jp	NZ, 00115$
;ff.c:4733: dp->obj.fs = fs;
	ld	l, c
	ld	h, b
	ld	a, -11 (ix)
	ld	(hl), a
	inc	hl
	ld	a, -10 (ix)
	ld	(hl), a
;ff.c:4735: res = follow_path(dp, path);			/* Follow the path to the directory */
	push	bc
	ld	e, -2 (ix)
	ld	d, -1 (ix)
	ld	l, c
;	spillPairReg hl
;	spillPairReg hl
	ld	h, b
;	spillPairReg hl
;	spillPairReg hl
	call	_follow_path
	pop	bc
;ff.c:4736: if (res == FR_OK) {						/* Follow completed */
	ld	-9 (ix), a
	or	a, a
	jp	NZ, 00111$
;ff.c:4737: if (!(dp->fn[NSFLAG] & NS_NONAME)) {	/* It is neither the origin directory itself nor dot name in exFAT */
	ld	hl, #0x001c
	add	hl, bc
	ld	de, #0x000b
	add	hl, de
	ld	a, (hl)
	rlca
	jr	C, 00107$
;ff.c:4738: if (dp->obj.attr & AM_DIR) {		/* This object is a sub-directory */
	ld	e, c
	ld	d, b
	ld	hl, #4
	add	hl, de
	bit	4, (hl)
	jr	Z, 00104$
;ff.c:4745: dp->obj.sclust = ld_clust(fs, dp->dir);	/* Get object allocation info */
	ld	hl, #0x0006
	add	hl, bc
	ld	-8 (ix), l
	ld	-7 (ix), h
	ld	e, c
	ld	d, b
	ld	hl, #26
	add	hl, de
	ld	e, (hl)
	inc	hl
	ld	d, (hl)
	push	bc
	ld	l, -11 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -10 (ix)
;	spillPairReg hl
;	spillPairReg hl
	call	_ld_clust
	ld	-6 (ix), e
	ld	-5 (ix), d
	ld	-4 (ix), l
	ld	-3 (ix), h
	ld	e, -8 (ix)
	ld	d, -7 (ix)
	ld	hl, #7
	add	hl, sp
	ld	bc, #0x0004
	ldir
	pop	bc
	jr	00107$
00104$:
;ff.c:4748: res = FR_NO_PATH;
	ld	-9 (ix), #0x05
00107$:
;ff.c:4751: if (res == FR_OK) {
	ld	a, -9 (ix)
	or	a, a
	jr	NZ, 00111$
;ff.c:4752: dp->obj.id = fs->id;		/* Set current volume mount ID */
	ld	hl, #0x0002
	add	hl, bc
	ld	-4 (ix), l
	ld	-3 (ix), h
	pop	de
	push	de
	ld	hl, #6
	add	hl, de
	ld	e, (hl)
	inc	hl
	ld	d, (hl)
	ld	l, -4 (ix)
	ld	h, -3 (ix)
	ld	(hl), e
	inc	hl
	ld	(hl), d
;ff.c:4753: res = dir_sdi(dp, 0);		/* Rewind directory */
	push	bc
	ld	hl, #0x0000
	push	hl
	push	hl
	ld	l, c
;	spillPairReg hl
;	spillPairReg hl
	ld	h, b
;	spillPairReg hl
;	spillPairReg hl
	call	_dir_sdi
	pop	bc
	ld	-9 (ix), a
00111$:
;ff.c:4767: if (res == FR_NO_FILE) res = FR_NO_PATH;
	ld	a, -9 (ix)
	sub	a, #0x04
	jr	NZ, 00115$
	ld	-9 (ix), #0x05
00115$:
;ff.c:4769: if (res != FR_OK) dp->obj.fs = 0;		/* Invalidate the directory object if function failed */
	ld	a, -9 (ix)
	or	a, a
	jr	Z, 00117$
	xor	a, a
	ld	(bc), a
	inc	bc
	ld	(bc), a
00117$:
;ff.c:4771: LEAVE_FF(fs, res);
	ld	a, -9 (ix)
00118$:
;ff.c:4772: }
	ld	sp, ix
	pop	ix
	ret
;ff.c:4781: FRESULT f_closedir (
;	---------------------------------
; Function f_closedir
; ---------------------------------
_f_closedir::
	push	ix
	ld	ix,#0
	add	ix,sp
	push	af
	ld	c, l
	ld	b, h
;ff.c:4789: res = validate(&dp->obj, &fs);	/* Check validity of the file object */
	push	bc
	ld	hl, #2
	add	hl, sp
	ex	de, hl
	ld	l, c
;	spillPairReg hl
;	spillPairReg hl
	ld	h, b
;	spillPairReg hl
;	spillPairReg hl
	call	_validate
	pop	bc
	ld	e, a
;ff.c:4790: if (res == FR_OK) {
;ff.c:4795: dp->obj.fs = 0;	/* Invalidate directory object */
	or	a,a
	jr	NZ, 00102$
	ld	(bc), a
	inc	bc
	ld	(bc), a
00102$:
;ff.c:4801: return res;
	ld	a, e
;ff.c:4802: }
	ld	sp, ix
	pop	ix
	ret
;ff.c:4811: FRESULT f_readdir (
;	---------------------------------
; Function f_readdir
; ---------------------------------
_f_readdir::
	push	ix
	ld	ix,#0
	add	ix,sp
	push	af
	push	af
	ld	c, l
	ld	b, h
	ld	-2 (ix), e
	ld	-1 (ix), d
;ff.c:4821: res = validate(&dp->obj, &fs);	/* Check validity of the directory object */
	push	bc
	ld	hl, #2
	add	hl, sp
	ex	de, hl
	ld	l, c
;	spillPairReg hl
;	spillPairReg hl
	ld	h, b
;	spillPairReg hl
;	spillPairReg hl
	call	_validate
	pop	bc
	ld	e, a
;ff.c:4827: fno->fname[0] = 0;				/* Clear file information */
	ld	a, -2 (ix)
	add	a, #0x09
	ld	l, a
;	spillPairReg hl
;	spillPairReg hl
	ld	a, -1 (ix)
	adc	a, #0x00
	ld	h, a
;	spillPairReg hl
;	spillPairReg hl
;ff.c:4822: if (res == FR_OK) {
	ld	a, e
	or	a, a
	jr	NZ, 00111$
;ff.c:4823: if (!fno) {
	ld	a, -1 (ix)
	or	a, -2 (ix)
	jr	NZ, 00108$
;ff.c:4824: res = dir_sdi(dp, 0);		/* Rewind the directory object */
	push	hl
	ld	de, #0x0000
	push	de
	push	de
	ld	l, c
;	spillPairReg hl
;	spillPairReg hl
	ld	h, b
;	spillPairReg hl
;	spillPairReg hl
	call	_dir_sdi
	pop	hl
	ld	e, a
	jr	00111$
00108$:
;ff.c:4827: fno->fname[0] = 0;				/* Clear file information */
	ld	(hl), #0x00
;ff.c:4828: res = DIR_READ_FILE(dp);		/* Read an item */
	push	hl
	push	bc
	ld	de, #0x0000
	ld	l, c
;	spillPairReg hl
;	spillPairReg hl
	ld	h, b
;	spillPairReg hl
;	spillPairReg hl
	call	_dir_read
	pop	bc
	pop	hl
;ff.c:4829: if (res == FR_NO_FILE) res = FR_OK;	/* Ignore end of directory */
	ld	e, a
	sub	a,#0x04
	jr	NZ, 00102$
	ld	e,a
00102$:
;ff.c:4830: if (res == FR_OK) {				/* A valid entry is found */
	ld	a, e
	or	a, a
	jr	NZ, 00111$
;ff.c:4831: get_fileinfo(dp, fno);		/* Get the object information */
	push	hl
	push	bc
	ld	e, -2 (ix)
	ld	d, -1 (ix)
	ld	l, c
;	spillPairReg hl
;	spillPairReg hl
	ld	h, b
;	spillPairReg hl
;	spillPairReg hl
	call	_get_fileinfo
	pop	bc
	ld	de, #0x0000
	ld	l, c
;	spillPairReg hl
;	spillPairReg hl
	ld	h, b
;	spillPairReg hl
;	spillPairReg hl
	call	_dir_next
	pop	hl
;ff.c:4833: if (res == FR_NO_FILE) res = FR_OK;	/* Ignore end of directory now */
	ld	e, a
	sub	a,#0x04
	jr	NZ, 00111$
	ld	e,a
00111$:
;ff.c:4839: if (fno && res != FR_OK) fno->fname[0] = 0;	/* Clear the file information if any error occured */
	ld	a, -1 (ix)
	or	a, -2 (ix)
	jr	Z, 00113$
	ld	a, e
	or	a, a
	jr	Z, 00113$
	ld	(hl), #0x00
00113$:
;ff.c:4840: LEAVE_FF(fs, res);
	ld	a, e
;ff.c:4841: }
	ld	sp, ix
	pop	ix
	ret
;ff.c:4902: FRESULT f_stat (
;	---------------------------------
; Function f_stat
; ---------------------------------
_f_stat::
	push	ix
	ld	ix,#0
	add	ix,sp
	ld	iy, #-43
	add	iy, sp
	ld	sp, iy
	ld	-2 (ix), l
	ld	-1 (ix), h
;ff.c:4913: res = mount_volume(&path, &dj.obj.fs, 0);
	push	de
	xor	a, a
	push	af
	inc	sp
	ld	hl, #3
	add	hl, sp
	ex	de, hl
	ld	hl, #44
	add	hl, sp
	call	_mount_volume
	pop	bc
;ff.c:4915: if (res == FR_OK) {
	ld	-3 (ix), a
	or	a, a
	jr	NZ, 00109$
;ff.c:4917: res = follow_path(&dj, path);	/* Follow the file path */
	push	bc
	ld	e, -2 (ix)
	ld	d, -1 (ix)
	ld	hl, #2
	add	hl, sp
	call	_follow_path
	pop	bc
;ff.c:4918: if (res == FR_OK) {				/* Follow completed */
	ld	-3 (ix), a
	or	a, a
	jr	NZ, 00109$
;ff.c:4919: if (dj.fn[NSFLAG] & NS_NONAME) {	/* It is origin directory */
	ld	a, -4 (ix)
	rlca
	jr	NC, 00104$
;ff.c:4920: res = FR_INVALID_NAME;
	ld	-3 (ix), #0x06
	jr	00109$
00104$:
;ff.c:4922: if (fno) get_fileinfo(&dj, fno);
	ld	a, b
	or	a, c
	jr	Z, 00109$
	push	bc
	ld	e, c
	ld	d, b
	ld	hl, #2
	add	hl, sp
	call	_get_fileinfo
	pop	bc
00109$:
;ff.c:4928: if (fno && res != FR_OK) fno->fname[0] = 0;	/* Invalidate the file information if an error occured */
	ld	a, b
	or	a, c
	jr	Z, 00111$
	ld	a, -3 (ix)
	or	a, a
	jr	Z, 00111$
	ld	hl, #0x0009
	add	hl, bc
	ld	(hl), #0x00
00111$:
;ff.c:4929: LEAVE_FF(dj.obj.fs, res);
	ld	a, -3 (ix)
;ff.c:4930: }
	ld	sp, ix
	pop	ix
	ret
;ff.c:4939: FRESULT f_getfree (
;	---------------------------------
; Function f_getfree
; ---------------------------------
_f_getfree::
	push	ix
	ld	ix,#0
	add	ix,sp
	ld	iy, #-39
	add	iy, sp
	ld	sp, iy
	ld	-10 (ix), l
	ld	-9 (ix), h
	ld	-12 (ix), e
	ld	-11 (ix), d
;ff.c:4954: res = mount_volume(&path, &fs, 0);
	xor	a, a
	push	af
	inc	sp
	ld	hl, #1
	add	hl, sp
	ex	de, hl
	ld	hl, #30
	add	hl, sp
	call	_mount_volume
;ff.c:4956: if (res == FR_OK) {
	ld	-23 (ix), a
	or	a, a
	jp	NZ, 00133$
;ff.c:4957: *fatfs = fs;				/* Return ptr to the fs object */
	ld	c, 4 (ix)
	ld	b, 5 (ix)
	ld	a, -39 (ix)
	ld	(bc), a
	inc	bc
	ld	a, -38 (ix)
	ld	(bc), a
;ff.c:4959: if (fs->free_clst <= fs->n_fatent - 2) {
	pop	bc
	push	bc
	ld	e, c
	ld	d, b
	push	bc
	ld	hl, #33
	add	hl, sp
	ex	de, hl
	ld	bc, #0x0010
	add	hl, bc
	ld	bc, #0x0004
	ldir
	pop	bc
	ld	e, c
	ld	d, b
	push	bc
	ld	hl, #37
	add	hl, sp
	ex	de, hl
	ld	bc, #0x0014
	add	hl, bc
	ld	bc, #0x0004
	ldir
	pop	bc
	ld	a, -4 (ix)
	add	a, #0xfe
	ld	e, a
	ld	a, -3 (ix)
	adc	a, #0xff
	ld	d, a
	ld	a, -2 (ix)
	adc	a, #0xff
	ld	l, a
;	spillPairReg hl
;	spillPairReg hl
	ld	a, -1 (ix)
	adc	a, #0xff
	ld	h, a
;	spillPairReg hl
;	spillPairReg hl
	ld	a, e
	sub	a, -8 (ix)
	ld	a, d
	sbc	a, -7 (ix)
	ld	a, l
	sbc	a, -6 (ix)
	ld	a, h
	sbc	a, -5 (ix)
	jr	C, 00130$
;ff.c:4960: *nclst = fs->free_clst;
	ld	e, -12 (ix)
	ld	d, -11 (ix)
	ld	hl, #31
	add	hl, sp
	ld	bc, #0x0004
	ldir
	jp	00133$
00130$:
;ff.c:4963: nfree = 0;
	xor	a, a
	ld	-22 (ix), a
	ld	-21 (ix), a
	ld	-20 (ix), a
	ld	-19 (ix), a
;ff.c:4964: if (fs->fs_type == FS_FAT12) {	/* FAT12: Scan bit field FAT entries */
	ld	a, (bc)
	dec	a
	jp	NZ,00125$
;ff.c:4965: clst = 2; obj.fs = fs;
	ld	-37 (ix), c
	ld	-36 (ix), b
;ff.c:4966: do {
	xor	a, a
	ld	-8 (ix), a
	ld	-7 (ix), a
	ld	-6 (ix), a
	ld	-5 (ix), a
	ld	-4 (ix), #0x02
	xor	a, a
	ld	-3 (ix), a
	ld	-2 (ix), a
	ld	-1 (ix), a
00107$:
;ff.c:4967: stat = get_fat(&obj, clst);
	ld	l, -2 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -1 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	ld	l, -4 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -3 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	ld	hl, #6
	add	hl, sp
	call	_get_fat
	pop	af
	pop	af
	ld	c, e
	ld	b, d
;ff.c:4968: if (stat == 0xFFFFFFFF) {
	ld	a, c
	and	a, b
	and	a, l
	and	a, h
	inc	a
	jr	NZ, 00102$
;ff.c:4969: res = FR_DISK_ERR; break;
	ld	-23 (ix), #0x01
	jp	00126$
00102$:
;ff.c:4971: if (stat == 1) {
	ld	a, c
	dec	a
	or	a, b
	or	a, l
	or	a, h
	jr	NZ, 00104$
;ff.c:4972: res = FR_INT_ERR; break;
	ld	-23 (ix), #0x02
	jp	00126$
00104$:
;ff.c:4974: if (stat == 0) nfree++;
	ld	a, h
	or	a, l
	or	a, b
	or	a, c
	jr	NZ, 00108$
	inc	-8 (ix)
	jr	NZ, 00213$
	inc	-7 (ix)
	jr	NZ, 00213$
	inc	-6 (ix)
	jr	NZ, 00213$
	inc	-5 (ix)
00213$:
	ld	hl, #17
	add	hl, sp
	ex	de, hl
	ld	hl, #31
	add	hl, sp
	ld	bc, #4
	ldir
00108$:
;ff.c:4975: } while (++clst < fs->n_fatent);
	inc	-4 (ix)
	jr	NZ, 00214$
	inc	-3 (ix)
	jr	NZ, 00214$
	inc	-2 (ix)
	jr	NZ, 00214$
	inc	-1 (ix)
00214$:
	pop	bc
	push	bc
	ld	hl, #20
	add	hl, bc
	ld	c, (hl)
	inc	hl
	ld	b, (hl)
	inc	hl
	ld	e, (hl)
	inc	hl
	ld	d, (hl)
	ld	a, -4 (ix)
	sub	a, c
	ld	a, -3 (ix)
	sbc	a, b
	ld	a, -2 (ix)
	sbc	a, e
	ld	a, -1 (ix)
	sbc	a, d
	jp	C, 00107$
	jp	00126$
00125$:
;ff.c:4999: clst = fs->n_fatent;	/* Number of entries */
	ld	a, -4 (ix)
	ld	-8 (ix), a
	ld	a, -3 (ix)
	ld	-7 (ix), a
	ld	a, -2 (ix)
	ld	-6 (ix), a
	ld	a, -1 (ix)
	ld	-5 (ix), a
;ff.c:5000: sect = fs->fatbase;		/* Top of the FAT */
	ld	e, c
	ld	d, b
	ld	hl, #35
	add	hl, sp
	ex	de, hl
	ld	bc, #0x0024
	add	hl, bc
	ld	bc, #0x0004
	ldir
;ff.c:5001: i = 0;					/* Offset in the sector */
	xor	a, a
	ld	-18 (ix), a
	ld	-17 (ix), a
;ff.c:5002: do {	/* Counts numbuer of entries with zero in the FAT */
00121$:
;ff.c:5003: if (i == 0) {	/* New sector? */
	ld	a, -17 (ix)
	or	a, -18 (ix)
	jr	NZ, 00113$
;ff.c:5004: res = move_window(fs, sect++);
	ld	c, -4 (ix)
	ld	b, -3 (ix)
	ld	e, -2 (ix)
	ld	d, -1 (ix)
	inc	-4 (ix)
	jr	NZ, 00215$
	inc	-3 (ix)
	jr	NZ, 00215$
	inc	-2 (ix)
	jr	NZ, 00215$
	inc	-1 (ix)
00215$:
	push	de
	push	bc
	ld	l, -39 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -38 (ix)
;	spillPairReg hl
;	spillPairReg hl
	call	_move_window
;ff.c:5005: if (res != FR_OK) break;
	ld	-23 (ix), a
	or	a, a
	jp	NZ, 00126$
00113$:
;ff.c:5007: if (fs->fs_type == FS_FAT16) {
	pop	hl
	push	hl
	ld	c, (hl)
;ff.c:5008: if (ld_16(fs->win + i) == 0) nfree++;	/* FAT16: Is this cluster free? */
	ld	de, #0x0030
	add	hl, de
	ld	a, -22 (ix)
	add	a, #0x01
	ld	-16 (ix), a
	ld	a, -21 (ix)
	adc	a, #0x00
	ld	-15 (ix), a
	ld	a, -20 (ix)
	adc	a, #0x00
	ld	-14 (ix), a
	ld	a, -19 (ix)
	adc	a, #0x00
	ld	-13 (ix), a
	ld	e, -18 (ix)
	ld	d, -17 (ix)
	add	hl, de
;ff.c:5007: if (fs->fs_type == FS_FAT16) {
	ld	a, c
	sub	a, #0x02
	jr	NZ, 00119$
;ff.c:5008: if (ld_16(fs->win + i) == 0) nfree++;	/* FAT16: Is this cluster free? */
	call	_ld_16
	ld	c, e
	ld	a, d
	or	a, c
	jr	NZ, 00115$
	ld	hl, #17
	add	hl, sp
	ex	de, hl
	ld	hl, #23
	add	hl, sp
	ld	bc, #4
	ldir
00115$:
;ff.c:5009: i += 2;	/* Next entry */
	ld	c, -18 (ix)
	ld	b, -17 (ix)
	inc	bc
	inc	bc
	jr	00120$
00119$:
;ff.c:5011: if ((ld_32(fs->win + i) & 0x0FFFFFFF) == 0) nfree++;	/* FAT32: Is this cluster free? */
	call	_ld_32
	ld	a, e
	or	a,a
	jr	NZ, 00117$
	or	a,d
	jr	NZ, 00117$
	or	a,l
	jr	NZ, 00117$
	ld	a, h
	and	a, #0x0f
	jr	NZ, 00117$
	ld	hl, #17
	add	hl, sp
	ex	de, hl
	ld	hl, #23
	add	hl, sp
	ld	bc, #4
	ldir
00117$:
;ff.c:5012: i += 4;	/* Next entry */
	ld	a, -18 (ix)
	add	a, #0x04
	ld	c, a
	ld	a, -17 (ix)
	adc	a, #0x00
	ld	b, a
00120$:
;ff.c:5014: i %= SS(fs);
	ld	-18 (ix), c
	ld	a, b
	and	a, #0x01
	ld	-17 (ix), a
;ff.c:5015: } while (--clst);
	ld	a, -8 (ix)
	add	a, #0xff
	ld	-8 (ix), a
	ld	a, -7 (ix)
	adc	a, #0xff
	ld	-7 (ix), a
	ld	a, -6 (ix)
	adc	a, #0xff
	ld	-6 (ix), a
	ld	a, -5 (ix)
	adc	a, #0xff
	ld	-5 (ix), a
	or	a, -6 (ix)
	or	a, -7 (ix)
	or	a, -8 (ix)
	jp	NZ, 00121$
00126$:
;ff.c:5018: if (res == FR_OK) {		/* Update parameters if succeeded */
	ld	a, -23 (ix)
	or	a, a
	jr	NZ, 00133$
;ff.c:5019: *nclst = nfree;			/* Return the free clusters */
	ld	e, -12 (ix)
	ld	d, -11 (ix)
	ld	hl, #17
	add	hl, sp
	ld	bc, #0x0004
	ldir
;ff.c:5020: fs->free_clst = nfree;	/* Now free cluster count is valid */
	pop	bc
	push	bc
	ld	hl, #0x0010
	add	hl, bc
	ex	de, hl
	ld	hl, #17
	add	hl, sp
	ld	bc, #0x0004
	ldir
;ff.c:5021: fs->fsi_flag |= 1;		/* FAT32/exfAT : Allocation information is to be updated */
	pop	bc
	push	bc
	ld	hl, #0x0005
	add	hl, bc
	set	0, (hl)
00133$:
;ff.c:5026: LEAVE_FF(fs, res);
	ld	a, -23 (ix)
;ff.c:5027: }
	ld	sp, ix
	pop	ix
	pop	hl
	pop	bc
	jp	(hl)
;ff.c:5036: FRESULT f_truncate (
;	---------------------------------
; Function f_truncate
; ---------------------------------
_f_truncate::
	push	ix
	ld	ix,#0
	add	ix,sp
	ld	iy, #-23
	add	iy, sp
	ld	sp, iy
	ld	-2 (ix), l
	ld	-1 (ix), h
;ff.c:5046: res = validate(&fp->obj, &fs);
	ld	hl, #4
	add	hl, sp
	ex	de, hl
	ld	l, -2 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -1 (ix)
;	spillPairReg hl
;	spillPairReg hl
	call	_validate
;ff.c:5047: if (res != FR_OK || (res = (FRESULT)fp->err) != FR_OK) LEAVE_FF(fs, res);
	ld	-17 (ix), a
	or	a, a
	jr	NZ, 00101$
	ld	a, -2 (ix)
	add	a, #0x0f
	ld	-16 (ix), a
	ld	a, -1 (ix)
	adc	a, #0x00
	ld	-15 (ix), a
	ld	l, -16 (ix)
	ld	h, -15 (ix)
	ld	a, (hl)
	ld	-17 (ix), a
	or	a, a
	jr	Z, 00102$
00101$:
	ld	a, -17 (ix)
	jp	00126$
00102$:
;ff.c:5048: if (!(fp->flag & FA_WRITE)) LEAVE_FF(fs, FR_DENIED);	/* Check access mode */
	ld	a, -2 (ix)
	add	a, #0x0e
	ld	-14 (ix), a
	ld	a, -1 (ix)
	adc	a, #0x00
	ld	-13 (ix), a
	ld	l, -14 (ix)
	ld	h, -13 (ix)
	bit	1, (hl)
	jr	NZ, 00105$
	ld	a, #0x07
	jp	00126$
00105$:
;ff.c:5050: if (fp->fptr < fp->obj.objsize) {	/* Process when fptr is not on the eof */
	ld	a, -2 (ix)
	add	a, #0x10
	ld	-12 (ix), a
	ld	a, -1 (ix)
	adc	a, #0x00
	ld	-11 (ix), a
	ld	e, -12 (ix)
	ld	d, -11 (ix)
	ld	hl, #0
	add	hl, sp
	ex	de, hl
	ld	bc, #0x0004
	ldir
	ld	a, -2 (ix)
	add	a, #0x0a
	ld	-10 (ix), a
	ld	a, -1 (ix)
	adc	a, #0x00
	ld	-9 (ix), a
	ld	e, -10 (ix)
	ld	d, -9 (ix)
	ld	hl, #17
	add	hl, sp
	ex	de, hl
	ld	bc, #0x0004
	ldir
	ld	a, -23 (ix)
	sub	a, -6 (ix)
	ld	a, -22 (ix)
	sbc	a, -5 (ix)
	ld	a, -21 (ix)
	sbc	a, -4 (ix)
	ld	a, -20 (ix)
	sbc	a, -3 (ix)
	jp	NC, 00125$
;ff.c:5051: if (fp->fptr == 0) {	/* When set file size to zero, remove entire cluster chain */
	ld	a, -20 (ix)
	or	a, -21 (ix)
	or	a, -22 (ix)
	or	a, -23 (ix)
	jr	NZ, 00114$
;ff.c:5052: res = remove_chain(&fp->obj, fp->obj.sclust, 0);
	ld	l, -2 (ix)
	ld	h, -1 (ix)
	ld	de, #0x0006
	add	hl, de
	push	hl
	ld	c, (hl)
	inc	hl
	ld	b, (hl)
	inc	hl
	ld	e, (hl)
	inc	hl
	ld	d, (hl)
	ld	hl, #0x0000
	push	hl
	push	hl
	push	de
	push	bc
	ld	l, -2 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -1 (ix)
;	spillPairReg hl
;	spillPairReg hl
	call	_remove_chain
	pop	hl
	ld	-17 (ix), a
;ff.c:5053: fp->obj.sclust = 0;
	xor	a, a
	ld	(hl), a
	inc	hl
	ld	(hl), a
	inc	hl
	ld	(hl), a
	inc	hl
	ld	(hl), a
	jp	00115$
00114$:
;ff.c:5055: ncl = get_fat(&fp->obj, fp->clust);
	ld	a, -2 (ix)
	add	a, #0x14
	ld	-8 (ix), a
	ld	a, -1 (ix)
	adc	a, #0x00
	ld	-7 (ix), a
	ld	l, -8 (ix)
	ld	h, -7 (ix)
	ld	c, (hl)
	inc	hl
	ld	b, (hl)
	inc	hl
	ld	e, (hl)
	inc	hl
	ld	d, (hl)
	push	de
	push	bc
	ld	l, -2 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -1 (ix)
;	spillPairReg hl
;	spillPairReg hl
	call	_get_fat
	pop	af
	pop	af
	ld	-6 (ix), e
	ld	-5 (ix), d
	ld	-4 (ix), l
	ld	-3 (ix), h
;ff.c:5056: res = FR_OK;
	ld	-17 (ix), #0x00
;ff.c:5057: if (ncl == 0xFFFFFFFF) res = FR_DISK_ERR;
	ld	a, -6 (ix)
	and	a, -5 (ix)
	and	a, -4 (ix)
	and	a, -3 (ix)
	inc	a
	jr	NZ, 00107$
	ld	-17 (ix), #0x01
00107$:
;ff.c:5058: if (ncl == 1) res = FR_INT_ERR;
	ld	a, -6 (ix)
	dec	a
	or	a, -5 (ix)
	or	a, -4 (ix)
	or	a, -3 (ix)
	jr	NZ, 00109$
	ld	-17 (ix), #0x02
00109$:
;ff.c:5059: if (res == FR_OK && ncl < fs->n_fatent) {
	ld	a, -17 (ix)
	or	a, a
	jr	NZ, 00115$
	ld	c, -19 (ix)
	ld	b, -18 (ix)
	ld	hl, #20
	add	hl, bc
	ld	c, (hl)
	inc	hl
	ld	b, (hl)
	inc	hl
	ld	e, (hl)
	inc	hl
	ld	d, (hl)
	ld	a, -6 (ix)
	sub	a, c
	ld	a, -5 (ix)
	sbc	a, b
	ld	a, -4 (ix)
	sbc	a, e
	ld	a, -3 (ix)
	sbc	a, d
	jr	NC, 00115$
;ff.c:5060: res = remove_chain(&fp->obj, ncl, fp->clust);
	ld	l, -8 (ix)
	ld	h, -7 (ix)
	ld	c, (hl)
	inc	hl
	ld	b, (hl)
	inc	hl
	ld	e, (hl)
	inc	hl
	ld	d, (hl)
	push	de
	push	bc
	ld	l, -4 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -3 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	ld	l, -6 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -5 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	ld	l, -2 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -1 (ix)
;	spillPairReg hl
;	spillPairReg hl
	call	_remove_chain
	ld	-17 (ix), a
00115$:
;ff.c:5063: fp->obj.objsize = fp->fptr;	/* Set file size to current read/write point */
	ld	l, -12 (ix)
	ld	h, -11 (ix)
	ld	c, (hl)
	inc	hl
	ld	b, (hl)
	inc	hl
	ld	e, (hl)
	inc	hl
	ld	d, (hl)
	ld	l, -10 (ix)
	ld	h, -9 (ix)
	ld	(hl), c
	inc	hl
	ld	(hl), b
	inc	hl
	ld	(hl), e
	inc	hl
	ld	(hl), d
;ff.c:5064: fp->flag |= FA_MODIFIED;
	ld	l, -14 (ix)
	ld	h, -13 (ix)
	ld	a, (hl)
	set	6, a
	ld	l, -14 (ix)
	ld	h, -13 (ix)
	ld	(hl), a
;ff.c:5066: if (res == FR_OK && (fp->flag & FA_DIRTY)) {
	ld	a, -17 (ix)
	or	a, a
	jr	NZ, 00120$
	ld	l, -14 (ix)
	ld	h, -13 (ix)
	ld	a, (hl)
	rlca
	jr	NC, 00120$
;ff.c:5067: if (disk_write(fs->pdrv, fp->buf, fp->sect, 1) != RES_OK) {
	ld	c, -2 (ix)
	ld	b, -1 (ix)
	ld	hl, #24
	add	hl, bc
	ld	c, (hl)
	inc	hl
	ld	b, (hl)
	inc	hl
	ld	e, (hl)
	inc	hl
	ld	d, (hl)
	ld	iy, #0x0022
	push	bc
	ld	c, -2 (ix)
	ld	b, -1 (ix)
	add	iy, bc
	pop	bc
	ld	l, -19 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -18 (ix)
;	spillPairReg hl
;	spillPairReg hl
	inc	hl
	ld	l, (hl)
;	spillPairReg hl
	push	hl
	ld	hl, #0x0001
	ex	(sp), hl
	push	de
	push	bc
	push	iy
	pop	de
	ld	a, l
	call	_disk_write
	or	a, a
	jr	Z, 00117$
;ff.c:5068: res = FR_DISK_ERR;
	ld	-17 (ix), #0x01
	jr	00120$
00117$:
;ff.c:5070: fp->flag &= (BYTE)~FA_DIRTY;
	ld	l, -14 (ix)
	ld	h, -13 (ix)
	ld	a, (hl)
	res	7, a
	ld	l, -14 (ix)
	ld	h, -13 (ix)
	ld	(hl), a
00120$:
;ff.c:5074: if (res != FR_OK) ABORT(fs, res);
	ld	a, -17 (ix)
	or	a, a
	jr	Z, 00125$
	ld	l, -16 (ix)
	ld	h, -15 (ix)
	ld	a, -17 (ix)
	ld	(hl), a
	ld	a, -17 (ix)
	jr	00126$
00125$:
;ff.c:5077: LEAVE_FF(fs, res);
	ld	a, -17 (ix)
00126$:
;ff.c:5078: }
	ld	sp, ix
	pop	ix
	ret
;ff.c:5087: FRESULT f_unlink (
;	---------------------------------
; Function f_unlink
; ---------------------------------
_f_unlink::
	push	ix
	ld	ix,#0
	add	ix,sp
	ld	iy, #-89
	add	iy, sp
	ld	sp, iy
	ld	-2 (ix), l
	ld	-1 (ix), h
;ff.c:5094: DWORD dclst = 0;
	xor	a, a
	ld	-6 (ix), a
	ld	-5 (ix), a
	ld	-4 (ix), a
	ld	-3 (ix), a
;ff.c:5101: res = mount_volume(&path, &fs, FA_WRITE);
	ld	a, #0x02
	push	af
	inc	sp
	ld	hl, #1
	add	hl, sp
	ex	de, hl
	ld	hl, #88
	add	hl, sp
	call	_mount_volume
;ff.c:5102: if (res == FR_OK) {
	ld	-7 (ix), a
	or	a, a
	jp	NZ, 00126$
;ff.c:5103: dj.obj.fs = fs;
	ld	a, -89 (ix)
	ld	-87 (ix), a
	ld	a, -88 (ix)
	ld	-86 (ix), a
;ff.c:5105: res = follow_path(&dj, path);	/* Follow the path to the object */
	ld	e, -2 (ix)
	ld	d, -1 (ix)
	ld	hl, #2
	add	hl, sp
	call	_follow_path
;ff.c:5109: } else if (dj.obj.attr & AM_RDO) {
;ff.c:5106: if (res == FR_OK) {
	ld	-7 (ix), a
	or	a, a
	jr	NZ, 00107$
;ff.c:5107: if (dj.fn[NSFLAG] & (NS_DOT | NS_NONAME)) {
	ld	a, -48 (ix)
	and	a, #0xa0
	jr	Z, 00104$
;ff.c:5108: res = FR_INVALID_NAME;	/* It must be a real object */
	ld	-7 (ix), #0x06
	jr	00107$
00104$:
;ff.c:5109: } else if (dj.obj.attr & AM_RDO) {
	ld	a, -83 (ix)
	rrca
	jr	NC, 00107$
;ff.c:5110: res = FR_DENIED;		/* The object must not be read-only */
	ld	-7 (ix), #0x07
00107$:
;ff.c:5117: if (res == FR_OK) {		/* The object is accessible */
	ld	a, -7 (ix)
	or	a, a
	jr	NZ, 00117$
;ff.c:5126: dclst = ld_clust(fs, dj.dir);
	ld	a, -61 (ix)
	ld	-4 (ix), a
	ld	a, -60 (ix)
	ld	-3 (ix), a
	ld	e, -4 (ix)
	ld	d, -3 (ix)
	pop	hl
	push	hl
	call	_ld_clust
	ld	-6 (ix), e
	ld	-5 (ix), d
	ld	-4 (ix), l
	ld	-3 (ix), h
;ff.c:5128: if (dj.obj.attr & AM_DIR) {		/* Is the object a sub-directory? */
	bit	4, -83 (ix)
	jr	Z, 00117$
;ff.c:5135: sdj.obj.fs = fs;		/* Open the sub-directory */
	ld	a, -89 (ix)
	ld	-47 (ix), a
	ld	a, -88 (ix)
	ld	-46 (ix), a
;ff.c:5136: sdj.obj.sclust = dclst;
	ld	hl, #48
	add	hl, sp
	ex	de, hl
	ld	hl, #83
	add	hl, sp
	ld	bc, #0x0004
	ldir
;ff.c:5143: res = dir_sdi(&sdj, 0);
	ld	hl, #0x0000
	push	hl
	push	hl
	ld	hl, #46
	add	hl, sp
	call	_dir_sdi
;ff.c:5144: if (res == FR_OK) {
	ld	-7 (ix), a
	or	a, a
	jr	NZ, 00117$
;ff.c:5145: res = DIR_READ_FILE(&sdj);			/* Check if the sub-directory is empty */
	ld	de, #0x0000
	ld	hl, #42
	add	hl, sp
	call	_dir_read
;ff.c:5146: if (res == FR_OK) res = FR_DENIED;	/* Not empty? */
	ld	-7 (ix), a
	or	a, a
	jr	NZ, 00109$
	ld	-7 (ix), #0x07
00109$:
;ff.c:5147: if (res == FR_NO_FILE) res = FR_OK;	/* Empty? */
	ld	a, -7 (ix)
	sub	a, #0x04
	jr	NZ, 00117$
	ld	-7 (ix), #0x00
00117$:
;ff.c:5152: if (res == FR_OK) {		/* It is ready to remove the object */
	ld	a, -7 (ix)
	or	a, a
	jr	NZ, 00126$
;ff.c:5153: res = dir_remove(&dj);				/* Remove the directory entry */
	ld	hl, #2
	add	hl, sp
	call	_dir_remove
;ff.c:5154: if (res == FR_OK && dclst != 0) {	/* Remove the cluster chain if exist */
	ld	-7 (ix), a
	or	a, a
	jr	NZ, 00119$
	ld	a, -3 (ix)
	or	a, -4 (ix)
	or	a, -5 (ix)
	or	a, -6 (ix)
	jr	Z, 00119$
;ff.c:5158: res = remove_chain(&dj.obj, dclst, 0);
	ld	hl, #0x0000
	push	hl
	push	hl
	ld	l, -4 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -3 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	ld	l, -6 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -5 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	ld	hl, #10
	add	hl, sp
	call	_remove_chain
	ld	-7 (ix), a
00119$:
;ff.c:5161: if (res == FR_OK) res = sync_fs(fs);
	ld	a, -7 (ix)
	or	a, a
	jr	NZ, 00126$
	pop	hl
	push	hl
	call	_sync_fs
	ld	-7 (ix), a
00126$:
;ff.c:5166: LEAVE_FF(fs, res);
	ld	a, -7 (ix)
;ff.c:5167: }
	ld	sp, ix
	pop	ix
	ret
;ff.c:5176: FRESULT f_mkdir (
;	---------------------------------
; Function f_mkdir
; ---------------------------------
_f_mkdir::
	push	ix
	ld	ix,#0
	add	ix,sp
	ld	iy, #-71
	add	iy, sp
	ld	sp, iy
	ld	-2 (ix), l
	ld	-1 (ix), h
;ff.c:5188: res = mount_volume(&path, &fs, FA_WRITE);	/* Get logical drive and mount the volume if needed */
	ld	a, #0x02
	push	af
	inc	sp
	ld	hl, #1
	add	hl, sp
	ex	de, hl
	ld	hl, #70
	add	hl, sp
	call	_mount_volume
;ff.c:5189: if (res == FR_OK) {
	ld	-15 (ix), a
	or	a, a
	jp	NZ, 00124$
;ff.c:5190: dj.obj.fs = fs;
	ld	a, -71 (ix)
	ld	-69 (ix), a
	ld	a, -70 (ix)
	ld	-68 (ix), a
;ff.c:5192: res = follow_path(&dj, path);			/* Follow the file path */
	ld	e, -2 (ix)
	ld	d, -1 (ix)
	ld	hl, #2
	add	hl, sp
	call	_follow_path
;ff.c:5193: if (res == FR_OK) {						/* Invalid name or name collision */
	ld	-15 (ix), a
	or	a, a
	jr	NZ, 00102$
;ff.c:5194: res = (dj.fn[NSFLAG] & (NS_DOT | NS_NONAME)) ? FR_INVALID_NAME : FR_EXIST;
	ld	a, -30 (ix)
	ld	-3 (ix), a
	and	a, #0xa0
	ld	a, #0x06
	jr	NZ, 00128$
	ld	a, #0x08
00128$:
	ld	-15 (ix), a
00102$:
;ff.c:5196: if (res == FR_NO_FILE) {				/* It is clear to create a new directory */
	ld	a, -15 (ix)
	sub	a, #0x04
	jp	NZ,00124$
;ff.c:5197: sobj.fs = fs;						/* New object ID to create a new chain */
	ld	a, -71 (ix)
	ld	-29 (ix), a
	ld	a, -70 (ix)
	ld	-28 (ix), a
;ff.c:5198: dcl = create_chain(&sobj, 0);		/* Allocate a cluster for the new directory */
	ld	hl, #0x0000
	push	hl
	push	hl
	ld	hl, #46
	add	hl, sp
	call	_create_chain
	pop	af
	pop	af
	ld	-6 (ix), e
	ld	-5 (ix), d
	ld	-4 (ix), l
	ld	-3 (ix), h
	ld	hl, #57
	add	hl, sp
	ex	de, hl
	ld	hl, #65
	add	hl, sp
	ld	bc, #4
	ldir
;ff.c:5199: res = FR_OK;
	ld	-15 (ix), #0x00
;ff.c:5200: if (dcl == 0) res = FR_DENIED;		/* No space to allocate a new cluster? */
	ld	a, -11 (ix)
	or	a, -12 (ix)
	or	a, -13 (ix)
	or	a, -14 (ix)
	jr	NZ, 00104$
	ld	-15 (ix), #0x07
00104$:
;ff.c:5201: if (dcl == 1) res = FR_INT_ERR;		/* Any insanity? */
	ld	a, -14 (ix)
	dec	a
	or	a, -13 (ix)
	or	a, -12 (ix)
	or	a, -11 (ix)
	jr	NZ, 00106$
	ld	-15 (ix), #0x02
00106$:
;ff.c:5202: if (dcl == 0xFFFFFFFF) res = FR_DISK_ERR;	/* Disk error? */
	ld	a, -14 (ix)
	and	a, -13 (ix)
	and	a, -12 (ix)
	and	a, -11 (ix)
	inc	a
	jr	NZ, 00108$
	ld	-15 (ix), #0x01
00108$:
;ff.c:5203: tm = GET_FATTIME();
	call	_get_fattime
	ld	-6 (ix), e
	ld	-5 (ix), d
	ld	-4 (ix), l
	ld	-3 (ix), h
	ld	hl, #61
	add	hl, sp
	ex	de, hl
	ld	hl, #65
	add	hl, sp
	ld	bc, #4
	ldir
;ff.c:5204: if (res == FR_OK) {
	ld	a, -15 (ix)
	or	a, a
	jp	NZ, 00115$
;ff.c:5205: res = dir_clear(fs, dcl);		/* Clear the allocated cluster as new direcotry table */
	ld	l, -12 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -11 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	ld	l, -14 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -13 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	ld	l, -71 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -70 (ix)
;	spillPairReg hl
;	spillPairReg hl
	call	_dir_clear
;ff.c:5206: if (res == FR_OK) {
	ld	-15 (ix), a
	or	a, a
	jp	NZ, 00115$
;ff.c:5208: memset(fs->win + DIR_Name, ' ', 11);	/* Create "." entry */
	pop	bc
	push	bc
	ld	hl, #0x0030
	add	hl, bc
	ld	b, #0x0b
00191$:
	ld	(hl), #0x20
	inc	hl
	djnz	00191$
;ff.c:5209: fs->win[DIR_Name] = '.';
	pop	bc
	push	bc
	ld	hl, #0x0030
	add	hl, bc
	ld	(hl), #0x2e
;ff.c:5210: fs->win[DIR_Attr] = AM_DIR;
	pop	bc
	push	bc
	ld	hl, #0x003b
	add	hl, bc
	ld	(hl), #0x10
;ff.c:5211: st_32(fs->win + DIR_ModTime, tm);
	pop	bc
	push	bc
	ld	hl, #0x0046
	add	hl, bc
	ld	e, -8 (ix)
	ld	d, -7 (ix)
	push	de
	ld	e, -10 (ix)
	ld	d, -9 (ix)
	push	de
	call	_st_32
;ff.c:5212: st_clust(fs, fs->win, dcl);
	pop	bc
	push	bc
	ld	hl, #0x0030
	add	hl, bc
	ex	de, hl
	ld	l, -12 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -11 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	ld	l, -14 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -13 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	ld	l, c
;	spillPairReg hl
;	spillPairReg hl
	ld	h, b
;	spillPairReg hl
;	spillPairReg hl
	call	_st_clust
;ff.c:5213: memcpy(fs->win + SZDIRE, fs->win, SZDIRE);	/* Create ".." entry */
	pop	bc
	push	bc
	ld	hl, #0x0050
	add	hl, bc
	ex	de, hl
	ld	hl, #0x0030
	add	hl, bc
	ld	bc, #0x0020
	ldir
;ff.c:5214: fs->win[SZDIRE + 1] = '.'; pcl = dj.obj.sclust;
	pop	bc
	push	bc
	ld	hl, #0x0051
	add	hl, bc
	ld	(hl), #0x2e
	ld	c, -63 (ix)
	ld	b, -62 (ix)
	ld	e, -61 (ix)
	ld	d, -60 (ix)
;ff.c:5215: st_clust(fs, fs->win + SZDIRE, pcl);
	pop	hl
	push	hl
	ld	iy, #0x0050
	push	bc
	ld	c, l
	ld	b, h
	add	iy, bc
	pop	bc
	push	de
	push	bc
	push	iy
	pop	de
	call	_st_clust
;ff.c:5216: fs->wflag = 1;
	pop	bc
	push	bc
	ld	hl, #0x0004
	add	hl, bc
	ld	(hl), #0x01
;ff.c:5218: res = dir_register(&dj);	/* Register the object to the parent directory */
	ld	hl, #2
	add	hl, sp
	call	_dir_register
	ld	-15 (ix), a
00115$:
;ff.c:5221: if (res == FR_OK) {
	ld	a, -15 (ix)
	or	a, a
	jp	NZ, 00119$
;ff.c:5235: st_32(dj.dir + DIR_CrtTime, tm);	/* Created time */
	ld	a, -43 (ix)
	ld	-6 (ix), a
	ld	a, -42 (ix)
	ld	-5 (ix), a
	ld	a, -6 (ix)
	add	a, #0x0e
	ld	-4 (ix), a
	ld	a, -5 (ix)
	adc	a, #0x00
	ld	-3 (ix), a
	ld	l, -8 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -7 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	ld	l, -10 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -9 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	ld	l, -4 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -3 (ix)
;	spillPairReg hl
;	spillPairReg hl
	call	_st_32
;ff.c:5236: st_32(dj.dir + DIR_ModTime, tm);
	ld	a, -43 (ix)
	ld	-6 (ix), a
	ld	a, -42 (ix)
	ld	-5 (ix), a
	ld	a, -6 (ix)
	add	a, #0x16
	ld	-4 (ix), a
	ld	a, -5 (ix)
	adc	a, #0x00
	ld	-3 (ix), a
	ld	l, -8 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -7 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	ld	l, -10 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -9 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	ld	l, -4 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -3 (ix)
;	spillPairReg hl
;	spillPairReg hl
	call	_st_32
;ff.c:5237: st_clust(fs, dj.dir, dcl);			/* Table start cluster */
	ld	e, -43 (ix)
	ld	d, -42 (ix)
	ld	l, -12 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -11 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	ld	l, -14 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -13 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	ld	l, -71 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -70 (ix)
;	spillPairReg hl
;	spillPairReg hl
	call	_st_clust
;ff.c:5238: dj.dir[DIR_Attr] = AM_DIR;			/* Attribute */
	ld	a, -43 (ix)
	ld	-6 (ix), a
	ld	a, -42 (ix)
	ld	-5 (ix), a
	ld	a, -6 (ix)
	add	a, #0x0b
	ld	-4 (ix), a
	ld	a, -5 (ix)
	adc	a, #0x00
	ld	-3 (ix), a
	ld	l, -4 (ix)
	ld	h, -3 (ix)
	ld	(hl), #0x10
;ff.c:5239: fs->wflag = 1;
	ld	a, -71 (ix)
	ld	-4 (ix), a
	ld	a, -70 (ix)
	ld	-3 (ix), a
	ld	l, -4 (ix)
	ld	h, -3 (ix)
	ld	de, #0x0004
	add	hl, de
	ld	(hl), #0x01
;ff.c:5241: if (res == FR_OK) {
	ld	a, -15 (ix)
	or	a, a
	jr	NZ, 00124$
;ff.c:5242: res = sync_fs(fs);
	pop	hl
	push	hl
	call	_sync_fs
	ld	-15 (ix), a
	jr	00124$
00119$:
;ff.c:5245: remove_chain(&sobj, dcl, 0);		/* Could not register, remove the allocated cluster */
	ld	hl, #0x0000
	push	hl
	push	hl
	ld	l, -12 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -11 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	ld	l, -14 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -13 (ix)
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	ld	hl, #50
	add	hl, sp
	call	_remove_chain
00124$:
;ff.c:5251: LEAVE_FF(fs, res);
	ld	a, -15 (ix)
;ff.c:5252: }
	ld	sp, ix
	pop	ix
	ret
;ff.c:5261: FRESULT f_rename (
;	---------------------------------
; Function f_rename
; ---------------------------------
_f_rename::
	push	ix
	ld	ix,#0
	add	ix,sp
	ld	iy, #-124
	add	iy, sp
	ld	sp, iy
	ld	-2 (ix), l
	ld	-1 (ix), h
	ld	-4 (ix), e
	ld	-3 (ix), d
;ff.c:5273: get_ldnumber(&path_new);	/* Snip the drive number of new name off */
	ld	hl, #120
	add	hl, sp
	call	_get_ldnumber
;ff.c:5274: res = mount_volume(&path_old, &fs, FA_WRITE);	/* Get logical drive of the old object */
	ld	a, #0x02
	push	af
	inc	sp
	ld	hl, #1
	add	hl, sp
	ex	de, hl
	ld	hl, #123
	add	hl, sp
	call	_mount_volume
;ff.c:5275: if (res == FR_OK) {
	ld	c, a
	or	a, a
	jp	NZ, 00129$
;ff.c:5276: djo.obj.fs = fs;
	ld	a, -124 (ix)
	ld	-122 (ix), a
	ld	a, -123 (ix)
	ld	-121 (ix), a
;ff.c:5278: res = follow_path(&djo, path_old);	/* Check old object */
	ld	e, -2 (ix)
	ld	d, -1 (ix)
	ld	hl, #2
	add	hl, sp
	call	_follow_path
;ff.c:5279: if (res == FR_OK) {
	ld	c, a
	or	a, a
	jr	NZ, 00104$
;ff.c:5280: if (djo.fn[NSFLAG] & (NS_DOT | NS_NONAME)) {
	ld	a, -83 (ix)
	and	a, #0xa0
	jr	Z, 00104$
;ff.c:5281: res = FR_INVALID_NAME;		/* Object must not be a dot name or blank name */
	ld	c, #0x06
00104$:
;ff.c:5288: if (res == FR_OK) {					/* It is ready to rename the object */
	ld	a, c
	or	a, a
	jp	NZ, 00129$
;ff.c:5327: memcpy(buf, djo.dir, SZDIRE);			/* Save directory entry of the object */
	ld	hl, #82
	add	hl, sp
	ex	de, hl
	ld	l, -96 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -95 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	bc, #0x0020
	ldir
;ff.c:5328: memcpy(&djn, &djo, sizeof djn);			/* Duplicate the directory object */
	ld	hl, #42
	add	hl, sp
	ex	de, hl
	ld	hl, #2
	add	hl, sp
	ld	bc, #0x0028
	ldir
;ff.c:5329: res = follow_path(&djn, path_new);		/* Make sure if new object name is not in use */
	ld	e, -4 (ix)
	ld	d, -3 (ix)
	ld	hl, #42
	add	hl, sp
	call	_follow_path
;ff.c:5331: res = (djn.obj.sclust == djo.obj.sclust && djn.dptr == djo.dptr) ? FR_NO_FILE : FR_EXIST;
;ff.c:5330: if (res == FR_OK) {						/* Is new name already in use by another object? */
	ld	c, a
	or	a, a
	jr	NZ, 00106$
;ff.c:5331: res = (djn.obj.sclust == djo.obj.sclust && djn.dptr == djo.dptr) ? FR_NO_FILE : FR_EXIST;
	ld	a, -76 (ix)
	ld	-8 (ix), a
	ld	a, -75 (ix)
	ld	-7 (ix), a
	ld	a, -74 (ix)
	ld	-6 (ix), a
	ld	a, -73 (ix)
	ld	-5 (ix), a
	ld	l, -116 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -115 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	c, -114 (ix)
	ld	b, -113 (ix)
	ld	e, -8 (ix)
	ld	d, -7 (ix)
	cp	a, a
	sbc	hl, de
	jr	NZ, 00132$
	ld	l, -6 (ix)
	ld	h, -5 (ix)
	cp	a, a
	sbc	hl, bc
	jr	NZ, 00132$
	ld	hl, #42
	add	hl, sp
	ex	de, hl
	ld	hl, #116
	add	hl, sp
	ex	de, hl
	ld	bc, #0x000e
	add	hl, bc
	ld	bc, #0x0004
	ldir
	ld	l, -108 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -107 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	c, -106 (ix)
	ld	b, -105 (ix)
	ld	e, -8 (ix)
	ld	d, -7 (ix)
	cp	a, a
	sbc	hl, de
	jr	NZ, 00132$
	ld	l, -6 (ix)
	ld	h, -5 (ix)
	cp	a, a
	sbc	hl, bc
	ld	c, #0x04
	jr	Z, 00133$
00132$:
	ld	c, #0x08
00133$:
00106$:
;ff.c:5333: if (res == FR_NO_FILE) { 				/* It is a valid path and no name collision */
	ld	a, c
	sub	a, #0x04
	jp	NZ,00121$
;ff.c:5334: res = dir_register(&djn);			/* Register the new entry */
	ld	hl, #42
	add	hl, sp
	call	_dir_register
;ff.c:5335: if (res == FR_OK) {
	ld	c, a
	or	a, a
	jp	NZ, 00121$
;ff.c:5336: dir = djn.dir;					/* Copy directory entry of the object except name */
	ld	a, -56 (ix)
	ld	-10 (ix), a
	ld	a, -55 (ix)
	ld	-9 (ix), a
;ff.c:5337: memcpy(dir + 13, buf + 13, SZDIRE - 13);
	ld	l, -10 (ix)
	ld	h, -9 (ix)
	ld	de, #0x000d
	add	hl, de
	push	bc
	ex	de, hl
	ld	hl, #97
	add	hl, sp
	ld	bc, #0x0013
	ldir
	pop	bc
;ff.c:5338: dir[DIR_Attr] = buf[DIR_Attr];
	ld	a, -10 (ix)
	add	a, #0x0b
	ld	e, a
	ld	a, -9 (ix)
	adc	a, #0x00
	ld	d, a
	ld	b, -31 (ix)
	ld	a, b
;ff.c:5339: if (!(dir[DIR_Attr] & AM_DIR)) dir[DIR_Attr] |= AM_ARC;	/* Set archive attribute if it is a file */
	ld	(de), a
	bit	4, b
	jr	NZ, 00108$
	set	5, a
	ld	(de), a
00108$:
;ff.c:5340: fs->wflag = 1;
	pop	hl
	push	hl
	inc	hl
	inc	hl
	inc	hl
	inc	hl
	ld	(hl), #0x01
;ff.c:5341: if ((dir[DIR_Attr] & AM_DIR) && djo.obj.sclust != djn.obj.sclust) {	/* Update .. entry in the sub-directory being moved if needed */
	ld	a, (de)
	bit	4, a
	jp	Z,00121$
	ld	a, -116 (ix)
	ld	-8 (ix), a
	ld	a, -115 (ix)
	ld	-7 (ix), a
	ld	a, -114 (ix)
	ld	-6 (ix), a
	ld	a, -113 (ix)
	ld	-5 (ix), a
	ld	b, -76 (ix)
	ld	l, -75 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	e, -74 (ix)
	ld	d, -73 (ix)
	ld	a, -8 (ix)
	sub	a, b
	jr	NZ, 00231$
	ld	a, -7 (ix)
	sub	a, l
	jr	NZ, 00231$
	ld	l, -6 (ix)
	ld	h, -5 (ix)
	cp	a, a
	sbc	hl, de
	jr	Z, 00121$
00231$:
;ff.c:5342: LBA_t sect = clst2sect(fs, ld_clust(fs, dir));
	ld	e, -10 (ix)
	ld	d, -9 (ix)
	pop	hl
	push	hl
	call	_ld_clust
	push	hl
	push	de
	ld	l, -124 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -123 (ix)
;	spillPairReg hl
;	spillPairReg hl
	call	_clst2sect
	pop	af
	pop	af
	push	hl
	pop	iy
	ld	c, e
	ld	b, d
;ff.c:5344: if (sect == 0) {
	ld	a, h
	or	a, l
	or	a, d
	or	a, e
	jr	NZ, 00113$
;ff.c:5345: res = FR_INT_ERR;
	ld	c, #0x02
	jr	00121$
00113$:
;ff.c:5348: res = move_window(fs, sect);
	push	iy
	push	bc
	ld	l, -124 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -123 (ix)
;	spillPairReg hl
;	spillPairReg hl
	call	_move_window
	ld	c, a
;ff.c:5349: dir = fs->win + SZDIRE * 1;	/* Pointer to .. entry */
	ld	a, -124 (ix)
	ld	-6 (ix), a
	ld	a, -123 (ix)
	ld	-5 (ix), a
	ld	a, -6 (ix)
	add	a, #0x50
	ld	e, a
	ld	a, -5 (ix)
	adc	a, #0x00
	ld	d, a
;ff.c:5350: if (res == FR_OK && dir[1] == '.') {
	ld	a, c
	or	a, a
	jr	NZ, 00121$
	ld	l, e
;	spillPairReg hl
;	spillPairReg hl
	ld	h, d
;	spillPairReg hl
;	spillPairReg hl
	inc	hl
	ld	a, (hl)
	sub	a, #0x2e
	jr	NZ, 00121$
;ff.c:5351: st_clust(fs, dir, djn.obj.sclust);
	ld	b, -76 (ix)
	ld	h, -75 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	l, -74 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	a, -73 (ix)
	push	bc
	push	hl
	ld	h, a
;	spillPairReg hl
;	spillPairReg hl
	ex	(sp), hl
	ld	l, b
;	spillPairReg hl
;	spillPairReg hl
	push	hl
	ld	l, -6 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, -5 (ix)
;	spillPairReg hl
;	spillPairReg hl
	call	_st_clust
	pop	bc
;ff.c:5352: fs->wflag = 1;
	pop	de
	push	de
	ld	hl, #0x0004
	add	hl, de
	ld	(hl), #0x01
00121$:
;ff.c:5359: if (res == FR_OK) {		/* New entry has been created */
	ld	a, c
	or	a, a
	jr	NZ, 00129$
;ff.c:5360: res = dir_remove(&djo);	/* Remove old entry */
	ld	hl, #2
	add	hl, sp
	call	_dir_remove
;ff.c:5361: if (res == FR_OK) {
	ld	c, a
	or	a, a
	jr	NZ, 00129$
;ff.c:5362: res = sync_fs(fs);
	pop	hl
	push	hl
	call	_sync_fs
	ld	c, a
00129$:
;ff.c:5370: LEAVE_FF(fs, res);
	ld	a, c
;ff.c:5371: }
	ld	sp, ix
	pop	ix
	ret
	.area _CODE
	.area _INITIALIZER
	.area _CABS (ABS)
