package logicalfile

import (
	"compress/gzip"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/aileron-projects/go-tester"
)

func TestNewManager(t *testing.T) {
	t.Parallel()
	t.Run("empty", func(t *testing.T) {
		_, err := newManager(ManagerConfig{})
		want := &Error{
			Op:  OpNewManager,
			Msg: "Pattern must not be empty",
		}
		tester.AssertEqualErr(t, want, err)
	})
	t.Run("ActiveFile contains directory", func(t *testing.T) {
		_, err := newManager(ManagerConfig{
			ActiveFile: "foo/bar.txt",
		})
		want := &Error{
			Op:  OpNewManager,
			Msg: "ActiveFile must not contain directories",
		}
		tester.AssertEqualErr(t, want, err)
	})
	t.Run("Pattern contains directory", func(t *testing.T) {
		_, err := newManager(ManagerConfig{
			Pattern: "foo/bar.txt",
		})
		want := &Error{
			Op:  OpNewManager,
			Msg: "Pattern must not contain directories",
		}
		tester.AssertEqualErr(t, want, err)
	})
	t.Run("ActiveFile contains invalid format specifier", func(t *testing.T) {
		_, err := newManager(ManagerConfig{
			ActiveFile: "test.%x.txt",
		})
		want := &Error{
			Op:  OpNewManager,
			Msg: "invalid active file pattern",
		}
		tester.AssertEqualErr(t, want, err)
	})
	t.Run("no format specifier", func(t *testing.T) {
		got, err := newManager(ManagerConfig{
			Pattern: "test.log",
		})
		tester.AssertEqual(t, nil, err)
		want := &manager{
			srcDir:      ".",
			dstDir:      ".",
			pattern:     "test.%i.log",
			activeFiles: map[string]struct{}{},
		}
		tester.AssertDeepEqual(t, want, got)
	})
	t.Run("invalid format specifier", func(t *testing.T) {
		_, err := newManager(ManagerConfig{
			Pattern: "test.%x.log",
		})
		want := &Error{Op: OpNewManager, Msg: "invalid pattern"}
		tester.AssertEqualErr(t, want, err)
	})
	t.Run("src dir create error", func(t *testing.T) {
		_, err := newManager(ManagerConfig{
			SrcDir:  "src-\x00",
			Pattern: "test.log",
		})
		want := &Error{Op: OpNewManager, Msg: "failed to create source directory"}
		tester.AssertEqualErr(t, want, err)
	})
	t.Run("dst dir create error", func(t *testing.T) {
		_, err := newManager(ManagerConfig{
			SrcDir:  t.TempDir(),
			DstDir:  "dst-\x00",
			Pattern: "test.log",
		})
		want := &Error{Op: OpNewManager, Msg: "failed to create destination directory"}
		tester.AssertEqualErr(t, want, err)
	})
}

