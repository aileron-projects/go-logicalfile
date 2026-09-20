package logicalfile

import (
	"bytes"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/aileron-projects/go-tester"
)

func TestNewFile(t *testing.T) {
	t.Parallel()
	t.Run("manger creation fails", func(t *testing.T) {
		_, err := NewFile(&FileConfig{
			SrcDir:  t.TempDir(),
			DstDir:  t.TempDir(),
			Pattern: "invalid.%x.%y.%z.log",
		})
		want := &Error{Op: OpNewManager, Msg: "invalid pattern"}
		tester.AssertEqualErr(t, want, err)
	})
	t.Run("custom error handler", func(t *testing.T) {
		called := false
		custom := func(error) {
			called = true
		}
		f, err := NewFile(&FileConfig{
			SrcDir:      t.TempDir(),
			DstDir:      t.TempDir(),
			Pattern:     "test.log",
			HandleError: custom,
		})
		tester.AssertEqualErr(t, nil, err)
		f.handleError(nil)
		tester.AssertEqual(t, true, called)
		f.Close()
	})
	t.Run("custom fallback writer", func(t *testing.T) {
		called := false
		custom := func([]byte) (int, error) {
			called = true
			return 0, nil
		}
		f, err := NewFile(&FileConfig{
			SrcDir:   t.TempDir(),
			DstDir:   t.TempDir(),
			Pattern:  "test.log",
			Fallback: custom,
		})
		tester.AssertEqualErr(t, nil, err)
		f.fallback(nil)
		tester.AssertEqual(t, true, called)
		f.Close()
	})
}

type testProvider struct {
	get      *bytes.Buffer
	put      *bytes.Buffer
	getErr   error
	putErr   error
	maxWrite int
}

func (p *testProvider) Put(w io.Writer) error {
	p.put = w.(*bytes.Buffer)
	return p.putErr
}

func (p *testProvider) Get() (io.Writer, int64, error) {
	if p.getErr != nil {
		return nil, 0, p.getErr
	}
	p.get = bytes.NewBuffer(nil)
	if p.maxWrite > 0 {
		w := tester.MaxErrorWriter(int64(p.maxWrite))
		p.get = w.Buf
		return w, 0, nil
	}
	return p.get, 0, nil
}

func (p *testProvider) Wait() {}

