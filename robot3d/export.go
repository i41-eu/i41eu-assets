package robot3d

import "image/color"

// Part is a piece of the model at rest (yaw 0, pitch 0) in the robot's space (millimetres, Y up,
// facing +Z, the yaw axis through x = 0, z = 0), for other renderers (e.g. glTF for a browser).
type Part struct {
	Name  string     // plate, servo, servo-cover, pitch-servo, battery, body, led-bar-left, led-bar-right, led-board-left, led-board-right, led-chips-left, led-chips-right, top-board, base-cover, core
	Joint string     // what moves it: "base" (nothing), "yaw" (turns about the yaw axis), "head" (yaw, then pitch about PitchPivot)
	Color color.RGBA // its base colour (details such as the screen, ports and labels are drawn, not modelled)
	// Triangles, three positions each, and a normal per position.
	Positions []V3
	Normals   []V3
}

// PitchPivot is a point on the head's pitch axis (the axis is along X): +pitch lifts the front.
// The yaw axis is the Y axis; +yaw turns the head to the robot's left (+X).
func PitchPivot() V3 { return pivot }

// Screen is the screen's rectangle at rest: its centre on the CoreS3's front, its width and
// height (320 x 240 pixels, the picture's top left at Centre - (Width/2, -Height/2)).
func Screen() (centre V3, width, height float64) {
	return V3{coreCentre.X, coreCentre.Y + screenCY, coreFront}, screenW, screenH
}

// Parts gives the model's parts at rest.
func Parts() []Part {
	type key struct{ part, mat int }
	names := map[key]string{
		{partPlate, matPlate}: "plate", {partServo, matServo}: "servo", {partServo, matServoCover}: "servo-cover",
		{partBody, matBody}: "body", {partCore, matCore}: "core",
		{partServo, matPitchServo}: "pitch-servo", {partBody, matTopBoard}: "top-board", {partPlate, matBaseCover}: "base-cover",
		{partServo, matBattery}: "battery",
	}
	joints := map[int]string{partPlate: "base", partServo: "yaw", partBody: "head", partCore: "head"}
	colours := map[int]rgb{matPlate: colPlate, matServo: colDisc, matBody: colShell,
		matBar: colBarOff, matCore: colShell, matPitchServo: colServo, matTopBoard: colShell, matBaseCover: colPlate, matLedBoard: hex(0xe9e9e2), matLedChip: hex(0xf2f2ee),
		matServoCover: colServoCover, matBattery: colBattery}
	byName := map[string]*Part{}
	var order []string
	for _, t := range mesh() {
		name := names[key{t.part, t.mat}]
		if t.mat == matLedBoard || t.mat == matLedChip { // per side, as the bars
			side := map[bool]string{true: "-left", false: "-right"}[t.v[0].p.X < 0]
			name = map[int]string{matLedBoard: "led-board", matLedChip: "led-chips"}[t.mat] + side
		}
		if t.mat == matBar {
			name = "led-bar-right"
			if t.v[0].p.X < 0 { // the firmware's left strip (LEDs 0-5): left as you face the screen
				name = "led-bar-left"
			}
		}
		p := byName[name]
		if p == nil {
			c := colours[t.mat]
			p = &Part{Name: name, Joint: joints[t.part], Color: color.RGBA{q8(c.R), q8(c.G), q8(c.B), 255}}
			byName[name] = p
			order = append(order, name)
		}
		for _, v := range t.v {
			p.Positions = append(p.Positions, v.p)
			p.Normals = append(p.Normals, v.n)
		}
	}
	out := make([]Part, 0, len(order))
	for _, n := range order {
		out = append(out, *byName[n])
	}
	return out
}

// Surface is how a part looks at a point at rest (p and n in the robot's rest space, as Parts
// gives them), before the lights: its base colour with the details the renderer draws (the glass
// front and the red ring, the sensors' dots, the vents, the ports and buttons, the labels and the
// back panel's sticker and ports; at the body's back its screws, its red LED and the stand's
// USB-C; the screen and the LEDs off), and whether it is glass (shiny, reflecting the room). For
// textures in other renderers.
func Surface(part string, p, n V3) (c color.RGBA, glass bool) {
	var sc scene
	var sf surface
	switch part {
	case "core":
		sf = sc.core(p, n)
	case "body":
		sf = sc.body(p, n, matBody)
	case "top-board":
		sf = sc.body(p, n, matTopBoard)
	case "led-board-left", "led-board-right":
		sf = sc.body(p, n, matLedBoard)
	case "led-chips-left", "led-chips-right":
		sf = sc.body(p, n, matLedChip)
	case "led-bar-left", "led-bar-right":
		sf = sc.body(p, n, matBar)
	case "plate":
		sf = lower(p, n, matPlate)
	case "base-cover":
		sf = material(matBaseCover)
	case "servo":
		sf = lower(p, n, matServo)
	case "servo-cover":
		sf = lower(p, n, matServoCover)
	case "pitch-servo":
		sf = lower(p, n, matPitchServo)
	case "battery":
		sf = material(matBattery)
	default:
		sf = material(-1)
	}
	v := sf.albedo.add(sf.emissive) // the red ring glows a little
	return color.RGBA{q8(v.R), q8(v.G), q8(v.B), 255}, sf.glass
}
