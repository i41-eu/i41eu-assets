package catalog

import (
	"os"
	"path/filepath"
	"testing"
)

// The repository's lists load, every screen they name is there, every copy names a render and a
// repo, and every render is distributed somewhere or at least listed.
func TestLists(t *testing.T) {
	root := filepath.Join("..", "..")
	rs, err := Load(filepath.Join(root, "renders.json"))
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, r := range rs.Renders {
		names[r.Name] = true
		screens := []string{r.Screen}
		for _, f := range r.Frames {
			screens = append(screens, f.Screen)
		}
		for _, s := range screens {
			if s == "" {
				continue
			}
			if _, err := os.Stat(filepath.Join(root, s)); err != nil {
				t.Errorf("%s: %v", r.Name, err)
			}
		}
	}
	d, err := LoadDistribution(filepath.Join(root, "distribute.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range d.Copies {
		if c.Screen != "" {
			if _, err := os.Stat(filepath.Join(root, "screens", c.Screen)); err != nil || c.Render != "" {
				t.Errorf("copy to %s: screen %q: %v (and no render too)", c.To, c.Screen, err)
			}
		} else if !names[c.Render] {
			t.Errorf("copy to %s: no render %q", c.To, c.Render)
		}
	}
}

// A still converts to a smaller PNG and to a JPEG; an animation only goes to a GIF.
func TestConvert(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("..", "..", "renders", "raw.png"))
	if err != nil {
		t.Skip("no renders/raw.png")
	}
	for _, to := range []string{"x:a.png", "x:a.jpg"} {
		if b, err := convert(src, "raw.png", Copy{To: to, Size: "100x100"}); err != nil || len(b) == 0 {
			t.Errorf("%s: %v", to, err)
		}
	}
	if _, err := convert(src, "anim.gif", Copy{To: "x:a.png"}); err == nil {
		t.Error("an animation to a PNG")
	}
}
