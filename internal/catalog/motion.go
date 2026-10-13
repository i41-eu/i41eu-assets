package catalog

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"math"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/i41-eu/i41eu-assets/robot3d"
)

// Motion plays an animation's frames as the scenes of one smooth loop: each frame's ms is how
// long its scene lasts; the head moves into the scene's pose (eased in and out), the screen and
// the LEDs crossfade into the scene's, and waves (Sway) go over the whole loop, e.g. the camera
// swaying from side to side. The GIF has FPS frames a second, transparent round the robot
// unless the render has a bg.
type Motion struct {
	FPS     float64 `json:"fps"`
	StartMs int     `json:"start_ms,omitempty"` // the loop's time at the GIF's first frame (the still a page shows before it plays)
	EaseMs  int     `json:"ease_ms,omitempty"`  // the move into a scene's pose: yaw, pitch, az, el, zoom; 0: a cut
	FadeMs  int     `json:"fade_ms,omitempty"`  // the screen and the LEDs crossfading into a scene's; 0: a cut
	Sway    []Sway  `json:"sway,omitempty"`
	Crop    *int    `json:"crop,omitempty"`   // trim to the robot's box over the loop, with this many pixels round it
	Colors  int     `json:"colors,omitempty"` // the GIF's palette, 3..256 (one of them transparent); 0: 256
	MaxKB   int     `json:"max_kb,omitempty"` // fail if the GIF is bigger (KiB): fewer colors, a smaller size or fewer fps
}

// Sway is a sine wave over the loop added to the pose: Deg degrees each way, Cycles times a loop
// (whole numbers keep the loop seamless).
type Sway struct {
	Of     string  `json:"of"` // yaw, pitch, az or el
	Deg    float64 `json:"deg"`
	Cycles float64 `json:"cycles"`
}

// scene is a frame of a Motion, resolved.
type scene struct {
	screen *image.RGBA
	leds   [12]color.RGBA
	pose   [5]float64 // yaw, pitch, az, el, zoom
	ms     float64
}

