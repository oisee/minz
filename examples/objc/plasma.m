// ObjC Plasma Demo — Retro Demoscene Effect
//
// Renders a classic plasma pattern using integer sine approximation.
// Demonstrates: protocol dispatch, canvas host functions, lookup tables.
//
// Output: 256x192 PNG with ZX Spectrum palette (mode 0)

// ── Canvas declarations ──────────────────────────────────────────────────────

void canvas_init(int w, int h, int mode);
void canvas_clear(int color);
void canvas_pixel(int x, int y, int color);
void canvas_palette(int idx, int r, int g, int b);
int  canvas_width(void);
int  canvas_height(void);

// ── Integer sine table (256 entries, range -127..+127) ──────────────────────
//
// sin_tab[i] ≈ 127 * sin(i * 2π / 256)
// Built at "compile time" by the palette generator below.

// We fake a sine with a triangular approximation:
//   0..63   →  0..+127  (rising)
//  64..127  → +127..0   (falling)
// 128..191  →  0..-127  (falling)
// 192..255  → -127..0   (rising)

int isin(int x) {
    int ix = x & 255;
    int quarter = ix & 63;
    int half_val;

    // Linear ramp 0..63 → 0..127
    half_val = quarter * 2;

    if (ix < 64) {
        return half_val;
    }
    if (ix < 128) {
        return 127 - half_val;
    }
    if (ix < 192) {
        return 0 - half_val;
    }
    return half_val - 127;
}

int icos(int x) {
    return isin(x + 64);
}

// ── Palette setup ────────────────────────────────────────────────────────────

void setup_plasma_palette(void) {
    int i = 0;
    while (i < 256) {
        // Cycle through RGB using sine waves with phase offsets
        int r = isin(i) + 128;            // 1..255
        int g = isin(i + 85) + 128;       // phase +1/3
        int b = isin(i + 170) + 128;      // phase +2/3

        // Clamp to 0-255
        if (r < 0) r = 0;
        if (r > 255) r = 255;
        if (g < 0) g = 0;
        if (g > 255) g = 255;
        if (b < 0) b = 0;
        if (b > 255) b = 255;

        canvas_palette(i, r, g, b);
        i = i + 1;
    }
}

// ── Effect Protocol ──────────────────────────────────────────────────────────

@protocol Effect
-(int)render:(int)t;
@end

// ── Plasma Effect ────────────────────────────────────────────────────────────

@interface Plasma <Effect> {
    int scale;
    int speed;
}
-(int)render:(int)t;
@end

@implementation Plasma
-(int)render:(int)t {
    int w = canvas_width();
    int h = canvas_height();
    int y = 0;
    while (y < h) {
        int x = 0;
        while (x < w) {
            // Classic plasma: sum of sine waves at different frequencies
            int v1 = isin(x * self->scale / 8 + t * self->speed);
            int v2 = icos(y * self->scale / 8 + t * self->speed / 2);
            int v3 = isin((x + y) * self->scale / 16 + t);
            int v4 = icos((x - y + 256) * self->scale / 16 + t / 2);

            // Sum and map to palette index (0-255)
            int color = (v1 + v2 + v3 + v4 + 512) / 4;
            if (color < 0) color = 0;
            if (color > 255) color = 255;

            canvas_pixel(x, y, color);
            x = x + 1;
        }
        y = y + 1;
    }
    return 0;
}
@end

// ── Diamond Pattern ──────────────────────────────────────────────────────────

@interface Diamond <Effect> {
    int spacing;
    int dummy;
}
-(int)render:(int)t;
@end

@implementation Diamond
-(int)render:(int)t {
    int w = canvas_width();
    int h = canvas_height();
    int y = 0;
    while (y < h) {
        int x = 0;
        while (x < w) {
            // Manhattan distance from center, animated
            int dx = x - w / 2;
            int dy = y - h / 2;
            if (dx < 0) dx = 0 - dx;
            if (dy < 0) dy = 0 - dy;
            int dist = dx + dy;

            int color = (dist * 2 + t * 3) & 255;
            canvas_pixel(x, y, color);
            x = x + 1;
        }
        y = y + 1;
    }
    return 0;
}
@end

// ── XOR Pattern (classic) ───────────────────────────────────────────────────

@interface XorPattern <Effect> {
    int shift;
    int dummy;
}
-(int)render:(int)t;
@end

@implementation XorPattern
-(int)render:(int)t {
    int w = canvas_width();
    int h = canvas_height();
    int y = 0;
    while (y < h) {
        int x = 0;
        while (x < w) {
            int color = ((x + t) ^ (y + t / 2)) & 255;
            canvas_pixel(x, y, color);
            x = x + 1;
        }
        y = y + 1;
    }
    return 0;
}
@end

// ── Test helpers ─────────────────────────────────────────────────────────────

int identity(int x) { return x; }

// Verify sine approximation (triangle wave: 0→127→0→-127→0)
// assert isin(0) == 0 via mir2
// assert isin(32) == 64 via mir2
// assert isin(64) == 127 via mir2
// assert isin(128) == 0 via mir2
// assert isin(192) == 0xFF81 via mir2

// Static dispatch tests
// assert-objc Plasma{scale:8, speed:1}.render(0) == 0
// assert-objc Diamond{spacing:4, dummy:0}.render(0) == 0
// assert-objc XorPattern{shift:0, dummy:0}.render(0) == 0

// Dynamic dispatch via protocol vtable
// assert-objc-dyn Effect Plasma{scale:8, speed:1}.render(0) == 0
// assert-objc-dyn Effect Diamond{spacing:4, dummy:0}.render(0) == 0
// assert-objc-dyn Effect XorPattern{shift:0, dummy:0}.render(0) == 0
