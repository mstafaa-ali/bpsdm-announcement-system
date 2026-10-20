package tray

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/draw"
	"image/png"
)

// GenerateIconBytes generates a crisp 32x32 blue speaker icon encoded as PNG or ICO
func GenerateIconPNG() []byte {
	img := image.NewRGBA(image.Rect(0, 0, 32, 32))

	blue := color.RGBA{R: 37, G: 99, B: 235, A: 255} // #2563eb
	white := color.RGBA{R: 255, G: 255, B: 255, A: 255}

	// Fill circle background
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			dx := float64(x - 16)
			dy := float64(y - 16)
			if dx*dx+dy*dy <= 14*14 {
				img.Set(x, y, blue)
			}
		}
	}

	// Draw speaker shape in white
	for y := 12; y <= 20; y++ {
		for x := 8; x <= 13; x++ {
			img.Set(x, y, white)
		}
	}
	// Cone
	for y := 8; y <= 24; y++ {
		halfH := float64(y - 16)
		if halfH < 0 {
			halfH = -halfH
		}
		startX := int(13 + halfH*0.7)
		for x := startX; x <= 19; x++ {
			img.Set(x, y, white)
		}
	}
	// Sound waves
	for y := 11; y <= 21; y++ {
		img.Set(23, y, white)
	}
	for y := 9; y <= 23; y++ {
		img.Set(26, y, white)
	}

	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

// GenerateIconICO generates a valid 16x16 Windows .ico file
func GenerateIconICO() []byte {
	width := 16
	height := 16

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	blue := color.RGBA{R: 37, G: 99, B: 235, A: 255}
	white := color.RGBA{R: 255, G: 255, B: 255, A: 255}
	draw.Draw(img, img.Bounds(), &image.Uniform{blue}, image.Point{}, draw.Src)

	// Speaker icon
	for y := 5; y <= 11; y++ {
		for x := 4; x <= 11; x++ {
			img.Set(x, y, white)
		}
	}

	var buf bytes.Buffer
	// ICONDIR
	binary.Write(&buf, binary.LittleEndian, uint16(0)) // reserved
	binary.Write(&buf, binary.LittleEndian, uint16(1)) // type: 1 = ICO
	binary.Write(&buf, binary.LittleEndian, uint16(1)) // count: 1 image

	// ICONDIRENTRY
	buf.WriteByte(byte(width))
	buf.WriteByte(byte(height))
	buf.WriteByte(0) // color count
	buf.WriteByte(0) // reserved
	binary.Write(&buf, binary.LittleEndian, uint16(1))  // planes
	binary.Write(&buf, binary.LittleEndian, uint16(32)) // bit count

	// Image data: PNG format inside ICO (standard in Windows Vista+)
	var pngBuf bytes.Buffer
	_ = png.Encode(&pngBuf, img)
	pngBytes := pngBuf.Bytes()

	binary.Write(&buf, binary.LittleEndian, uint32(len(pngBytes))) // bytes in res
	binary.Write(&buf, binary.LittleEndian, uint32(6+16))          // image offset

	buf.Write(pngBytes)
	return buf.Bytes()
}
