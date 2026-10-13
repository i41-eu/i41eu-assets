package robot3d

import (
	"math"
	"sync"
)

// The robot in millimetres: M5Stack's structure files (m5stack/, MIT) placed by M5Stack's drawing
// (54 wide, 70.5 high, 61.5 deep; the plate 8 high), and the CoreS3 in front of the main body
// drawn here (54 x 54 x 15.5, the screen 320x240 = 40.8 x 30.6 in its black glass front).
//
// The robot's space: the yaw axis at x = 0, z = 0 on the ground (y = 0); the robot faces +Z.
const (
	robotH      = 70.5
	headBottom  = robotH - 54 // the head (CoreS3 + main body) is 54 high
	coreFront   = 33.6        // the yaw axis is 33.6 behind the front
	coreD       = 15.5        // the CoreS3's depth
	bodyFront   = coreFront - coreD
	coreR       = 3.5 // the CoreS3's rounded edges: the front and the corners
	coreBackR   = 0.3 // its back's edges, against the main body: a thin seam
	coreOverLip = 0.7 // the CoreS3 slides over the main body's front lip (z 17.4 to its front): its back covers it
	screenW     = 40.8
	screenH     = 30.6
	screenCY    = 1.0 // the screen's centre above the CoreS3's centre
	rim         = 1.4 // the shell's rim around the glass front
	// The servo body's foot rests on the plate's ring round the turntable (its top at 11.1; the
	// plate itself is 8 high): M5Stack's drawing and product photos of the back put its screws,
	// its LED window and its back cover there, and its horn at the main body's pitch boss (y 43.5).
	servoBottom = 11.2
	pitchAxisY  = servoBottom + 32.3
)

// coreCentre is the CoreS3's centre at rest.
var coreCentre = V3{0, headBottom + 27, coreFront - coreD/2}

// pivot is the pitch axis (along X) at rest: the servo's horn, over the yaw axis.
var pivot = V3{0, pitchAxisY, 0}

// Parts move differently: the plate stays, the servo body turns with yaw, the head (main body
// and CoreS3) with yaw and pitch.
const (
	partPlate = iota
	partServo
	partBody
	partCore
)

// Materials.
const (
	matPlate = iota
	matServo
	matServoCover
	matBody
	matBar // the light guide bars of the LEDs
	matCore
	matPitchServo // the pitch servo (Feetech SCS0009, black), in the servo body's pocket
	matLedBoard   // the LEDs' board inside each side, under the light guide (the owner's photo of the inside, 2026-10-12)
	matLedChip    // its six LEDs (WS2812C, 2 x 2 mm), one under each sixth of the light guide
	matTopBoard   // the head's top behind the CoreS3: the touch pads, the IR LED and receiver, ports B and C
	matBaseCover  // the base's bottom cover (M5Stack's BaseCover.stl): the base's underside is flat
	matBattery    // the 550 mAh battery behind the servo body's back cover (blue)
)

// The pitch servo, drawn (M5Stack's files leave the servos out): the pocket the servo body
// leaves on the robot's left, round the pitch axis, between its side and the main body's boss.
const (
	pitchServoX0, pitchServoX1 = 11.0, 22.0
	pitchServoY0, pitchServoY1 = servoBottom + 27.5, servoBottom + 39.0
	pitchServoZ0, pitchServoZ1 = -17.0, 10.0
)

// The back of the servo body (M5Stack's drawing and product photos of the back). The battery,
// drawn (M5Stack's files leave it out), fills the compartment behind the back cover; the cover's
// cutout above its screws shows its blue wrap (M5Stack's annotated photo: 550 mAh BATTERY).
const (
	batteryX             = 14.6
	batteryY0, batteryY1 = servoBottom + 6.5, servoBottom + 22.7
	batteryZ0, batteryZ1 = -19.2, -10.3
	screwX, screwY       = 10.6, servoBottom + 3.15 // the back cover's two screws (x ±), in the white band
	backLedX, backLedY   = 0.25, servoBottom + 3.2  // the red LED between them, in its window
	standUsbX, standUsbY = 0.6, 7.55                // the stand's USB-C, in the opening M5Stack's base leaves on its back
	standUsbW, standUsbH = 8.9, 3.2
)

