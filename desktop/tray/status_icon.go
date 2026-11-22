package tray

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math"
	"sync"

	appassets "github.com/samosvalishe/free-turn-proxy/desktop/assets"
)

type iconKey struct {
	Phase       Phase
	Light       bool
	Frame, Size int
}

var statusImages sync.Map
var statusPNGs sync.Map
var brandImages [2]image.Image
var brandOnce sync.Once
var brandError error

func statusPNG(phase Phase, light bool, frame, size int) ([]byte, error) {
	size = max(16, min(size, 256))
	key := iconKey{phase, light, animationFrame(phase, frame), size}
	if data, ok := statusPNGs.Load(key); ok {
		return data.([]byte), nil
	}
	img, err := statusImage(phase, light, frame, size)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	if err = png.Encode(&out, img); err != nil {
		return nil, err
	}
	data := out.Bytes()
	actual, _ := statusPNGs.LoadOrStore(key, data)
	return actual.([]byte), nil
}

func statusImage(phase Phase, light bool, frame, size int) (*image.RGBA, error) {
	size = max(16, min(size, 256))
	key := iconKey{phase, light, animationFrame(phase, frame), size}
	if cached, ok := statusImages.Load(key); ok {
		return cached.(*image.RGBA), nil
	}
	brandOnce.Do(func() {
		brandImages[0], brandError = png.Decode(bytes.NewReader(appassets.TrayIconPNG))
		if brandError == nil {
			brandImages[1], brandError = png.Decode(bytes.NewReader(appassets.TrayLightIconPNG))
		}
	})
	if brandError != nil {
		return nil, brandError
	}
	index := 0
	if light {
		index = 1
	}
	// Eight samples per logical unit keep the badge readable at 16–24 px.
	const scale = 8
	const resolution = 28 * scale
	large := image.NewRGBA(image.Rect(0, 0, resolution, resolution))
	for y := 0; y < resolution; y++ {
		for x := 0; x < resolution; x++ {
			px, py := (float64(x)+.5)/scale, (float64(y)+.5)/scale
			if px >= 25 || py < 1.5 || py >= 26.5 || math.Hypot(px-21, py-21) < 7 || (phase == Attention && math.Hypot(px-23, py-5) < 6) {
				continue
			}
			large.SetRGBA(x, y, bilinear(brandImages[index], px/25*64-.5, (py-1.5)/25*64-.5))
		}
	}
	muted, blue, green, amber, red, ink := color.RGBA{127, 140, 162, 255}, color.RGBA{69, 106, 192, 255}, color.RGBA{22, 133, 76, 255}, color.RGBA{203, 155, 41, 255}, color.RGBA{195, 70, 76, 255}, color.RGBA{255, 255, 255, 255}
	if light {
		muted = color.RGBA{137, 146, 166, 255}
		blue = color.RGBA{121, 167, 255, 255}
		green = color.RGBA{81, 206, 130, 255}
		amber = color.RGBA{236, 195, 97, 255}
		red = color.RGBA{250, 119, 121, 255}
		ink = color.RGBA{19, 35, 50, 255}
	}
	paint := func(c color.RGBA, inside func(float64, float64) bool) {
		for y := 0; y < resolution; y++ {
			for x := 0; x < resolution; x++ {
				if inside((float64(x)+.5)/scale, (float64(y)+.5)/scale) {
					large.SetRGBA(x, y, c)
				}
			}
		}
	}
	disc := func(cx, cy, r float64, c color.RGBA) {
		paint(c, func(x, y float64) bool { return math.Hypot(x-cx, y-cy) <= r })
	}
	line := func(ax, ay, bx, by, width float64, c color.RGBA) {
		paint(c, func(x, y float64) bool {
			dx, dy := bx-ax, by-ay
			t := max(0., min(1., ((x-ax)*dx+(y-ay)*dy)/(dx*dx+dy*dy)))
			return math.Hypot(x-ax-t*dx, y-ay-t*dy) <= width/2
		})
	}
	if phase == Idle {
		paint(muted, func(x, y float64) bool { return math.Abs(math.Hypot(x-21, y-21)-3.8) <= .9 })
	} else {
		badge := blue
		switch phase {
		case Connected, Attention:
			badge = green
		case Captcha:
			badge = amber
		case Failed:
			badge = red
		}
		disc(21, 21, 5.5, badge)
		switch phase {
		case Connecting:
			rotation := float64(key.Frame) * 2 * math.Pi / AnimationFrames
			paint(ink, func(x, y float64) bool {
				angle := math.Mod(math.Atan2(y-21, x-21)-rotation+4*math.Pi, 2*math.Pi)
				return math.Abs(math.Hypot(x-21, y-21)-3) <= .75 && angle < 4.0
			})
		case Connected, Attention:
			line(18, 21, 20, 23, 1.8, ink)
			line(20, 23, 24, 19, 1.8, ink)
		case Captcha:
			line(21, 18, 21, 21.5, 1.8, color.RGBA{37, 29, 9, 255})
			disc(21, 24, .9, color.RGBA{37, 29, 9, 255})
		case Failed:
			line(19, 19, 23, 23, 1.7, ink)
			line(23, 19, 19, 23, 1.7, ink)
		}
	}
	if phase == Attention {
		paint(amber, func(x, y float64) bool { return y >= 1.5 && y <= 9.5 && math.Abs(x-23) <= (y-1.5)*4.5/8 })
		line(23, 4, 23, 6.2, 1.1, color.RGBA{37, 29, 9, 255})
		disc(23, 8, .55, color.RGBA{37, 29, 9, 255})
	}
	result := image.NewRGBA(image.Rect(0, 0, size, size))
	if size > resolution {
		for y := 0; y < size; y++ {
			for x := 0; x < size; x++ {
				result.SetRGBA(x, y, bilinear(large, (float64(x)+.5)*resolution/float64(size)-.5, (float64(y)+.5)*resolution/float64(size)-.5))
			}
		}
		actual, _ := statusImages.LoadOrStore(key, result)
		return actual.(*image.RGBA), nil
	}
	// Average supersamples in premultiplied space: transparent edges stay clean
	// on both light and dark system panels, without an opaque background tile.
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			var r, g, b, a uint64
			var count uint64
			for sy := y * resolution / size; sy < (y+1)*resolution/size; sy++ {
				for sx := x * resolution / size; sx < (x+1)*resolution/size; sx++ {
					p := large.RGBAAt(sx, sy)
					r += uint64(p.R)
					g += uint64(p.G)
					b += uint64(p.B)
					a += uint64(p.A)
					count++
				}
			}
			if count > 0 {
				result.SetRGBA(x, y, color.RGBA{uint8(r / count), uint8(g / count), uint8(b / count), uint8(a / count)})
			}
		}
	}
	actual, _ := statusImages.LoadOrStore(key, result)
	return actual.(*image.RGBA), nil
}

func bilinear(img image.Image, x, y float64) color.RGBA {
	x = max(0., min(x, float64(img.Bounds().Dx()-1)))
	y = max(0., min(y, float64(img.Bounds().Dy()-1)))
	x0, y0 := int(x), int(y)
	x1, y1 := min(x0+1, img.Bounds().Dx()-1), min(y0+1, img.Bounds().Dy()-1)
	weights := [4]float64{(1 - (x - float64(x0))) * (1 - (y - float64(y0))), (x - float64(x0)) * (1 - (y - float64(y0))), (1 - (x - float64(x0))) * (y - float64(y0)), (x - float64(x0)) * (y - float64(y0))}
	var channels [4]float64
	for i, p := range [4]image.Point{{x0, y0}, {x1, y0}, {x0, y1}, {x1, y1}} {
		r, g, b, a := img.At(p.X, p.Y).RGBA()
		for c, v := range [4]uint32{r, g, b, a} {
			channels[c] += float64(v>>8) * weights[i]
		}
	}
	return color.RGBA{uint8(channels[0]), uint8(channels[1]), uint8(channels[2]), uint8(channels[3])}
}
