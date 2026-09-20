package logicalfile

import "errors"

const (
	OpCompress   = "compress"
	OpListFiles  = "list files"
	OpRemove     = "remove file "
	OpOpen       = "open file "
	OpClose      = "close file "
	OpRename     = "rename file "
	OpArchive    = "archive"
	OpNewManager = "new manager"
)

var (
	ErrFileClosed = errors.New("go-logicalfile/logicalfile: file already closed")
	ErrEmptyExt   = errors.New("go-logicalfile/logicalfile: file extension must no be empty")
	ErrNilFunc    = errors.New("go-logicalfile/logicalfile: compression function must no be nil")
	ErrInplace    = errors.New("go-logicalfile/logicalfile: inplace compression is not supported")
	ErrNilWriter  = errors.New("go-logicalfile/logicalfile: inner writer is nil. cannot write")
)

// Error is the error type.
type Error struct {
	Inner  error  // Inner is the inner error.
	Op     string // Op is the operation.
	Msg    string // Msg is the error message.
	Detail string // Details is the error detail.
}

func (e *Error) Unwrap() error {
	return e.Inner
}

func (e *Error) Error() string {
	s := "go-logicalfile/logicalfile: " + e.Op + ":"
	if e.Msg != "" {
		s += " " + e.Msg
	}
	if e.Detail != "" {
		s += ". " + e.Detail
	}
	if e.Inner != nil {
		s = s + " [" + e.Inner.Error() + "]"
	}
	return s
}

func (e *Error) Is(target error) bool {
	ee, ok := target.(*Error)
	if ok {
		return e.Op == ee.Op && e.Msg == ee.Msg
	}
	return false
}
