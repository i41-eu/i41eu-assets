package catalog

import (
	"image"
	"image/color"
	"image/draw"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

var (
	fontOnce sync.Once
	fontTT   *opentype.Font
)

// caption writes text centred in a dark band (the Go font, sized to the band).
func caption(img *image.RGBA, band image.Rectangle, text string) {
	draw.Draw(img, band, &image.Uniform{color.RGBA{0x23, 0x2a, 0x5c, 0xff}}, image.Point{}, draw.Src)
	fontOnce.Do(func() {
		var err error
		if fontTT, err = opentype.Parse(goregular.TTF); err != nil {
			panic(err)
		}
	})
	size := float64(band.Dy()) * 0.5
	var face font.Face
	for ; size > 6; size-- { // shrink until it fits
		f, err := opentype.NewFace(fontTT, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingFull})
		if err != nil {
			panic(err)
		}
		if font.MeasureString(f, text).Ceil() <= band.Dx()-band.Dy()/2 {
			face = f
			break
		}
		f.Close()
	}
	if face == nil {
		return
	}
	defer face.Close()
	w := font.MeasureString(face, text).Ceil()
	m := face.Metrics()
	y := band.Min.Y + (band.Dy()+m.Ascent.Ceil()-m.Descent.Ceil())/2
	d := font.Drawer{Dst: img, Src: image.White, Face: face, Dot: fixed.P(band.Min.X+(band.Dx()-w)/2, y)}
	d.DrawString(text)
}
