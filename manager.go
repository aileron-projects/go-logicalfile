package logicalfile

import (
	"bytes"
	"cmp"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ManagerConfig is the configuration for the physical file manager.
type ManagerConfig struct {
	// MaxAge is the maximum age to keep the archived files.
	// Age is calculated based on the last modified time
	// obtained by [io/fs.FileInfo].ModTime().
	// Set UseParsedAge to true to calculate ages using
	// the time parsed from archived files.
	MaxAge time.Duration
	// MaxHistory is the maximum number of archived files to keep.
	// Zero or negative value means no limit.
	MaxHistory int
	// MaxTotalBytes is the maximum total byte size to keep archived files.
	// Zero or negative value means no limit.
	MaxTotalBytes uint64
	// UseParsedAge, if true, calculate file ages based on the time
	// parsed from archived file names.
	// Note that time information must be contained in the Pattern.
	// Missing time components are initialized to their minimum values.
	// If %Y is not in the Pattern, it will be 1.
	// If %M is not in the Pattern, it will be 1 (January).
	// If %D is not in the Pattern, it will be 1.
	// If %h is not in the Pattern, it will be 00.
	// If %m is not in the Pattern, it will be 00.
	// If %s is not in the Pattern, it will be 00.
	UseParsedAge bool
	// CompressLv specifies the compression level.
	// Zero means no compression.
	CompressLv int
	// SrcDir is the directory containing source files.
	// If empty, DstDir is used.
	// If both SrcDir and DstDir are empty, current directory is used.
	SrcDir string
	// DstDir is the directory containing archived files.
	// If empty, SrcDir is used.
	// If both DstDir and SrcDir are empty, current directory is used.
	DstDir string
	// Pattern is the file name pattern to be manged.
	// Pattern must not contain directories.
	// Pattern must not be empty when ActiveFile is not specified.
	// A ".%i" will be added when no specifiers are found.
	// Following specifiers can be used in the pattern.
	//	%Y : YYYY 4 digits year. 0 <= YYYY
	//	%M : MM 2 digits month. 1 <= MM <= 12
	//	%D : DD 2 digits day of month. 1 <= DD <= 31
	//	%h : hh 2 digits hour. 0 <= hh <= 23
	//	%m : mm 2 digits minute. 0 <= mm <= 59
	//	%s : ss 2 digits second. 0 <= ss <= 59
	//	%u : unix second with free digits. 0 <= unix
	//	%i : index with free digits. 0 <= index
	//	%H : hostname
	//	%U : user id. "-1" on windows.
	//	%G : user group id. "-1" on windows.
	//	%p : pid (process id)
	//	%P : ppid (parent process id)
	Pattern string
	// ActiveFile is the file name that will be used for write target.
	// ActiveFile is optional and can be used to use fixed file name.
	// ActiveFile must not contain directories.
	// Following specifiers can be used in the name.
	//	%H : hostname
	//	%U : user id. "-1" on windows.
	//	%G : user group id. "-1" on windows.
	//	%p : pid (process id)
	//	%P : ppid (parent process id)
	ActiveFile string
}

// newManager creates a new [manager].
func newManager(c ManagerConfig) (*manager, *Error) {
	if dir, _ := filepath.Split(c.Pattern); dir != "" {
		return nil, &Error{Op: OpNewManager, Msg: "Pattern must not contain directories", Detail: c.Pattern}
	}
	if dir, _ := filepath.Split(c.ActiveFile); dir != "" {
		return nil, &Error{Op: OpNewManager, Msg: "ActiveFile must not contain directories", Detail: c.ActiveFile}
	}

	activeFile := repalceFixedParams(c.ActiveFile)
	if _, _, char := scanFormat(activeFile); char != 0x00 {
		return nil, &Error{Op: OpNewManager, Msg: "invalid active file pattern", Detail: c.ActiveFile}
	}

	pattern := repalceFixedParams(cmp.Or(c.Pattern, activeFile))
	if pattern == "" {
		return nil, &Error{Op: OpNewManager, Msg: "Pattern must not be empty"}
	}
	// Add '%i' when no specifier found.
	if _, _, char := scanFormat(pattern); char == 0x00 || !bytes.ContainsAny([]byte{char}, "YMDhmsui") {
		ext := filepath.Ext(pattern)
		pattern = strings.TrimSuffix(pattern, ext) + ".%i" + ext
	}
	// Validate the pattern.
	if _, ok := formatFileName(pattern, time.Time{}, 123); !ok {
		return nil, &Error{Op: OpNewManager, Msg: "invalid pattern", Detail: c.Pattern}
	}

	c.SrcDir = filepath.Clean(cmp.Or(c.SrcDir, c.DstDir))
	c.DstDir = filepath.Clean(cmp.Or(c.DstDir, c.SrcDir))
	if err := os.MkdirAll(c.SrcDir, os.ModePerm); err != nil {
		return nil, &Error{Inner: err, Op: OpNewManager, Msg: "failed to create source directory"}
	}
	if err := os.MkdirAll(c.DstDir, os.ModePerm); err != nil {
		return nil, &Error{Inner: err, Op: OpNewManager, Msg: "failed to create destination directory"}
	}
	m := &manager{
		maxAge:      c.MaxAge,
		maxHistory:  c.MaxHistory,
		maxTotal:    c.MaxTotalBytes,
		compressLv:  c.CompressLv,
		srcDir:      c.SrcDir,
		dstDir:      c.DstDir,
		pattern:     pattern,
		activeFile:  activeFile,
		activeFiles: map[string]struct{}{},
	}
	if err := m.manageArchives(); err != nil { // Manage to update index.
		return nil, err
	}
	m.Wait()
	if err := m.archiveFiles(); err != nil { // May be remains.
		return nil, err
	}
	if err := m.manageArchives(); err != nil { // Manage again.
		return nil, err
	}
	m.Wait()
	return m, nil
}

// manager manages files that have the specified format in a directory.
// It archives file from srcDir to dstDir and manages their life.
// Use [NewManager] to instantiate [Manager].
//
// Manager has the following features:
//
//   - maxAge: Remove archived files older than the age.
//   - maxHistory: Limit the number of archived files.
//   - maxTotalSize: Limit the total size of archived files.
//   - gzip: Compress archived files.
type manager struct {
	mu sync.Mutex
	// maxAge is the maximum age of the backup files in second.
	// Files older than this age will be removed.
	// Zero or negative means no limitation.
	maxAge time.Duration
	// maxBackup is the maximum number of the backup files.
	// If the number of backup files exceeded this value,
	// backup files will be removed from the older one.
	// Zero or negative means no limitation.
	maxHistory int
	// maxTotal is the maximum total file size in bytes.
	// Zero or negative means no limitation.
	maxTotal uint64
	// UseParsedAge, if true, calculate file ages based on the time
	// parsed from archived file names.
	useParsedAge bool
	// compressLv is the compression level.
	// Zero means no compression.
	compressLv int
	// srcDir is the source directory.
	// Files will be moved from srcDir to dstDir when archiving them.
	// Current working directory is used if empty.
	srcDir string
	// dstDir is the destination directory.
	// Files will be moved from srcDir to dstDir when archiving them.
	// Current working directory is used if empty.
	dstDir string
	// pattern is archived file name pattern.
	pattern string
	// activeFile is active file name.
	activeFile string
	// index is the current index number.
	// This won't be reset otherwise the instance of
	// FileManage had been recreated.
	index atomic.Uint64
	// activeFiles is the file list that are not returned to
	// the manager by Put even they are issued by Get.
	// activeFiles are excluded from archive file management.
	activeFiles map[string]struct{}
	// wg is the wait group that wait all tasks that
	// manageArchives spawns.
	wg sync.WaitGroup
}

func (m *manager) Get() (io.Writer, int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	path := ""
	if m.activeFile != "" {
		path = filepath.Join(m.srcDir, m.activeFile)
	} else {
		name, _ := formatFileName(m.pattern, time.Now(), m.index.Add(1))
		path = filepath.Join(m.srcDir, name)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, os.ModePerm)
	if err != nil {
		return nil, 0, err
	}
	size := int64(0)
	if stat, err := f.Stat(); err == nil {
		size = stat.Size()
	}
	m.activeFiles[path] = struct{}{}
	return f, size, nil
}

func (m *manager) Put(w io.Writer) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	f := w.(*os.File)
	delete(m.activeFiles, f.Name())
	if err := f.Close(); err != nil {
		return err
	}
	tmpPath := f.Name()
	if m.activeFile != "" {
		name, _ := formatFileName(m.pattern, time.Now(), m.index.Add(1))
		tmpPath = filepath.Join(m.srcDir, name)
	}
	unix := strconv.FormatInt(time.Now().Unix(), 10)
	if err := os.Rename(f.Name(), tmpPath+".tmp-"+unix); err != nil {
		return &Error{Inner: err, Op: OpRename}
	}

	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		err := m.archiveFiles()
		_ = err
		err = m.manageArchives()
		_ = err
	}()
	return nil
}

