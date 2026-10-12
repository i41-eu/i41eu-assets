// Package robot3d draws a simplified 3D model of an M5Stack Stackchan robot with any picture on
// its screen: a screenshot taken over USB (320x240), a frame of a recording, a mock-up. Pure Go:
// a small software renderer, no GPU, no cgo, the same pixels on every machine.
//
// The robot faces +Z, Y is up, sizes are in millimetres. The head turns like the robot's look
// command: Yaw in degrees, + to the robot's left; Pitch in degrees up from level (the robot
// allows 5 to 85).
package robot3d

import (
	"image"
	"image/color"
	"math"
)

// Options say what to draw.
type Options struct {
	Width, Height int // the picture; 0: 800 x 800

	Screen     image.Image     // what the screen shows (any size, drawn 4:3); nil: off
	Brightness float64         // the screen, 0..1; 0 means 1
	LEDs       [12]color.Color // the robot's LEDs as its "leds" command numbers them; nil: off

	Yaw, Pitch float64 // the head, in degrees

	Azimuth, Elevation float64 // the camera around the robot in degrees: 0, 0 straight in front; + azimuth to the robot's left, + elevation from above
	Zoom               float64 // 1: the robot fits in any pose (a steady frame for videos); 0 means 1
	FOV                float64 // vertical field of view in degrees; 0 means 25

	Background  color.Color // nil: transparent
	NoShadow    bool
	Supersample int // samples per pixel along each axis; 0 means 3
}

// target is what the camera looks at, radius what must fit: the robot in any pose.
var (
	target = V3{0, 37, 2}
	radius = 52.0
)

// Render draws the robot.
func Render(o Options) *image.RGBA {
	if o.Width <= 0 || o.Height <= 0 {
		o.Width, o.Height = 800, 800
	}
	if o.Brightness <= 0 {
		o.Brightness = 1
	}
	if o.Zoom <= 0 {
		o.Zoom = 1
	}
	if o.FOV <= 0 {
		o.FOV = 25
	}
	if o.Supersample <= 0 {
		o.Supersample = 3
	}
	sc := &scene{brightness: o.Brightness}
	if o.Screen != nil {
		sc.screen = newTexture(o.Screen)
	}
	for i, c := range o.LEDs {
		if c == nil {
			continue
		}
		r, g, b, _ := c.RGBA()
		if r|g|b == 0 {
			continue
		}
		v := rgb{float64(r) / 0xffff, float64(g) / 0xffff, float64(b) / 0xffff}
		sc.leds[i] = &v
	}

	ss := o.Supersample
	fb := newFrame(o.Width*ss, o.Height*ss)
	cam := newCamera(o, fb.w, fb.h)
	parts := transforms(o.Yaw, o.Pitch)

	// world space vertices, once per triangle
	tris := mesh()
	world := make([][3]vert, len(tris))
	for i, t := range tris {
		m := parts[t.part]
		for k := 0; k < 3; k++ {
			world[i][k] = vert{m.Point(t.v[k].p), m.Dir(t.v[k].n)}
		}
	}

	// pass 1: the nearest triangle at each sample and where on it
	for i := range tris {
		var sp [3]V3 // screen x, y and 1/depth
		ok := true
		for k := 0; k < 3; k++ {
			if sp[k], ok = cam.project(world[i][k].p); !ok {
				break
			}
		}
		if ok {
			fb.raster(sp, int32(i))
		}
	}

	// pass 2: shade each sample
	var shadow []float64
	if !o.NoShadow {
		shadow = shadowMask(world, cam, fb.w, fb.h, ss)
	}
	var bg rgb
	bgA := 0.0
	if o.Background != nil {
		r, g, b, a := o.Background.RGBA()
		if a > 0 {
			bg = rgb{float64(r) / float64(a), float64(g) / float64(a), float64(b) / float64(a)}
			bgA = float64(a) / 0xffff
		}
	}
	out := image.NewRGBA(image.Rect(0, 0, o.Width, o.Height))
	inv := 1 / float64(ss*ss)
	for py := 0; py < o.Height; py++ {
		for px := 0; px < o.Width; px++ {
			var acc rgb // premultiplied
			accA := 0.0
			for sy := 0; sy < ss; sy++ {
				for sx := 0; sx < ss; sx++ {
					x, y := px*ss+sx, py*ss+sy
					i := y*fb.w + x
					if id := fb.id[i]; id >= 0 {
						c := shadeSample(sc, tris[id], world[id], fb.bary[i], cam.eye)
						c = rgb{clamp(c.R, 0, 1), clamp(c.G, 0, 1), clamp(c.B, 0, 1)} // before averaging: edges stay premultiplied
						acc, accA = acc.add(c), accA+1
						continue
					}
					// the ground: background under the shadow
					sa := 0.0
					if shadow != nil {
						sa = shadow[i]
					}
					a := bgA + sa*(1-bgA)
					if a > 0 {
						acc = acc.add(bg.mul(bgA * (1 - sa)))
						accA += a
					}
				}
			}
			acc, accA = acc.mul(inv), accA*inv
			out.SetRGBA(px, py, color.RGBA{q8(acc.R), q8(acc.G), q8(acc.B), q8(accA)})
		}
	}
	return out
}

