/* FatFS low-level functions — testable through MinZ C89 pipeline.
 * Extracted from FatFS R0.16 ff.c.
 * Uses wrapper functions since assert framework passes ints, not pointers.
 */

typedef unsigned char BYTE;
typedef unsigned short WORD;
typedef unsigned int UINT;

/* ---- Byte-level load/store (little-endian) ---- */

WORD ld_word(const BYTE* ptr) {
    WORD rv;
    rv = ptr[1];
    rv = (WORD)(rv << 8) | ptr[0];
    return rv;
}

void st_word(BYTE* ptr, WORD val) {
    ptr[0] = (BYTE)val;
    ptr[1] = (BYTE)(val >> 8);
}

/* ---- DBCS support (simplified: code page 437 = no DBCS) ---- */

int dbc_1st(BYTE c) { return 0; }
int dbc_2nd(BYTE c) { return 0; }

/* ---- FAT entry classification ---- */

BYTE classify_fat12(WORD val) {
    if (val == 0) return 0;       /* free */
    if (val == 1) return 1;       /* reserved */
    if (val >= 0xFF8) return 3;   /* EOC (end of chain) */
    if (val >= 0xFF0) return 4;   /* reserved/bad */
    return 2;                     /* in-use (next cluster) */
}

/* ---- Directory entry helpers ---- */

BYTE is_deleted(BYTE b) {
    if (b == 0xE5) return 1;
    if (b == 0x00) return 2;
    return 0;
}

/* ---- Sector/cluster arithmetic ---- */

WORD clst2sect_simple(WORD data_start, WORD clst, BYTE sects_per_clust) {
    if (clst < 2) return 0;
    return data_start + (WORD)((clst - 2) * sects_per_clust);
}

/* ---- SFN checksum (8.3 filename, 11 bytes) ---- */

BYTE sfn_checksum(const BYTE* dir) {
    BYTE sum = 0;
    BYTE i = 0;
    while (i < 11) {
        sum = (BYTE)((sum >> 1) + (sum << 7) + dir[i]);
        i = i + 1;
    }
    return sum;
}

/* ---- FAT12 bit packing ---- */

WORD read_fat12(const BYTE* fat, WORD clst) {
    WORD ofs = clst + (clst >> 1);
    WORD val = ld_word(fat + ofs);
    if (clst & 1) {
        val = val >> 4;
    } else {
        val = val & 0x0FFF;
    }
    return val;
}

/* ---- Chain follower ---- */

BYTE follow_chain(const BYTE* fat, WORD start, BYTE max_hops) {
    WORD clst = start;
    BYTE count = 0;
    do {
        count = count + 1;
        clst = read_fat12(fat, clst);
        if (clst == 0) break;
        if (clst >= 0xFF8) break;
        if (count >= max_hops) break;
    } while (clst >= 2 && clst < 0xFF0);
    return count;
}

/* ---- BPB field extraction ---- */

WORD bpb_bytes_per_sect(const BYTE* bpb) { return ld_word(bpb + 11); }
BYTE bpb_sects_per_clust(const BYTE* bpb) { return bpb[13]; }
WORD bpb_reserved_sects(const BYTE* bpb) { return ld_word(bpb + 14); }
BYTE bpb_num_fats(const BYTE* bpb) { return bpb[16]; }
WORD bpb_root_entry_count(const BYTE* bpb) { return ld_word(bpb + 17); }
WORD bpb_fat_size_16(const BYTE* bpb) { return ld_word(bpb + 22); }

WORD root_dir_start(const BYTE* bpb) {
    return bpb_reserved_sects(bpb) + (WORD)(bpb_num_fats(bpb) * bpb_fat_size_16(bpb));
}

/* ==== Wrapper functions for assert testing ==== */
/* These set up memory buffers and call the real functions */

/* Global buffers for test data */
BYTE g_buf[32];

/* ld_word test: store two bytes, call ld_word */
WORD test_ld_word(BYTE lo, BYTE hi) {
    g_buf[0] = lo;
    g_buf[1] = hi;
    return ld_word(g_buf);
}

/* st_word + ld_word round-trip */
WORD test_st_ld_roundtrip(WORD val) {
    st_word(g_buf, val);
    return ld_word(g_buf);
}

/* sfn_checksum test: "HELLO   TXT" = H,E,L,L,O,spc,spc,spc,T,X,T */
BYTE test_sfn_hello(void) {
    g_buf[0] = 0x48; g_buf[1] = 0x45; g_buf[2] = 0x4C;
    g_buf[3] = 0x4C; g_buf[4] = 0x4F; g_buf[5] = 0x20;
    g_buf[6] = 0x20; g_buf[7] = 0x20; g_buf[8] = 0x54;
    g_buf[9] = 0x58; g_buf[10] = 0x54;
    return sfn_checksum(g_buf);
}

