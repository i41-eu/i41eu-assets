package robot3d

import (
	"math"
)

// rgb is a linear-ish colour, 0..1 a channel.
type rgb struct{ R, G, B float64 }

func (a rgb) add(b rgb) rgb             { return rgb{a.R + b.R, a.G + b.G, a.B + b.B} }
func (a rgb) mul(s float64) rgb         { return rgb{a.R * s, a.G * s, a.B * s} }
func (a rgb) mulc(b rgb) rgb            { return rgb{a.R * b.R, a.G * b.G, a.B * b.B} }
func (a rgb) lerp(b rgb, t float64) rgb { return a.add(b.add(a.mul(-1)).mul(t)) }

func hex(v uint32) rgb {
	return rgb{float64(v>>16&0xff) / 255, float64(v>>8&0xff) / 255, float64(v&0xff) / 255}
}

var (
	colShell    = hex(0xd6d7d9) // the head's light grey shell (the owner's photos of the robot)
	colGlass    = hex(0x0d0e11) // the black glass front
	colVent     = hex(0x55595e)
	colPortDark = hex(0x1c1d20)
	colServo    = hex(0x50555b) // the open back: the servo inside
	colBarOff   = hex(0x9aa5b0) // the LED bar, unlit: a light guide a little darker grey than the shell, bluish (the owner's photos)
	colRing     = hex(0xc9573e) // the camera's red ring under the screen (the owner's photos)
	colLens     = hex(0x3a4c56) // the camera's lens
	colSensor   = hex(0x2a2c31) // the light and proximity sensor's two windows: dark (the owner's photos)
	colBoard    = hex(0x1f3a31) // StackChan's board behind the CoreS3, seen through the open back: dark green
	colPortTeal = hex(0x2a9da5) // port C (UART) on the head's top
	colPlate    = hex(0x7d8087) // the base: dark grey (the owner's photos; it renders darker)
	colDisc     = hex(0xc9ccd0)
)

// lights: a key light from the upper left front, a fill from the right, a soft one from behind,
// a sky above.
var (
	keyDir  = V3{-0.45, 0.8, 0.55}.Norm()
	fillDir = V3{0.75, 0.25, 0.45}.Norm()
	backDir = V3{0.2, 0.5, -0.85}.Norm()
)

// surface is what a point looks like before the lights: its colour, shine, and light of its own.
type surface struct {
	albedo   rgb
	spec     float64 // specular strength
	shine    float64 // specular exponent
	emissive rgb     // light of its own (the screen, lit LEDs)
	glass    bool    // reflects the room
}

// scene holds what a frame's shading needs.
type scene struct {
	screen     *texture
	brightness float64
	leds       [12]*rgb
}

