package catalog

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	xdraw "golang.org/x/image/draw"

	"github.com/i41-eu/i41eu-assets/robot3d"
)

// Distribution is distribute.json: where each render goes.
type Distribution struct {
	Repos  map[string]string `json:"repos"` // name: its checkout ("~/" is home)
	Copies []Copy            `json:"copies"`
}

// Copy puts a render into a repository: "repo:path". The target's extension picks the format:
// .png (scaled to Size if given), .jpg (on Bg, white if empty), .gif (an animation as it is).
type Copy struct {
	Render string `json:"render,omitempty"` // a render (renders.json), or
	Screen string `json:"screen,omitempty"` // a screen as it is (screens/<name>), e.g. for s3d's robot`
	To     string `json:"to"`
	Size   string `json:"size,omitempty"`
	Bg     string `json:"bg,omitempty"`
}

// Result is what a copy did or would do.
type Result struct {
	Repo, Path, Render string
	State              string // "same", "changed", "new"
}

// LoadDistribution reads distribute.json.
func LoadDistribution(path string) (*Distribution, error) {
	var d Distribution
	if err := readJSON(path, &d); err != nil {
		return nil, err
	}
	return &d, nil
}

// Run makes each copy's bytes from the renders in dir, compares them with the target, and writes
// the ones that differ unless check.
func (d *Distribution) Run(dir string, renders *Renders, check bool) ([]Result, error) {
	byName := map[string]Render{}
	for _, r := range renders.Renders {
		byName[r.Name] = r
	}
	home, _ := os.UserHomeDir()
	var out []Result
	for _, c := range d.Copies {
		repo, rel, ok := strings.Cut(c.To, ":")
		root, known := d.Repos[repo]
		if !ok || !known || rel == "" || filepath.IsAbs(rel) || strings.Contains(rel, "..") {
			return out, fmt.Errorf("copy to %q: want repo:path with a repo from repos", c.To)
		}
		if strings.HasPrefix(root, "~/") {
			root = filepath.Join(home, root[2:])
		}
		var src []byte
		var name, what string
		var err error
		switch {
		case c.Screen != "" && c.Render == "":
			name, what = c.Screen, "screens/"+c.Screen
			if src, err = os.ReadFile(filepath.Join(dir, "..", "screens", c.Screen)); err != nil {
				return out, err
			}
		case c.Render != "" && c.Screen == "":
			r, ok := byName[c.Render]
			if !ok {
				return out, fmt.Errorf("copy to %q: no render %q", c.To, c.Render)
			}
			name, what = r.File(), c.Render
			if src, err = os.ReadFile(filepath.Join(dir, r.File())); err != nil {
				return out, fmt.Errorf("%v (run robot3d-renders first)", err)
			}
		default:
			return out, fmt.Errorf("copy to %q: a render or a screen", c.To)
		}
		b := src // a PNG to a PNG of the same size: as it is
		if !(strings.EqualFold(filepath.Ext(name), filepath.Ext(rel)) && c.Size == "") {
			if b, err = convert(src, name, c); err != nil {
				return out, fmt.Errorf("copy to %q: %v", c.To, err)
			}
		}
		target := filepath.Join(root, rel)
		res := Result{Repo: repo, Path: rel, Render: what, State: "new"}
		if old, err := os.ReadFile(target); err == nil {
			res.State = "changed"
			if bytes.Equal(old, b) {
				res.State = "same"
			}
		}
		if !check && res.State != "same" {
			if _, err := os.Stat(root); err != nil {
				return out, fmt.Errorf("repo %s: %v", repo, err)
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return out, err
			}
			if err := os.WriteFile(target, b, 0o644); err != nil {
				return out, err
			}
		}
		out = append(out, res)
	}
	return out, nil
}

// convert makes the target's format from a render's file.
func convert(src []byte, name string, c Copy) ([]byte, error) {
	ext := strings.ToLower(filepath.Ext(c.To))
	if filepath.Ext(name) == ".gif" {
		if ext != ".gif" || c.Size != "" {
			return nil, fmt.Errorf("an animation goes to a .gif as it is")
		}
		return src, nil
	}
	img, err := png.Decode(bytes.NewReader(src))
	if err != nil {
		return nil, err
	}
	if c.Size != "" {
		var w, h int
		if _, err := fmt.Sscanf(c.Size, "%dx%d", &w, &h); err != nil || w <= 0 || h <= 0 {
			return nil, fmt.Errorf("size %q: want WxH", c.Size)
		}
		dst := image.NewRGBA(image.Rect(0, 0, w, h))
		xdraw.CatmullRom.Scale(dst, dst.Bounds(), img, img.Bounds(), xdraw.Src, nil)
		img = dst
	}
	var buf bytes.Buffer
	switch ext {
	case ".png":
		err = png.Encode(&buf, img)
	case ".jpg", ".jpeg":
		bgHex := c.Bg
		if bgHex == "" {
			bgHex = "#ffffff"
		}
		bg, perr := robot3d.ParseColor(bgHex)
		if perr != nil {
			return nil, perr
		}
		flat := image.NewRGBA(img.Bounds())
		draw.Draw(flat, flat.Bounds(), &image.Uniform{bg}, image.Point{}, draw.Src)
		draw.Draw(flat, flat.Bounds(), img, img.Bounds().Min, draw.Over)
		err = jpeg.Encode(&buf, flat, &jpeg.Options{Quality: 90})
	default:
		return nil, fmt.Errorf("format %q: .png, .jpg or .gif", ext)
	}
	return buf.Bytes(), err
}
