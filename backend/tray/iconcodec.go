// iconcodec.go PNG → ICO 转换：Windows 托盘与窗口图标要求 ICO 格式。
package tray

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/png"

	"golang.org/x/image/draw"
)

// PngToICO 将 PNG 字节包装为单条目 ICO（PNG 压缩，Windows Vista+ 支持）。
func PngToICO(pngBytes []byte) ([]byte, error) {
	cfg, err := png.DecodeConfig(bytes.NewReader(pngBytes))
	if err != nil {
		return nil, err
	}
	w, h := cfg.Width, cfg.Height
	wh := byte(0)
	if w < 256 {
		wh = byte(w)
	}
	hh := byte(0)
	if h < 256 {
		hh = byte(h)
	}

	var out bytes.Buffer
	_ = binary.Write(&out, binary.LittleEndian, uint16(0))
	_ = binary.Write(&out, binary.LittleEndian, uint16(1))
	_ = binary.Write(&out, binary.LittleEndian, uint16(1))
	out.WriteByte(wh)
	out.WriteByte(hh)
	out.WriteByte(0)
	out.WriteByte(0)
	_ = binary.Write(&out, binary.LittleEndian, uint16(1))
	_ = binary.Write(&out, binary.LittleEndian, uint16(32))
	_ = binary.Write(&out, binary.LittleEndian, uint32(len(pngBytes)))
	_ = binary.Write(&out, binary.LittleEndian, uint32(22))
	out.Write(pngBytes)
	return out.Bytes(), nil
}

// MustICO 生成 ICO，失败 panic（仅用于内嵌可信图标）
func MustICO(pngBytes []byte) []byte {
	b, err := PngToICO(pngBytes)
	if err != nil {
		panic("生成托盘图标失败: " + err.Error())
	}
	return b
}

// PngToICOScaled 生成多尺寸 ICO（窗口图标）
func PngToICOScaled(pngBytes []byte, sizes []int) ([]byte, error) {
	src, _, err := image.Decode(bytes.NewReader(pngBytes))
	if err != nil {
		return nil, err
	}
	images := make([][]byte, 0, len(sizes))
	for _, s := range sizes {
		dst := image.NewRGBA(image.Rect(0, 0, s, s))
		draw.CatmullRom.Scale(dst, dst.Rect, src, src.Bounds(), draw.Over, nil)
		var buf bytes.Buffer
		if err := png.Encode(&buf, dst); err != nil {
			return nil, err
		}
		images = append(images, buf.Bytes())
	}

	var out bytes.Buffer
	_ = binary.Write(&out, binary.LittleEndian, uint16(0))
	_ = binary.Write(&out, binary.LittleEndian, uint16(1))
	_ = binary.Write(&out, binary.LittleEndian, uint16(len(images)))
	offset := 6 + 16*len(images)
	for i, data := range images {
		s := sizes[i]
		wh := byte(s)
		if s >= 256 {
			wh = 0
		}
		out.WriteByte(wh)
		out.WriteByte(wh)
		out.WriteByte(0)
		out.WriteByte(0)
		_ = binary.Write(&out, binary.LittleEndian, uint16(1))
		_ = binary.Write(&out, binary.LittleEndian, uint16(32))
		_ = binary.Write(&out, binary.LittleEndian, uint32(len(data)))
		_ = binary.Write(&out, binary.LittleEndian, uint32(offset))
		offset += len(data)
	}
	for _, data := range images {
		out.Write(data)
	}
	return out.Bytes(), nil
}

// MustICOScaled 生成多尺寸 ICO，失败 panic
func MustICOScaled(pngBytes []byte, sizes []int) []byte {
	b, err := PngToICOScaled(pngBytes, sizes)
	if err != nil {
		panic("生成窗口图标失败: " + err.Error())
	}
	return b
}
