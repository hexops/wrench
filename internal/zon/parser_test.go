package zon

import (
	"bytes"
	"testing"

	"github.com/hexops/autogold/v2"
)

func TestParse(t *testing.T) {
	tree, err := Parse(build_zig_zon)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := tree.Write(&buf, "    ", ""); err != nil {
		t.Fatal(err)
	}
	autogold.Expect(`.{
    .name = "mach",
    .version = "0.2.0",
    .dependencies = .{
        .mach_ecs = .{
            .url = "https://github.com/hexops/mach-ecs/archive/3986581a241960babe1682ac5d5132be602cef38.tar.gz",
            .hash = "1220949ae605cc45e2d84cd0522a27ea09be0f93df13e7babba06c4ec349bf4afe0a",
        },
        .mach_earcut = .{
            .url = "https://github.com/hexops/mach-earcut/archive/5a34772313a6a0679cc6c83f310a59741d03c246.tar.gz",
            .hash = "1220c7059f62cf479e1c3de773cddb71aeb5824ac74528392cd38715f586d7a52e2f",
        },
    },
}`).Equal(t, buf.String())
}

const build_zig_zon = `.{
    .name = "mach",
    .version = "0.2.0",
    .dependencies = .{
        .mach_ecs = .{
            .url = "https://github.com/hexops/mach-ecs/archive/3986581a241960babe1682ac5d5132be602cef38.tar.gz",
            .hash = "1220949ae605cc45e2d84cd0522a27ea09be0f93df13e7babba06c4ec349bf4afe0a",
        },
        .mach_earcut = .{
            .url = "https://github.com/hexops/mach-earcut/archive/5a34772313a6a0679cc6c83f310a59741d03c246.tar.gz",
            .hash = "1220c7059f62cf479e1c3de773cddb71aeb5824ac74528392cd38715f586d7a52e2f",
        },
    },
}
`
