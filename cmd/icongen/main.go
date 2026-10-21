// Command icongen draws the PNG app icons with the standard library only:
//
//	go run ./cmd/icongen web android/res
package main

import (
	"image"
	"image/color"
	"image/png"
	"log"
	"math"
	"os"
	"path/filepath"
)

var (
	bg   = color.RGBA{0x14, 0x18, 0x1f, 0xff}
	disc = color.RGBA{0x1d, 0x23, 0x2c, 0xff}
	gold = color.RGBA{0xf2, 0xb3, 0x3d, 0xff}
)

// bolt is the lightning bolt from icon.svg on a 512 grid.
var bolt = [][2]float64{{290, 70}, {150, 290}, {246, 290}, {216, 442}, {362, 210}, {264, 210}}

func main() {
	dir := "web"
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}
	write(filepath.Join(dir, "icon-192.png"), draw(192, 1, true))
	write(filepath.Join(dir, "icon-512.png"), draw(512, 1, true))
	// Maskable icons keep the art inside the central 80% safe zone.
	write(filepath.Join(dir, "icon-maskable-512.png"), draw(512, 0.78, false))
	if len(os.Args) > 2 {
		// Android launcher icons, one per screen density.
		sizes := map[string]int{"mdpi": 48, "hdpi": 72, "xhdpi": 96, "xxhdpi": 144, "xxxhdpi": 192}
		for density, size := range sizes {
			d := filepath.Join(os.Args[2], "mipmap-"+density)
			if err := os.MkdirAll(d, 0o755); err != nil {
				log.Fatal(err)
			}
			write(filepath.Join(d, "ic_launcher.png"), draw(size, 1, true))
		}
	}
}

func draw(size int, scale float64, rounded bool) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	k := float64(size) / 512
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			// Sample in 512-space, scaled about the centre.
			px := ((float64(x)+0.5)/k-256)/scale + 256
			py := ((float64(y)+0.5)/k-256)/scale + 256
			c := color.RGBA{}
			if !rounded || insideRounded(float64(x)+0.5, float64(y)+0.5, float64(size), 96*k) {
				c = bg
			}
			d := math.Hypot(px-256, py-256)
			switch {
			case d <= 181:
				c = disc
			case d <= 199:
				c = gold
			}
			if inPoly(px, py, bolt) {
				c = gold
			}
			img.SetRGBA(x, y, c)
		}
	}
	return img
}

func insideRounded(x, y, size, r float64) bool {
	cx := math.Max(r, math.Min(size-r, x))
	cy := math.Max(r, math.Min(size-r, y))
	return math.Hypot(x-cx, y-cy) <= r
}

func inPoly(x, y float64, poly [][2]float64) bool {
	in := false
	for i, j := 0, len(poly)-1; i < len(poly); j, i = i, i+1 {
		a, b := poly[i], poly[j]
		if (a[1] > y) != (b[1] > y) && x < (b[0]-a[0])*(y-a[1])/(b[1]-a[1])+a[0] {
			in = !in
		}
	}
	return in
}

func write(path string, img image.Image) {
	f, err := os.Create(path)
	if err != nil {
		log.Fatal(err)
	}
	if err := png.Encode(f, img); err != nil {
		log.Fatal(err)
	}
	if err := f.Close(); err != nil {
		log.Fatal(err)
	}
}
