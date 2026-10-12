// Package catalog renders the robot pictures listed in renders.json (stills as PNG, animations
// as GIF) and copies them into other repositories as distribute.json says.
package catalog

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	_ "image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"slices"

	"github.com/i41-eu/i41eu-assets/robot3d"
)

// Renders is renders.json.
type Renders struct {
	Renders []Render `json:"renders"`
}

// Render is one picture: a still (Screen) or an animation (Frames).
type Render struct {
	Name  string `json:"name"`
	About string `json:"about,omitempty"`
	Size  string `json:"size"`         // WxH
	Bg    string `json:"bg,omitempty"` // "#rrggbb"; empty: transparent (stills only)
	Pose
	Screen string  `json:"screen,omitempty"` // a still's screen
	Frames []Frame `json:"frames,omitempty"` // an animation's frames
}

// Pose is where the head and the camera are, and the LEDs. In a frame, a field left out keeps
// the render's value.
type Pose struct {
	Yaw   *float64 `json:"yaw,omitempty"`
	Pitch *float64 `json:"pitch,omitempty"`
	Az    *float64 `json:"az,omitempty"`
	El    *float64 `json:"el,omitempty"`
	Zoom  *float64 `json:"zoom,omitempty"`
	LEDs  *string  `json:"leds,omitempty"`
}

// Frame is one frame of an animation.
type Frame struct {
	Screen  string `json:"screen"`
	Caption string `json:"caption,omitempty"`
	Ms      int    `json:"ms"`
	Pose
}

// Load reads renders.json.
func Load(path string) (*Renders, error) {
	var r Renders
	if err := readJSON(path, &r); err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, x := range r.Renders {
		if x.Name == "" || seen[x.Name] {
			return nil, fmt.Errorf("%s: a render without a name, or %q twice", path, x.Name)
		}
		seen[x.Name] = true
		if (x.Screen == "") == (len(x.Frames) == 0) {
			return nil, fmt.Errorf("%s: %s: a screen (a still) or frames (an animation)", path, x.Name)
		}
	}
	return &r, nil
}

// File is the render's file name: name.png for a still, name.gif for an animation.
func (r Render) File() string {
	if len(r.Frames) > 0 {
		return r.Name + ".gif"
	}
	return r.Name + ".png"
}

// Make renders it; paths are relative to base.
func (r Render) Make(base string) ([]byte, error) {
	var w, h int
	if _, err := fmt.Sscanf(r.Size, "%dx%d", &w, &h); err != nil || w <= 0 || h <= 0 {
		return nil, fmt.Errorf("%s: size %q: want WxH", r.Name, r.Size)
	}
	if len(r.Frames) == 0 {
		o, err := options(r.Pose, Pose{}, w, h)
		if err != nil {
			return nil, fmt.Errorf("%s: %v", r.Name, err)
		}
		if o.Screen, err = loadImage(filepath.Join(base, r.Screen)); err != nil {
			return nil, err
		}
		if r.Bg != "" {
			if o.Background, err = robot3d.ParseColor(r.Bg); err != nil {
				return nil, fmt.Errorf("%s: %v", r.Name, err)
			}
		}
		var buf bytes.Buffer
		if err := png.Encode(&buf, robot3d.Render(o)); err != nil {
			return nil, err
		}
		return buf.Bytes(), nil
	}
	return r.animation(base, w, h)
}

