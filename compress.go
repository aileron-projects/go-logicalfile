package logicalfile

import (
	"compress/gzip"
	"io"
	"os"
)

var (
	compressExt  = ".gz"
	compressFile = gzipCompressFile
)

// gzipCompress compressed src files to the dst file.
// src will be removed after compression successfully finished.
// gzipCompressFile do nothing when src==dst or level==gzip.NoCompression.
// Gzip compression level MUST be 0 to 9.
// See also [compress/gzip].
func gzipCompressFile(src, dst string, level int) error {
	if src == dst {
		if level == gzip.NoCompression {
			return nil
		} else {
			return ErrInplace
		}
	}
	if level == gzip.NoCompression {
		return os.Rename(src, dst)
	}

	srcFile, err := os.OpenFile(src, os.O_RDONLY, os.ModePerm)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.ModePerm)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	level = max(gzip.HuffmanOnly, min(level, gzip.BestCompression))
	gw, _ := gzip.NewWriterLevel(dstFile, level)
	defer gw.Close()

	if _, err := io.CopyBuffer(gw, srcFile, make([]byte, 1024<<10)); err != nil {
		return err
	}
	if err := srcFile.Close(); err != nil { // Close and remove the source file.
		return err
	}
	if err := os.Remove(src); err != nil {
		return err
	}
	return nil
}

// ReplaceCompressor replaces default compression method.
// It changes global state.
// ext should be file extension such as ".gz". Empty is not allowed.
// compress is the function that compress src file to dst file with the level.
// compress must not be nil.
func ReplaceCompressor(ext string, compress func(src, dst string, level int) error) error {
	if ext == "" {
		return ErrEmptyExt
	}
	if compress == nil {
		return ErrNilFunc
	}
	compressExt = ext
	compressFile = compress
	return nil
}
