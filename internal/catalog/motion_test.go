package catalog

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"math"
	"os"
	"path/filepath"
	"testing"
)

// A small motion: the same bytes twice, its frames add up to the loop, the background is
// transparent and each frame clears it for the next.
func TestMotionGIF(t *testing.T) {
	f := func(v float64) *float64 { return &v }
	leds := "#ff0000*12"
	crop := 2
	r := Render{Name: "m", Size: "72x72", Pose: Pose{El: f(10)}, Frames: []Frame{
		{Screen: "screens/launcher.png", Ms: 400, Pose: Pose{Pitch: f(10)}},
		{Screen: "screens/raw-face.png", Ms: 300, Pose: Pose{Yaw: f(10), Pitch: f(16), LEDs: &leds}},
		{Screen: "screens/focus.png", Ms: 300, Pose: Pose{Yaw: f(-8)}},
	}, Motion: &Motion{FPS: 10, StartMs: 50, EaseMs: 200, FadeMs: 100, Crop: &crop, Colors: 32,
		Sway: []Sway{{Of: "az", Deg: 20, Cycles: 1}, {Of: "yaw", Deg: 2, Cycles: 3}}}}
	root := filepath.Join("..", "..")
	a, err := r.Make(root)
	if err != nil {
		t.Fatal(err)
	}
	b, err := r.Make(root)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a, b) {
		t.Fatal("two runs give different bytes")
	}
	g, err := gif.DecodeAll(bytes.NewReader(a))
	if err != nil {
		t.Fatal(err)
	}
	cs := 0
	for i, d := range g.Delay {
		cs += d
		if g.Disposal[i] != gif.DisposalBackground {
			t.Errorf("frame %d: disposal %d, want the background (transparent) for the next", i, g.Disposal[i])
		}
	}
	if len(g.Image) != 10 || cs != 100 || g.LoopCount != 0 {
		t.Fatalf("%d frames, %d cs, loop %d: want 10 frames, 100 cs, looping forever", len(g.Image), cs, g.LoopCount)
	}
	pal := g.Image[0].Palette // the global one, with the frame's transparent colour
	if _, _, _, alpha := pal[0].RGBA(); alpha != 0 || len(pal) > 32 {
		t.Fatalf("palette of %d, the first %v: want 32 at most, the first transparent", len(pal), pal[0])
	}
	if g.Config.Width >= 72 && g.Config.Height >= 72 {
		t.Errorf("%dx%d: not cropped to the robot", g.Config.Width, g.Config.Height)
	}
	for i, p := range g.Image {
		if !p.Bounds().In(image.Rect(0, 0, g.Config.Width, g.Config.Height)) || p.Bounds().Dx() < 20 {
			t.Errorf("frame %d: %v in %dx%d", i, p.Bounds(), g.Config.Width, g.Config.Height)
		}
	}
	r.Motion.MaxKB = 1
	if _, err := r.Make(root); err == nil {
		t.Error("bigger than max_kb, no error")
	}
}

// The head eases into each scene's pose, the screen and the LEDs crossfade, the waves go over
// the loop; the scene before the first is the last one (the loop).
func TestMotionPose(t *testing.T) {
	screen := func(v uint8) *image.RGBA {
		img := image.NewRGBA(image.Rect(0, 0, 4, 3))
		for i := range img.Pix {
			img.Pix[i] = v
		}
		return img
	}
	red, green := color.RGBA{200, 0, 0, 255}, color.RGBA{0, 100, 0, 255}
	s := []scene{
		{screen: screen(0), pose: [5]float64{0, 10, 0, 8, 1}, ms: 1000},
		{screen: screen(200), pose: [5]float64{20, 30, 0, 8, 1}, ms: 1000},
	}
	s[0].leds[0], s[1].leds[0] = red, green
	m := &Motion{EaseMs: 400, FadeMs: 200}
	o := m.pose(s, 1000, 2000) // scene 1 starts: as scene 0 ended
	if o.Yaw != 0 || o.Pitch != 10 || o.Screen.(*image.RGBA).Pix[0] != 0 || o.LEDs[0] != red {
		t.Errorf("at a scene's start: yaw %v pitch %v screen %v LED %v, want the scene before's", o.Yaw, o.Pitch, o.Screen.(*image.RGBA).Pix[0], o.LEDs[0])
	}
	o = m.pose(s, 1100, 2000) // half the fade, a quarter of the move (eased: less)
	if px := o.Screen.(*image.RGBA).Pix[0]; px != 100 || o.Yaw <= 0 || o.Yaw >= 5 {
		t.Errorf("half way through the fade: screen %d (want 100), yaw %v (want 0..5)", px, o.Yaw)
	}
	o = m.pose(s, 1400, 2000) // moved and faded
	if o.Yaw != 20 || o.Pitch != 30 || o.Screen != s[1].screen || o.LEDs[0] != green {
		t.Errorf("after the move: yaw %v pitch %v LED %v, want scene 1's", o.Yaw, o.Pitch, o.LEDs[0])
	}
	o = m.pose(s, 100, 2000) // scene 0, coming from scene 1
	if o.Yaw <= 0 || o.Yaw >= 20 {
		t.Errorf("scene 0 after the loop's end: yaw %v, want on its way from 20 to 0", o.Yaw)
	}
	m.Sway = []Sway{{Of: "az", Deg: 26, Cycles: 1}}
	if o := m.pose(s, 500, 2000); math.Abs(o.Azimuth-26) > 1e-9 {
		t.Errorf("a quarter into the loop: az %v, want 26", o.Azimuth)
	}
}

// The committed motion GIFs are as renders.json says: their frame count, under max_kb.
func TestMotionFiles(t *testing.T) {
	root := filepath.Join("..", "..")
	rs, err := Load(filepath.Join(root, "renders.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rs.Renders {
		if r.Motion == nil {
			continue
		}
		b, err := os.ReadFile(filepath.Join(root, "renders", r.File()))
		if err != nil {
			t.Errorf("%s: %v (run robot3d-renders)", r.Name, err)
			continue
		}
		g, err := gif.DecodeAll(bytes.NewReader(b))
		if err != nil {
			t.Fatalf("%s: %v", r.Name, err)
		}
		loop := 0
		for _, f := range r.Frames {
			loop += f.Ms
		}
		if want := int(math.Round(float64(loop) * r.Motion.FPS / 1000)); len(g.Image) != want {
			t.Errorf("%s: %d frames, want %d", r.Name, len(g.Image), want)
		}
		if r.Motion.MaxKB > 0 && len(b) > r.Motion.MaxKB*1024 {
			t.Errorf("%s: %d bytes, max_kb %d", r.Name, len(b), r.Motion.MaxKB)
		}
	}
}
