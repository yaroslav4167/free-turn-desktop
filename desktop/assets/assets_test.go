package assets

import (
	"bytes"
	"image/png"
	"testing"
)

func TestApplicationImages(t *testing.T) {
	for _, icon := range []struct {
		size int
		data []byte
	}{{512, IconPNG}, {64, Icon64PNG}, {64, TrayIconPNG}, {64, TrayLightIconPNG}} {
		image, err := png.Decode(bytes.NewReader(icon.data))
		if err != nil {
			t.Fatal(err)
		}
		if image.Bounds().Dx() != icon.size || image.Bounds().Dy() != icon.size {
			t.Fatalf("expected %dx%d icon, got %v", icon.size, icon.size, image.Bounds())
		}
	}
	if !bytes.Contains(IconSVG, []byte("<svg")) {
		t.Fatal("missing SVG application icon")
	}
	if bytes.Equal(IconSVG, InterfaceSVG) || bytes.Equal(Icon64PNG, TrayIconPNG) {
		t.Fatal("application, interface and tray artwork must stay separate")
	}
	if bytes.Contains(IconSVG, []byte("<rect")) || bytes.Contains(InterfaceSVG, []byte("<rect")) || bytes.Contains(InterfaceSVG, []byte("Gradient")) {
		t.Fatal("background tile or gradient found in the flat interface mark")
	}
	for _, data := range [][]byte{IconPNG, Icon64PNG, TrayIconPNG, TrayLightIconPNG} {
		img, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range [][2]int{{0, 0}, {img.Bounds().Dx() - 1, 0}, {0, img.Bounds().Dy() - 1}, {img.Bounds().Dx() - 1, img.Bounds().Dy() - 1}} {
			_, _, _, a := img.At(p[0], p[1]).RGBA()
			if a != 0 {
				t.Fatal("icon corner is not transparent")
			}
		}
	}
}
