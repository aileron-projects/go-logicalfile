package logicalfile

import (
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"os"
	"testing"

	"github.com/aileron-projects/go-tester"
)

func TestGzipCompressFile(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	srcFile := tmp + "/test.txt"
	dstFile := tmp + "/compressed.gz"

	testCases := map[string]struct {
		src, dst string
		level    int
		err      error
	}{
		"invalid src":            {src: "not-found", dst: dstFile, level: gzip.BestSpeed, err: errors.New("dummy")},
		"invalid dst":            {src: srcFile, dst: "invalid-\x00", level: gzip.BestSpeed, err: errors.New("dummy")},
		"src=dst no compression": {src: srcFile, dst: srcFile, level: gzip.NoCompression},
		"src=dst":                {src: srcFile, dst: srcFile, level: gzip.HuffmanOnly, err: ErrInplace},
		"no compression":         {src: srcFile, dst: dstFile, level: gzip.NoCompression},
		"huffman only":           {src: srcFile, dst: dstFile, level: gzip.HuffmanOnly},
		"best compression":       {src: srcFile, dst: dstFile, level: gzip.BestCompression},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			os.Remove(dstFile)
			os.Remove(srcFile)
			os.WriteFile(srcFile, []byte("testdata"), os.ModePerm)

			err := gzipCompressFile(tc.src, tc.dst, tc.level)
			if tc.err != nil {
				// Currently we only check if the error is nil or not because the PathError
				// contains platform dependent errors.
				tester.AssertEqual(t, true, err != nil)
				return
			}
			tester.AssertEqual(t, nil, err)
			b, err := os.ReadFile(tc.dst)
			tester.AssertEqual(t, nil, err)
			var r io.Reader
			if tc.level == gzip.NoCompression {
				r = bytes.NewReader(b)
			} else {
				r, _ = gzip.NewReader(bytes.NewReader(b))
			}
			bb, _ := io.ReadAll(r)
			tester.AssertEqual(t, "testdata", string(bb))
		})
	}
}

func TestReplaceCompressor(t *testing.T) {
	ext, compress := compressExt, compressFile
	defer func() {
		compressExt, compressFile = ext, compress
	}()

	dummyFunc := func(src, dst string, lv int) error {
		return errors.New(src + dst)
	}

	t.Run("empty ext", func(t *testing.T) {
		err := ReplaceCompressor("", dummyFunc)
		tester.AssertEqual(t, ErrEmptyExt, err)
	})
	t.Run("nil func", func(t *testing.T) {
		err := ReplaceCompressor(".zst", nil)
		tester.AssertEqual(t, ErrNilFunc, err)
	})
	t.Run("replace", func(t *testing.T) {
		err := ReplaceCompressor(".zst", dummyFunc)
		tester.AssertEqual(t, nil, err)
		tester.AssertEqual(t, compressExt, ".zst")
		err = compressFile("src", "dst", 0)
		tester.AssertDeepEqual(t, "srcdst", err.Error())
	})
}