// The LED bars (one each side, at the top): a light guide in the slot M5Stack's main body cuts
// through its rounded top edge (radius edgeR about x ±edgeCX, y edgeCY), across the edge's
// diagonal at 45°, guideHalf either side of it, its ends round: flush with the edge, so the light
// shows about 1.7 mm high on the side and 1.7 mm wide on the top (M5Stack's drawing and product
// photos), the shell's round going on above and below it.
const (
	barZ0, barZ1          = -16.1, 13.1
	edgeR, edgeCX, edgeCY = 5.0, 22.03, 65.5
	guideHalf, guideDepth = 1.16, 2.6
	barY0, barY1          = 68.1, robotH // where the guide shows on the side (its lower lip), up to the top
	// The LEDs' board stands inside the side, between its three holes and the top, against the
	// light guide's inner face; its six chips on its inner side, one under each sixth of the guide
	// (the owner's photo of the inside through the open back, 2026-10-12).
	ledBoardX0, ledBoardX1 = 21.9, 22.7
	ledBoardY0, ledBoardY1 = 62.5, 68.6
	ledChipY, ledChipHalf  = 64.8, 1.0
	ledChipD               = 0.9
	// The top board fills the opening M5Stack's main body leaves in its top behind the CoreS3
	// (M5Stack's annotated picture of the robot: touch pads x 3, IR Tx / Rx, GPIO port B, UART port C).
	topBoardX              = 21.0
	topBoardZ0, topBoardZ1 = 12.6, bodyFront
	topBoardY              = robotH - 0.05 // flush with the top (the owner's photos)
)

// The side stickers: a light label with a dark end (drawn without its text, in body's shading).
const (
	stickerZ0, stickerZ1 = 10.9, 16.2
	stickerY0, stickerY1 = headBottom + 9.5, robotH - 5.5
)

type vert struct{ p, n V3 } // at rest, in the robot's space

type tri struct {
	v    [3]vert
	part int
	mat  int
}

var (
	meshOnce sync.Once
	meshTris []tri
)

func mesh() []tri {
	meshOnce.Do(func() {
		var m []tri
		// STL anchors: the turntable's centre (base, servo body), the main body's front.
		m = append(m, stlPart("StackChan-Base.stl", 106.2, -320.2, -32.6, V3{0, 0, 0}, partPlate, matPlate)...)
		// The base's bottom cover, in its own frame: centred in the base's open underside (its notch
		// at the base's front notch), flush with the bottom.
		m = append(m, stlPart("StackChan-BaseCover.stl", -0.53, 18.245, -42.79, V3{0, 0, 0}, partPlate, matBaseCover)...)
		m = append(m, stlPart("StackChan-ServoBody.stl", 229.45, -320.9, -22.3, V3{0, servoBottom, 0}, partServo, matServo)...)
		m = append(m, stlPart("StackChan-ServoSideCover.stl", 229.45, -320.9, -22.3, V3{0, servoBottom, 0}, partServo, matServoCover)...)
		m = append(m, stlPart("StackChan-ServoCover.stl", 229.45, -320.9, -22.3, V3{0, servoBottom, 0}, partServo, matServo)...) // the servo body's top, inside the head
		m = append(m, stlPart("StackChan-MainBody.stl", 475.7, -352.0, -17.7, V3{0, headBottom, bodyFront}, partBody, matBody)...)
		add := func(tris []tri, at V3) {
			for _, t := range tris {
				for k := range t.v {
					t.v[k].p = t.v[k].p.Add(at)
				}
				m = append(m, t)
			}
		}
		add(roundedBoxBack(V3{27, 27, (coreD + coreOverLip) / 2}, coreR, coreBackR, 6, partCore, matCore), coreCentre.Add(V3{0, 0, -coreOverLip / 2}))
		add(roundedBox(V3{(pitchServoX1 - pitchServoX0) / 2, (pitchServoY1 - pitchServoY0) / 2, (pitchServoZ1 - pitchServoZ0) / 2}, 0.8, 2, partServo, matPitchServo),
			V3{(pitchServoX0 + pitchServoX1) / 2, (pitchServoY0 + pitchServoY1) / 2, (pitchServoZ0 + pitchServoZ1) / 2})
		add(roundedBox(V3{batteryX, (batteryY1 - batteryY0) / 2, (batteryZ1 - batteryZ0) / 2}, 0.8, 2, partServo, matBattery),
			V3{0, (batteryY0 + batteryY1) / 2, (batteryZ0 + batteryZ1) / 2})
		// The red LED in the servo body's back window and the USB-C receptacle in the stand's back
		// opening (M5Stack's files leave both open through): shaded as their part's details.
		add(roundedBox(V3{0.75, 1.5, 0.75}, 0.2, 1, partServo, matServo), V3{backLedX, backLedY, -22.4})
		add(roundedBox(V3{standUsbW / 2, standUsbH / 2, 3.5}, 1.4, 2, partPlate, matPlate), V3{standUsbX, standUsbY, -23.3 + 3.5})
		for _, sx := range []float64{-1, 1} {
			m = append(m, lightGuide(sx, partBody, matBar)...)
			add(roundedBox(V3{(ledBoardX1 - ledBoardX0) / 2, (ledBoardY1 - ledBoardY0) / 2, (barZ1 - barZ0) / 2}, 0.1, 1, partBody, matLedBoard),
				V3{sx * (ledBoardX0 + ledBoardX1) / 2, (ledBoardY0 + ledBoardY1) / 2, (barZ0 + barZ1) / 2})
			for k := 0; k < 6; k++ {
				add(roundedBox(V3{ledChipD / 2, ledChipHalf, ledChipHalf}, 0.15, 1, partBody, matLedChip),
					V3{sx * (ledBoardX0 - ledChipD/2), ledChipY, ledZ(k)})
			}
		}

		// The side stickers (drawn in body's shading) lie flush on the side, over the pocket M5Stack's
		// main body leaves under them (seen as a step behind the CoreS3 on the owner's photos).
		for _, x := range []float64{-27, 27} {
			add(roundedBox(V3{0.06, (stickerY1 - stickerY0) / 2, (stickerZ1 - stickerZ0) / 2}, 0.03, 1, partBody, matBody),
				V3{x, (stickerY0 + stickerY1) / 2, (stickerZ0 + stickerZ1) / 2})
		}
		add(roundedBox(V3{topBoardX, 0.8, (topBoardZ1 - topBoardZ0) / 2}, 0.2, 1, partBody, matTopBoard),
			V3{0, topBoardY - 0.8, (topBoardZ0 + topBoardZ1) / 2})
		meshTris = m
	})
	return meshTris
}

