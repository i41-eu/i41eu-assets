# i41eu-assets

Assets for the Stackchan projects on [s.i41.eu](https://s.i41.eu).

## robot3d: the robot in 3D, with any screen

A simplified 3D model of an M5Stack Stackchan robot, built in code from photos of a real one, and
a small renderer, all in Go (standard library only, no GPU, no cgo; the same pixels on every
machine). Put any picture on the robot's screen (a screenshot taken over USB, a frame of a
recording, a mock-up), turn its head, move the camera round it, light its LEDs.

```bash
go install github.com/i41-eu/i41eu-assets/cmd/robot3d@latest

i41eu-usb screenshot -o screen.jpg                  # the robot's screen over USB (320x240)
robot3d -screen screen.jpg -o robot.png                # the robot showing it (800x800, transparent)
robot3d -screen screen.jpg -az 30 -el 12 -pitch 25 -yaw 10 -o look-up.png
robot3d -screen screen.jpg -leds "#ff6a00*12" -bg "#f4f6ff" -o orange.png
robot3d -screen screen.jpg -frames 72 -o spin/%03d.png # the camera once round the robot
ffmpeg -framerate 24 -i spin/%03d.png -pix_fmt yuv420p spin.mp4
```

| Flag | |
|---|---|
| `-screen` | the screen's picture, JPEG or PNG, drawn 4:3; empty: the screen is off |
| `-brightness` | the screen, 0..1 |
| `-leds` | 12 colours, `#rrggbb` comma separated, numbered as the robot's `leds` command (left 0-5, right 6-11, left and right as you face the screen; 0 and 11 at the front); `#rrggbb*12` for all |
| `-yaw`, `-pitch` | the head in degrees, as the robot's `look` command: yaw + to the robot's left, pitch up from level (the robot allows 5..85) |
| `-az`, `-el` | the camera round the robot in degrees: 0, 0 is straight in front; + azimuth to the robot's left |
| `-zoom`, `-fov` | 1 fits the robot in any pose, so a moving head keeps a steady frame |
| `-size`, `-bg`, `-no-shadow`, `-ss` | the picture's size, its background (transparent if empty), the soft shadow on the ground, supersampling |
| `-frames`, `-turn` | frames of the camera going round by `-turn` degrees; `-o` is then a pattern |

From Go:

```go
img := robot3d.Render(robot3d.Options{Width: 800, Height: 800, Screen: screen, Pitch: 20, Azimuth: 25, Elevation: 10})
```

The model: M5Stack's structure files for the StackChan (`robot3d/m5stack/`, MIT, from
[m5stack/M5_Hardware](https://github.com/m5stack/M5_Hardware/tree/master/Products/K151_StackChan/Structures)):
the main body (the head behind the CoreS3, with its holes and LED slots), the base, the servo body
with the turntable, its back cover and its top cover (inside the head), placed by the dimensions in M5Stack's drawing
([docs](https://docs.m5stack.com/en/StackChan): 54 x 70.5 x 61.5 mm, the plate 8 high; the servo
body's foot on the plate's ring round the turntable). Drawn here: the CoreS3 (54 x 54 x 15.5) with
the screen (40.8 x 30.6) in its black glass front, the red ring and the sensors' dots, vents on the
left, the power button (in its black ring, the power sign above it), USB-C and Grove port on the
right; the light guides of the LEDs in the slots through the head's rounded top edges (pale teal
when off), the labels (without text), the upper
back's panel (down to 29 mm below the top) and the black pitch servo in the servo body's pocket on
the left (M5Stack's files leave the servos out; sized by the pocket and a photo of the back); at
the body's back the mid grey cover, the blue battery through its cutout, the two screws and the red
LED in the band below it, and the stand's USB-C (M5Stack's drawing and product photos of the back).
The head pitches about the servo's horn at the main body's boss, 27 mm above its bottom, right over
the yaw axis.

## The pictures in the other repositories

All robot pictures in the docs, READMEs and pages come from here, so one change (the model, a
screen, a pose) refreshes them everywhere:

1. `screens/`: the robot's screens (320x240), e.g. from `i41eu-usb screenshot -o x.jpg`. They
   show no private data, also not in a QR code: `screens/qr.png`'s code is a placeholder pairing
   address.
2. `renders.json`: each picture: a still (one screen) or an animation (frames with captions), the
   head's pose, the camera, the LEDs, the size.
3. `go run ./cmd/robot3d-renders` writes `renders/` (stills PNG, animations GIF); `-only a,b`
   for some, `-check` fails if `renders/` is out of date. The same inputs give the same bytes.
4. `distribute.json`: where each picture goes (`repo:path`; the extension picks PNG, JPEG or GIF,
   `size` scales a still down); `repos` maps names to checkouts.
5. `go run ./cmd/robot3d-distribute -check` lists what is out of date; without `-check` it writes
   those files. It only writes: commit and push in each repository by its own rules.

Stack-chan was created by Shinya Ishikawa and the Stack-chan community ([github.com/stack-chan/stack-chan](https://github.com/stack-chan/stack-chan)); this project is independent of it and of M5Stack.

## License

Apache License 2.0, see [LICENSE](LICENSE), except M5Stack's StackChan structure files in
[`robot3d/m5stack/`](robot3d/m5stack/): MIT, Copyright (c) 2021 M5Stack, see
[their LICENSE](robot3d/m5stack/LICENSE). A program built from this repository carries those
files (embedded in the robot3d package), so it carries M5Stack's notice too.