func TestManager_Get(t *testing.T) {
	t.Parallel()
	t.Run("active name only", func(t *testing.T) {
		tmp := t.TempDir()
		srcDir, dstDir := filepath.Join(tmp, "src"), filepath.Join(tmp, "dst")
		os.MkdirAll(srcDir, os.ModePerm)
		os.MkdirAll(dstDir, os.ModePerm)
		m := &manager{
			srcDir:      srcDir,
			dstDir:      dstDir,
			activeFile:  "active.log",
			activeFiles: map[string]struct{}{},
		}
		w, n, err := m.Get()
		tester.AssertEqual(t, nil, err)
		tester.AssertEqual(t, 0, n)
		f := w.(*os.File)
		tester.AssertEqual(t, filepath.Join(srcDir, "active.log"), f.Name())
		f.Close()
	})
	t.Run("active and archive name", func(t *testing.T) {
		tmp := t.TempDir()
		srcDir, dstDir := filepath.Join(tmp, "src"), filepath.Join(tmp, "dst")
		os.MkdirAll(srcDir, os.ModePerm)
		os.MkdirAll(dstDir, os.ModePerm)
		m := &manager{
			srcDir:      srcDir,
			dstDir:      dstDir,
			pattern:     "active.%i.log",
			activeFile:  "active.log",
			activeFiles: map[string]struct{}{},
		}
		w, n, err := m.Get()
		tester.AssertEqual(t, nil, err)
		tester.AssertEqual(t, 0, n)
		f := w.(*os.File)
		tester.AssertEqual(t, filepath.Join(srcDir, "active.log"), f.Name())
		f.Close()
	})
	t.Run("archive name only", func(t *testing.T) {
		tmp := t.TempDir()
		srcDir, dstDir := filepath.Join(tmp, "src"), filepath.Join(tmp, "dst")
		os.MkdirAll(srcDir, os.ModePerm)
		os.MkdirAll(dstDir, os.ModePerm)
		m := &manager{
			srcDir:      srcDir,
			dstDir:      dstDir,
			pattern:     "archive.%i.log",
			activeFiles: map[string]struct{}{},
		}
		w, n, err := m.Get()
		tester.AssertEqual(t, nil, err)
		tester.AssertEqual(t, 0, n)
		f := w.(*os.File)
		tester.AssertEqual(t, filepath.Join(srcDir, "archive.1.log"), f.Name())
		f.Close()
	})
	t.Run("open file error", func(t *testing.T) {
		tmp := t.TempDir()
		srcDir, dstDir := filepath.Join(tmp, "src"), filepath.Join(tmp, "dst")
		m := &manager{
			srcDir:     srcDir,
			dstDir:     dstDir,
			activeFile: "active.log",
		}
		w, n, err := m.Get()
		tester.AssertEqual(t, true, err != nil)
		tester.AssertEqual(t, nil, w)
		tester.AssertEqual(t, 0, n)
	})
}

func TestManager_Put(t *testing.T) {
	t.Parallel()
	t.Run("active name used", func(t *testing.T) {
		tmp := t.TempDir()
		m := &manager{
			srcDir:      tmp,
			dstDir:      tmp,
			pattern:     "test.%i.log",
			activeFile:  "test.log",
			activeFiles: map[string]struct{}{},
		}
		w, _, err := m.Get()
		tester.AssertEqual(t, nil, err)
		_, err = os.Stat(filepath.Join(tmp, "test.log"))
		tester.AssertEqual(t, nil, err)
		err = m.Put(w)
		tester.AssertEqual(t, nil, err)
		m.Wait()
		_, err = os.Stat(filepath.Join(tmp, "test.1.log"))
		tester.AssertEqual(t, nil, err)
	})
	t.Run("archive name only", func(t *testing.T) {
		tmp := t.TempDir()
		m := &manager{
			srcDir:      tmp,
			dstDir:      tmp,
			pattern:     "test.%i.log",
			activeFiles: map[string]struct{}{},
		}
		w, _, err := m.Get()
		tester.AssertEqual(t, nil, err)
		_, err = os.Stat(filepath.Join(tmp, "test.1.log"))
		tester.AssertEqual(t, nil, err)
		err = m.Put(w)
		tester.AssertEqual(t, nil, err)
		m.Wait()
		_, err = os.Stat(filepath.Join(tmp, "test.1.log"))
		tester.AssertEqual(t, nil, err)
	})
	t.Run("close error", func(t *testing.T) {
		tmp := t.TempDir()
		m := &manager{
			srcDir:      tmp,
			dstDir:      tmp,
			pattern:     "test.%i.log",
			activeFiles: map[string]struct{}{},
		}
		w, _, err := m.Get()
		tester.AssertEqual(t, nil, err)
		f := w.(*os.File)
		f.Close()
		err = m.Put(w)
		tester.AssertEqual(t, true, err != nil) // already cosed
	})
}

