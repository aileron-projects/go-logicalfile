package logicalfile

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// fileInfo is the file information of archived files.
type fileInfo struct {
	name    string        // name is the file name. '/' not contained.
	path    string        // path is the file path. '/' may be contained.
	size    uint64        // size is the file size in bytes.
	created time.Time     // created is the created timestamp. created can be zero.
	age     time.Duration // age is the file age based on the created time. age can be zero.
	index   uint64        // index is the incremental index number. index can be zero.
}

// listFiles returns file info in the dir those file names
// match to the given pattern.
// Given pattern must be valid for [scanFileName].
// It does not search files of sub-directories of the dir.
// Returned slice of fileInfo is sorted by age and index.
func listFiles(dir, pattern string, parsedAge bool, exclude func(*fileInfo) bool) ([]*fileInfo, *Error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, &Error{Inner: err, Op: OpListFiles}
	}
	now := time.Now().Local()
	files := make([]*fileInfo, 0, len(entries))
	for _, entry := range entries {
		info, err := entry.Info()
		if !entry.Type().IsRegular() || err != nil {
			continue
		}
		created, index, ok := parseTimeIndex(pattern, entry.Name())
		if !ok {
			continue
		}
		var age time.Duration
		if parsedAge || !created.IsZero() {

			age = now.Sub(created)
		} else {
			age = now.Sub(info.ModTime())
		}
		fileInfo := &fileInfo{
			name:    entry.Name(),
			path:    filepath.Join(dir, entry.Name()), // On windows, '\' is used.
			size:    uint64(info.Size()),
			created: created,
			age:     age,
			index:   index,
		}
		if exclude != nil && exclude(fileInfo) {
			continue // excluded
		}
		files = append(files, fileInfo)
	}
	sort.SliceStable(files, func(i, j int) bool {
		if files[i].age != files[j].age {
			return files[i].age < files[j].age
		}
		return files[i].index > files[j].index
	})
	return files, nil
}

// repalceFixedParams replaces following format specifiers to value.
//   - %H : hostname
//   - %U : user id. "-1" on windows.
//   - %G : user group id. "-1" on windows.
//   - %p : pid (process id)
//   - %P : ppid (parent process id)
func repalceFixedParams(pattern string) string {
	hostname, _ := os.Hostname()
	pattern = strings.ReplaceAll(pattern, "%H", hostname)
	pattern = strings.ReplaceAll(pattern, "%U", strconv.Itoa(os.Getuid()))  // -1 on windows.
	pattern = strings.ReplaceAll(pattern, "%G", strconv.Itoa(os.Getegid())) // -1 on windows.
	pattern = strings.ReplaceAll(pattern, "%p", strconv.Itoa(os.Getpid()))
	pattern = strings.ReplaceAll(pattern, "%P", strconv.Itoa(os.Getppid()))
	return pattern
}

// formatFileName returns formatted string.
// Use [scanFileName] to parse time and index from file names.
// Timezone is always [time.Local].
//
// Allowed format specifiers are:
//
//	%Y : YYYY 4 digits year. 0 <= YYYY
//	%M : MM 2 digits month. 1 <= MM <= 12
//	%D : DD 2 digits day of month. 1 <= DD <= 31
//	%h : hh 2 digits hour. 0 <= hh <= 23
//	%m : mm 2 digits minute. 0 <= mm <= 59
//	%s : ss 2 digits second. 0 <= ss <= 59
//	%u : unix second with free digits. 0 <= unix
//	%i : index with free digits. 0 <= index
func formatFileName(format string, t time.Time, index uint64) (str string, ok bool) {
	var builder strings.Builder
	builder.Grow(len(format) + 5) // Add +5 just in case.
	var prefix string
	var char byte
	t = t.Local() // Force local time.
	year, month, day := t.Date()
	for format != "" {
		prefix, format, char = scanFormat(format)
		_, _ = builder.WriteString(prefix)
		if char == 0x00 {
			return builder.String(), format == ""
		}
		switch char {
		case 'Y':
			_, _ = fmt.Fprintf(&builder, "%04d", year)
		case 'M':
			_, _ = fmt.Fprintf(&builder, "%02d", month)
		case 'D':
			_, _ = fmt.Fprintf(&builder, "%02d", day)
		case 'h':
			_, _ = fmt.Fprintf(&builder, "%02d", t.Hour())
		case 'm':
			_, _ = fmt.Fprintf(&builder, "%02d", t.Minute())
		case 's':
			_, _ = fmt.Fprintf(&builder, "%02d", t.Second())
		case 'u':
			_, _ = fmt.Fprint(&builder, strconv.FormatInt(t.Unix(), 10))
		case 'i':
			_, _ = fmt.Fprint(&builder, strconv.FormatUint(index, 10))
		default:
			return "", false // Invalid '%'.
		}
	}
	return builder.String(), true
}

