package dsnfile

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const fakeURL = "postgres://app:not-a-real-secret@db.invalid:5432/sakuraops"

func write(t *testing.T, content string, mode os.FileMode) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "url")
	if err := os.WriteFile(p, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
	// WriteFile is subject to umask; set the exact mode.
	if err := os.Chmod(p, mode); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestReadOK(t *testing.T) {
	t.Parallel()
	res, err := Read(write(t, "  "+fakeURL+"\n", 0o600))
	if err != nil {
		t.Fatal(err)
	}
	if res.URL != fakeURL || len(res.Warnings) != 0 {
		t.Fatalf("got %+v", res)
	}
}

func TestReadGroupReadableWarns(t *testing.T) {
	t.Parallel()
	res, err := Read(write(t, fakeURL, 0o640))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Warnings) != 1 {
		t.Fatalf("want one warning, got %v", res.Warnings)
	}
}

func TestReadRefusals(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		path func(t *testing.T) string
		want error
	}{
		{"world readable", func(t *testing.T) string { return write(t, fakeURL, 0o644) }, ErrTooOpen},
		{"group writable", func(t *testing.T) string { return write(t, fakeURL, 0o660) }, ErrTooOpen},
		{"world writable", func(t *testing.T) string { return write(t, fakeURL, 0o602) }, ErrTooOpen},
		{"empty", func(t *testing.T) string { return write(t, "", 0o600) }, ErrEmpty},
		{"whitespace only", func(t *testing.T) string { return write(t, " \n\t\n", 0o600) }, ErrEmpty},
		{"too large", func(t *testing.T) string { return write(t, strings.Repeat("x", maxSize+1), 0o600) }, ErrTooLarge},
		{"directory", func(t *testing.T) string { return t.TempDir() }, ErrNotRegular},
		{"missing", func(t *testing.T) string { return filepath.Join(t.TempDir(), "nope") }, os.ErrNotExist},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := Read(tc.path(t))
			if !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
		})
	}
	if _, err := Read(""); err == nil {
		t.Fatal("empty path must fail")
	}
}

func TestErrorsNeverContainContent(t *testing.T) {
	t.Parallel()
	for _, mode := range []os.FileMode{0o644, 0o600} {
		content := fakeURL
		if mode == 0o600 {
			content = strings.Repeat("y", maxSize+10) + "not-a-real-secret"
		}
		_, err := Read(write(t, content, mode))
		if err == nil {
			t.Fatal("expected error")
		}
		if strings.Contains(err.Error(), "not-a-real-secret") {
			t.Fatalf("error leaks content: %v", err)
		}
	}
}