func TestFile_Write(t *testing.T) {
	t.Parallel()
	t.Run("no rotate", func(t *testing.T) {
		tp := &testProvider{}
		f := &File{
			provider: tp,
		}
		n, err := f.Write([]byte("1234567890"))
		tester.AssertEqual(t, 10, n)
		tester.AssertEqual(t, "1234567890", tp.get.String())
		tester.AssertEqual(t, nil, err)
	})
	t.Run("rotate", func(t *testing.T) {
		tp := &testProvider{}
		f := &File{
			provider: tp,
			maxBytes: 15,
		}
		n, err := f.Write([]byte("1234567890"))
		tester.AssertEqual(t, 10, n)
		tester.AssertEqual(t, "1234567890", tp.get.String())
		tester.AssertEqual(t, nil, err)
		n, err = f.Write([]byte("abcdefg"))
		tester.AssertEqual(t, 7, n)
		tester.AssertEqual(t, "abcdefg", tp.get.String())
		tester.AssertEqual(t, nil, err)
	})
	t.Run("exceed maxtotal", func(t *testing.T) {
		tp := &testProvider{}
		f := &File{
			provider: tp,
			maxBytes: 5,
		}
		n, err := f.Write([]byte("1234567890")) // no rotation
		tester.AssertEqual(t, 10, n)
		tester.AssertEqual(t, "1234567890", tp.get.String())
		tester.AssertEqual(t, nil, err)
		n, err = f.Write([]byte("abc")) // should be roteted
		tester.AssertEqual(t, 3, n)
		tester.AssertEqual(t, "abc", tp.get.String())
		tester.AssertEqual(t, nil, err)
	})
	t.Run("exceed maxtotal", func(t *testing.T) {
		tp := &testProvider{}
		f := &File{
			provider: tp,
			maxBytes: 5,
		}
		n, err := f.Write([]byte("123")) // no rotation
		tester.AssertEqual(t, 3, n)
		tester.AssertEqual(t, "123", tp.get.String())
		tester.AssertEqual(t, nil, err)
		n, err = f.Write([]byte("abc")) // should be roteted
		tester.AssertEqual(t, 3, n)
		tester.AssertEqual(t, "abc", tp.get.String())
		tester.AssertEqual(t, nil, err)
	})
	t.Run("rotate error for nil writer", func(t *testing.T) {
		tp := &testProvider{
			getErr: errors.New("get error"),
		}
		var got []byte
		f := &File{
			provider:    tp,
			handleError: func(err error) {},
			fallback: func(b []byte) (int, error) {
				got = b
				return len(b), nil
			},
		}
		n, err := f.Write([]byte("123"))
		tester.AssertEqual(t, 3, n)
		tester.AssertEqual(t, 0, f.curBytes)
		tester.AssertEqual(t, true, tp.get == nil)
		tester.AssertEqual(t, nil, err)
		tester.AssertEqual(t, "123", string(got))
	})
	t.Run("rotate error when exceeded maxTotal", func(t *testing.T) {
		tp := &testProvider{
			getErr: errors.New("get error"),
		}
		var got []byte
		f := &File{
			provider:    tp,
			handleError: func(err error) {},
			fallback: func(b []byte) (int, error) {
				got = b
				return len(b), nil
			},
			maxBytes: 10,
			curBytes: 5,
		}
		n, err := f.Write([]byte("123456"))
		tester.AssertEqual(t, 6, n)
		tester.AssertEqual(t, 0, f.curBytes)
		tester.AssertEqual(t, true, tp.get == nil)
		tester.AssertEqual(t, nil, err)
		tester.AssertEqual(t, "123456", string(got))
	})
	t.Run("write error", func(t *testing.T) {
		tp := &testProvider{
			maxWrite: 5,
		}
		var got []byte
		f := &File{
			provider:    tp,
			handleError: func(err error) {},
			fallback: func(b []byte) (int, error) {
				got = b
				return len(b), nil
			},
		}
		n, err := f.Write([]byte("123"))
		tester.AssertEqual(t, nil, err)
		tester.AssertEqual(t, 3, n)
		tester.AssertEqual(t, "123", tp.get.String())
		n, err = f.Write([]byte("abc"))
		tester.AssertEqual(t, nil, err)
		tester.AssertEqual(t, "123ab", tp.get.String())
		tester.AssertEqual(t, "abc", string(got))
	})
	t.Run("file already closed", func(t *testing.T) {
		tp := &testProvider{}
		f := &File{
			provider: tp,
			closed:   make(chan struct{}),
		}
		n, err := f.Write([]byte("123"))
		tester.AssertEqual(t, nil, err)
		tester.AssertEqual(t, 3, n)
		tester.AssertEqual(t, "123", tp.get.String())
		err = f.Close()
		tester.AssertEqual(t, nil, err)
		n, err = f.Write([]byte("abc"))
		tester.AssertEqual(t, 0, n)
		tester.AssertEqualErr(t, ErrFileClosed, err)
	})
}

func TestFile_Close(t *testing.T) {
	t.Parallel()
	t.Run("close success", func(t *testing.T) {
		tp := &testProvider{}
		f := &File{
			provider: tp,
			closed:   make(chan struct{}),
		}
		n, err := f.Write([]byte("123"))
		tester.AssertEqual(t, nil, err)
		tester.AssertEqual(t, 3, n)
		tester.AssertEqual(t, "123", tp.get.String())
		err = f.Close()
		tester.AssertEqual(t, nil, err)
		n, err = f.Write([]byte("abc"))
		tester.AssertEqual(t, 0, n)
		tester.AssertEqualErr(t, ErrFileClosed, err)
	})
	t.Run("close multi-times", func(t *testing.T) {
		tp := &testProvider{}
		f := &File{
			provider: tp,
			closed:   make(chan struct{}),
		}
		n, err := f.Write([]byte("123"))
		tester.AssertEqual(t, nil, err)
		tester.AssertEqual(t, 3, n)
		tester.AssertEqual(t, "123", tp.get.String())
		err = f.Close()
		tester.AssertEqual(t, nil, err)
		err = f.Close()
		tester.AssertEqual(t, nil, err)
	})
	t.Run("close without write", func(t *testing.T) {
		tp := &testProvider{}
		f := &File{
			provider: tp,
			closed:   make(chan struct{}),
		}
		err := f.Close()
		tester.AssertEqual(t, nil, err)
		err = f.Close()
		tester.AssertEqual(t, nil, err)
	})
}