func q8(v float64) uint8 { return uint8(clamp(v, 0, 1)*255 + 0.5) }

func shadeSample(sc *scene, t tri, w [3]vert, b [3]float64, eye V3) rgb {
	lp := t.v[0].p.Mul(b[0]).Add(t.v[1].p.Mul(b[1])).Add(t.v[2].p.Mul(b[2]))
	ln := t.v[0].n.Mul(b[0]).Add(t.v[1].n.Mul(b[1])).Add(t.v[2].n.Mul(b[2])).Norm()
	wp := w[0].p.Mul(b[0]).Add(w[1].p.Mul(b[1])).Add(w[2].p.Mul(b[2]))
	wn := w[0].n.Mul(b[0]).Add(w[1].n.Mul(b[1])).Add(w[2].n.Mul(b[2])).Norm()
	var sf surface
	switch t.part {
	case partCore:
		sf = sc.core(lp, ln)
	case partBody:
		sf = sc.body(lp, ln, t.mat)
	case partServo, partPlate:
		sf = lower(lp, ln, t.mat)
	default:
		sf = material(t.mat)
	}
	v := eye.Sub(wp).Norm()
	if wn.Dot(v) < 0 { // seen from behind (a thin part): light the side we see
		wn = wn.Mul(-1)
	}
	return light(sf, wn, v)
}

/* ---------------------------------- camera -------------------------------- */

type camera struct {
	eye          V3
	right, up, f V3 // f: forward
	focal        float64
	cx, cy       float64
}

func newCamera(o Options, w, h int) camera {
	az, el := rad(o.Azimuth), rad(o.Elevation)
	half := rad(o.FOV) / 2
	dist := radius / math.Sin(half) / o.Zoom
	dir := V3{math.Sin(az) * math.Cos(el), math.Sin(el), math.Cos(az) * math.Cos(el)}
	eye := target.Add(dir.Mul(dist))
	f := target.Sub(eye).Norm()
	right := f.Cross(V3{0, 1, 0}).Norm()
	if right.Len() == 0 { // straight above or below
		right = V3{1, 0, 0}
	}
	up := right.Cross(f)
	// the field of view covers the shorter side, so any shape keeps the robot whole
	return camera{eye: eye, right: right, up: up, f: f, focal: float64(min(w, h)) / 2 / math.Tan(half), cx: float64(w) / 2, cy: float64(h) / 2}
}

// project gives a point's screen position and 1/depth.
func (c camera) project(p V3) (V3, bool) {
	d := p.Sub(c.eye)
	z := d.Dot(c.f)
	if z < 1 {
		return V3{}, false
	}
	return V3{c.cx + c.focal*d.Dot(c.right)/z, c.cy - c.focal*d.Dot(c.up)/z, 1 / z}, true
}

/* ---------------------------------- raster -------------------------------- */

type frame struct {
	w, h int
	invZ []float64
	id   []int32
	bary [][3]float64 // perspective-correct
}

func newFrame(w, h int) *frame {
	f := &frame{w: w, h: h, invZ: make([]float64, w*h), id: make([]int32, w*h), bary: make([][3]float64, w*h)}
	for i := range f.id {
		f.id[i] = -1
	}
	return f
}

