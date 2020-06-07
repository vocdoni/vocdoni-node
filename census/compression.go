package census

import (
	"time"

	"github.com/klauspost/compress/zstd"
	"gitlab.com/vocdoni/go-dvote/log"
)

type compressor struct {
	encoder *zstd.Encoder
	decoder *zstd.Decoder
}

func newCompressor() compressor {
	var c compressor
	var err error
	c.encoder, err = zstd.NewWriter(nil)
	if err != nil {
		panic(err) // we don't use options, this shouldn't happen
	}
	c.decoder, err = zstd.NewReader(nil)
	if err != nil {
		panic(err) // we don't use options, this shouldn't happen
	}
	return c
}

// compressBytes compresses the input via zstd.
func (c compressor) compressBytes(src []byte) []byte {
	// ~50KiB of JSON containing base64 tends to compress to ~10% of its
	// original size. This size also seems like a good starting point for
	// most realistic compression ratios.
	estimate := len(src) / 10
	start := time.Now()
	dst := c.encoder.EncodeAll(src, make([]byte, 0, estimate))
	elapsed := time.Since(start)
	log.Debugf("compressed %.2f KiB to %.2f KiB in %s with zstd, %.1f%% of the original size",
		float64(len(src))/1000,
		float64(len(dst))/1000,
		elapsed,
		float64(len(dst)*100)/float64(len(src)))
	return dst
}

// isZstd reports whether the input bytes begin with zstd's magic number,
// 0xFD2FB528 in little-endian format.
//
// There are "magic number detection" modules, but most are pretty heavy and
// unnecessary, and we only need to detect zstd v1.
func isZstd(src []byte) bool {
	return len(src) >= 4 &&
		src[0] == 0x28 && src[1] == 0xB5 &&
		src[2] == 0x2f && src[3] == 0xFD
}

// decompressBytes tries to decompress the input as best it can. If it detects
// the input to be zstd, it decompresses using that algorithm. Otherwise, it
// assumes the input bytes aren't compressed and returns them as-is.
func (c compressor) decompressBytes(src []byte) []byte {
	if !isZstd(src) {
		// We assume that no compression is used, e.g. before we started
		// compressing census dumps when publishing to ipfs.
		return src
	}
	// We use a compressione stimate of 1/10th the size. Let's use 5x as a
	// starting point, following the same rule while being conservative.
	estimate := len(src) * 5
	start := time.Now()
	dst, err := c.decoder.DecodeAll(src, make([]byte, 0, estimate))
	if err != nil {
		log.Errorf("could not decompress zstd: %v", err)
		return nil
	}
	elapsed := time.Since(start)
	log.Debugf("decompressed %.2f KiB to %.2f KiB in %s with zstd, %.1f%% of the original size",
		float64(len(src))/1000,
		float64(len(dst))/1000,
		elapsed,
		float64(len(dst)*100)/float64(len(src)))
	return dst
}
