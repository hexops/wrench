package zon

import (
	"bytes"
	"os"
	"testing"

	"github.com/hexops/autogold/v2"
)

func TestParse(t *testing.T) {
	input, err := os.ReadFile("build.zig.zon")
	if err != nil {
		t.Fatal(err)
	}
	tree, err := Parse(string(input))
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := tree.Write(&buf, "    ", ""); err != nil {
		t.Fatal(err)
	}
	autogold.ExpectFile(t, autogold.Raw(buf.String()))
}
