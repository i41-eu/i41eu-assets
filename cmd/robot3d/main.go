// Command robot3d draws the robot (package robot3d) with a picture on its screen.
//
//	i41eu-usb screenshot -o screen.jpg                 # the robot's screen over USB
//	robot3d -screen screen.jpg -o robot.png               # the robot showing it
//	robot3d -screen screen.jpg -az 30 -el 12 -pitch 25 -o look-up.png
//	robot3d -screen screen.jpg -leds "#ff0000*12" -o red.png
//	robot3d -screen screen.jpg -turn 360 -frames 72 -o spin/%03d.png   # frames for a video
//	ffmpeg -framerate 24 -i spin/%03d.png -pix_fmt yuv420p spin.mp4
package main

import (
	"flag"
	"fmt"
	"image"
	_ "image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"github.com/i41-eu/i41eu-assets/robot3d"
)

func main() {
	var (
		screen  = flag.String("screen", "", "the screen's picture (JPEG or PNG, e.g. from i41eu-usb screenshot); empty: off")
		bright  = flag.Float64("brightness", 1, "the screen's brightness, 0..1")
		leds    = flag.String("leds", "", `the 12 LEDs, comma separated "#rrggbb" (left 0-5, right 6-11, left and right as you face the screen; 0 and 11 at the front); "#rrggbb*12" for all`)
		yaw     = flag.Float64("yaw", 0, "the head's yaw in degrees, + to the robot's left")
		pitch   = flag.Float64("pitch", 0, "the head's pitch in degrees up from level (the robot: 5..85)")
		az      = flag.Float64("az", 0, "the camera's azimuth in degrees, + to the robot's left")
		el      = flag.Float64("el", 8, "the camera's elevation in degrees")
		zoom    = flag.Float64("zoom", 1, "1: the robot fits in any pose; more: closer")
		fov     = flag.Float64("fov", 25, "vertical field of view in degrees")
		size    = flag.String("size", "800x800", "the picture's size, WxH")
		bg      = flag.String("bg", "", `background "#rrggbb"; empty: transparent`)
		noShade = flag.Bool("no-shadow", false, "no shadow on the ground")
		ss      = flag.Int("ss", 3, "supersampling per axis (antialiasing)")
		out     = flag.String("o", "robot.png", "the PNG to write; with -frames a pattern like spin/%03d.png")
		frames  = flag.Int("frames", 0, "frames of a turn (the camera's azimuth goes round by -turn)")
		turn    = flag.Float64("turn", 360, "with -frames: degrees the camera goes round")
	)
	flag.Parse()

	o := robot3d.Options{Brightness: *bright, Yaw: *yaw, Pitch: *pitch, Azimuth: *az, Elevation: *el,
		Zoom: *zoom, FOV: *fov, NoShadow: *noShade, Supersample: *ss}
	if _, err := fmt.Sscanf(*size, "%dx%d", &o.Width, &o.Height); err != nil {
		fail("-size %q: want WxH", *size)
	}
	if *screen != "" {
		f, err := os.Open(*screen)
		if err != nil {
			fail("%v", err)
		}
		img, _, err := image.Decode(f)
		f.Close()
		if err != nil {
			fail("%s: %v", *screen, err)
		}
		o.Screen = img
	}
	var err error
	if o.LEDs, err = robot3d.ParseLEDs(*leds); err != nil {
		fail("-leds: %v", err)
	}
	if *bg != "" {
		if o.Background, err = robot3d.ParseColor(*bg); err != nil {
			fail("-bg: %v", err)
		}
	}

	if *frames <= 0 {
		write(*out, robot3d.Render(o))
		return
	}
	if !strings.Contains(*out, "%") {
		fail("-frames: -o needs a pattern like spin/%%03d.png")
	}
	start := o.Azimuth
	for i := 0; i < *frames; i++ {
		o.Azimuth = start + *turn*float64(i)/float64(*frames)
		write(fmt.Sprintf(*out, i), robot3d.Render(o))
	}
}

func write(path string, img image.Image) {
	if dir := filepath.Dir(path); dir != "." {
		os.MkdirAll(dir, 0o755)
	}
	f, err := os.Create(path)
	if err != nil {
		fail("%v", err)
	}
	if err := png.Encode(f, img); err != nil {
		fail("%v", err)
	}
	if err := f.Close(); err != nil {
		fail("%v", err)
	}
}

func fail(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "robot3d: "+format+"\n", a...)
	os.Exit(1)
}