// lightGuide is the LED bar on side sx (+1: +X) in M5Stack's slot through the main body's
// rounded top edge: its outer face on the edge's round, its sides on the slot's walls, its inside
// guideDepth in, its ends round as the slot's (half circles of guideHalf).
func lightGuide(sx float64, part, mat int) []tri {
	a := V3{sx * math.Sqrt2 / 2, math.Sqrt2 / 2, 0}  // the edge's diagonal, outward
	t := V3{-sx * math.Sqrt2 / 2, math.Sqrt2 / 2, 0} // across the slot, toward the top
	c := V3{sx * edgeCX, edgeCY, 0}
	zz := V3{0, 0, 1}
	// Stations along the bar: its half width there, and its sides' normal (along z, across).
	type station struct{ z, hw, nz, nt float64 }
	var st []station
	const n = 6 // steps round each end
	h := guideHalf
	for k := 0; k <= n; k++ { // the back end
		b := float64(k) * math.Pi / 2 / n
		st = append(st, station{barZ0 + h - h*math.Cos(b), h * math.Sin(b), -math.Cos(b), math.Sin(b)})
	}
	for k := n; k >= 0; k-- { // the front end
		b := float64(k) * math.Pi / 2 / n
		st = append(st, station{barZ1 - h + h*math.Cos(b), h * math.Sin(b), math.Cos(b), math.Sin(b)})
	}
	across := []float64{-1, -0.5, 0, 0.5, 1}
	outer := func(s station, u float64) vert { // on the edge's round
		phi := math.Asin(u / edgeR)
		d := a.Mul(math.Cos(phi)).Add(t.Mul(math.Sin(phi)))
		return vert{c.Add(d.Mul(edgeR)).Add(zz.Mul(s.z)), d}
	}
	inner := func(s station, u float64) V3 { return c.Add(a.Mul(edgeR - guideDepth)).Add(t.Mul(u)).Add(zz.Mul(s.z)) }
	in := a.Mul(-1)
	var out []tri
	for i := 0; i+1 < len(st); i++ {
		s0, s1 := st[i], st[i+1]
		for j := 0; j+1 < len(across); j++ {
			u00, u01, u10, u11 := across[j]*s0.hw, across[j+1]*s0.hw, across[j]*s1.hw, across[j+1]*s1.hw
			out = append(out, quad(outer(s0, u00), outer(s0, u01), outer(s1, u11), outer(s1, u10), part, mat)...)
			out = append(out, quad(vert{inner(s0, u00), in}, vert{inner(s1, u10), in}, vert{inner(s1, u11), in}, vert{inner(s0, u01), in}, part, mat)...)
		}
		for _, side := range []float64{-1, 1} {
			n0, n1 := t.Mul(side*s0.nt).Add(zz.Mul(s0.nz)), t.Mul(side*s1.nt).Add(zz.Mul(s1.nz))
			out = append(out, quad(vert{outer(s0, side*s0.hw).p, n0}, vert{outer(s1, side*s1.hw).p, n1},
				vert{inner(s1, side*s1.hw), n1}, vert{inner(s0, side*s0.hw), n0}, part, mat)...)
		}
	}
	// Only real triangles (the round ends close to a point), wound counter-clockwise seen from
	// the side their normals point to.
	keep := out[:0]
	for _, tr := range out {
		f := tr.v[1].p.Sub(tr.v[0].p).Cross(tr.v[2].p.Sub(tr.v[0].p))
		if f.Len() < 1e-9 {
			continue
		}
		if f.Dot(tr.v[0].n.Add(tr.v[1].n).Add(tr.v[2].n)) < 0 {
			tr.v[1], tr.v[2] = tr.v[2], tr.v[1]
		}
		keep = append(keep, tr)
	}
	return keep
}

