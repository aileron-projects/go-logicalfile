<!-- markdownlint-disable MD033 MD041 -->

<div align="center">

[![Release](https://img.shields.io/github/v/release/aileron-projects/go-logicalfile?sort=semver)](https://github.com/aileron-projects/go-logicalfile/releases)
[![Reference](https://pkg.go.dev/badge/github.com/aileron-projects/go-logicalfile.svg)](https://pkg.go.dev/github.com/aileron-projects/go-logicalfile)
[![DeepWiki](https://deepwiki.com/badge.svg)](https://deepwiki.com/aileron-projects/go-logicalfile)
[![Test](https://github.com/aileron-projects/go-logicalfile/actions/workflows/test.yaml/badge.svg)](https://github.com/aileron-projects/go-logicalfile/actions/workflows/test.yaml)

[![Insights](https://badgen.net/badge/Insights/open%2Fsource%2Finsights/cyan)](https://deps.dev/go/github.com%2Faileron-projects%2Fgo-logicalfile)
[![Insights](https://badgen.net/badge/Insights/OSS%2FInsight/orange)](https://ossinsight.io/analyze/aileron-projects/go-logicalfile)

</div>

# go-logicalfile

**A logical file library that abstracts away physical file management for Go.**

`go-logicalfile` provides a logical `io.Writer` whose underlying physical files are managed automatically.
It separates application-level file writing from concerns such as file rotation, archiving, retention, compression, and file naming.

```txt
Application
    ↓
io.Writer
    ↓
go-logicalfile
    ↓
active physical file
    ↓
archive / rotation / compression
```

## Features

- Logical file abstraction
  - Write to a logical file without managing physical files directly.
  - Automatically rotates and replaces the underlying physical writer when necessary.
- Automatic file rotation
  - Rotate files based on maximum file size.
  - Rotate files at a specified time.
- Archived file management
  - `MaxAge` — Remove archived files older than the specified age.
  - `MaxHistory` — Remove archived files when the number of archives exceeds the specified limit.
  - `MaxTotalBytes` — Remove archived files when their total size exceeds the specified limit.
- Optional compression of archived files.
- Flexible file naming
  - Configure independent patterns for active and archived files.
  - Generate file names using timestamps, indexes, host information, process IDs, and other values.

```go
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
  //   %Y : YYYY 4 digits year. 0 <= YYYY
  //   %M : MM 2 digits month. 1 <= MM <= 12
  //   %D : DD 2 digits day of month. 1 <= DD <= 31
  //   %h : hh 2 digits hour. 0 <= hh <= 23
  //   %m : mm 2 digits minute. 0 <= mm <= 59
  //   %s : ss 2 digits second. 0 <= ss <= 59
  //   %u : unix second with free digits. 0 <= unix
  //   %i : index with free digits. 0 <= index
  //   %H : hostname
  //   %U : user id. "-1" on windows.
  //   %G : user group id. "-1" on windows.
  //   %p : pid (process id)
  //   %P : ppid (parent process id)
  Pattern string
  // ActiveFile is the file name that will be used for write target.
  // ActiveFile is optional and can be used to use fixed file name.
  // ActiveFile must not contain directories.
  // Following specifiers can be used in the name.
  //   %H : hostname
  //   %U : user id. "-1" on windows.
  //   %G : user group id. "-1" on windows.
  //   %p : pid (process id)
  //   %P : ppid (parent process id)
  ActiveFile string
}
```

The `Pattern` is the archived file name patterns and the `ActiveFile` is the file name of current write target.
If `ActiveFile` is not set, the `Pattern` is used to create a new physical file.

```txt
ActiveFile @ SrcDir
  application.log
      │
      │ rotation
      ▼
Archive files @ DstDir
  - application.0.log
  - application.1.log
  - application.2.log
```

## Usages

### MaxAge

Use `MaxAge` to remove archived files older than the specified age.

For `UseParsedAge=false`, file ages are calculated from the file's last modification time.
For `UseParsedAge=true`, file ages are calculated from timestamp parsed from the file names.
The `Pattern` must contain timestamp when `UseParsedAge=true`.

- `UseParsedAge=false`: age is calculated from the file modification time.
- `UseParsedAge=true`: age is calculated from the timestamp parsed from the filename.

Examples:

```go
// Use last modified time to calculate file ages.
config1 = &logicalfile.FileConfig{
  DstDir:       "./logs",
  Pattern:      "application.%i.log",
  MaxAge:       30 * time.Second,
  UseParsedAge: false, // Use last modified time.
  MaxBytes:     500,   // for single file
}
```

```go
// Use timestamp parsed from file name to calculate file ages.
// Timestamp points the time when files are archived.
var config2 = &logicalfile.FileConfig{
  DstDir:       "./logs",
  Pattern:      "application.%Y-%M-%D.%h-%m-%s.log",
  ActiveFile:   "application.log", // set non-empty
  MaxAge:       30 * time.Second,
  UseParsedAge: true, // Parse timestamp from filename.
  MaxBytes:     500,  // for single file
}
```

```go
// Use timestamp parsed from file name to calculate file ages.
// Timestamp points the time when files are created.
var config3 = &logicalfile.FileConfig{
  DstDir:       "./logs",
  Pattern:      "application.%Y-%M-%D.%h-%m-%s.log",
  ActiveFile:   "", // do not set
  MaxAge:       30 * time.Second,
  UseParsedAge: true, // Parse timestamp from filename.
  MaxBytes:     500,  // for single file
}
```

### MaxTotalBytes

`MaxTotalBytes` limits the total size of archived files.

Example:

```go
config := &logicalfile.FileConfig{
  DstDir:        "./logs",
  Pattern:       "application.%i.log",
  ActiveFile:    "application.log",
  MaxTotalBytes: 4 * 500, // 4 files
  MaxBytes:      500,     // for single file
}
```

### MaxHistory

`MaxHistory` limits the number of archived files.

Example:

```go
config := &logicalfile.FileConfig{
  DstDir:     "./logs",
  Pattern:    "application.%i.log",
  ActiveFile: "application.log",
  MaxHistory: 3,
  MaxBytes:   500, // for single file
}
```

### Use all configs

If multiple retention limits are configured, an archived file is removed when any of the configured retention limits is exceeded:

`MaxAge exceeded` **OR** `MaxHistory exceeded` **OR** `MaxTotalBytes exceeded`

```go
config := &logicalfile.FileConfig{
  SrcDir:        "./src",
  DstDir:        "./dst",
  Pattern:       "application.%i.log",
  ActiveFile:    "application.log",
  MaxBytes:      500, // for single file
  MaxAge:        30 * time.Second,
  MaxHistory:    3,
  MaxTotalBytes: 4 * 500,
}
```

### Rotate at specific time

`RotateAt` can specify the time when the active file is rotated next time.

This example is the configuration to rotate files every 10 seconds.

```go
now := time.Now()

config := &logicalfile.FileConfig{
  DstDir:     "./logs",
  Pattern:    "application.%h-%m-%s.log",
  ActiveFile: "application.log",
  RotateAt: func() time.Time {
    next := now.Add(10 * time.Second)
    now = next
    return next
  },
}
```

### No rotations

If no rotation limits are configured, the physical file will not be rotated.

```go
// Create config with no physical file rotations.
config := &logicalfile.FileConfig{
  ActiveFile: "application.log",
}

f, err := logicalfile.NewFile(config)
if err != nil {
  panic(err)
}
defer f.Close()

// Use logical file for log output.
log.SetOutput(f)

for {
  log.Println("hello, gopher !!")
  time.Sleep(time.Second)
}
```

## Docs & Examples

- GoDoc: <https://pkg.go.dev/github.com/aileron-projects/go-logicalfile>
- Examples:
  - `MaxAge`: [examples/max-age/](./examples/max-age/)
  - `MaxHistory`: [examples/max-history/](./examples/max-history/)
  - `MaxTotalBytes`: [examples/max-total/](./examples/max-total/)
  - All limits: [examples/all/](./examples/all/)
  - Rotate at specific time: [examples/cron/](./examples/cron/)
  - No rotation: [examples/no-rotations/](./examples/no-rotations/)

## References

- [github.com/natefinch/lumberjack](https://github.com/natefinch/lumberjack)
- [github.com/DeRuina/timberjack](https://github.com/DeRuina/timberjack)
- [github.com/easyCZ/logrotate](https://github.com/easyCZ/logrotate)
- [github.com/lestrrat-go/file-rotatelogs](https://github.com/lestrrat-go/file-rotatelogs)
- [Rolling file - logback](https://logback.qos.ch/manual/appenders-rolling.html)
- [Rolling file appenders -log4j 2](https://logging.apache.org/log4j/2.x/manual/appenders/rolling-file.html)
