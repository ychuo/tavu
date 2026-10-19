// Command icon prepares Tavu's approved mascot artwork for application packaging.
package main

import (
	"image"
	"image/png"
	"log"
	"os"
	"path/filepath"

	"golang.org/x/image/draw"
)

func main() {
	const source = "resources/mascot/app-icon.png"
	const destination = "resources/icon.png"
	f, err := os.Open(source)
	if err != nil {
		log.Fatal(err)
	}
	artwork, err := png.Decode(f)
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		log.Fatal(err)
	}
	if bounds := artwork.Bounds(); bounds.Dx() != bounds.Dy() || bounds.Dx() == 0 {
		log.Fatal("mascot app icon must be a nonempty square PNG")
	}

	// Resize only: the approved character, tile and transparency come from the
	// source artwork. Running this tool must never regenerate the former T icon.
	img := image.NewNRGBA(image.Rect(0, 0, 1024, 1024))
	draw.CatmullRom.Scale(img, img.Bounds(), artwork, artwork.Bounds(), draw.Src, nil)
	out, err := os.CreateTemp(filepath.Dir(destination), ".icon-*.png")
	if err != nil {
		log.Fatal(err)
	}
	defer os.Remove(out.Name())
	if err = png.Encode(out, img); err == nil {
		err = out.Chmod(0o644)
	}
	if closeErr := out.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		log.Fatal(err)
	}
	if err = os.Rename(out.Name(), destination); err != nil {
		log.Fatal(err)
	}
}
