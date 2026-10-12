package robot3d

import (
	"image"
	"image/color"
	"testing"
)

// A front view shows the screen's picture in the middle of the head, the corners stay
// transparent, and the same options give the same pixels.
func TestRenderFront(t *testing.T) {
	o := Options{Width: 200, Height: 200, Screen: green(320, 240), Supersample: 2}
	img := Render(o)
	if c := img.RGBAAt(100, 85); !(c.G > 150 && c.R < 80 && c.B < 80) {
		t.Fatalf("the screen's middle is %v, want green", c)
	}
	if c := img.RGBAAt(2, 2); c.A != 0 {
		t.Fatalf("a corner is %v, want transparent", c)
	}
	if again := Render(o); string(again.Pix) != string(img.Pix) {
		t.Fatal("two renders differ")
	}
	// premultiplied: no channel above alpha anywhere
	for i := 0; i < len(img.Pix); i += 4 {
		if a := img.Pix[i+3]; img.Pix[i] > a || img.Pix[i+1] > a || img.Pix[i+2] > a {
			t.Fatalf("pixel %d is not premultiplied: %v", i/4, img.Pix[i:i+4])
		}
	}
}

// The screen turns with the head: seen from behind, there is no green; looking up moves it.
func TestRenderPose(t *testing.T) {
	count := func(img *image.RGBA) (n int, top int) {
		top = img.Bounds().Dy()
		for y := 0; y < img.Bounds().Dy(); y++ {
			for x := 0; x < img.Bounds().Dx(); x++ {
				if c := img.RGBAAt(x, y); c.G > 150 && c.R < 80 && c.B < 80 {
					n++
					top = min(top, y)
				}
			}
		}
		return
	}
	sub := green(32, 24)
	front, ftop := count(Render(Options{Width: 160, Height: 160, Screen: sub, Supersample: 1}))
	back, _ := count(Render(Options{Width: 160, Height: 160, Screen: sub, Supersample: 1, Azimuth: 180}))
	up, utop := count(Render(Options{Width: 160, Height: 160, Screen: sub, Supersample: 1, Pitch: 40}))
	if front < 1000 || back != 0 || up == 0 || utop >= ftop {
		t.Fatalf("green pixels: front %d (top %d), back %d, looking up %d (top %d)", front, ftop, back, up, utop)
	}
}

func green(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{0, 200, 0, 255})
		}
	}
	return img
}

// Parts covers every triangle of the model once, each part named and on a joint.
func TestParts(t *testing.T) {
	n := 0
	seen := map[string]bool{}
	for _, p := range Parts() {
		if p.Name == "" || p.Joint == "" || seen[p.Name] || len(p.Positions)%3 != 0 || len(p.Normals) != len(p.Positions) {
			t.Fatalf("part %+v", p.Name)
		}
		seen[p.Name] = true
		n += len(p.Positions) / 3
	}
	if n != len(mesh()) {
		t.Fatalf("%d triangles in parts, %d in the model", n, len(mesh()))
	}
	for _, want := range []string{"plate", "servo", "body", "core", "led-bar-left", "led-bar-right"} {
		if !seen[want] {
			t.Errorf("no part %s", want)
		}
	}
	if c, w, h := Screen(); c.Z != coreFront || w != screenW || h != screenH {
		t.Errorf("screen %v %v %v", c, w, h)
	}
}

// LED 0 (the firmware's left strip) lights the bar on the left as you face the screen: the
// robot's own right side, -X, seen from there (azimuth -90); from the other side nothing red.
func TestLEDSides(t *testing.T) {
	var leds [12]color.Color
	leds[0] = color.RGBA{255, 0, 0, 255}
	red := func(az float64) int {
		img := Render(Options{Width: 200, Height: 200, LEDs: leds, Azimuth: az, Elevation: 20, Supersample: 1, NoShadow: true})
		n := 0
		for i := 0; i < len(img.Pix); i += 4 {
			if r, g, b := img.Pix[i], img.Pix[i+1], img.Pix[i+2]; r > 150 && g < 90 && b < 90 {
				n++
			}
		}
		return n
	}
	if l, r := red(-90), red(90); l == 0 || r != 0 {
		t.Fatalf("LED 0 seen from -X: %d red pixels, from +X: %d", l, r)
	}
}
