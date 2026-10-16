// Command icon creates Tavu's deterministic, local application icon.
package main

import (
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log"
	"os"
)

func main() {
	img := image.NewRGBA(image.Rect(0, 0, 1024, 1024))
	bg := color.RGBA{18, 25, 32, 255}
	for y := 64; y < 960; y++ {
		for x := 64; x < 960; x++ {
			cx, cy := x, y
			if x < 260 {
				cx = 260
			} else if x > 764 {
				cx = 764
			}
			if y < 260 {
				cy = 260
			} else if y > 764 {
				cy = 764
			}
			dx, dy := x-cx, y-cy
			if dx*dx+dy*dy <= 196*196 {
				img.SetRGBA(x, y, bg)
			}
		}
	}
	accent := image.NewUniform(color.RGBA{126, 192, 174, 255})
	draw.Draw(img, image.Rect(250, 265, 774, 399), accent, image.Point{}, draw.Src)
	draw.Draw(img, image.Rect(445, 375, 579, 755), accent, image.Point{}, draw.Src)
	draw.Draw(img, image.Rect(662, 675, 792, 755), image.NewUniform(color.RGBA{218, 229, 236, 255}), image.Point{}, draw.Src)
	f, err := os.Create("resources/icon.png")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	if err = png.Encode(f, img); err != nil {
		log.Fatal(err)
	}
}
