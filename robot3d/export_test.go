package robot3d

import (
	"image/color"
	"math"
	"strings"
	"testing"
)

// Surface gives the renderer's details: the glass at the screen, the power button and USB-C on
// the CoreS3's right, StackChan's board seen through the open back; the plain shell elsewhere.
func TestSurface(t *testing.T) {
	shell, _ := Surface("core", V3{10, coreCentre.Y, coreCentre.Z}, V3{0, 1, 0}) // the top, off its label
	centre, _, _ := Screen()
	if _, glass := Surface("core", centre, V3{0, 0, 1}); !glass {
		t.Error("the screen's centre is not glass")
	}
	right := V3{-1, 0, 0}
	for name, p := range map[string]V3{
		"power button":      coreCentre.Add(V3{-27, 12.9, -0.6}),
		"power button ring": coreCentre.Add(V3{-27, 12.9, 3.9}),
		"power sign":        coreCentre.Add(V3{-27, 19.2 + 0.6, -0.2 + 0.6}),
		"USB-C":             coreCentre.Add(V3{-27, 0, -0.4}),
		"Grove port":        coreCentre.Add(V3{-27, -14.1, -1.2}),
	} {
		if c, _ := Surface("core", p, right); c == shell {
			t.Errorf("%s: the plain shell", name)
		}
	}
	// M5Stack's drawing: the button 9.9 wide and 7.5 high: below it, behind it and in front of
	// it the shell (the old one was 6 x 5.2, 1 mm higher and further forward).
	for _, d := range []V3{{0, 12.9 - 4.2, -0.6}, {0, 12.9, -0.6 - 5.4}, {0, 12.9, -0.6 + 5.4}, {0, 12.9 + 4.2, -0.6}} {
		if c, _ := Surface("core", coreCentre.Add(V3{-27, d.Y, d.Z}), right); c != shell {
			t.Errorf("next to the power button at y %+.1f z %+.1f: %v, want the shell", d.Y, d.Z, c)
		}
	}
	back := coreCentre.Add(V3{0, 0, -coreD/2 - coreOverLip})
	if c, _ := Surface("core", back.Add(V3{20, -10, 0}), V3{0, 0, -1}); c.G <= c.R || c.G > 90 {
		t.Errorf("the board through the open back, not dark green: %v", c)
	}
	if c, _ := Surface("core", back.Add(V3{-7, 10, 0}), V3{0, 0, -1}); c.B <= c.R+20 {
		t.Errorf("the ribbon cable: %v", c)
	}
	if c, glass := Surface("core", coreCentre.Add(V3{-27, -20, 0.4}), right); glass || c != shell {
		t.Errorf("the right side below the ports: %v %v, want the shell %v", c, glass, shell)
	}
}

// The body's back as on M5Stack's product photos: the back cover mid grey between the light grey
// servo body and the dark stand, two silver screws and the red LED in the light band at its
// foot, the battery blue through the cover's cutout, the stand's USB-C dark in a metal shell.
func TestBack(t *testing.T) {
	back := V3{0, 0, -1}
	at := func(part string, p V3) color.RGBA {
		c, _ := Surface(part, p, back)
		return c
	}
	lum := func(c color.RGBA) int { return (299*int(c.R) + 587*int(c.G) + 114*int(c.B)) / 1000 }
	band, cover, stand := at("servo", V3{6, screwY, -23.3}), at("servo-cover", V3{0, 25, -22.4}), at("plate", V3{12, 4, -23.6})
	if lum(cover) > lum(band)-60 || lum(cover) < lum(stand)-30 || lum(cover) > lum(stand)+30 {
		t.Errorf("the back cover %v: not mid grey between the band %v and the stand %v", cover, band, stand)
	}
	if c := at("servo", V3{backLedX, backLedY, -23.15}); c.R < 120 || int(c.R) < 2*int(c.G) {
		t.Errorf("the LED between the screws: %v, not red", c)
	}
	for _, sx := range []float64{-screwX, screwX} {
		if c := at("servo-cover", V3{sx + 1, screwY, -22}); lum(c) < lum(cover)+40 || c.R > c.B {
			t.Errorf("the screw at x %+.1f: %v, not silver", sx, c)
		}
	}
	if c := at("battery", V3{0, batteryY0 + 1, batteryZ0}); int(c.B) < int(c.R)+80 {
		t.Errorf("the battery: %v, not blue", c)
	}
	if c := at("plate", V3{standUsbX + 3.8, standUsbY, -23.3}); lum(c) > 40 {
		t.Errorf("the stand's USB-C: %v, not dark", c)
	}
}