// core shades the CoreS3 (p, n in the robot's rest space): the glass front with the screen, the
// red ring and the sensors' dots; vents on the robot's left (+X), the power button, USB-C and the
// Grove port on its right (sizes from M5Stack's drawing).
func (s *scene) core(p, n V3) surface {
	sf := surface{albedo: colShell, spec: 0.18, shine: 24}
	p = p.Sub(coreCentre)
	switch {
	case n.Z > 0.97: // the front
		if roundRect(p.X, p.Y, 27-rim, 27-rim, coreR-rim) <= 0 {
			sf = surface{albedo: colGlass, spec: 0.9, shine: 90, glass: true}
			sx, sy := p.X+screenW/2, (screenCY+screenH/2)-p.Y // from the screen's top left
			if sx >= 0 && sx <= screenW && sy >= 0 && sy <= screenH {
				if s.screen != nil {
					sf.emissive = s.screen.sample(sx/screenW, sy/screenH).mul(s.brightness)
				}
			} else {
				frontMarks(p, &sf)
			}
		}
	case n.X > 0.97: // left: the vents, a narrow staggered band toward the back (M5Stack's photos)
		if p.Y > -15.6 && p.Y < 16 && p.Z > -6.3 && p.Z < 1.1 {
			const step = 2.6
			row := math.Round(p.Y / step)
			cols := [2]float64{-5.6, -1.6}
			if int(row)%2 != 0 {
				cols = [2]float64{-3.6, 0.4}
			}
			for _, cz := range cols {
				if math.Hypot(p.Z-cz, p.Y-row*step) < 0.5 {
					sf.albedo, sf.spec = colVent, 0
				}
			}
		}
	case n.Z < -0.97: // the back, seen through the head's open back: StackChan's board (M5Stack's photo of the back, the owner's photos)
		sf.albedo, sf.spec, sf.shine = colBoard, 0.3, 30
		switch { // seen from the back: +X on the left
		case box(p.X, p.Y, 8, 16, 15, 19.5): // the white 7-pin connector
			sf.albedo = hex(0xe8e8e4)
		case box(p.X, p.Y, 1, 8, 9, 17.5): // its black cable
			sf.albedo, sf.spec = hex(0x15171a), 0.15
		case box(p.X, p.Y, -9.5, -4, -5, 26): // the ribbon cable
			sf.albedo = hex(0xa9c3d6)
		case box(p.X, p.Y, 14, 5, 19, 10), box(p.X, p.Y, -17, 12, -12.5, 16), box(p.X, p.Y, -3, 19, 0.5, 22.5): // chips
			sf.albedo = hex(0x15171a)
		}
	case n.Y < -0.97: // the bottom: the microSD slot, the red power LED, two slits, the reboot button (M5Stack's photos)
		switch {
		case box(p.X, p.Z, -13, -0.5, 0, 0.5):
			sf.albedo, sf.spec = colPortDark, 0.1
		case math.Hypot(p.X-6.8, p.Z-3.35) < 0.45: // the power / charge LED (red; M5Stack's photo of the bottom)
			sf.albedo, sf.emissive = hex(0xc8281e), hex(0xc8281e).mul(0.5)
		case box(p.X, p.Z, 6.0, -4.6, 6.5, -3.0), box(p.X, p.Z, 7.1, -4.6, 7.6, -3.0):
			sf.albedo = colPortDark
		}
		if d := math.Hypot(p.X-14.2, p.Z-1.45); d < 3 {
			sf.albedo = colShell.mul(0.95)
			if d > 2.6 {
				sf.albedo = colPortDark
			}
		}
	case n.Y > 0.97 && p.X > -15.6 && p.X < 0.5 && p.Z > -6 && p.Z < 6.5: // the top: the label (drawn without its text)
		sf.albedo, sf.spec = hex(0xf2f2f0), 0.15
		if p.X < -13.4 {
			sf.albedo = hex(0x9a9ca0)
		}
	case n.X < -0.97: // right: the power button, USB-C, the Grove port
		const cz = 0.4
		if d := roundRect(p.Z-cz, p.Y-14, 3, 2.6, 1.2); d <= 0 {
			sf.albedo = colShell.mul(0.9)
			if d > -0.35 {
				sf.albedo = colShell.mul(0.6)
			}
		}
		if roundRect(p.Z-cz, p.Y+0.7, 1.5, 4.4, 1.4) <= 0 {
			sf.albedo, sf.spec = colPortDark, 0.2
		}
		if d := roundRect(p.Z-cz, p.Y+13.5, 2.5, 4, 0.4); d <= 0 {
			sf.albedo = hex(0xa83a3c)
			if d < -0.8 {
				sf.albedo = hex(0x3a1416)
			}
		}
	}
	return sf
}

// frontMarks: under the screen, the red ring and the sensors' dots (p from the CoreS3's centre).
func frontMarks(p V3, sf *surface) {
	const y = -27 + 6.7
	d := math.Hypot(p.X, p.Y-y)
	if d < 1.9 && d > 1.25 {
		sf.albedo, sf.glass = colRing, false
		return
	}
	if d <= 1.25 {
		sf.albedo = colLens
		return
	}
	for _, dot := range [][2]float64{{-14.5, 0.55}, {5.5, 0.3}, {7.5, 0.3}, {13.5, 0.5}} {
		if math.Hypot(p.X-dot[0], p.Y-y) < dot[1] {
			sf.albedo = hex(0x2a2c31) // the microphones
			if dot[1] < 0.4 {
				sf.albedo, sf.glass = colSensor, false // the LTR-553's windows
			}
		}
	}
}

