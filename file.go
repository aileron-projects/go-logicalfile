package logicalfile

import (
	"io"
	"log"
	"os"
	"sync"
	"time"
)

// FileProvider provides file as io writer.
type FileProvider interface {
	Put(io.Writer) error
	Get() (io.Writer, int64, error)
	Wait()
}

// FileConfig is the configuration for a logical file.
// Use [NewFile] to create a new logical file.
type FileConfig struct {
	// ManagerConfig is the configuration of physical file manager.
	ManagerConfig
	// MaxBytes is the maximum size in bytes.
	// Target writer will be renewed before exceeding the size.
	// Zero or negative means no limit, or no writer rotation.
	MaxBytes uint64
	// RotateAt optionally specifies the time to rotate.
	// RotateAt can be used for rotating physical file .
	// RotateAt must return the future time.
	RotateAt func() time.Time
	// HandleError optionally handles errors occurred in a logical file.
	HandleError func(error)
	// Fallback is the function that called when writing to the
	// current target io writer failed.
	// If nil, os.Stderr.Write is used by default.
	Fallback func([]byte) (int, error)
}

// NewFile returns a new [File].
func NewFile(c *FileConfig) (*File, error) {
	m, err := newManager(c.ManagerConfig)
	if err != nil {
		return nil, err
	}
	handleError := func(err error) { log.Println(err) }
	if he := c.HandleError; he != nil {
		handleError = he
	}
	fallback := os.Stderr.Write
	if fb := c.Fallback; fb != nil {
		fallback = fb
	}
	f := &File{
		provider:    m,
		maxBytes:    c.MaxBytes,
		rotateAt:    c.RotateAt,
		handleError: handleError,
		fallback:    fallback,
		closed:      make(chan struct{}),
	}
	f.cronRotate() // run cron job to rotate in another goroutine if any.
	return f, f.Rotate()
}

// File is a logical file.
type File struct {
	mu sync.Mutex
	// provider provides writer.
	provider FileProvider
	// writer is the current target writer.
	writer io.Writer
	// maxBytes is the maximum size in byte that can be written to the writer.
	// writer will be renewed using the provider before exceeding the size.
	// Zero or negative means no limit, or no writer rotation.
	maxBytes uint64
	// curBytes is the current bytes written to the writer.
	curBytes uint64
	// rotateAt optionally specifies the time to rotate.
	rotateAt func() time.Time
	// handleError handles errors occurred in the file.
	// handleError must not be nil.
	handleError func(error)
	// fallback is the hook function that will be called
	// when physical file operation failed.
	// onFallback will be called after fallback, from physical file to stderr,
	// had been completed with non-nil error.
	// fallback must not be nil.
	fallback func([]byte) (int, error)
	// closed will be closed when the file has closed by the Close.
	// Once closed, the file must not be used any more.
	closed chan struct{}
}

// Write writes the given data in to the file.
// It will be immediately written to the underlying physical file.
// Write implements [io.Writer.Write].
// Write is safe for concurrent call.
func (f *File) Write(b []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	select {
	case <-f.closed:
		return 0, ErrFileClosed
	default:
	}
	if f.maxBytes > 0 && (f.curBytes >= f.maxBytes || uint64(len(b)) > f.maxBytes-f.curBytes) {
		if err := f.rotate(); err != nil { // Write should not depends on the rotate error.
			f.handleError(err)
			return f.fallback(b) // Fallback if non nil.
		}
	}
	if f.writer == nil {
		if err := f.rotate(); err != nil {
			f.handleError(err)
			return f.fallback(b) // Fallback if non nil.
		}
	}
	n, err := f.writer.Write(b)
	f.curBytes += uint64(n)
	if err != nil {
		f.handleError(err)
		return f.fallback(b) // Fallback if non nil.
	}
	return n, nil
}

// Close closes the logical file.
// Once Close is called, this file cannot be used to write.
// It returns nil when the file is already closed.
func (f *File) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	select {
	case <-f.closed:
		return nil // already closed
	default:
		close(f.closed)
	}
	defer f.provider.Wait() // Block until rotation finished.

	put := f.writer // return the writer
	f.writer = nil  // reset
	f.curBytes = 0  // reset
	if put != nil {
		return f.provider.Put(put)
	}
	return nil
}

func (f *File) cronRotate() {
	if f.rotateAt == nil {
		return
	}
	go func() {
		for {
			wait := time.Until(f.rotateAt())
			select {
			case <-time.After(wait):
				// continue
			case <-f.closed:
				return
			}
			if err := f.Rotate(); err != nil {
				f.handleError(err)
			}
		}
	}()
}

// Rotate renew the underlying writer which means file rotation.
// Calling Rotate forces file rotation.
// It returns [ErrFileClosed] if the file has already been closed.
func (f *File) Rotate() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	select {
	case <-f.closed:
		return ErrFileClosed
	default:
	}
	return f.rotate()
}

func (f *File) rotate() error {
	if f.writer != nil {
		put := f.writer // return the writer
		f.writer = nil  // reset
		f.curBytes = 0  // reset
		if err := f.provider.Put(put); err != nil {
			return err
		}
	}
	writer, bytes, err := f.provider.Get()
	if err != nil {
		f.writer, f.curBytes = nil, 0
		return err
	}
	f.writer, f.curBytes = writer, uint64(bytes)
	return nil
}
