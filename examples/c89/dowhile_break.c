/* C89 corpus: do-while loops with break/continue (FatFS patterns) */

typedef unsigned char u8;

/* Basic do-while: count down */
u8 count_down(u8 start) {
    u8 n = 0;
    u8 v = start;
    do {
        n = n + 1;
        v = v - 1;
    } while (v > 0);
    return n;
}

/* do-while with break: walk an array chain (FatFS remove_chain pattern) */
u8 g_chain[8];

u8 chain_length(u8 a, u8 b, u8 c, u8 d) {
    /* Simulate a FAT chain via global array: slot[1]=a, slot[2]=b, etc. */
    g_chain[0] = 0; g_chain[1] = a; g_chain[2] = b;
    g_chain[3] = c; g_chain[4] = d; g_chain[5] = 0;
    u8 count = 0;
    u8 clst = 1;
    do {
        count = count + 1;
        if (clst < 6) clst = g_chain[clst];
        else clst = 0;
        if (clst == 0) break;
    } while (clst < 255);
    return count;
}

/* do-while with early break on first iteration */
u8 early_break(u8 x) {
    u8 r = 0;
    do {
        if (x == 0) break;
        r = r + x;
        x = x - 1;
    } while (x > 0);
    return r;
}

/* nested do-while */
u8 nested_dowhile(u8 rows, u8 cols) {
    u8 total = 0;
    u8 r = 0;
    do {
        u8 c = 0;
        do {
            total = total + 1;
            c = c + 1;
        } while (c < cols);
        r = r + 1;
    } while (r < rows);
    return total;
}

/* do-while with continue */
u8 skip_odds(u8 n) {
    u8 sum = 0;
    u8 i = 0;
    do {
        i = i + 1;
        if (i & 1) continue;  /* skip odd values */
        sum = sum + i;
    } while (i < n);
    return sum;
}

/* switch inside do-while (FatFS get_fat pattern) */
u8 state_machine(u8 input) {
    u8 state = 0;
    u8 idx = 0;
    do {
        u8 ch;
        if (idx == 0) ch = input;
        else if (idx == 1) ch = input + 1;
        else if (idx == 2) ch = input + 2;
        else ch = 0;

        switch (ch) {
        case 0:
            state = 99;
            break;
        case 10:
            state = 1;
            break;
        case 11:
            state = 2;
            break;
        case 12:
            state = 3;
            break;
        default:
            state = state + 1;
            break;
        }
        idx = idx + 1;
    } while (state < 10 && idx < 4);
    return state;
}

/* break inside nested if inside do-while (exact FatFS pattern) */
u8 find_threshold(u8 start, u8 threshold) {
    u8 val = start;
    u8 steps = 0;
    do {
        steps = steps + 1;
        val = val + 3;
        if (val > 200) {
            if (val > threshold) {
                break;
            }
        }
    } while (steps < 50);
    return steps;
}

// assert count_down(1) == 1 via mir2
// assert count_down(5) == 5 via mir2
// assert count_down(10) == 10 via mir2

// assert chain_length(2, 3, 4, 0) == 4 via mir2
// assert chain_length(2, 3, 0, 0) == 3 via mir2
// assert chain_length(0, 0, 0, 0) == 1 via mir2
// assert chain_length(2, 3, 4, 5) == 5 via mir2

// assert early_break(0) == 0 via mir2
// assert early_break(3) == 6 via mir2
// assert early_break(1) == 1 via mir2

// assert nested_dowhile(3, 4) == 12 via mir2
// assert nested_dowhile(2, 2) == 4 via mir2
// assert nested_dowhile(1, 5) == 5 via mir2

// assert skip_odds(6) == 12 via mir2
// assert skip_odds(4) == 6 via mir2

// assert state_machine(10) == 99 via mir2
// assert state_machine(0) == 99 via mir2

// assert find_threshold(100, 250) == 50 via mir2
// assert find_threshold(0, 210) == 50 via mir2
