package tray

import (
	"bytes"
	"crypto/sha256"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"strings"
	"testing"

	"github.com/samosvalishe/free-turn-proxy/desktop/locale"
)

func TestStatusPriority(t *testing.T) {
	for _, test := range []struct {
		status Status
		phase  Phase
	}{
		{Status{State: "idle", HasCaptcha: true}, Idle},
		{Status{State: "idle", Preparing: true}, Connecting},
		{Status{State: "connecting"}, Connecting},
		{Status{State: "connecting", HasCaptcha: true}, Captcha},
		{Status{State: "captcha"}, Captcha},
		{Status{State: "connecting", Preparing: true, HasCaptcha: true}, Captcha},
		{Status{State: "captcha", Preparing: true}, Captcha},
		{Status{State: "connected", Streams: 3, Preparing: true, HasCaptcha: true}, Captcha},
		{Status{State: "connected", Streams: 3, Total: 12}, Connected},
		{Status{State: "connected", Streams: 3, Total: 12, HasCaptcha: true}, Attention},
		{Status{State: "captcha", Streams: 3, Total: 12}, Attention},
		{Status{State: "connected", Preparing: true}, Connecting},
		{Status{State: "connected", PreparationFailed: true}, Attention},
		{Status{State: "idle", PreparationFailed: true}, Failed},
		{Status{State: "error", HasCaptcha: true}, Failed},
	} {
		if got := test.status.Phase(); got != test.phase {
			t.Fatalf("%+v: phase %d, want %d", test.status, got, test.phase)
		}
	}
}

func TestTooltipKeepsWorkingVPNAndLocalizes(t *testing.T) {
	defer locale.Set("en")
	status := Status{State: "connected", Streams: 3, Total: 12, HasCaptcha: true}
	for _, language := range []string{"ru", "en"} {
		locale.Set(language)
		tip := status.Tooltip()
		if !strings.Contains(tip, "FreeTurn") || !strings.Contains(tip, "TURN 3/12") {
			t.Fatal(tip)
		}
		if language == "ru" && !strings.Contains(tip, "VPN работает") {
			t.Fatal(tip)
		}
		if language == "en" && !strings.Contains(tip, "VPN is working") {
			t.Fatal(tip)
		}
		if len([]rune(tip)) > 127 {
			t.Fatal("tooltip cannot fit Windows tray")
		}
	}
	preparing := Status{State: "connected", Streams: 3, Total: 12, Preparing: true}
	if strings.Contains(preparing.Tooltip(), "VPN is working") {
		t.Fatal("helper preparation pretends routes are already ready")
	}
}

func TestStatusRasterHasTransparentEdgesAndDistinctBadges(t *testing.T) {
	for _, light := range []bool{false, true} {
		for _, size := range []int{16, 20, 24, 32, 64, 256} {
			seen := map[[32]byte]bool{}
			for phase := Idle; phase <= Failed; phase++ {
				data, err := statusPNG(phase, light, 0, size)
				if err != nil {
					t.Fatal(err)
				}
				img, err := png.Decode(bytes.NewReader(data))
				if err != nil {
					t.Fatal(err)
				}
				if img.Bounds().Dx() != size || img.Bounds().Dy() != size {
					t.Fatal("wrong raster dimensions")
				}
				if _, _, _, a := img.At(0, 0).RGBA(); a != 0 {
					t.Fatal("opaque background tile")
				}
				hash := sha256.Sum256(data)
				if seen[hash] {
					t.Fatalf("indistinguishable status %d at %d px", phase, size)
				}
				seen[hash] = true
			}
		}
	}
	img, err := statusImage(Attention, true, 0, 64)
	if err != nil {
		t.Fatal(err)
	}
	green, amber := false, false
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			p := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
			if p.A > 200 && int(p.G) > int(p.R)+40 && p.G > p.B {
				green = true
			}
			if p.A > 200 && p.R > 200 && p.G > 150 && p.B < 150 {
				amber = true
			}
		}
	}
	if !green || !amber {
		t.Fatal("captcha warning must not replace the working VPN green badge")
	}
}

func TestOnlyConnectingAnimatesAndFramesAreBounded(t *testing.T) {
	for phase := Idle; phase <= Failed; phase++ {
		a, _ := statusPNG(phase, true, 0, 20)
		b, _ := statusPNG(phase, true, 1, 20)
		if (phase == Connecting) == bytes.Equal(a, b) {
			t.Fatalf("wrong animation policy for %d", phase)
		}
		wrapped, _ := statusPNG(phase, true, AnimationFrames, 20)
		if !bytes.Equal(a, wrapped) {
			t.Fatal("animation cache is unbounded")
		}
	}
}

// Optional QA contact sheet from the actual runtime rasterizer, not SVG mockups.
func TestWriteStatusIconPreview(t *testing.T) {
	path := os.Getenv("FREETURN_TRAY_PREVIEW")
	if path == "" {
		t.Skip("set FREETURN_TRAY_PREVIEW to export runtime icons")
	}
	canvas := image.NewRGBA(image.Rect(0, 0, 576, 192))
	for row, light := range []bool{true, false} {
		background := color.RGBA{32, 36, 47, 255}
		if !light {
			background = color.RGBA{241, 243, 248, 255}
		}
		draw.Draw(canvas, image.Rect(0, row*96, 576, (row+1)*96), &image.Uniform{C: background}, image.Point{}, draw.Src)
		for phase := Idle; phase <= Failed; phase++ {
			for _, size := range []int{48, 20} {
				icon, err := statusImage(phase, light, 0, size)
				if err != nil {
					t.Fatal(err)
				}
				x, y := int(phase)*96+8, row*96+24
				if size == 20 {
					x += 60
					y += 14
				}
				draw.Draw(canvas, image.Rect(x, y, x+size, y+size), icon, image.Point{}, draw.Over)
			}
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err = png.Encode(f, canvas); err != nil {
		t.Fatal(err)
	}
}
