package zon

import (
	"bytes"
	"fmt"
	"os"
	"testing"

	"github.com/hexops/autogold/v2"
)

func TestParseComments(t *testing.T) {
	tree, err := Parse(`
// comment at the start

// and

// blank space!
`)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := tree.Write(&buf, "    ", ""); err != nil {
		t.Fatal(err)
	}
	autogold.Expect(`// comment at the start
// and
// blank space!

`).Equal(t, buf.String())
}

func TestParseStruct(t *testing.T) {
	tree, err := Parse(`
// comment at the start

// and

// blank space!
.{}
`)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := tree.Write(&buf, "    ", ""); err != nil {
		t.Fatal(err)
	}
	autogold.Expect(`// comment at the start
// and
// blank space!
.{}
`).Equal(t, buf.String())
}

func TestParseStructDotString(t *testing.T) {
	tree, err := Parse(`
// comment at the start

// and

// blank space!
.{
    .name = "foo",
}
`)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := tree.Write(&buf, "    ", ""); err != nil {
		t.Fatal(err)
	}
	autogold.Expect(`// comment at the start
// and
// blank space!
.{
    .name = "foo",
}
`).Equal(t, buf.String())
}

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
	fmt.Println(buf.String())
	autogold.ExpectFile(t, autogold.Raw(buf.String()))
}
