// Package dsnfile reads a database URL from a file. The URL carries the database
// password, so it is never accepted on a command line and never appears in an
// error message.
package dsnfile

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// maxSize bounds how much of the file is read; a URL is far smaller.
const maxSize = 4096

// Sentinel errors, matched with errors.Is. Messages never include file content.
var (
	ErrNotRegular = errors.New("database URL file is not a regular file")
	ErrTooOpen    = errors.New("database URL file permissions are too open (allowed: 0600 or 0640)")
	ErrEmpty      = errors.New("database URL file is empty")
	ErrTooLarge   = errors.New("database URL file is too large")
)

// Result is the file content plus non-fatal findings the caller should log.
type Result struct {
	URL      string
	Warnings []string
}

// Read opens path, verifies it and returns the trimmed URL.
//
// Policy: refuse unless the mode is a subset of 0640 (no write for group, no
// access at all for others). Group read (0640) is accepted with a warning, since
// container secret mounts commonly need it; 0600 is the recommendation. The
// checks run on the opened descriptor, so they cannot be raced by swapping the
// path after the check.
func Read(path string) (Result, error) {
	if path == "" {
		return Result{}, errors.New("database URL file path is empty")
	}
	f, err := os.Open(path)
	if err != nil {
		// The *PathError text contains only the path and errno, no content.
		return Result{}, fmt.Errorf("open database URL file: %w", err)
	}
	defer f.Close()

	st, err := f.Stat()
	if err != nil {
		return Result{}, fmt.Errorf("stat database URL file: %w", err)
	}
	if !st.Mode().IsRegular() {
		return Result{}, ErrNotRegular
	}
	perm := st.Mode().Perm()
	if perm&^0o640 != 0 {
		return Result{}, fmt.Errorf("%w: mode is %04o", ErrTooOpen, perm)
	}
	var res Result
	if perm&0o040 != 0 {
		res.Warnings = append(res.Warnings, fmt.Sprintf("database URL file is group-readable (mode %04o); 0600 is recommended", perm))
	}

	b, err := io.ReadAll(io.LimitReader(f, maxSize+1))
	if err != nil {
		return Result{}, fmt.Errorf("read database URL file: %w", err)
	}
	if len(b) > maxSize {
		return Result{}, ErrTooLarge
	}
	res.URL = strings.TrimSpace(string(b))
	if res.URL == "" {
		return Result{}, ErrEmpty
	}
	return res, nil
}
