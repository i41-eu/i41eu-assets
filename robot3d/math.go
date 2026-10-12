package robot3d

import "math"

// V3 is a point or a direction (millimetres; Y up, the robot faces +Z).
type V3 struct{ X, Y, Z float64 }

func (a V3) Add(b V3) V3             { return V3{a.X + b.X, a.Y + b.Y, a.Z + b.Z} }
func (a V3) Sub(b V3) V3             { return V3{a.X - b.X, a.Y - b.Y, a.Z - b.Z} }
func (a V3) Mul(s float64) V3        { return V3{a.X * s, a.Y * s, a.Z * s} }
func (a V3) Dot(b V3) float64        { return a.X*b.X + a.Y*b.Y + a.Z*b.Z }
func (a V3) Len() float64            { return math.Sqrt(a.Dot(a)) }
func (a V3) Cross(b V3) V3           { return V3{a.Y*b.Z - a.Z*b.Y, a.Z*b.X - a.X*b.Z, a.X*b.Y - a.Y*b.X} }
func (a V3) Lerp(b V3, t float64) V3 { return a.Add(b.Sub(a).Mul(t)) }

func (a V3) Norm() V3 {
	if l := a.Len(); l > 0 {
		return a.Mul(1 / l)
	}
	return a
}

// M4 is a 4x4 matrix, row major, for column vectors.
type M4 [16]float64

func identity() M4 { return M4{1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1} }

func (a M4) Mul(b M4) M4 {
	var m M4
	for r := 0; r < 4; r++ {
		for c := 0; c < 4; c++ {
			for k := 0; k < 4; k++ {
				m[r*4+c] += a[r*4+k] * b[k*4+c]
			}
		}
	}
	return m
}

// Point transforms a point (w = 1).
func (a M4) Point(p V3) V3 {
	return V3{a[0]*p.X + a[1]*p.Y + a[2]*p.Z + a[3], a[4]*p.X + a[5]*p.Y + a[6]*p.Z + a[7], a[8]*p.X + a[9]*p.Y + a[10]*p.Z + a[11]}
}

// Dir transforms a direction (w = 0; the rotations here keep lengths, so normals too).
func (a M4) Dir(p V3) V3 {
	return V3{a[0]*p.X + a[1]*p.Y + a[2]*p.Z, a[4]*p.X + a[5]*p.Y + a[6]*p.Z, a[8]*p.X + a[9]*p.Y + a[10]*p.Z}
}

func translate(v V3) M4 { return M4{1, 0, 0, v.X, 0, 1, 0, v.Y, 0, 0, 1, v.Z, 0, 0, 0, 1} }

// rotX turns by a radians about X: +a takes +Y towards +Z.
func rotX(a float64) M4 {
	s, c := math.Sincos(a)
	return M4{1, 0, 0, 0, 0, c, -s, 0, 0, s, c, 0, 0, 0, 0, 1}
}

// rotY turns by a radians about Y: +a takes +Z towards +X.
func rotY(a float64) M4 {
	s, c := math.Sincos(a)
	return M4{c, 0, s, 0, 0, 1, 0, 0, -s, 0, c, 0, 0, 0, 0, 1}
}

func rad(deg float64) float64 { return deg * math.Pi / 180 }

func clamp(x, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, x)) }

func smoothstep(e0, e1, x float64) float64 {
	t := clamp((x-e0)/(e1-e0), 0, 1)
	return t * t * (3 - 2*t)
}
