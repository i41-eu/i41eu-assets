package robot3d

import "testing"

// Surface gives the renderer's details: the glass at the screen, the power button and USB-C on
// the CoreS3's right, StackChan's board seen through the open back; the plain shell elsewhere.
func TestSurface(t *testing.T) {
	shell, _ := Surface("core", V3{10, coreCentre.Y, coreCentre.Z}, V3{0, 1, 0}) // the top, off its label
	centre, _, _ := Screen()
	if _, glass := Surface("core", centre, V3{0, 0, 1}); !glass {
		t.Error("the screen's centre is not glass")
	}
	right := V3{-1, 0, 0}
	for name, p := range map[string]V3{
		"power button": coreCentre.Add(V3{-27, 14, 0.4}),
		"USB-C":        coreCentre.Add(V3{-27, -0.7, 0.4}),
	} {
		if c, _ := Surface("core", p, right); c == shell {
			t.Errorf("%s: the plain shell", name)
		}
	}
	back := coreCentre.Add(V3{0, 0, -coreD/2 - coreOverLip})
	if c, _ := Surface("core", back.Add(V3{20, -10, 0}), V3{0, 0, -1}); c.G <= c.R || c.G > 90 {
		t.Errorf("the board through the open back, not dark green: %v", c)
	}
	if c, _ := Surface("core", back.Add(V3{-7, 10, 0}), V3{0, 0, -1}); c.B <= c.R+20 {
		t.Errorf("the ribbon cable: %v", c)
	}
	if c, glass := Surface("core", coreCentre.Add(V3{-27, -20, 0.4}), right); glass || c != shell {
		t.Errorf("the right side below the ports: %v %v, want the shell %v", c, glass, shell)
	}
}

// Every part's triangles are wound counter-clockwise seen from the side their normals point to
// (as glTF and other renderers that cull or light by winding expect).
func TestPartsWound(t *testing.T) {
	for _, p := range Parts() {
		inward := 0
		for i := 0; i < len(p.Positions); i += 3 {
			face := p.Positions[i+1].Sub(p.Positions[i]).Cross(p.Positions[i+2].Sub(p.Positions[i]))
			if face.Dot(p.Normals[i].Add(p.Normals[i+1]).Add(p.Normals[i+2])) < 0 {
				inward++
			}
		}
		if inward > 0 {
			t.Errorf("%s: %d of %d triangles wound against their normals", p.Name, inward, len(p.Positions)/3)
		}
	}
}
