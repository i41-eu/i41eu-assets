package robot3d

import (
	"embed"
	"encoding/binary"
	"fmt"
	"math"
)

//go:embed m5stack/*.stl
var stlFS embed.FS

// stlPart loads one of M5Stack's binary STL files and places it: STL x stays x, STL z becomes
// up (y), STL y becomes back (-z); the part's (cx, cy, cz) in STL space lands on at in the
// robot's space. Normals are smoothed across edges flatter than 40 degrees.
func stlPart(name string, cx, cy, cz float64, at V3, part, mat int) []tri {
	b, err := stlFS.ReadFile("m5stack/" + name)
	if err != nil {
		panic(err)
	}
	if len(b) < 84 {
		panic(fmt.Sprintf("%s: not an STL", name))
	}
	n := int(binary.LittleEndian.Uint32(b[80:84]))
	if len(b) < 84+50*n {
		panic(fmt.Sprintf("%s: %d triangles, %d bytes", name, n, len(b)))
	}
	f := func(o int) float64 { return float64(math.Float32frombits(binary.LittleEndian.Uint32(b[o:]))) }
	place := func(x, y, z float64) V3 { return V3{x - cx + at.X, z - cz + at.Y, -(y - cy) + at.Z} }

	pos := make([][3]V3, 0, n)
	face := make([]V3, 0, n) // area-weighted normals
	for i := 0; i < n; i++ {
		o := 84 + 50*i + 12
		var t [3]V3
		for k := 0; k < 3; k++ {
			t[k] = place(f(o+12*k), f(o+12*k+4), f(o+12*k+8))
		}
		fn := t[1].Sub(t[0]).Cross(t[2].Sub(t[0]))
		if fn.Len() < 1e-12 {
			continue
		}
		pos, face = append(pos, t), append(face, fn)
	}
	type key [3]int64
	q := func(p V3) key {
		return key{int64(math.Round(p.X * 1000)), int64(math.Round(p.Y * 1000)), int64(math.Round(p.Z * 1000))}
	}
	shared := map[key][]int{}
	for i, t := range pos {
		for _, p := range t {
			shared[q(p)] = append(shared[q(p)], i)
		}
	}
	cosLimit := math.Cos(rad(40))
	out := make([]tri, len(pos))
	for i, t := range pos {
		me := face[i].Norm()
		for k, p := range t {
			var sum V3
			for _, j := range shared[q(p)] {
				if face[j].Norm().Dot(me) >= cosLimit {
					sum = sum.Add(face[j])
				}
			}
			out[i].v[k] = vert{p, sum.Norm()}
		}
		out[i].part, out[i].mat = part, mat
	}
	return out
}
