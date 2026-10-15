package images

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"image"
	"io"
	"sync"
)

const (
	DefaultMaxPixels    = 40000000
	DefaultMaxDimension = 16384
)

var (
	ErrDimensionsExceeded = errors.New("image dimensions exceed processing budget")
	ErrProcessingBusy     = errors.New("image processing is busy")
	budgetMu              sync.RWMutex
	maxPixels             = DefaultMaxPixels
	maxDimension          = DefaultMaxDimension
	processingSlot        = make(chan struct{}, 1)
)

type slotKey struct{}

// ConfigureBudget is process-only; settings in the database cannot increase these limits.
func ConfigureBudget(pixels, dimension int) error {
	if pixels == 0 {
		pixels = DefaultMaxPixels
	}
	if dimension == 0 {
		dimension = DefaultMaxDimension
	}
	if pixels < 1 || pixels > DefaultMaxPixels || dimension < 1 || dimension > DefaultMaxDimension {
		return errors.New("image budget must be positive and cannot exceed defaults")
	}
	budgetMu.Lock()
	defer budgetMu.Unlock()
	maxPixels, maxDimension = pixels, dimension
	return nil
}

// AcquireProcessing also covers URL/CLI buffering. Nested processing retains the caller's slot.
func AcquireProcessing(ctx context.Context) (context.Context, func(), error) {
	if err := ctx.Err(); err != nil {
		return ctx, nil, err
	}
	if held, _ := ctx.Value(slotKey{}).(bool); held {
		return ctx, func() {}, nil
	}
	select {
	case processingSlot <- struct{}{}:
		return context.WithValue(ctx, slotKey{}, true), func() { <-processingSlot }, nil
	default:
		return ctx, nil, ErrProcessingBusy
	}
}

func validateDimensions(width, height int) error {
	budgetMu.RLock()
	defer budgetMu.RUnlock()
	if width <= 0 || height <= 0 {
		return ErrUnsupportedFormat
	}
	if width > maxDimension || height > maxDimension || width > maxPixels/height {
		return ErrDimensionsExceeded
	}
	return nil
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}

func ReadBounded(ctx context.Context, r io.Reader, limit int64) ([]byte, error) {
	if limit <= 0 {
		limit = 10 << 20
	}
	data, err := io.ReadAll(io.LimitReader(contextReader{ctx, r}, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, ErrFileTooLarge
	}
	return data, ctx.Err()
}

func inspectConfig(data []byte) (image.Config, string, bool, error) {
	if len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP" {
		width, height, animated, err := inspectWebP(data)
		if err != nil {
			return image.Config{}, "", false, err
		}
		err = validateDimensions(width, height)
		return image.Config{Width: width, Height: height}, "webp", animated, err
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || (format != "jpeg" && format != "png" && format != "gif") {
		return image.Config{}, "", false, ErrUnsupportedFormat
	}
	return cfg, format, false, validateDimensions(cfg.Width, cfg.Height)
}

type riffChunk struct {
	kind string
	data []byte
}

func riffChunks(data []byte) ([]riffChunk, error) {
	var chunks []riffChunk
	for len(data) > 0 {
		if len(data) < 8 {
			return nil, ErrUnsupportedFormat
		}
		size := uint64(binary.LittleEndian.Uint32(data[4:8]))
		padded := size + size%2
		if padded > uint64(len(data)-8) {
			return nil, ErrUnsupportedFormat
		}
		chunks = append(chunks, riffChunk{string(data[:4]), data[8 : 8+int(size)]})
		data = data[8+int(padded):]
	}
	return chunks, nil
}

func uint24(data []byte) int { return int(data[0]) | int(data[1])<<8 | int(data[2])<<16 }
func bitstreamDimensions(chunk riffChunk) (int, int, error) {
	data := chunk.data
	switch chunk.kind {
	case "VP8L":
		if len(data) >= 5 && data[0] == 0x2f && data[4]>>5 == 0 {
			bits := binary.LittleEndian.Uint32(data[1:5])
			return int(bits&0x3fff) + 1, int((bits>>14)&0x3fff) + 1, nil
		}
	case "VP8 ":
		if len(data) >= 10 && data[0]&1 == 0 && bytes.Equal(data[3:6], []byte{0x9d, 0x01, 0x2a}) {
			return int(binary.LittleEndian.Uint16(data[6:8]) & 0x3fff), int(binary.LittleEndian.Uint16(data[8:10]) & 0x3fff), nil
		}
	}
	return 0, 0, ErrUnsupportedFormat
}

// Inspect every frame header without decoding pixels. A VP8X-only or truncated RIFF is not an image.
func inspectWebP(data []byte) (int, int, bool, error) {
	fail := func() (int, int, bool, error) { return 0, 0, false, ErrUnsupportedFormat }
	if len(data) < 20 || uint64(binary.LittleEndian.Uint32(data[4:8]))+8 != uint64(len(data)) {
		return fail()
	}
	chunks, err := riffChunks(data[12:])
	if err != nil {
		return fail()
	}
	width, height, frames := 0, 0, 0
	animated, animationHeader := false, false
	for index, chunk := range chunks {
		switch chunk.kind {
		case "VP8X":
			if index != 0 || len(chunk.data) != 10 || chunk.data[0]&0xc1 != 0 || chunk.data[1] != 0 || chunk.data[2] != 0 || chunk.data[3] != 0 {
				return fail()
			}
			width, height = uint24(chunk.data[4:7])+1, uint24(chunk.data[7:10])+1
			if err := validateDimensions(width, height); err != nil {
				return 0, 0, false, err
			}
			animated = chunk.data[0]&2 != 0
		case "ANIM":
			if !animated || animationHeader || frames != 0 || len(chunk.data) != 6 {
				return fail()
			}
			animationHeader = true
		case "ANMF":
			if !animated || !animationHeader || len(chunk.data) < 16 || chunk.data[15]&0xfc != 0 {
				return fail()
			}
			x, y := uint24(chunk.data[:3])*2, uint24(chunk.data[3:6])*2
			fw, fh := uint24(chunk.data[6:9])+1, uint24(chunk.data[9:12])+1
			if fw > width || fh > height || x > width-fw || y > height-fh {
				return fail()
			}
			inner, err := riffChunks(chunk.data[16:])
			if err != nil {
				return fail()
			}
			found := false
			for _, sub := range inner {
				if sub.kind == "ALPH" {
					continue
				}
				bw, bh, err := bitstreamDimensions(sub)
				if err != nil || found || bw != fw || bh != fh {
					return fail()
				}
				// Validate the complete compressed frame as a standalone WebP, without pixel allocation.
				if !validWebPBitstream(sub) {
					return fail()
				}
				found = true
			}
			if !found {
				return fail()
			}
			frames++
		case "VP8 ", "VP8L":
			if animated || frames != 0 {
				return fail()
			}
			bw, bh, err := bitstreamDimensions(chunk)
			if err != nil {
				return fail()
			}
			if width != 0 && (width != bw || height != bh) {
				return fail()
			}
			width, height, frames = bw, bh, 1
		}
	}
	if frames == 0 || width == 0 || height == 0 {
		return fail()
	}
	return width, height, animated, nil
}
