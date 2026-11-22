//go:build linux

package tray

import (
	"bytes"
	"image/color"
	"image/png"

	appassets "github.com/samosvalishe/free-turn-proxy/desktop/assets"
)

type iconPixmap struct {
	Width  int32
	Height int32
	Pixels []byte
}

func applicationIcon(light bool) ([]iconPixmap, error) {
	data := appassets.TrayIconPNG
	if light {
		data = appassets.TrayLightIconPNG
	}
	image, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	bounds := image.Bounds()
	pixels := make([]byte, 0, bounds.Dx()*bounds.Dy()*4)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			pixel := color.NRGBAModel.Convert(image.At(x, y)).(color.NRGBA)
			// StatusNotifierItem uses ARGB32 in network (big-endian) order.
			pixels = append(pixels, pixel.A, pixel.R, pixel.G, pixel.B)
		}
	}
	return []iconPixmap{{int32(bounds.Dx()), int32(bounds.Dy()), pixels}}, nil
}

func statusPixmaps(phase Phase, light bool, frame int) ([]iconPixmap, error) {
	var result []iconPixmap
	for _, size := range []int{16, 20, 22, 24, 32, 48, 64} {
		img, err := statusImage(phase, light, frame, size)
		if err != nil {
			return nil, err
		}
		pixels := make([]byte, 0, size*size*4)
		for y := 0; y < size; y++ {
			for x := 0; x < size; x++ {
				p := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
				pixels = append(pixels, p.A, p.R, p.G, p.B)
			}
		}
		result = append(result, iconPixmap{int32(size), int32(size), pixels})
	}
	return result, nil
}
