package main

import (
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

const chrome = "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"

// TOGO_BANNER=1 go test -run TestBanner . regenerates docs/banner.png: the
// view is rendered without a window, then headless Chrome lays out
// docs/banner/index.html around it.
func TestBanner(t *testing.T) {
	if os.Getenv("TOGO_BANNER") == "" {
		t.Skip("set TOGO_BANNER=1 to regenerate docs/banner.png")
	}
	dir := t.TempDir()
	writePNG(t, filepath.Join(dir, "app.png"), renderDemoApp(t))
	copyFile(t, "docs/banner/index.html", filepath.Join(dir, "index.html"))
	copyFile(t, "resources/icon.png", filepath.Join(dir, "icon.png"))

	out, err := filepath.Abs("docs/banner.png")
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(chrome, "--headless", "--hide-scrollbars", "--force-device-scale-factor=2",
		"--window-size=1280,640", "--screenshot="+out, "file://"+filepath.Join(dir, "index.html"))
	if msg, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("chrome: %v\n%s", err, msg)
	}
}

// renderDemoApp fills the window with made-up tasks and hovers one, so the
// tray shows.
func renderDemoApp(t *testing.T) *image.RGBA {
	h := newHarness(t,
		[]string{"写季度复盘", "给新版设计稿留评论", "整理下周的会议议程"},
		[]string{"买咖啡豆", "约牙医", "给妈妈打电话", "更新简历", "读完那本搁了很久的书", "周末去爬山"})
	h.app.togglePriority(h.app.ongoing[0].ID)
	h.app.togglePriority(h.app.inbox[1].ID)

	const scale = 2
	h.tt.SetSize(360, 492)
	h.tt.SetScale(scale)
	h.tt.Frame()
	h.hover("给新版设计稿留评论")

	// The traffic lights are drawn by macOS, so a windowless render lacks
	// them; paint them where TrafficLightPosition puts them.
	img := h.tt.Image()
	for i, c := range []color.RGBA{{0xFF, 0x5F, 0x57, 0xFF}, {0xFE, 0xBC, 0x2E, 0xFF}, {0x28, 0xC8, 0x40, 0xFF}} {
		fillCircle(img, (20+float64(i)*20)*scale, 17*scale, 6*scale, c)
	}
	return img
}

// fillCircle paints a solid circle with an antialiased edge.
func fillCircle(img *image.RGBA, cx, cy, radius float64, c color.RGBA) {
	for y := int(cy - radius - 1); y <= int(cy+radius+1); y++ {
		for x := int(cx - radius - 1); x <= int(cx+radius+1); x++ {
			coverage := min(radius-math.Hypot(float64(x)+0.5-cx, float64(y)+0.5-cy)+0.5, 1)
			if coverage <= 0 {
				continue
			}
			bg := img.RGBAAt(x, y)
			mix := func(a, b uint8) uint8 { return uint8(float64(a)*(1-coverage) + float64(b)*coverage) }
			img.SetRGBA(x, y, color.RGBA{mix(bg.R, c.R), mix(bg.G, c.G), mix(bg.B, c.B), max(bg.A, uint8(255*coverage))})
		}
	}
}

func writePNG(t *testing.T, path string, img image.Image) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

func copyFile(t *testing.T, from, to string) {
	t.Helper()
	data, err := os.ReadFile(from)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(to, data, 0o644); err != nil {
		t.Fatal(err)
	}
}
