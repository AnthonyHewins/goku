package goku

import (
	"bytes"
	"embed"
	"fmt"
	"strings"
	"testing"
)

//go:embed testdata
var files embed.FS

func TestGenInterface(t *testing.T) {
	for i := range 1 {
		arg, err := files.ReadFile(fmt.Sprintf("testdata/gen_interface/%d-arg.go", i))
		if err != nil {
			t.Fatalf("test file unreadable %s", err)
		}

		want, err := files.ReadFile(fmt.Sprintf("testdata/gen_interface/%d-expected.txt", i))
		if err != nil {
			t.Fatalf("test file unreadable %s", err)
		}

		i := NewStructInfoGen("Target")
		if err = i.AddSrc(string(arg)); err != nil {
			t.Errorf("should not err on generating source %s", err)
			return
		}

		x, err := i.StructInfo()
		if err != nil {
			t.Errorf("should not err on gen interface test %s", err)
			continue
		}

		var b bytes.Buffer
		if err = x.GenInterface(&b, "TargetInterface", GenMock("Mock"), IncludePrivate(), OverridePkg("override")); err != nil {
			t.Errorf("should not err on gen interface test %s", err)
			continue
		}

		got := strings.Split(strings.TrimSpace(b.String()), "\n")
		wanted := strings.Split(strings.TrimSpace(string(want)), "\n")

		if x, y := len(got), len(wanted); x != y {
			t.Errorf("failed due to line count %d != %d, want\n%s\ngot\n%s", x, y, want, got)
			return
		}

		for i, v := range wanted {
			if strings.TrimSpace(v) == strings.TrimSpace(got[i]) {
				continue
			}

			t.Errorf("failed at line %d:\n-%s\n+%s\n\nWant:%s\nGot:%s", i+1, v, got[i], strings.Join(wanted[:i+1], "\n"), strings.Join(got[:i+1], "\n"))
		}
	}
}