// motion renders a Motion's loop as a GIF.
func (r Render) motion(base string, w, h int) ([]byte, error) {
	m := r.Motion
	if m.FPS <= 0 || m.FPS > 50 || m.Colors < 0 || m.Colors > 256 || (m.Colors > 0 && m.Colors < 3) {
		return nil, fmt.Errorf("%s: motion: fps 0..50, colors 3..256 (0: 256)", r.Name)
	}
	for _, s := range m.Sway {
		if poseIndex(s.Of) < 0 || poseIndex(s.Of) > 3 {
			return nil, fmt.Errorf("%s: sway of %q: yaw, pitch, az or el", r.Name, s.Of)
		}
	}
	var bg color.Color
	if r.Bg != "" {
		var err error
		if bg, err = robot3d.ParseColor(r.Bg); err != nil {
			return nil, fmt.Errorf("%s: %v", r.Name, err)
		}
	}
	scenes := make([]scene, len(r.Frames))
	loop := 0.0
	for i, f := range r.Frames {
		if f.Caption != "" || f.Ms <= 0 || f.Screen == "" {
			return nil, fmt.Errorf("%s frame %d: a motion's frames have a screen and ms, no caption", r.Name, i)
		}
		o, err := options(r.Pose, f.Pose, w, h)
		if err != nil {
			return nil, fmt.Errorf("%s frame %d: %v", r.Name, i, err)
		}
		img, err := loadImage(filepath.Join(base, f.Screen))
		if err != nil {
			return nil, err
		}
		s := scene{screen: rgba(img), ms: float64(f.Ms), pose: [5]float64{o.Yaw, o.Pitch, o.Azimuth, o.Elevation, o.Zoom}}
		if i > 0 && s.screen.Bounds() != scenes[0].screen.Bounds() {
			return nil, fmt.Errorf("%s frame %d: %s is not the size of the first frame's screen", r.Name, i, f.Screen)
		}
		for j, c := range o.LEDs {
			if c != nil {
				s.leds[j] = color.RGBAModel.Convert(c).(color.RGBA)
			}
		}
		scenes[i] = s
		loop += s.ms
	}
	n := max(1, int(math.Round(loop*m.FPS/1000)))

	// the frames, rendered in parallel, each one at its place in the loop
	frames := make([]*image.RGBA, n)
	parallel(n, func(i int) {
		t := math.Mod(float64(i)*loop/float64(n)+float64(m.StartMs), loop)
		o := m.pose(scenes, t, loop)
		o.Width, o.Height, o.NoShadow, o.Background = w, h, true, bg
		frames[i] = robot3d.Render(o)
	})

	box := image.Rect(0, 0, w, h)
	if m.Crop != nil {
		u := image.Rectangle{}
		for _, f := range frames {
			u = u.Union(opaqueBox(f, f.Bounds()))
		}
		if !u.Empty() {
			box = u.Inset(-*m.Crop).Intersect(box)
		}
	}
	colors := m.Colors
	if colors == 0 {
		colors = 256
	}
	var pal color.Palette
	if bg == nil {
		pal = append(color.Palette{color.RGBA{}}, medianCut(opaqueSamples(frames, box), colors-1)...)
	} else {
		pal = medianCut(opaqueSamples(frames, box), colors)
	}

	anim := &gif.GIF{LoopCount: 0, Config: image.Config{ColorModel: pal, Width: box.Dx(), Height: box.Dy()}}
	anim.Image = make([]*image.Paletted, n)
	parallel(n, func(i int) {
		anim.Image[i] = quantize(frames[i], box, pal, bg == nil)
	})
	for i := range n { // centiseconds, adding up to the loop
		at := func(k int) int { return int(math.Round(float64(k) * loop / float64(n) / 10)) }
		anim.Delay = append(anim.Delay, max(2, at(i+1)-at(i)))
		if bg == nil {
			anim.Disposal = append(anim.Disposal, gif.DisposalBackground) // the next frame starts transparent
		} else {
			anim.Disposal = append(anim.Disposal, gif.DisposalNone)
		}
	}
	var buf bytes.Buffer
	if err := gif.EncodeAll(&buf, anim); err != nil {
		return nil, err
	}
	if kb := (buf.Len() + 1023) / 1024; m.MaxKB > 0 && kb > m.MaxKB {
		return nil, fmt.Errorf("%s: %d KiB, more than max_kb %d: fewer colors, a smaller size or fewer fps", r.Name, kb, m.MaxKB)
	}
	return buf.Bytes(), nil
}

// pose is the robot at time t (ms) of the loop: the scene it is in, easing and fading from the
// one before, and the waves.
func (m *Motion) pose(scenes []scene, t, loop float64) robot3d.Options {
	k, u := 0, t
	for k < len(scenes)-1 && u >= scenes[k].ms {
		u -= scenes[k].ms
		k++
	}
	cur, prev := scenes[k], scenes[(k+len(scenes)-1)%len(scenes)]
	fade, ease := ramp(u, float64(m.FadeMs)), ramp(u, float64(m.EaseMs))
	var p [5]float64
	for j := range p {
		p[j] = lerp(prev.pose[j], cur.pose[j], ease)
	}
	for _, s := range m.Sway {
		p[poseIndex(s.Of)] += s.Deg * math.Sin(2*math.Pi*s.Cycles*t/loop)
	}
	o := robot3d.Options{Yaw: p[0], Pitch: p[1], Azimuth: p[2], Elevation: p[3], Zoom: p[4], Screen: cur.screen}
	if fade < 1 {
		o.Screen = blend(prev.screen, cur.screen, fade)
	}
	for j := range o.LEDs {
		a, b := prev.leds[j], cur.leds[j]
		c := color.RGBA{mix8(a.R, b.R, fade), mix8(a.G, b.G, fade), mix8(a.B, b.B, fade), 255}
		if c.R|c.G|c.B != 0 {
			o.LEDs[j] = c
		}
	}
	return o
}

func poseIndex(of string) int {
	switch of {
	case "yaw":
		return 0
	case "pitch":
		return 1
	case "az":
		return 2
	case "el":
		return 3
	case "zoom":
		return 4
	}
	return -1
}

// ramp is 0 at the start, 1 after d, eased in and out (smoothstep); d 0: 1 at once.
func ramp(u, d float64) float64 {
	if d <= 0 {
		return 1
	}
	x := math.Max(0, math.Min(1, u/d))
	return x * x * (3 - 2*x)
}

