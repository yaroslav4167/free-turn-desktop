package tray

import (
	"bytes"
	"image/png"
	"testing"

	appassets "github.com/samosvalishe/free-turn-proxy/desktop/assets"
)

func TestDenseTrayICOFrames(t *testing.T) {
	for _, data := range [][]byte{appassets.TrayIconICO, appassets.TrayLightIconICO} {
		for _, size := range []int{16, 20, 22, 24, 32, 40, 48, 64} {
			frame, err := icoFrame(data, size)
			if err != nil {
				t.Fatal(err)
			}
			img, err := png.Decode(bytes.NewReader(frame))
			if err != nil {
				t.Fatal(err)
			}
			if img.Bounds().Dx() != size || img.Bounds().Dy() != size {
				t.Fatalf("wrong tray size: %d %v", size, img.Bounds())
			}
			_, _, _, alpha := img.At(0, 0).RGBA()
			if alpha != 0 {
				t.Fatal("tray background must be transparent")
			}
		}
		frame, err := icoFrame(data, 18)
		if err != nil {
			t.Fatal(err)
		}
		img, err := png.Decode(bytes.NewReader(frame))
		if err != nil {
			t.Fatal(err)
		}
		if img.Bounds().Dx() != 20 {
			t.Fatal("intermediate DPI should prefer the next larger frame")
		}
	}
}

func TestTrayICORejectsInvalidResources(t *testing.T) {
	for _, data := range [][]byte{nil, []byte("not ICO"), appassets.TrayIconICO[:20]} {
		if _, err := icoFrame(data, 16); err == nil {
			t.Fatal("invalid ICO accepted")
		}
	}
}
