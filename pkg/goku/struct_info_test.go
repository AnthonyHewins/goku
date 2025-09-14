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
	stop := 1
	for i := 0; i < stop; i++ {
		arg, err := files.ReadFile(fmt.Sprintf("testdata/gen_interface/%d-arg.go", i))
		if err != nil {
			t.Fatalf("test file unreadable %s", err)
		}

		want, err := files.ReadFile(fmt.Sprintf("testdata/gen_interface/%d-expected.go", i))
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

		got := b.String()
		if strings.TrimSpace(got) == strings.TrimSpace(string(want)) {
			continue
		}

		t.Errorf("failed test, want\n%s\ngot\n%s", want, got)
	}
}
