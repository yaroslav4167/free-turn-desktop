package tray

import (
	"encoding/binary"
	"errors"
)

// Select a PNG-backed ICO frame at the requested size, preferring downscaling
// the next larger frame rather than upscaling a smaller icon on high-DPI panels.
func icoFrame(data []byte, size int) ([]byte, error) {
	if len(data) < 6 || binary.LittleEndian.Uint16(data) != 0 || binary.LittleEndian.Uint16(data[2:]) != 1 {
		return nil, errors.New("invalid tray ICO")
	}
	count := int(binary.LittleEndian.Uint16(data[4:]))
	if count == 0 || count > (len(data)-6)/16 {
		return nil, errors.New("invalid tray ICO directory")
	}
	best, bestSize := -1, 0
	for i := 0; i < count; i++ {
		entry := data[6+i*16 : 6+(i+1)*16]
		width := int(entry[0])
		if width == 0 {
			width = 256
		}
		height := int(entry[1])
		if height == 0 {
			height = 256
		}
		if width != height {
			return nil, errors.New("non-square tray ICO")
		}
		if best == -1 || (width >= size && (bestSize < size || width < bestSize)) || (width < size && bestSize < size && width > bestSize) {
			best, bestSize = i, width
		}
	}
	entry := data[6+best*16 : 6+(best+1)*16]
	length, offset := uint64(binary.LittleEndian.Uint32(entry[8:])), uint64(binary.LittleEndian.Uint32(entry[12:]))
	if offset < uint64(6+count*16) || length < 8 || offset+length > uint64(len(data)) {
		return nil, errors.New("invalid tray ICO frame")
	}
	frame := data[offset : offset+length]
	if string(frame[:8]) != "\x89PNG\r\n\x1a\n" {
		return nil, errors.New("tray ICO frame is not PNG")
	}
	return frame, nil
}
