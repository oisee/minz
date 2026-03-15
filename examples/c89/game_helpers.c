/* C89 corpus: retro game helper functions */

/* Score BCD increment (2-digit BCD, 0-99) */
unsigned char bcd_inc(unsigned char bcd) {
    unsigned char lo = bcd & 0x0F;
    unsigned char hi = (bcd >> 4) & 0x0F;
    lo = lo + 1;
    if (lo > 9) {
        lo = 0;
        hi = hi + 1;
        if (hi > 9) {
            hi = 0;
        }
    }
    return (hi << 4) | lo;
}

/* Tile map index from x,y (32-column map) */
unsigned int tile_index(unsigned char x, unsigned char y) {
    return (unsigned int)y * 32 + (unsigned int)x;
}

/* Distance approximation (Manhattan) */
unsigned char manhattan(unsigned char x1, unsigned char y1, unsigned char x2, unsigned char y2) {
    unsigned char dx;
    unsigned char dy;
    if (x1 > x2) { dx = x1 - x2; } else { dx = x2 - x1; }
    if (y1 > y2) { dy = y1 - y2; } else { dy = y2 - y1; }
    return dx + dy;
}

/* Sprite animation frame (cycles 0..max-1) */
unsigned char next_frame(unsigned char frame, unsigned char max_frames) {
    frame = frame + 1;
    if (frame >= max_frames) frame = 0;
    return frame;
}

/* Simple state machine: advance state */
unsigned char advance_state(unsigned char state, unsigned char input) {
    /* 0=IDLE, 1=MOVING, 2=ATTACK, 3=DEAD */
    if (state == 0) {
        if (input == 1) return 1;  /* start moving */
        if (input == 2) return 2;  /* attack */
        return 0;
    }
    if (state == 1) {
        if (input == 0) return 0;  /* stop */
        if (input == 2) return 2;  /* attack while moving */
        return 1;
    }
    if (state == 2) {
        return 0;  /* attack done → idle */
    }
    return 3;  /* dead stays dead */
}

// assert bcd_inc(0x00) == 1 via mir2
// assert bcd_inc(0x09) == 16 via mir2
// assert bcd_inc(0x19) == 32 via mir2
// assert tile_index(0, 0) == 0 via mir2
// assert tile_index(1, 0) == 1 via mir2
// assert tile_index(0, 1) == 32 via mir2
// assert tile_index(31, 7) == 255 via mir2
// assert manhattan(0, 0, 3, 4) == 7 via mir2
// assert manhattan(10, 10, 10, 10) == 0 via mir2
// assert next_frame(0, 4) == 1 via mir2
// assert next_frame(3, 4) == 0 via mir2
// assert advance_state(0, 0) == 0 via mir2
// assert advance_state(0, 1) == 1 via mir2
// assert advance_state(2, 0) == 0 via mir2