// animation: each frame the robot with its screen under a caption band, on an opaque background.
func (r Render) animation(base string, w, h int) ([]byte, error) {
	bgHex := r.Bg
	if bgHex == "" {
		bgHex = "#ffffff"
	}
	bg, err := robot3d.ParseColor(bgHex)
	if err != nil {
		return nil, fmt.Errorf("%s: %v", r.Name, err)
	}
	band := 0
	for _, f := range r.Frames {
		if f.Caption != "" {
			band = h / 11
		}
	}
	var frames []*image.RGBA
	var delays []int
	for i, f := range r.Frames {
		o, err := options(r.Pose, f.Pose, w, h-band)
		if err != nil {
			return nil, fmt.Errorf("%s frame %d: %v", r.Name, i, err)
		}
		if o.Screen, err = loadImage(filepath.Join(base, f.Screen)); err != nil {
			return nil, err
		}
		o.Background = bg
		img := image.NewRGBA(image.Rect(0, 0, w, h))
		draw.Draw(img, img.Bounds(), &image.Uniform{bg}, image.Point{}, draw.Src)
		draw.Draw(img, image.Rect(0, band, w, h), robot3d.Render(o), image.Point{}, draw.Over)
		if band > 0 {
			caption(img, image.Rect(0, 0, w, band), f.Caption)
		}
		frames = append(frames, img)
		ms := f.Ms
		if ms <= 0 {
			ms = 1500
		}
		delays = append(delays, ms/10)
	}
	pal := medianCut(frames, 256)
	anim := &gif.GIF{LoopCount: 0}
	for i, img := range frames {
		p := image.NewPaletted(img.Bounds(), pal)
		draw.FloydSteinberg.Draw(p, img.Bounds(), img, image.Point{})
		anim.Image = append(anim.Image, p)
		anim.Delay = append(anim.Delay, delays[i])
	}
	var buf bytes.Buffer
	if err := gif.EncodeAll(&buf, anim); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// options merges a frame's pose over the render's.
func options(base, over Pose, w, h int) (robot3d.Options, error) {
	pick := func(a, b *float64, def float64) float64 {
		if b != nil {
			return *b
		}
		if a != nil {
			return *a
		}
		return def
	}
	o := robot3d.Options{Width: w, Height: h, NoShadow: true, // no shadows, as on s.i41.eu/s3d/
		Yaw: pick(base.Yaw, over.Yaw, 0), Pitch: pick(base.Pitch, over.Pitch, 0),
		Azimuth: pick(base.Az, over.Az, 0), Elevation: pick(base.El, over.El, 8), Zoom: pick(base.Zoom, over.Zoom, 1)}
	leds := ""
	if base.LEDs != nil {
		leds = *base.LEDs
	}
	if over.LEDs != nil {
		leds = *over.LEDs
	}
	var err error
	o.LEDs, err = robot3d.ParseLEDs(leds)
	return o, err
}

func loadImage(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %v", path, err)
	}
	return img, nil
}

func readJSON(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, v); err != nil {
		return fmt.Errorf("%s: %v", path, err)
	}
	return nil
}

// medianCut picks a palette of n colours for the frames: boxes of colours split at the median
// of their widest channel, the biggest first.
func medianCut(frames []*image.RGBA, n int) color.Palette {
	var px [][3]uint8
	for _, img := range frames {
		step := max(1, len(img.Pix)/4/150000)
		for i := 0; i < len(img.Pix); i += 4 * step {
			px = append(px, [3]uint8{img.Pix[i], img.Pix[i+1], img.Pix[i+2]})
		}
	}
	boxes := [][][3]uint8{px}
	widest := func(b [][3]uint8) (int, int) {
		ch, rng := 0, -1
		for c := 0; c < 3; c++ {
			lo, hi := 255, 0
			for _, p := range b {
				lo, hi = min(lo, int(p[c])), max(hi, int(p[c]))
			}
			if hi-lo > rng {
				ch, rng = c, hi-lo
			}
		}
		return ch, rng
	}
	for len(boxes) < n {
		best, score, ch := -1, 0, 0
		for i, b := range boxes {
			c, rng := widest(b)
			if s := rng * len(b); len(b) > 1 && rng > 0 && s > score {
				best, score, ch = i, s, c
			}
		}
		if best < 0 {
			break
		}
		b := boxes[best]
		slices.SortFunc(b, func(x, y [3]uint8) int { return int(x[ch]) - int(y[ch]) })
		mid := len(b) / 2
		boxes[best] = b[:mid:mid]
		boxes = append(boxes, b[mid:])
	}
	pal := make(color.Palette, 0, len(boxes))
	for _, b := range boxes {
		var r, g, bl int
		for _, p := range b {
			r, g, bl = r+int(p[0]), g+int(p[1]), bl+int(p[2])
		}
		k := max(1, len(b))
		pal = append(pal, color.RGBA{uint8(r / k), uint8(g / k), uint8(bl / k), 255})
	}
	return pal
}
