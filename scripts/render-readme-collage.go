//go:build ignore

// Rebuild the README collage from the existing, non-sensitive demo screenshots:
// go run scripts/render-readme-collage.go
package main

import (
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"os"
)

func main() {
	canvas := image.NewRGBA(image.Rect(0, 0, 1920, 1100))
	for y := 0; y < canvas.Bounds().Dy(); y++ {
		for x := 0; x < canvas.Bounds().Dx(); x++ {
			glow := math.Exp(-math.Pow(float64(x-360)/900, 2) - math.Pow(float64(y-460)/750, 2))
			canvas.SetRGBA(x, y, color.RGBA{uint8(10 + 5*glow), uint8(12 + 9*glow), uint8(18 + 12*glow), 255})
		}
	}
	card(canvas, load("docs/screenshots/connection.png"), 54, 154, 1100, 800)
	card(canvas, load("docs/screenshots/servers.png"), 1216, 76, 640, 465)
	// The settings detail is cropped to its controls, not stretched; the full
	// original remains linked from the README alongside the other screenshots.
	settings := image.NewRGBA(image.Rect(0, 0, 1100, 660))
	draw.Draw(settings, settings.Bounds(), load("docs/screenshots/settings.png"), image.Point{}, draw.Src)
	card(canvas, settings, 1216, 630, 640, 384)
	out, err := os.Create("docs/screenshots/interface-collage.png")
	check(err)
	encoder := png.Encoder{CompressionLevel: png.BestCompression}
	check(encoder.Encode(out, canvas))
	check(out.Close())
}

func load(path string) image.Image {
	f, err := os.Open(path)
	check(err)
	defer f.Close()
	img, err := png.Decode(f)
	check(err)
	return img
}

func card(dst *image.RGBA, src image.Image, x, y, w, h int) {
	rounded(dst, image.Rect(x-20, y-12, x+w+20, y+h+28), 26, color.RGBA{4, 5, 9, 255})
	rounded(dst, image.Rect(x-20, y-20, x+w+20, y+h+20), 26, color.RGBA{57, 64, 79, 255})
	rounded(dst, image.Rect(x-19, y-19, x+w+19, y+h+19), 25, color.RGBA{22, 26, 35, 255})
	b := src.Bounds()
	for py := 0; py < h; py++ {
		for px := 0; px < w; px++ {
			if !inside(px, py, w, h, 10) {
				continue
			}
			sx := math.Max(0, (float64(px)+0.5)*float64(b.Dx())/float64(w)-0.5)
			sy := math.Max(0, (float64(py)+0.5)*float64(b.Dy())/float64(h)-0.5)
			x0, y0 := int(sx), int(sy)
			x1, y1 := min(x0+1, b.Dx()-1), min(y0+1, b.Dy()-1)
			fx, fy := sx-float64(x0), sy-float64(y0)
			points := [4]color.RGBA{}
			for i, p := range [4]image.Point{{x0, y0}, {x1, y0}, {x0, y1}, {x1, y1}} {
				points[i] = color.RGBAModel.Convert(src.At(b.Min.X+p.X, b.Min.Y+p.Y)).(color.RGBA)
			}
			blend := func(v [4]uint8) uint8 {
				return uint8((float64(v[0])*(1-fx)+float64(v[1])*fx)*(1-fy) + (float64(v[2])*(1-fx)+float64(v[3])*fx)*fy + 0.5)
			}
			dst.SetRGBA(x+px, y+py, color.RGBA{
				blend([4]uint8{points[0].R, points[1].R, points[2].R, points[3].R}),
				blend([4]uint8{points[0].G, points[1].G, points[2].G, points[3].G}),
				blend([4]uint8{points[0].B, points[1].B, points[2].B, points[3].B}), 255})
		}
	}
}

func rounded(dst *image.RGBA, r image.Rectangle, radius int, c color.RGBA) {
	for y := 0; y < r.Dy(); y++ {
		for x := 0; x < r.Dx(); x++ {
			if inside(x, y, r.Dx(), r.Dy(), radius) {
				dst.SetRGBA(r.Min.X+x, r.Min.Y+y, c)
			}
		}
	}
}

func inside(x, y, w, h, r int) bool {
	x = max(r-x, x-(w-r-1), 0)
	y = max(r-y, y-(h-r-1), 0)
	return x*x+y*y <= r*r
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}
