// Nested struct field access test

typedef struct { int x; int y; } Point;
typedef struct { Point origin; int width; int height; } Rect;

// Verify Point still works
int test_point(void) {
    Point p = {10, 20};
    return p.x + p.y;
}
// assert test_point() == 30 via mir2

// Rect with nested init — access non-nested fields
int test_rect_width(void) {
    Rect r = {{0, 0}, 100, 50};
    return r.width;
}
// assert test_rect_width() == 100 via mir2

int test_rect_height(void) {
    Rect r = {{0, 0}, 100, 50};
    return r.height;
}
// assert test_rect_height() == 50 via mir2

// Nested access: r.origin.x, r.origin.y
int test_nested_x(void) {
    Rect r = {{10, 20}, 100, 50};
    return r.origin.x;
}
// assert test_nested_x() == 10 via mir2

int test_nested_y(void) {
    Rect r = {{10, 20}, 100, 50};
    return r.origin.y;
}
// assert test_nested_y() == 20 via mir2

// All fields together
int test_nested_all(void) {
    Rect r = {{3, 4}, 10, 20};
    return r.origin.x + r.origin.y + r.width + r.height;
}
// assert test_nested_all() == 37 via mir2