/* read_fat12 test: set up FAT table, read entries */
/* FAT12: entries 0=FF8, 1=FFF, 2=003, 3=004, 4=FFF
   Packed bytes: F8 FF FF 03 40 00 FF 0F 00 */
WORD test_fat12_entry(BYTE idx) {
    g_buf[0] = 0xF8; g_buf[1] = 0xFF; g_buf[2] = 0xFF;
    g_buf[3] = 0x03; g_buf[4] = 0x40; g_buf[5] = 0x00;
    g_buf[6] = 0xFF; g_buf[7] = 0x0F; g_buf[8] = 0x00;
    return read_fat12(g_buf, idx);
}

/* follow_chain test: chain 2->3->4->EOC = 3 hops */
BYTE test_follow_chain(BYTE start, BYTE max) {
    g_buf[0] = 0xF8; g_buf[1] = 0xFF; g_buf[2] = 0xFF;
    g_buf[3] = 0x03; g_buf[4] = 0x40; g_buf[5] = 0x00;
    g_buf[6] = 0xFF; g_buf[7] = 0x0F; g_buf[8] = 0x00;
    return follow_chain(g_buf, start, max);
}

/* BPB field test: set up a minimal BPB */
WORD test_bpb_bytes_per_sect(void) {
    /* offset 11-12: bytes_per_sector = 512 = 0x0200 */
    g_buf[11] = 0x00; g_buf[12] = 0x02;
    return bpb_bytes_per_sect(g_buf);
}

BYTE test_bpb_spc(void) {
    /* offset 13: sectors_per_cluster = 1 */
    g_buf[13] = 1;
    return bpb_sects_per_clust(g_buf);
}

WORD test_bpb_rsvd(void) {
    /* offset 14-15: reserved_sectors = 1 */
    g_buf[14] = 0x01; g_buf[15] = 0x00;
    return bpb_reserved_sects(g_buf);
}

/* ======== ASSERTIONS ======== */

/* ld_word / st_word */
// assert test_ld_word(0x34, 0x12) == 0x1234 via mir2
// assert test_ld_word(0x00, 0x00) == 0 via mir2
// assert test_ld_word(0xFF, 0xFF) == 0xFFFF via mir2
// assert test_ld_word(0x01, 0x00) == 1 via mir2

/* round-trip */
// assert test_st_ld_roundtrip(0x1234) == 0x1234 via mir2
// assert test_st_ld_roundtrip(0xABCD) == 0xABCD via mir2
// assert test_st_ld_roundtrip(0) == 0 via mir2

/* classify_fat12 */
// assert classify_fat12(0) == 0 via mir2
// assert classify_fat12(1) == 1 via mir2
// assert classify_fat12(100) == 2 via mir2
// assert classify_fat12(0xFF8) == 3 via mir2
// assert classify_fat12(0xFFF) == 3 via mir2
// assert classify_fat12(0xFF0) == 4 via mir2

/* is_deleted */
// assert is_deleted(0xE5) == 1 via mir2
// assert is_deleted(0x00) == 2 via mir2
// assert is_deleted(0x41) == 0 via mir2

/* clst2sect */
// assert clst2sect_simple(100, 2, 4) == 100 via mir2
// assert clst2sect_simple(100, 3, 4) == 104 via mir2
// assert clst2sect_simple(100, 10, 4) == 132 via mir2
// assert clst2sect_simple(100, 0, 4) == 0 via mir2
// assert clst2sect_simple(100, 1, 4) == 0 via mir2

/* sfn_checksum */
// assert test_sfn_hello() == 241 via mir2

/* FAT12 entry reading */
// assert test_fat12_entry(0) == 0xFF8 via mir2
// assert test_fat12_entry(2) == 3 via mir2
// assert test_fat12_entry(3) == 4 via mir2
// assert test_fat12_entry(4) == 0xFFF via mir2

/* chain following */
// assert test_follow_chain(2, 10) == 3 via mir2
// assert test_follow_chain(4, 10) == 1 via mir2

/* BPB parsing */
// assert test_bpb_bytes_per_sect() == 512 via mir2
// assert test_bpb_spc() == 1 via mir2
// assert test_bpb_rsvd() == 1 via mir2

/* dbc (no DBCS) */
// assert dbc_1st(65) == 0 via mir2
// assert dbc_2nd(65) == 0 via mir2
