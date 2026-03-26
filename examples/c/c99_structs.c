/*
 * C99 Struct features — comprehensive Z80 testing
 * Designated init, compound literals, nested, anonymous
 *
 * mzv: echo "" | mzv -H c99_structs.c
 */
#include <stdint.h>
#include <stdbool.h>

typedef struct { uint8_t x; uint8_t y; } Point;
typedef struct { uint8_t r; uint8_t g; uint8_t b; } Color;
typedef struct { Point pos; uint8_t id; } Entity;

/* Basic field access */
uint8_t point_x(void) {
    Point p = {10, 20};
    return p.x;
}

uint8_t point_y(void) {
    Point p = {10, 20};
    return p.y;
}

/* Designated initializer — out of order */
uint8_t color_green(void) {
    Color c = {.b = 30, .r = 10, .g = 20};
    return c.g;
}

/* Compound literal */
uint8_t compound_point(void) {
    Point p = (Point){.x = 42, .y = 99};
    return p.x;
}

/* Struct arithmetic */
uint8_t manhattan(void) {
    Point a = {10, 20};
    Point b = {15, 30};
    uint8_t dx = b.x - a.x;
    uint8_t dy = b.y - a.y;
    return dx + dy;
}

/* Nested struct with flattened fields */
uint8_t entity_x(void) {
    Entity e;
    e.pos.x = 42;
    e.pos.y = 99;
    e.id = 1;
    return e.pos.x;
}

uint8_t entity_id(void) {
    Entity e;
    e.pos.x = 42;
    e.pos.y = 99;
    e.id = 7;
    return e.id;
}

/* Anonymous struct (C11) — fields promoted */
struct Vec3 {
    struct { uint8_t x; uint8_t y; };
    uint8_t z;
};

uint8_t vec3_z(void) {
    struct Vec3 v;
    v.x = 10;
    v.y = 20;
    v.z = 30;
    return v.z;
}

uint8_t vec3_sum(void) {
    struct Vec3 v;
    v.x = 10;
    v.y = 20;
    v.z = 30;
    return v.x + v.y + v.z;
}

/* Struct as return via promotion (ADR-0025) */
Point make_point(uint8_t x, uint8_t y) {
    return (Point){x, y};
}

uint8_t test_return_struct(void) {
    Point p = make_point(42, 99);
    return p.x;
}

// assert point_x() == 10 via mir2
// assert point_y() == 20 via mir2
// assert color_green() == 20 via mir2
// assert compound_point() == 42 via mir2
// assert manhattan() == 15 via mir2
// assert entity_x() == 42 via mir2
// assert entity_id() == 7 via mir2
// assert vec3_z() == 30 via mir2
// assert vec3_sum() == 60 via mir2
// assert test_return_struct() == 42 via mir2
