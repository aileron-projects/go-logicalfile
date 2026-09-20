package logicalfile

import (
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/aileron-projects/go-tester"
)

func TestError(t *testing.T) {
	t.Parallel()
	t.Run("unwrap", func(t *testing.T) {
		err := &Error{Inner: io.EOF}
		inner := err.Unwrap()
		tester.AssertEqualErr(t, io.EOF, inner)
	})
	t.Run("error message", func(t *testing.T) {
		err := &Error{Inner: io.EOF, Op: "op", Msg: "msg", Detail: ""}
		msg := err.Error()
		tester.AssertEqual(t, "go-logicalfile/logicalfile: op: msg [EOF]", msg)
	})
	t.Run("error detail", func(t *testing.T) {
		err := &Error{Inner: io.EOF, Op: "op", Msg: "msg", Detail: "detail"}
		msg := err.Error()
		tester.AssertEqual(t, "go-logicalfile/logicalfile: op: msg. detail [EOF]", msg)
	})
	t.Run("empty message", func(t *testing.T) {
		err := &Error{Inner: io.EOF, Op: "op", Msg: ""}
		msg := err.Error()
		tester.AssertEqual(t, "go-logicalfile/logicalfile: op: [EOF]", msg)
	})
	t.Run("empty message with detail", func(t *testing.T) {
		err := &Error{Inner: io.EOF, Op: "op", Msg: "", Detail: "detail"}
		msg := err.Error()
		tester.AssertEqual(t, "go-logicalfile/logicalfile: op:. detail [EOF]", msg)
	})
	t.Run("nil error", func(t *testing.T) {
		var err *Error
		tester.AssertEqual(t, false, err.Is(nil))
	})
	t.Run("nil target", func(t *testing.T) {
		err := &Error{Op: "type", Msg: "aaa", Inner: nil}
		tester.AssertEqual(t, false, err.Is(nil))
	})
	t.Run("errors equal", func(t *testing.T) {
		target := &Error{Op: "op", Msg: "msg", Inner: nil}
		err := &Error{Op: "op", Msg: "msg", Inner: io.EOF}
		tester.AssertEqual(t, true, errors.Is(err, target))
	})
	t.Run("errors not equal", func(t *testing.T) {
		target := &Error{Op: "foo"}
		err := &Error{Op: "bar"}
		tester.AssertEqual(t, false, errors.Is(err, target))
	})
	t.Run("wrapped error equal", func(t *testing.T) {
		target := &Error{Op: "op"}
		inner := &Error{Op: "op"}
		err := fmt.Errorf("outer error [%w]", inner)
		tester.AssertEqual(t, true, errors.Is(err, target))
	})
	t.Run("wrapped error not equal", func(t *testing.T) {
		target := &Error{Op: "op"}
		err := fmt.Errorf("outer error [%w]", io.EOF)
		tester.AssertEqual(t, false, errors.Is(err, target))
	})
	t.Run("wrapped errors equal", func(t *testing.T) {
		target := &Error{Op: "op"}
		inner := &Error{Op: "op"}
		err := fmt.Errorf("outer error [%w] [%w]", io.EOF, inner)
		tester.AssertEqual(t, true, errors.Is(err, target))
	})
	t.Run("wrapped error not equal", func(t *testing.T) {
		target := &Error{Op: "op"}
		err := fmt.Errorf("outer error [%w] [%w]", io.EOF, io.ErrUnexpectedEOF)
		tester.AssertEqual(t, false, errors.Is(err, target))
	})
}
