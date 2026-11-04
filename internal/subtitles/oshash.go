package subtitles

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

const osHashChunkSize = 65536

// OSHash computes the OpenSubtitles moviehash for a local video file.
// hash = size + sum(uint64_le first 64KB) + sum(uint64_le last 64KB), all uint64 wrap.
func OSHash(path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return "", 0, err
	}
	size := fi.Size()
	if size < osHashChunkSize*2 {
		return "", size, fmt.Errorf("file too small for oshash (%d bytes)", size)
	}

	buf := make([]byte, osHashChunkSize)
	var hash uint64 = uint64(size)

	if _, err := io.ReadFull(f, buf); err != nil {
		return "", size, err
	}
	hash += sumUint64LE(buf)

	if _, err := f.Seek(size-osHashChunkSize, io.SeekStart); err != nil {
		return "", size, err
	}
	if _, err := io.ReadFull(f, buf); err != nil {
		return "", size, err
	}
	hash += sumUint64LE(buf)

	return fmt.Sprintf("%016x", hash), size, nil
}

func sumUint64LE(buf []byte) uint64 {
	var sum uint64
	for i := 0; i+8 <= len(buf); i += 8 {
		sum += binary.LittleEndian.Uint64(buf[i : i+8])
	}
	return sum
}