func (m *manager) Wait() {
	m.wg.Wait()
}

// manageArchives manages archived files in the dstDir.
// It obtain archived files and apply maxHistory, maxAge and maxTotal restriction.
// Files more than maxHistory, older than maxAge or larger than maxTotal will be removed.
func (m *manager) manageArchives() *Error {
	m.mu.Lock()
	defer m.mu.Unlock()
	ext := ""
	if m.compressLv != 0 {
		ext = compressExt
	}
	exclude := func(info *fileInfo) bool {
		_, ok := m.activeFiles[info.path]
		return ok
	}
	files, err := listFiles(m.dstDir, m.pattern+ext, m.useParsedAge, exclude)
	if err != nil {
		return err
	}
	if len(files) > 0 && files[0].index >= m.index.Load() {
		m.index.Store(files[0].index) // Update current index.
	}
	var errs []error
	totalSize := uint64(0)
	for i, file := range files {
		if m.maxHistory > 0 && i >= m.maxHistory {
			errs = appendNonNil(errs, os.Remove(file.path))
			continue
		}
		if m.maxAge > 0 && file.age > 0 && file.age > m.maxAge {
			errs = appendNonNil(errs, os.Remove(file.path))
			continue
		}
		if m.maxTotal > 0 && (m.maxTotal < file.size || totalSize > m.maxTotal-file.size) {
			errs = appendNonNil(errs, os.Remove(file.path))
			continue
		}
		totalSize += file.size
	}
	if err := errors.Join(errs...); err != nil {
		return &Error{Inner: err, Op: OpRemove}
	}
	return nil
}

// archive archives files from srcDir to dstDir.
// Gzip compression will be applied when configured.
func (m *manager) archiveFiles() *Error {
	m.mu.Lock()
	defer m.mu.Unlock()
	files, err := listFiles(m.srcDir, m.pattern+".tmp-%u", m.useParsedAge, nil)
	if err != nil {
		return &Error{Inner: err, Op: OpArchive}
	}

	var errs []error
	for _, file := range files {
		name, _, _ := strings.CutLast(file.name, ".tmp-")
		dstPath := filepath.Join(m.dstDir, name)
		if m.compressLv == 0 {
			errs = appendNonNil(errs, os.Rename(file.path, dstPath))
			continue
		}
		errs = appendNonNil(errs, compressFile(file.path, dstPath+compressExt, m.compressLv))
	}
	if err := errors.Join(errs...); err != nil {
		return &Error{Inner: err, Op: OpArchive}
	}
	return nil
}

func appendNonNil(errs []error, err error) []error {
	if err == nil {
		return errs
	}
	return append(errs, err)
}