// The servo body stands on the plate's ring round the turntable, not in it, and the head
// pitches about the main body's boss (M5Stack's drawing of the back: the screws, the LED window
// and the horn 3 mm higher than with the servo body's foot on the plate's 8 mm top).
func TestServoOnPlate(t *testing.T) {
	foot, ring := math.Inf(1), math.Inf(-1)
	for _, p := range Parts() {
		for _, v := range p.Positions {
			switch p.Name {
			case "servo", "servo-cover":
				foot = math.Min(foot, v.Y)
			case "plate":
				ring = math.Max(ring, v.Y)
			}
		}
	}
	if foot < ring {
		t.Errorf("the servo body's foot at y %.2f, in the plate (up to %.2f)", foot, ring)
	}
	if math.Abs(pivot.Y-43.5) > 0.05 {
		t.Errorf("the pitch axis at y %.2f, the main body's boss at 43.5", pivot.Y)
	}
}

// Every part's triangles are wound counter-clockwise seen from the side their normals point to
// (as glTF and other renderers that cull or light by winding expect).
func TestPartsWound(t *testing.T) {
	for _, p := range Parts() {
		inward := 0
		for i := 0; i < len(p.Positions); i += 3 {
			face := p.Positions[i+1].Sub(p.Positions[i]).Cross(p.Positions[i+2].Sub(p.Positions[i]))
			if face.Dot(p.Normals[i].Add(p.Normals[i+1]).Add(p.Normals[i+2])) < 0 {
				inward++
			}
		}
		if inward > 0 {
			t.Errorf("%s: %d of %d triangles wound against their normals", p.Name, inward, len(p.Positions)/3)
		}
	}
}

// The LED bars follow the head's rounded top edge (M5Stack's main body: radius 5 mm), as the
// light guides on M5Stack's product photos do: nothing of them stands out of that round, and the
// light shows in the slot through it, about 1.7 mm high on the side and 1.7 mm wide on the top
// (the drawing: y 67.9 to 69.9 on the side; the top photo: x 24.8 to 26.1).
func TestLightGuideFlush(t *testing.T) {
	var all []V3 // triangles, three points each, with their part
	var names []string
	for _, p := range Parts() {
		for i, v := range p.Positions {
			all = append(all, v)
			if i%3 == 0 {
				names = append(names, p.Name)
			}
			if strings.HasPrefix(p.Name, "led-bar-") {
				if r := math.Hypot(math.Abs(v.X)-edgeCX, v.Y-edgeCY); r > edgeR+1e-6 && v.Y > edgeCY {
					t.Fatalf("%s: a point %.2f mm out of the head's rounded edge at %+v", p.Name, r-edgeR, v)
				}
			}
		}
	}
	first := func(o, d V3) string { // the part a ray from o along d meets first
		best, part := math.Inf(1), ""
		for i := 0; i+2 < len(all); i += 3 {
			e1, e2 := all[i+1].Sub(all[i]), all[i+2].Sub(all[i])
			p := d.Cross(e2)
			det := e1.Dot(p)
			if math.Abs(det) < 1e-12 {
				continue
			}
			s := o.Sub(all[i])
			u, q := s.Dot(p)/det, s.Cross(e1)
			v := d.Dot(q) / det
			if dist := e2.Dot(q) / det; u >= 0 && v >= 0 && u+v <= 1 && dist > 0 && dist < best {
				best, part = dist, names[i/3]
			}
		}
		return part
	}
	span := func(o func(s float64) V3, d V3, from, to float64) (lo, hi float64) {
		lo, hi = math.Inf(1), math.Inf(-1)
		for s := from; s <= to; s += 0.05 {
			if first(o(s), d) == "led-bar-right" {
				lo, hi = math.Min(lo, s), math.Max(hi, s)
			}
		}
		return
	}
	if lo, hi := span(func(y float64) V3 { return V3{40, y, -1.5} }, V3{-1, 0, 0}, 64, 71); lo < 67.8 || hi > 70.1 || hi-lo < 1.4 || hi-lo > 2.2 {
		t.Errorf("from the side the light shows from y %.2f to %.2f, want about 68.1 to 69.8", lo, hi)
	}
	if lo, hi := span(func(x float64) V3 { return V3{x, 90, -1.5} }, V3{0, -1, 0}, 20, 28); lo < 24.3 || hi > 26.8 || hi-lo < 1.4 || hi-lo > 2.2 {
		t.Errorf("from above the light shows from x %.2f to %.2f, want about 24.6 to 26.3", lo, hi)
	}
}