func lerp(a, b, t float64) float64 { return a + (b-a)*t }

func mix8(a, b uint8, t float64) uint8 { return uint8(math.Round(lerp(float64(a), float64(b), t))) }

func blend(a, b *image.RGBA, t float64) *image.RGBA {
	out := image.NewRGBA(b.Bounds())
	for i := range out.Pix {
		out.Pix[i] = mix8(a.Pix[i], b.Pix[i], t)
	}
	return out
}

func rgba(img image.Image) *image.RGBA {
	b := img.Bounds()
	out := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(out, out.Bounds(), img, b.Min, draw.Src)
	return out
}

// parallel runs f(0..n-1) on every CPU; each call writes only its own results.
func parallel(n int, f func(i int)) {
	next := make(chan int)
	var wg sync.WaitGroup
	for range min(n, runtime.NumCPU()) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range next {
				f(i)
			}
		}()
	}
	for i := range n {
		next <- i
	}
	close(next)
	wg.Wait()
}

// opaque: what stays in a GIF with a transparent background (half covered or more).
func opaque(a uint8) bool { return a >= 128 }

// opaqueBox is the box round the opaque pixels of img within r.
func opaqueBox(img *image.RGBA, r image.Rectangle) image.Rectangle {
	box := image.Rectangle{}
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			if opaque(img.Pix[img.PixOffset(x, y)+3]) {
				box = box.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	return box
}

// unpremultiply gives a pixel's own colour (image.RGBA keeps it multiplied by its alpha): the
// robot's edge keeps the robot's colour instead of fading to black.
func unpremultiply(p []uint8) [3]uint8 {
	a := uint32(p[3])
	if a == 0 || a == 255 {
		return [3]uint8{p[0], p[1], p[2]}
	}
	return [3]uint8{uint8((uint32(p[0])*255 + a/2) / a), uint8((uint32(p[1])*255 + a/2) / a), uint8((uint32(p[2])*255 + a/2) / a)}
}

// opaqueSamples are about a million of the frames' opaque pixels in box, for the palette.
func opaqueSamples(frames []*image.RGBA, box image.Rectangle) [][3]uint8 {
	step := max(1, len(frames)*box.Dx()*box.Dy()/1_000_000)
	var px [][3]uint8
	k := 0
	for _, img := range frames {
		for y := box.Min.Y; y < box.Max.Y; y++ {
			for x := box.Min.X; x < box.Max.X; x++ {
				p := img.Pix[img.PixOffset(x, y):]
				if !opaque(p[3]) {
					continue
				}
				if k++; k%step == 0 {
					px = append(px, unpremultiply(p))
				}
			}
		}
	}
	return px
}

// quantize maps img within box to the palette, each pixel to its nearest colour (no dithering:
// it keeps the frames small); with transparent, palette[0] is the background and the frame is
// only the box round the robot.
func quantize(img *image.RGBA, box image.Rectangle, pal color.Palette, transparent bool) *image.Paletted {
	first := 0
	r := box
	if transparent {
		first = 1
		if r = opaqueBox(img, box); r.Empty() {
			r = image.Rect(box.Min.X, box.Min.Y, box.Min.X+1, box.Min.Y+1)
		}
	}
	rgb := make([][3]int, len(pal))
	for i, c := range pal {
		cr, cg, cb, _ := c.RGBA()
		rgb[i] = [3]int{int(cr >> 8), int(cg >> 8), int(cb >> 8)}
	}
	cache := map[[3]uint8]uint8{}
	out := image.NewPaletted(r.Sub(box.Min), pal)
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			p := img.Pix[img.PixOffset(x, y):]
			o := out.PixOffset(x-box.Min.X, y-box.Min.Y)
			if transparent && !opaque(p[3]) {
				out.Pix[o] = 0
				continue
			}
			c := unpremultiply(p)
			idx, ok := cache[c]
			if !ok {
				best := -1
				for i := first; i < len(rgb); i++ {
					dr, dg, db := int(c[0])-rgb[i][0], int(c[1])-rgb[i][1], int(c[2])-rgb[i][2]
					if d := dr*dr + dg*dg + db*db; best < 0 || d < best {
						best, idx = d, uint8(i)
					}
				}
				cache[c] = idx
			}
			out.Pix[o] = idx
		}
	}
	return out
}
