// goto + labels test

// Basic goto: skip over assignment
int test_skip(void) {
    int x = 1;
    goto done;
    x = 99;
done:
    return x;
}
// assert test_skip() == 1 via mir2

// Error cleanup pattern (forward goto only, no variable crossing)
int test_cleanup(int x) {
    if (x < 0) goto fail;
    return x * 2;
fail:
    return -1;
}
// assert test_cleanup(5) == 10 via mir2
// assert test_cleanup(-1) == 0xFFFF via mir2