// body shades M5Stack's main body (p, n in the robot's rest space): on each side the label (a
// light sticker with a dark end, drawn without its text) and the LED bar along the top (on the
// firmware's left strip, LEDs 0-5, LED 0 at the front: on the left as you face the screen, -X;
// its right strip, LEDs 6-11, 11 at the front, on +X); at the back a sticker and two ports.
func (s *scene) body(p, n V3, mat int) surface {
	sf := surface{albedo: colShell, spec: 0.18, shine: 24}
	top := robotH
	switch {
	case mat == matBar:
		s.ledBar(p.Z, p.X < 0, &sf)
	case mat == matLedBoard:
		sf.albedo, sf.spec = hex(0xe9e9e2), 0.15 // a white board (the owner's photo of the inside)
	case mat == matLedChip:
		s.ledChip(p.Z, p.X < 0, &sf)
	case math.Abs(n.X) > 0.97 && math.Abs(p.X) > 26:
		left := p.X > 0
		if p.Z > stickerZ0 && p.Z < stickerZ1 && p.Y > stickerY0 && p.Y < stickerY1 {
			sf.albedo, sf.spec = hex(0xf3f3f1), 0.25
			if (left && p.Y > top-18) || (!left && p.Y < headBottom+20) {
				sf.albedo = colPortDark
				// its face, turned on its side (the owner's photos): two eyes toward the front, the mouth behind
				cy := (top - 18 + stickerY1) / 2
				if !left {
					cy = (stickerY0 + headBottom + 20) / 2
				}
				zc := (stickerZ0 + stickerZ1) / 2
				if math.Hypot(p.Z-zc-1.1, p.Y-cy-3.4) < 0.55 || math.Hypot(p.Z-zc-1.1, p.Y-cy+3.4) < 0.55 ||
					(math.Abs(p.Z-zc+1.1) < 0.22 && math.Abs(p.Y-cy) < 2.4) {
					sf.albedo = hex(0xf3f3f1)
				}
			}
		}
		s.barGlow(p.Z, p.Y, p.X < 0, &sf)
	case mat == matTopBoard && n.Y > 0.9: // M5Stack's photo from above: port C (teal) on the left, port B (black) on the right, the IR receiver at the front
		pins := func(x0 float64) bool { // four pins in a row
			k := math.Round((p.X - x0) / 2)
			return k >= 0 && k <= 3 && math.Abs(p.X-x0-2*k) < 0.35 && math.Abs(p.Z-15.6) < 0.35
		}
		switch {
		case box(p.X, p.Z, 5.5, 13.0, 17.0, 18.1):
			sf.albedo, sf.spec = colPortTeal, 0.2
			if pins(8.3) {
				sf.albedo = hex(0xc8ccd0)
			}
		case box(p.X, p.Z, -16.0, 13.0, -4.6, 18.1):
			sf.albedo, sf.spec = colPortDark, 0.1
			if pins(-13.3) {
				sf.albedo = hex(0xc8ccd0)
			}
		case math.Hypot(p.X-0.3, p.Z-17.6) < 0.5:
			sf.albedo = hex(0x1e5a3c)
		}
	}
	return sf
}

// led is the LED under a point of a bar: six along it. left is the firmware's left strip
// (leftNeonLight, LEDs 0-5, LED 0 at the front), on the left as you face the screen: the robot's
// own right side, -X (the owner's look at the robot, 2026-10-11); its right strip has 6-11, 11 at
// the front.
func (s *scene) led(z float64, left bool) *rgb {
	k := int(clamp((barZ1-z)/((barZ1-barZ0)/6), 0, 5))
	if !left {
		k = 11 - k
	}
	return s.leds[k]
}

// glow is the light in a light guide at z: each lit LED's colour, strongest over its chip and
// fading along the guide (one LED lights about two thirds of it, the owner's photo), mixed where
// several overlap. a: how lit it is there, 0..1.
func (s *scene) glow(z float64, left bool) (c rgb, a float64) {
	var sum rgb
	w := 0.0
	for k := 0; k < 6; k++ {
		idx := k
		if !left {
			idx = 11 - k
		}
		if l := s.leds[idx]; l != nil {
			wk := math.Exp(-math.Abs(z-ledZ(k)) / 6)
			sum, w = sum.add(l.mul(wk)), w+wk
		}
	}
	if w == 0 {
		return rgb{}, 0
	}
	return sum.mul(1 / math.Max(w, 1)), math.Min(w, 1)
}

// ledChip shades an LED on its board: a white package, its colour when lit (it lights the inside
// of the head too, the owner's photo).
func (s *scene) ledChip(z float64, left bool, sf *surface) {
	sf.albedo, sf.spec, sf.shine = hex(0xf2f2ee), 0.3, 40
	if c := s.led(z, left); c != nil {
		sf.albedo, sf.emissive = c.mul(0.5), c.add(rgb{0.4, 0.4, 0.4}.mulc(*c))
	}
}