func TestManager_manageArchives(t *testing.T) {
	t.Parallel()
	t.Run("no files", func(t *testing.T) {
		srcDir := t.TempDir()
		dstDir := t.TempDir()
		m := &manager{
			srcDir:     srcDir,
			dstDir:     dstDir,
			pattern:    "test.%i.log",
			compressLv: 0,
		}
		err := m.manageArchives()
		tester.AssertEqual(t, (*Error)(nil), err)
		archives := listTestFiles(t, dstDir)
		want := []string{}
		tester.AssertDeepEqual(t, want, archives)
	})
	t.Run("maxAge", func(t *testing.T) {
		dstDir := t.TempDir()
		now := time.Now()
		testFiles := []string{
			"test." + ageUnix(now, 50*time.Second) + ".log", // older
			"test." + ageUnix(now, 40*time.Second) + ".log",
			"test." + ageUnix(now, 30*time.Second) + ".log",
			"test." + ageUnix(now, 20*time.Second) + ".log",
			"test." + ageUnix(now, 10*time.Second) + ".log", //newer
		}
		createTestFiles(t, dstDir, testFiles)
		m := &manager{
			dstDir:       dstDir,
			pattern:      "test.%u.log",
			useParsedAge: true,
			maxAge:       35 * time.Second,
		}
		err := m.manageArchives()
		tester.AssertEqual(t, (*Error)(nil), err)
		archives := listTestFiles(t, dstDir)
		want := testFiles[2:]
		tester.AssertDeepEqual(t, want, archives)
	})
	t.Run("maxAge remove all", func(t *testing.T) {
		dstDir := t.TempDir()
		now := time.Now()
		testFiles := []string{
			"test." + ageUnix(now, 50*time.Second) + ".log", // older
			"test." + ageUnix(now, 40*time.Second) + ".log",
			"test." + ageUnix(now, 30*time.Second) + ".log",
			"test." + ageUnix(now, 20*time.Second) + ".log",
			"test." + ageUnix(now, 10*time.Second) + ".log", // newer
		}
		createTestFiles(t, dstDir, testFiles)
		m := &manager{
			dstDir:       dstDir,
			pattern:      "test.%u.log",
			useParsedAge: true,
			maxAge:       5 * time.Second,
		}
		err := m.manageArchives()
		tester.AssertEqual(t, (*Error)(nil), err)
		archives := listTestFiles(t, dstDir)
		want := []string{}
		tester.AssertDeepEqual(t, want, archives)
	})
	t.Run("maxAge remove none", func(t *testing.T) {
		dstDir := t.TempDir()
		now := time.Now()
		testFiles := []string{
			"test." + ageUnix(now, 50*time.Second) + ".log", // older
			"test." + ageUnix(now, 40*time.Second) + ".log",
			"test." + ageUnix(now, 30*time.Second) + ".log",
			"test." + ageUnix(now, 20*time.Second) + ".log",
			"test." + ageUnix(now, 10*time.Second) + ".log", // newer
		}
		createTestFiles(t, dstDir, testFiles)
		m := &manager{
			dstDir:       dstDir,
			pattern:      "test.%u.log",
			useParsedAge: true,
			maxAge:       60 * time.Second,
		}
		err := m.manageArchives()
		tester.AssertEqual(t, (*Error)(nil), err)
		archives := listTestFiles(t, dstDir)
		want := testFiles
		tester.AssertDeepEqual(t, want, archives)
	})
	t.Run("maxHistory", func(t *testing.T) {
		dstDir := t.TempDir()
		testFiles := []string{
			"test.1.log", // older
			"test.2.log",
			"test.3.log",
			"test.4.log",
			"test.5.log", // newer
		}
		createTestFiles(t, dstDir, testFiles)
		m := &manager{
			dstDir:     dstDir,
			pattern:    "test.%i.log",
			maxHistory: 3,
		}
		err := m.manageArchives()
		tester.AssertEqual(t, (*Error)(nil), err)
		archives := listTestFiles(t, dstDir)
		want := testFiles[2:]
		tester.AssertDeepEqual(t, want, archives)
	})
	t.Run("maxHistory remain 1", func(t *testing.T) {
		dstDir := t.TempDir()
		testFiles := []string{
			"test.1.log", // older
			"test.2.log",
			"test.3.log",
			"test.4.log",
			"test.5.log", // newer
		}
		createTestFiles(t, dstDir, testFiles)
		m := &manager{
			dstDir:     dstDir,
			pattern:    "test.%i.log",
			maxHistory: 1,
		}
		err := m.manageArchives()
		tester.AssertEqual(t, (*Error)(nil), err)
		archives := listTestFiles(t, dstDir)
		want := testFiles[4:]
		tester.AssertDeepEqual(t, want, archives)
	})
	t.Run("maxTotal", func(t *testing.T) {
		dstDir := t.TempDir()
		testFiles := []string{ // all files have 100 bytes
			"test.1.log", // older
			"test.2.log",
			"test.3.log",
			"test.4.log",
			"test.5.log", // newer
		}
		createTestFiles(t, dstDir, testFiles)
		m := &manager{
			dstDir:   dstDir,
			pattern:  "test.%i.log",
			maxTotal: 300,
		}
		err := m.manageArchives()
		tester.AssertEqual(t, (*Error)(nil), err)
		archives := listTestFiles(t, dstDir)
		want := testFiles[2:]
		tester.AssertDeepEqual(t, want, archives)
	})
	t.Run("maxTotal remove all", func(t *testing.T) {
		dstDir := t.TempDir()
		testFiles := []string{ // all files have 100 bytes
			"test.1.log", // older
			"test.2.log",
			"test.3.log",
			"test.4.log",
			"test.5.log", // newer
		}
		createTestFiles(t, dstDir, testFiles)
		m := &manager{
			dstDir:   dstDir,
			pattern:  "test.%i.log",
			maxTotal: 99,
		}
		err := m.manageArchives()
		tester.AssertEqual(t, (*Error)(nil), err)
		archives := listTestFiles(t, dstDir)
		want := []string{}
		tester.AssertDeepEqual(t, want, archives)
	})
	t.Run("compressed file", func(t *testing.T) {
		dstDir := t.TempDir()
		testFiles := []string{
			"test.1.log.gz", // older
			"test.2.log.gz",
			"test.3.log.gz",
			"test.4.log.gz",
			"test.5.log.gz", // newer
		}
		createTestFiles(t, dstDir, testFiles)
		m := &manager{
			dstDir:     dstDir,
			pattern:    "test.%i.log",
			compressLv: gzip.BestSpeed,
			maxHistory: 3,
		}
		err := m.manageArchives()
		tester.AssertEqual(t, (*Error)(nil), err)
		archives := listTestFiles(t, dstDir)
		want := testFiles[2:]
		tester.AssertDeepEqual(t, want, archives)
	})
	t.Run("exclude file", func(t *testing.T) {
		dstDir := t.TempDir()
		testFiles := []string{
			"test.1.log", // older
			"test.2.log",
			"test.0.log", // should be excluded
			"test.3.log",
			"test.4.log",
			"test.5.log", // newer
		}
		createTestFiles(t, dstDir, testFiles)
		m := &manager{
			dstDir:      dstDir,
			pattern:     "test.%i.log",
			maxHistory:  3,
			activeFiles: map[string]struct{}{filepath.Join(dstDir, "test.0.log"): {}},
		}
		err := m.manageArchives()
		tester.AssertEqual(t, (*Error)(nil), err)
		archives := listTestFiles(t, dstDir)
		want := testFiles[2:]
		tester.AssertDeepEqual(t, want, archives)
	})
	t.Run("listfile error", func(t *testing.T) {
		m := &manager{
			dstDir:  filepath.Join(t.TempDir(), "dst"), // not exist
			pattern: "test.%i.log",
		}
		err := m.manageArchives()
		tester.AssertEqualErr(t, &Error{Op: OpListFiles}, err)
	})
}