// parseTimeIndex parses time and index from str with the fromat.
// It parses timestamp and index from the str.
// It returns false when the str does not comply with the format.
// It returns parsed time and index with true when the str complies with the format.
// Use [formatFileName] to generate file names that comply with the format.
// Time zone of parsed time is always [time.Local].
// If both time '%u' and other time specifiers are exist in the format,
// it uses unix time as the returned time t.
// It identifies time that are not exist in the format as zero.
// For example, when the format has '%Y-%M-%D', the other time of
// '%h', '%m' and '%s' are recognized as zero.
// Returned time always be zero value if '%Y', '%M' or '%D' not exist.
// See also https://www.w3.org/TR/NOTE-datetime.
//
// Allowed format specifiers are:
//
//   - %Y : YYYY 4 digits year. 0 <= YYYY
//   - %M : MM 2 digits month. 1 <= MM <= 12
//   - %D : DD 2 digits day of month. 1 <= DD <= 31
//   - %h : hh 2 digits hour. 0 <= hh <= 23
//   - %m : mm 2 digits minute. 0 <= mm <= 59
//   - %s : ss 2 digits second. 0 <= ss <= 59
//   - %u : unix second with free digits. 0 <= unix
//   - %i : index with free digits. 0 <= index
func parseTimeIndex(format, str string) (t time.Time, index uint64, ok bool) {
	var prefix string
	var char byte
	var year, month, day, hour, minute, second, unix uint64
	for format != "" {
		prefix, format, char = scanFormat(format)
		if !strings.HasPrefix(str, prefix) {
			return time.Time{}, 0, false
		}
		str = strings.TrimPrefix(str, prefix)
		if char == 0x00 {
			break
		}
		var ok bool
		switch char {
		case 'Y':
			year, str, ok = scanNumber(str, 4)
		case 'M':
			month, str, ok = scanNumber(str, 2)
		case 'D':
			day, str, ok = scanNumber(str, 2)
		case 'h':
			hour, str, ok = scanNumber(str, 2)
		case 'm':
			minute, str, ok = scanNumber(str, 2)
		case 's':
			second, str, ok = scanNumber(str, 2)
		case 'u':
			unix, str, ok = scanNumber(str, -1) // Free digits.
		case 'i':
			index, str, ok = scanNumber(str, -1) // Free digits.
		default:
			ok = false // Unsupported format specifier.
		}
		if !ok {
			return time.Time{}, 0, false
		}
	}
	if month > 12 || day > 31 || hour > 23 || minute > 59 || second > 59 {
		return time.Time{}, 0, false // Invalid number.
	}
	if unix > 0 {
		t = time.Unix(int64(unix), 0)
	} else if year > 0 && month > 0 && day > 0 {
		t = time.Date(int(year), time.Month(month), int(day), int(hour), int(minute), int(second), 0, time.Local)
	}
	return t, index, str == "" && format == ""
}

// scanFormat scans the format string and identifies the first format specifier
// indicated by '%'. It returns the string before the specifier as 'prefix',
// the string after the specifier as 'rest', and the format specifier itself as 'char'.
//
// For example, given the string "foo%dbar", it returns:
//
//	prefix: "foo"
//	rest:   "bar"
//	char:   'd'
//
// If no format specifier is found, the entire format string is returned as 'prefix',
// with an empty 'rest' and 0x00 as 'char'.
func scanFormat(format string) (prefix, rest string, char byte) {
	i := strings.Index(format, "%")
	switch i {
	case -1:
		return format, "", 0x00
	case len(format) - 1:
		return format[:i], "", 0x00
	default:
		return format[:i], format[i+2:], format[i+1]
	}
}

// scanNumber scans numbers with the specified digit from the beginning of the str.
// It returns parsed number and the rest of str.
// It returns false when numbers with given digits were not found.
// For digit<=0, scanNumber parses all available numbers from the begining of the str.
//
// Examples:
//
//	scanNumber("012alice", -1) -> 12, "alice", true
//	scanNumber("012alice", 0) -> 12, "alice", true
//	scanNumber("012alice", 1) -> 0, "12alice", true
//	scanNumber("012alice", 2) -> 1, "2alice", true
//	scanNumber("012alice", 3) -> 12, "alice", true
//	scanNumber("alice012", 2) -> 0, "", false
func scanNumber(str string, digit int) (num uint64, rest string, ok bool) {
	n := 0
	for i := range str {
		if '0' <= str[i] && str[i] <= '9' {
			n = i + 1
			continue
		}
		break
	}
	if digit <= 0 {
		digit = n // Use all available numbers.
	} else {
		if n < digit {
			return 0, "", false // Insufficient number.
		}
	}
	// Format "123" or "0123" are allowed.
	// Empty string results in an error.
	num, err := strconv.ParseUint(str[:digit], 10, 64)
	if err != nil {
		return 0, "", false
	}
	return num, str[digit:], true
}