// raster fills a triangle (screen x, y, 1/depth) where it is nearer than what is there.
func (f *frame) raster(p [3]V3, id int32) {
	area := edge(p[0], p[1], p[2])
	if math.Abs(area) < 1e-9 {
		return
	}
	minX := int(math.Max(0, math.Floor(math.Min(p[0].X, math.Min(p[1].X, p[2].X)))))
	maxX := int(math.Min(float64(f.w-1), math.Ceil(math.Max(p[0].X, math.Max(p[1].X, p[2].X)))))
	minY := int(math.Max(0, math.Floor(math.Min(p[0].Y, math.Min(p[1].Y, p[2].Y)))))
	maxY := int(math.Min(float64(f.h-1), math.Ceil(math.Max(p[0].Y, math.Max(p[1].Y, p[2].Y)))))
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			s := V3{float64(x) + 0.5, float64(y) + 0.5, 0}
			w0, w1, w2 := edge(p[1], p[2], s)/area, edge(p[2], p[0], s)/area, edge(p[0], p[1], s)/area
			if w0 < 0 || w1 < 0 || w2 < 0 {
				continue
			}
			iz := w0*p[0].Z + w1*p[1].Z + w2*p[2].Z
			i := y*f.w + x
			if iz <= f.invZ[i] {
				continue
			}
			f.invZ[i], f.id[i] = iz, id
			b0, b1, b2 := w0*p[0].Z/iz, w1*p[1].Z/iz, w2*p[2].Z/iz
			f.bary[i] = [3]float64{b0, b1, b2}
		}
	}
}

func edge(a, b, c V3) float64 { return (b.X-a.X)*(c.Y-a.Y) - (b.Y-a.Y)*(c.X-a.X) }

/* ---------------------------------- shadow -------------------------------- */

// shadowMask is the robot's shadow on the ground (y = 0): the triangles cast straight down,
// softened.
func shadowMask(world [][3]vert, cam camera, w, h, ss int) []float64 {
	mask := make([]float64, w*h)
	cast := func(dir V3, strength float64, blur int) {
		m := make([]float64, w*h)
		for _, t := range world {
			var sp [3]V3
			ok := true
			for k := 0; k < 3; k++ {
				p := t[k].p
				g := p.Sub(dir.Mul(p.Y / dir.Y)) // along dir down to the ground
				if sp[k], ok = cam.project(g); !ok {
					break
				}
			}
			if ok {
				fillTri(m, w, h, sp)
			}
		}
		boxBlur(m, w, h, blur)
		boxBlur(m, w, h, blur)
		for i := range mask {
			mask[i] = 1 - (1-mask[i])*(1-m[i]*strength)
		}
	}
	cast(V3{0, 1, 0}, 0.3, 5*ss) // one soft shadow, straight down (two, cast apart, read as a double shadow)
	return mask
}

func fillTri(m []float64, w, h int, p [3]V3) {
	area := edge(p[0], p[1], p[2])
	if math.Abs(area) < 1e-9 {
		return
	}
	minX := int(math.Max(0, math.Floor(math.Min(p[0].X, math.Min(p[1].X, p[2].X)))))
	maxX := int(math.Min(float64(w-1), math.Ceil(math.Max(p[0].X, math.Max(p[1].X, p[2].X)))))
	minY := int(math.Max(0, math.Floor(math.Min(p[0].Y, math.Min(p[1].Y, p[2].Y)))))
	maxY := int(math.Min(float64(h-1), math.Ceil(math.Max(p[0].Y, math.Max(p[1].Y, p[2].Y)))))
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			s := V3{float64(x) + 0.5, float64(y) + 0.5, 0}
			w0, w1, w2 := edge(p[1], p[2], s)/area, edge(p[2], p[0], s)/area, edge(p[0], p[1], s)/area
			if w0 >= 0 && w1 >= 0 && w2 >= 0 {
				m[y*w+x] = 1
			}
		}
	}
}

// boxBlur blurs in place with a box of radius r, rows then columns.
func boxBlur(m []float64, w, h, r int) {
	if r < 1 {
		return
	}
	tmp := make([]float64, max(w, h))
	pass := func(n int, at func(int) int) {
		for i := 0; i < n; i++ {
			tmp[i] = m[at(i)]
		}
		sum := 0.0
		for i := -r; i <= r; i++ {
			sum += tmp[min(max(i, 0), n-1)]
		}
		for i := 0; i < n; i++ {
			m[at(i)] = sum / float64(2*r+1)
			sum += tmp[min(i+r+1, n-1)] - tmp[max(i-r, 0)]
		}
	}
	for y := 0; y < h; y++ {
		pass(w, func(i int) int { return y*w + i })
	}
	for x := 0; x < w; x++ {
		pass(h, func(i int) int { return i*w + x })
	}
}

func newTexture(img image.Image) *texture {
	b := img.Bounds()
	t := &texture{w: b.Dx(), h: b.Dy(), px: make([]rgb, b.Dx()*b.Dy())}
	for y := 0; y < t.h; y++ {
		for x := 0; x < t.w; x++ {
			r, g, bl, _ := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
			t.px[y*t.w+x] = rgb{float64(r) / 0xffff, float64(g) / 0xffff, float64(bl) / 0xffff}
		}
	}
	return t
}
