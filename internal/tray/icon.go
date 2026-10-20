package tray

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
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

// GenerateIconICO generates a 100% compliant, uncompressed 16x16 32-bit Win32 BMP .ico file
func GenerateIconICO() []byte {
	width := 16
	height := 16

	var buf bytes.Buffer

	// 1. ICONDIR (6 bytes)
	_ = binary.Write(&buf, binary.LittleEndian, uint16(0)) // Reserved
	_ = binary.Write(&buf, binary.LittleEndian, uint16(1)) // Type 1 = ICO
	_ = binary.Write(&buf, binary.LittleEndian, uint16(1)) // Count = 1 image

	// Calculate sizes
	headerSize := 40                      // BITMAPINFOHEADER
	xorSize := width * height * 4         // 16 * 16 * 4 = 1024 bytes (32-bit BGRA)
	andRowSize := ((width + 31) / 32) * 4 // 4 bytes per row
	andSize := andRowSize * height        // 4 * 16 = 64 bytes
	imageSize := uint32(headerSize + xorSize + andSize)

	// 2. ICONDIRENTRY (16 bytes)
	buf.WriteByte(byte(width))
	buf.WriteByte(byte(height))
	buf.WriteByte(0) // Color count (0 for 32bpp)
	buf.WriteByte(0) // Reserved
	_ = binary.Write(&buf, binary.LittleEndian, uint16(1))  // Color planes
	_ = binary.Write(&buf, binary.LittleEndian, uint16(32)) // Bits per pixel
	_ = binary.Write(&buf, binary.LittleEndian, imageSize)  // Size of image data
	_ = binary.Write(&buf, binary.LittleEndian, uint32(22)) // Image offset (6 + 16)

	// 3. BITMAPINFOHEADER (40 bytes)
	_ = binary.Write(&buf, binary.LittleEndian, uint32(40))      // biSize
	_ = binary.Write(&buf, binary.LittleEndian, int32(width))    // biWidth
	_ = binary.Write(&buf, binary.LittleEndian, int32(height*2)) // biHeight (doubled for XOR + AND masks in ICO)
	_ = binary.Write(&buf, binary.LittleEndian, uint16(1))       // biPlanes
	_ = binary.Write(&buf, binary.LittleEndian, uint16(32))      // biBitCount
	_ = binary.Write(&buf, binary.LittleEndian, uint32(0))       // biCompression (BI_RGB)
	_ = binary.Write(&buf, binary.LittleEndian, uint32(xorSize)) // biSizeImage
	_ = binary.Write(&buf, binary.LittleEndian, int32(0))        // biXPelsPerMeter
	_ = binary.Write(&buf, binary.LittleEndian, int32(0))        // biYPelsPerMeter
	_ = binary.Write(&buf, binary.LittleEndian, uint32(0))       // biClrUsed
	_ = binary.Write(&buf, binary.LittleEndian, uint32(0))       // biClrImportant

	// 4. XOR mask (Bottom-to-top, 32-bit BGRA)
	// Blue background: #2563EB -> B: 235, G: 99, R: 37, A: 255
	// White icon: #FFFFFF -> B: 255, G: 255, R: 255, A: 255
	for y := height - 1; y >= 0; y-- {
		for x := 0; x < width; x++ {
			dx := float64(x - 8)
			dy := float64(y - 8)
			if dx*dx+dy*dy <= 7.0*7.0 {
				if (x >= 4 && x <= 7 && y >= 6 && y <= 10) || (x >= 8 && x <= 11 && y >= 4 && y <= 12) {
					buf.Write([]byte{255, 255, 255, 255})
				} else {
					buf.Write([]byte{235, 99, 37, 255})
				}
			} else {
				buf.Write([]byte{0, 0, 0, 0})
			}
		}
	}

	// 5. AND mask (Bottom-to-top, 1 bit per pixel, 0 = opaque, 1 = transparent, padded to 32 bits)
	for y := height - 1; y >= 0; y-- {
		var rowMask uint16
		for x := 0; x < width; x++ {
			dx := float64(x - 8)
			dy := float64(y - 8)
			if dx*dx+dy*dy > 7.0*7.0 {
				rowMask |= (1 << (15 - x))
			}
		}
		_ = binary.Write(&buf, binary.BigEndian, rowMask)
		_ = binary.Write(&buf, binary.LittleEndian, uint16(0))
	}

	return buf.Bytes()
}
