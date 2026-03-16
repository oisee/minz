// ObjC Canvas Demo — Protocol-based Shape Drawing
//
// Demonstrates dynamic dispatch + cross-language canvas:
// - @protocol Shape with draw/area methods
// - Circle, Rect, Triangle implementations
// - Draws to canvas via host functions, saves PNG
//
// Works on: MIR2 VM (mzv), QBE native (future)

// ── Canvas host function declarations ────────────────────────────────────────

void canvas_init(int w, int h, int mode);
void canvas_clear(int color);
void canvas_pixel(int x, int y, int color);
void canvas_line(int x0, int y0, int x1, int y1, int color);
void canvas_rect(int x, int y, int w, int h, int color);
void canvas_fill_rect(int x, int y, int w, int h, int color);
void canvas_circle(int cx, int cy, int r, int color);
int  canvas_width(void);
int  canvas_height(void);

// ── Protocol & Classes ───────────────────────────────────────────────────────

@protocol Shape
-(int)draw;
-(int)area;
@end

@interface Circle <Shape> {
    int cx;
    int cy;
    int radius;
    int color;
}
-(int)draw;
-(int)area;
@end

@interface Rect <Shape> {
    int x;
    int y;
    int w;
    int h;
    int color;
}
-(int)draw;
-(int)area;
@end

@interface FilledRect <Shape> {
    int x;
    int y;
    int w;
    int h;
    int color;
}
-(int)draw;
-(int)area;
@end

// ── Implementations ──────────────────────────────────────────────────────────

@implementation Circle
-(int)draw {
    canvas_circle(self->cx, self->cy, self->radius, self->color);
    return 1;
}
-(int)area {
    // Approximate: 3 * r * r (integer pi ~ 3)
    return 3 * self->radius * self->radius;
}
@end

@implementation Rect
-(int)draw {
    canvas_rect(self->x, self->y, self->w, self->h, self->color);
    return 1;
}
-(int)area {
    return self->w * self->h;
}
@end

@implementation FilledRect
-(int)draw {
    canvas_fill_rect(self->x, self->y, self->w, self->h, self->color);
    return 1;
}
-(int)area {
    return self->w * self->h;
}
@end

// ── Helpers ──────────────────────────────────────────────────────────────────

int abs_val(int x) {
    if (x < 0) return 0 - x;
    return x;
}

int identity(int x) { return x; }

// ── Static dispatch tests ────────────────────────────────────────────────────

// assert-objc Circle{cx:0, cy:0, radius:10, color:0}.area() == 300
// assert-objc Rect{x:0, y:0, w:20, h:10, color:0}.area() == 200
// assert-objc FilledRect{x:0, y:0, w:5, h:8, color:0}.area() == 40

// ── Dynamic dispatch tests ───────────────────────────────────────────────────

// assert-objc-dyn Shape Circle{cx:0, cy:0, radius:7, color:0}.area() == 147
// assert-objc-dyn Shape Rect{x:0, y:0, w:12, h:3, color:0}.area() == 36
// assert-objc-dyn Shape FilledRect{x:0, y:0, w:6, h:6, color:0}.area() == 36