// ledZ is the middle of the k-th sixth of a light guide, 0 at the front: where its LED's light is
// strongest.
func ledZ(k int) float64 { return barZ1 - (float64(k)+0.5)*(barZ1-barZ0)/6 }

// transforms gives each part's matrix (rest to world) for a pose.
func transforms(yaw, pitch float64) [4]M4 {
	ry := rotY(rad(yaw)) // +yaw: to the robot's left (+X), counter-clockwise seen from above
	head := ry.Mul(translate(pivot)).Mul(rotX(-rad(pitch))).Mul(translate(pivot.Mul(-1)))
	return [4]M4{identity(), ry, head, head}
}

func quad(a, b, c, d vert, part, mat int) []tri {
	return []tri{{[3]vert{a, b, c}, part, mat}, {[3]vert{a, c, d}, part, mat}}
}

// roundedBox is a box of half sizes h around the origin with edges rounded by r: a grid on each
// face, pushed out from the inner box, denser where the edges bend.
func roundedBox(h V3, r float64, steps int, part, mat int) []tri {
	return roundedBoxBack(h, r, r, steps, part, mat)
}

// roundedBoxBack is roundedBox with the back face's edges (-Z) rounded by rb: the CoreS3 is
// rounded at its front and its corners, and nearly square where it meets the main body (M5Stack's
// photos show a thin seam there, not a groove).
func roundedBoxBack(h V3, r, rb float64, steps int, part, mat int) []tri {
	in := V3{h.X - r, h.Y - r, h.Z - r}
	inBack := -(h.Z - rb)                       // the inner box's back
	side := func(inner, rr float64) []float64 { // from the edge to the inner box, one side
		var c []float64
		for k := steps; k >= 1; k-- {
			c = append(c, inner+rr*math.Tan(float64(k)*math.Pi/4/float64(steps)))
		}
		return c
	}
	coords := func(axis int) []float64 {
		lo, hi, rlo := -[3]float64{in.X, in.Y, in.Z}[axis], [3]float64{in.X, in.Y, in.Z}[axis], r
		if axis == 2 {
			lo, rlo = inBack, rb
		}
		var c []float64
		for _, v := range side(-lo, rlo) {
			c = append(c, -v)
		}
		c = append(c, lo, hi)
		back := side(hi, r)
		for k := len(back) - 1; k >= 0; k-- {
			c = append(c, back[k])
		}
		return c
	}
	get := func(v V3, i int) float64 { return [3]float64{v.X, v.Y, v.Z}[i] }
	point := func(p V3) vert {
		q := V3{clamp(p.X, -in.X, in.X), clamp(p.Y, -in.Y, in.Y), clamp(p.Z, inBack, in.Z)}
		d := p.Sub(q).Norm()
		rz := r
		if p.Z < 0 {
			rz = rb
		}
		n := V3{d.X / r, d.Y / r, d.Z / rz}.Norm() // the normal of the rounding (an ellipse at the back)
		return vert{q.Add(V3{d.X * r, d.Y * r, d.Z * rz}), n}
	}
	var out []tri
	for axis := 0; axis < 3; axis++ {
		u, v := (axis+1)%3, (axis+2)%3
		cu, cv := coords(u), coords(v)
		for _, s := range []float64{-1, 1} {
			for i := 0; i+1 < len(cu); i++ {
				for j := 0; j+1 < len(cv); j++ {
					var c [4]vert
					for k, ij := range [4][2]int{{i, j}, {i + 1, j}, {i + 1, j + 1}, {i, j + 1}} {
						var a [3]float64
						a[axis] = s * get(h, axis)
						a[u], a[v] = cu[ij[0]], cv[ij[1]]
						c[k] = point(V3{a[0], a[1], a[2]})
					}
					if s < 0 { // counter-clockwise seen from outside, as the + side
						c[1], c[3] = c[3], c[1]
					}
					out = append(out, quad(c[0], c[1], c[2], c[3], part, mat)...)
				}
			}
		}
	}
	for i, t := range out { // where the corners meet a smaller back rounding, a tiny triangle can turn
		face := t.v[1].p.Sub(t.v[0].p).Cross(t.v[2].p.Sub(t.v[0].p))
		if face.Dot(t.v[0].n.Add(t.v[1].n).Add(t.v[2].n)) < 0 {
			out[i].v[1], out[i].v[2] = t.v[2], t.v[1]
		}
	}
	return out
}