func TestFile_Rotate(t *testing.T) {
	t.Parallel()
	t.Run("rotate success", func(t *testing.T) {
		tp := &testProvider{}
		f := &File{
			provider: tp,
			closed:   make(chan struct{}),
		}
		n, err := f.Write([]byte("123"))
		tester.AssertEqual(t, nil, err)
		tester.AssertEqual(t, 3, n)
		tester.AssertEqual(t, "123", tp.get.String())
		err = f.Rotate()
		tester.AssertEqual(t, nil, err)
	})
	t.Run("rotate for closed file", func(t *testing.T) {
		tp := &testProvider{}
		f := &File{
			provider: tp,
			closed:   make(chan struct{}),
		}
		n, err := f.Write([]byte("123"))
		tester.AssertEqual(t, nil, err)
		tester.AssertEqual(t, 3, n)
		tester.AssertEqual(t, "123", tp.get.String())
		err = f.Rotate()
		tester.AssertEqual(t, nil, err)
		f.Close()
		err = f.Rotate()
		tester.AssertEqual(t, ErrFileClosed, err)
	})
	t.Run("rotate error", func(t *testing.T) {
		tp := &testProvider{
			putErr: errors.New("put error"),
		}
		f := &File{
			provider: tp,
			closed:   make(chan struct{}),
		}
		n, err := f.Write([]byte("123"))
		tester.AssertEqual(t, nil, err)
		tester.AssertEqual(t, 3, n)
		tester.AssertEqual(t, "123", tp.get.String())
		err = f.Rotate()
		tester.AssertEqual(t, "put error", err.Error())
	})
}

func TestFile_cronRotate(t *testing.T) {
	t.Parallel()
	t.Run("rotate success", func(t *testing.T) {
		tp := &testProvider{}
		f := &File{
			provider: tp,
			closed:   make(chan struct{}),
			rotateAt: func() time.Time {
				return time.Now().Add(100 * time.Millisecond)
			},
		}
		f.cronRotate()
		n, err := f.Write([]byte("123"))
		tester.AssertEqual(t, nil, err)
		tester.AssertEqual(t, 3, n)
		tester.AssertEqual(t, "123", tp.get.String())
		tester.AssertEqual(t, 3, f.curBytes)
		time.Sleep(150 * time.Millisecond)
		n, err = f.Write([]byte("abcde"))
		tester.AssertEqual(t, nil, err)
		tester.AssertEqual(t, 5, n)
		tester.AssertEqual(t, "abcde", tp.get.String())
		tester.AssertEqual(t, 5, f.curBytes)
	})
	t.Run("closed", func(t *testing.T) {
		tp := &testProvider{}
		f := &File{
			provider: tp,
			closed:   make(chan struct{}),
			rotateAt: func() time.Time {
				return time.Now().Add(100 * time.Millisecond)
			},
		}
		f.cronRotate()
		n, err := f.Write([]byte("123"))
		tester.AssertEqual(t, nil, err)
		tester.AssertEqual(t, 3, n)
		tester.AssertEqual(t, "123", tp.get.String())
		tester.AssertEqual(t, 3, f.curBytes)
		f.Close() // Rotation in cron job canceled
		tester.AssertEqual(t, 0, f.curBytes)
		tester.AssertEqual(t, "123", tp.put.String())
	})
}