func ageUnix(now time.Time, age time.Duration) string {
	unix := now.Unix() - int64(age.Seconds())
	return strconv.FormatInt(unix, 10)
}

func TestManager_archiveFiles(t *testing.T) {
	t.Parallel()
	t.Run("no files", func(t *testing.T) {
		srcDir := t.TempDir()
		dstDir := t.TempDir()
		m := &manager{
			srcDir:     srcDir,
			dstDir:     dstDir,
			pattern:    "test.%i.log",
			compressLv: 0,
		}
		err := m.archiveFiles()
		tester.AssertEqualErr(t, (*Error)(nil), err)
		archives := listTestFiles(t, dstDir)
		want := []string{}
		tester.AssertDeepEqual(t, want, archives)
	})
	t.Run("src!=dst, no compresssion", func(t *testing.T) {
		srcDir := t.TempDir()
		dstDir := t.TempDir()
		testFiles := []string{
			"test.1.log.tmp-1789874230",
			"test.2.log.tmp-1789874240",
			"test.3.log.tmp-1789874330",
			"test.4.log.tmp-1789875230",
			"test.5.log.tmp-1789884230",
		}
		createTestFiles(t, srcDir, testFiles)
		m := &manager{
			srcDir:     srcDir,
			dstDir:     dstDir,
			pattern:    "test.%i.log",
			compressLv: 0,
		}
		err := m.archiveFiles()
		tester.AssertEqual(t, (*Error)(nil), err)
		archives := listTestFiles(t, dstDir)
		want := []string{
			"test.1.log",
			"test.2.log",
			"test.3.log",
			"test.4.log",
			"test.5.log",
		}
		tester.AssertDeepEqual(t, want, archives)
	})
	t.Run("src!=dst, compresssion<0", func(t *testing.T) {
		srcDir := t.TempDir()
		dstDir := t.TempDir()
		testFiles := []string{
			"test.1.log.tmp-1789874230",
			"test.2.log.tmp-1789874240",
			"test.3.log.tmp-1789874330",
			"test.4.log.tmp-1789875230",
			"test.5.log.tmp-1789884230",
		}
		createTestFiles(t, srcDir, testFiles)
		m := &manager{
			srcDir:     srcDir,
			dstDir:     dstDir,
			pattern:    "test.%i.log",
			compressLv: gzip.HuffmanOnly, // -2
		}
		err := m.archiveFiles()
		tester.AssertEqual(t, (*Error)(nil), err)
		archives := listTestFiles(t, dstDir)
		want := []string{
			"test.1.log.gz",
			"test.2.log.gz",
			"test.3.log.gz",
			"test.4.log.gz",
			"test.5.log.gz",
		}
		tester.AssertDeepEqual(t, want, archives)
	})
	t.Run("src!=dst, compresssion>0", func(t *testing.T) {
		srcDir := t.TempDir()
		dstDir := t.TempDir()
		testFiles := []string{
			"test.1.log.tmp-1789874230",
			"test.2.log.tmp-1789874240",
			"test.3.log.tmp-1789874330",
			"test.4.log.tmp-1789875230",
			"test.5.log.tmp-1789884230",
		}
		createTestFiles(t, srcDir, testFiles)
		m := &manager{
			srcDir:     srcDir,
			dstDir:     dstDir,
			pattern:    "test.%i.log",
			compressLv: gzip.BestCompression, // 9
		}
		err := m.archiveFiles()
		tester.AssertEqual(t, (*Error)(nil), err)
		archives := listTestFiles(t, dstDir)
		want := []string{
			"test.1.log.gz",
			"test.2.log.gz",
			"test.3.log.gz",
			"test.4.log.gz",
			"test.5.log.gz",
		}
		tester.AssertDeepEqual(t, want, archives)
	})
	t.Run("src=dst, no compresssion", func(t *testing.T) {
		srcDir := t.TempDir()
		dstDir := srcDir
		testFiles := []string{
			"test.1.log.tmp-1789874230",
			"test.2.log.tmp-1789874240",
			"test.3.log.tmp-1789874330",
			"test.4.log.tmp-1789875230",
			"test.5.log.tmp-1789884230",
		}
		createTestFiles(t, srcDir, testFiles)
		m := &manager{
			srcDir:     srcDir,
			dstDir:     dstDir,
			pattern:    "test.%i.log",
			compressLv: 0,
		}
		err := m.archiveFiles()
		tester.AssertEqual(t, (*Error)(nil), err)
		archives := listTestFiles(t, dstDir)
		want := []string{
			"test.1.log",
			"test.2.log",
			"test.3.log",
			"test.4.log",
			"test.5.log",
		}
		tester.AssertDeepEqual(t, want, archives)
	})
	t.Run("src=dst, compresssion<0", func(t *testing.T) {
		srcDir := t.TempDir()
		dstDir := srcDir
		testFiles := []string{
			"test.1.log.tmp-1789874230",
			"test.2.log.tmp-1789874240",
			"test.3.log.tmp-1789874330",
			"test.4.log.tmp-1789875230",
			"test.5.log.tmp-1789884230",
		}
		createTestFiles(t, srcDir, testFiles)
		m := &manager{
			srcDir:     srcDir,
			dstDir:     dstDir,
			pattern:    "test.%i.log",
			compressLv: gzip.HuffmanOnly, // -2
		}
		err := m.archiveFiles()
		tester.AssertEqual(t, (*Error)(nil), err)
		archives := listTestFiles(t, dstDir)
		want := []string{
			"test.1.log.gz",
			"test.2.log.gz",
			"test.3.log.gz",
			"test.4.log.gz",
			"test.5.log.gz",
		}
		tester.AssertDeepEqual(t, want, archives)
	})
	t.Run("src=dst, compresssion>0", func(t *testing.T) {
		srcDir := t.TempDir()
		dstDir := srcDir
		testFiles := []string{
			"test.1.log.tmp-1789874230",
			"test.2.log.tmp-1789874240",
			"test.3.log.tmp-1789874330",
			"test.4.log.tmp-1789875230",
			"test.5.log.tmp-1789884230",
		}
		createTestFiles(t, srcDir, testFiles)
		m := &manager{
			srcDir:     srcDir,
			dstDir:     dstDir,
			pattern:    "test.%i.log",
			compressLv: gzip.BestCompression, // 9
		}
		err := m.archiveFiles()
		tester.AssertEqual(t, (*Error)(nil), err)
		archives := listTestFiles(t, dstDir)
		want := []string{
			"test.1.log.gz",
			"test.2.log.gz",
			"test.3.log.gz",
			"test.4.log.gz",
			"test.5.log.gz",
		}
		tester.AssertDeepEqual(t, want, archives)
	})
	t.Run("listfile error", func(t *testing.T) {
		tmp := t.TempDir()
		m := &manager{
			srcDir:  filepath.Join(tmp, "src"), // not exist
			dstDir:  tmp,
			pattern: "test.%i.log",
		}
		err := m.archiveFiles()
		tester.AssertEqualErr(t, &Error{Op: OpListFiles}, err)
	})
	t.Run("compresssion error", func(t *testing.T) {
		srcDir := t.TempDir()
		dstDir := filepath.Join(t.TempDir(), "dst") // not exist
		testFiles := []string{
			"test.1.log.tmp-1789874230",
			"test.2.log.tmp-1789874240",
		}
		createTestFiles(t, srcDir, testFiles)
		m := &manager{
			srcDir:     srcDir,
			dstDir:     dstDir,
			pattern:    "test.%i.log",
			compressLv: gzip.BestSpeed,
		}
		err := m.archiveFiles()
		tester.AssertEqualErr(t, &Error{Op: OpArchive}, err)
	})
}

func createTestFiles(t *testing.T, dir string, files []string) []string {
	t.Helper()
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		t.Error(err)
		return nil
	}
	content := strings.Repeat("1234567890", 10)
	paths := make([]string, 0, len(files))
	for _, file := range files {
		path := filepath.Join(dir, file)
		err := os.WriteFile(path, []byte(content), os.ModePerm)
		if err != nil {
			t.Error(err)
			return nil
		}
		paths = append(paths, path)
	}
	return paths
}

func listTestFiles(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Error(err)
		return nil
	}
	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		files = append(files, entry.Name())
	}
	return files
}