// ledBar shades a light guide bar: milky when off, its LEDs' light when on.
func (s *scene) ledBar(z float64, left bool, sf *surface) {
	sf.albedo, sf.spec, sf.shine = colBarOff, 0.3, 40
	if c, a := s.glow(z, left); a > 0 {
		sf.albedo, sf.spec = colBarOff.lerp(c.mul(0.35/math.Max(a, 1e-6)), a), 0.15 // the light, not the plastic
		sf.emissive = c.mul(0.75)
	}
}

// barGlow: a lit LED's light on the shell around its bar.
func (s *scene) barGlow(z, y float64, left bool, sf *surface) {
	dz := math.Max(0, math.Max(barZ0-z, z-barZ1))
	dy := math.Max(0, math.Max(barY0-y, y-barY1))
	if d := math.Hypot(dz, dy); d < 4 {
		if c, a := s.glow(clamp(z, barZ0, barZ1), left); a > 0 {
			sf.emissive = sf.emissive.add(c.mul(0.35 * math.Exp(-d/1.1)))
		}
	}
}

func box(x, y, x0, y0, x1, y1 float64) bool { return x >= x0 && x <= x1 && y >= y0 && y <= y1 }

// roundRect is the signed distance to a rectangle of half sizes hx, hy with corners rounded by r.
func roundRect(x, y, hx, hy, r float64) float64 {
	qx, qy := math.Abs(x)-hx+r, math.Abs(y)-hy+r
	return math.Hypot(math.Max(qx, 0), math.Max(qy, 0)) + math.Min(math.Max(qx, qy), 0) - r
}

// light shades a surface: n the world normal, v the direction to the camera.
func light(sf surface, n, v V3) rgb {
	amb := 0.52 + 0.06*n.Y
	diff := amb + 0.4*math.Max(0, n.Dot(keyDir)) + 0.22*math.Max(0, n.Dot(fillDir)) + 0.2*math.Max(0, n.Dot(backDir))
	c := sf.albedo.mul(diff)
	for _, l := range []struct {
		d V3
		i float64
	}{{keyDir, 1}, {fillDir, 0.35}} {
		h := l.d.Add(v).Norm()
		c = c.add(rgb{1, 1, 1}.mul(sf.spec * l.i * math.Pow(math.Max(0, n.Dot(h)), sf.shine)))
	}
	if sf.glass { // the room in the glass: brighter above, more at grazing angles
		r := n.Mul(2 * n.Dot(v)).Sub(v)
		f := 0.05 + 0.6*math.Pow(1-math.Max(0, n.Dot(v)), 4)
		env := hex(0x3a3d44).lerp(hex(0xd8dde6), smoothstep(-0.2, 0.9, r.Y))
		c = c.add(env.mul(f * 0.55))
	}
	return c.add(sf.emissive)
}

// texture is an image as floats, sampled bilinearly.
type texture struct {
	w, h int
	px   []rgb
}

func (t *texture) sample(u, v float64) rgb {
	x := clamp(u*float64(t.w)-0.5, 0, float64(t.w-1))
	y := clamp(v*float64(t.h)-0.5, 0, float64(t.h-1))
	x0, y0 := int(x), int(y)
	x1, y1 := min(x0+1, t.w-1), min(y0+1, t.h-1)
	fx, fy := x-float64(x0), y-float64(y0)
	a := t.px[y0*t.w+x0].lerp(t.px[y0*t.w+x1], fx)
	b := t.px[y1*t.w+x0].lerp(t.px[y1*t.w+x1], fx)
	return a.lerp(b, fy)
}

// material is the plate's and the servo's look.
func material(mat int) surface {
	switch mat {
	case matPlate, matBaseCover:
		return surface{albedo: colPlate, spec: 0.12, shine: 20}
	case matServo:
		return surface{albedo: colDisc, spec: 0.25, shine: 30}
	case matServoCover:
		return surface{albedo: colServo, spec: 0.15, shine: 20}
	case matPitchServo:
		return surface{albedo: colServo, spec: 0.2, shine: 30} // as the servo body around it: nothing black shows at the back (the owner's photo)
	}
	return surface{albedo: colShell}
}
