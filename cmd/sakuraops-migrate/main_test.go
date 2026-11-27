package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func noEnv(string) string { return "" }

func runCLI(t *testing.T, env func(string) string, args ...string) (int, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := run(args, env, &out, &errb)
	return code, out.String() + errb.String()
}

func TestUsageErrors(t *testing.T) {
	t.Parallel()
	for name, args := range map[string][]string{
		"no command":      {},
		"two commands":    {"up", "status"},
		"down is absent":  {"down"},
		"unknown":         {"drop"},
		"bad flag":        {"-nope", "up"},
		"zero timeout":    {"-timeout", "0s", "up"},
		"negative":        {"-timeout", "-1s", "up"},
		"flag after verb": {"up", "-database-url-file", "x"},
	} {
		if code, _ := runCLI(t, noEnv, args...); code != exitUsage {
			t.Errorf("%s: exit %d, want %d", name, code, exitUsage)
		}
	}
}

func TestConfigErrors(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	write := func(name, content string, mode os.FileMode) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(content), mode); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(p, mode); err != nil {
			t.Fatal(err)
		}
		return p
	}
	const secret = "s3cretpassword"
	cases := map[string]struct {
		args []string
		env  func(string) string
	}{
		"no file configured": {[]string{"up"}, noEnv},
		"missing file":       {[]string{"-database-url-file", filepath.Join(dir, "missing"), "up"}, noEnv},
		"empty file":         {[]string{"-database-url-file", write("empty", "", 0o600), "up"}, noEnv},
		"world readable":     {[]string{"-database-url-file", write("open", "postgres://u:"+secret+"@h/d", 0o644), "up"}, noEnv},
		"garbage URL":        {[]string{"-database-url-file", write("bad", "postgres://u:"+secret+"@h:notaport/d", 0o600), "up"}, noEnv},
		"via environment": {[]string{"status"}, func(k string) string {
			if k == envURLFile {
				return write("envbad", "postgres://u:"+secret+"@h:notaport/d", 0o600)
			}
			return ""
		}},
	}
	for name, tc := range cases {
		code, out := runCLI(t, tc.env, tc.args...)
		if code != exitConfig {
			t.Errorf("%s: exit %d, want %d (output %q)", name, code, exitConfig, out)
		}
		if strings.Contains(out, secret) {
			t.Errorf("%s: output leaks the password", name)
		}
	}
}

func TestConnectionClassHidesServerMessage(t *testing.T) {
	t.Parallel()
	for _, code := range []string{"28P01", "28000", "3D000", "08006", "57P01", "", "2"} {
		if !connectionClass(code) {
			t.Errorf("%q must hide the server message", code)
		}
	}
	for _, code := range []string{"42501", "23505", "42P07", "XX000"} {
		if connectionClass(code) {
			t.Errorf("%q must keep the server message", code)
		}
	}
}
