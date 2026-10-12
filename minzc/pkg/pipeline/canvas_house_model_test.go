package pipeline

// Independent framebuffer for the frozen Canvas API domain (mode0, indexed8).
// It does not call production Canvas drawing methods.
func canvasHousePixels(events []canvasHouseEvent) []byte {
	p := make([]byte, 256*192)
	put := func(x, y int, c byte) {
		if x >= 0 && x < 256 && y >= 0 && y < 192 {
			p[y*256+x] = c
		}
	}
	var line func(int, int, int, int, byte)
	line = func(x, y, u, v int, c byte) {
		dx := u - x
		if dx < 0 {
			dx = -dx
		}
		dy := v - y
		if dy < 0 {
			dy = -dy
		}
		sx, sy := 1, 1
		if x > u {
			sx = -1
		}
		if y > v {
			sy = -1
		}
		err := dx - dy
		for {
			put(x, y, c)
			if x == u && y == v {
				return
			}
			twice := 2 * err
			if twice > -dy {
				err -= dy
				x += sx
			}
			if twice < dx {
				err += dx
				y += sy
			}
		}
	}
	for _, e := range events {
		a := e.Args
		switch e.Name {
		case "canvas_init":
			p = make([]byte, 256*192)
		case "canvas_clear":
			for n := range p {
				p[n] = byte(a[0])
			}
		case "canvas_fill_rect":
			for y := int(a[1]); y < int(a[1]+a[3]); y++ {
				for x := int(a[0]); x < int(a[0]+a[2]); x++ {
					put(x, y, byte(a[4]))
				}
			}
		case "canvas_line":
			line(int(a[0]), int(a[1]), int(a[2]), int(a[3]), byte(a[4]))
		case "canvas_rect":
			x, y, w, h, c := int(a[0]), int(a[1]), int(a[2]), int(a[3]), byte(a[4])
			line(x, y, x+w-1, y, c)
			line(x+w-1, y, x+w-1, y+h-1, c)
			line(x+w-1, y+h-1, x, y+h-1, c)
			line(x, y+h-1, x, y, c)
		case "canvas_circle":
			cx, cy, x, y, d, c := int(a[0]), int(a[1]), int(a[2]), 0, 1-int(a[2]), byte(a[3])
			for x >= y {
				for _, q := range [][2]int{{x, y}, {-x, y}, {x, -y}, {-x, -y}, {y, x}, {-y, x}, {y, -x}, {-y, -x}} {
					put(cx+q[0], cy+q[1], c)
				}
				y++
				if d < 0 {
					d += 2*y + 1
				} else {
					x--
					d += 2*(y-x) + 1
				}
			}
		}
	}
	return p
}
