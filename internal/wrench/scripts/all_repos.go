package scripts

type Repo struct {
	Name  string
	ZigCI bool
}

var AllRepos = []Repo{
	// Critical repositories
	{Name: "hexops/mach", ZigCI: true},
	{Name: "hexops/mach-core", ZigCI: true},
	{Name: "hexops/mach-examples", ZigCI: true},

	// Mach packages
	{Name: "hexops/mach-gpu", ZigCI: true},
	{Name: "hexops/mach-gpu-dawn", ZigCI: true},
	{Name: "hexops/mach-basisu", ZigCI: true},
	{Name: "hexops/mach-freetype", ZigCI: true},
	{Name: "hexops/mach-glfw", ZigCI: true},
	{Name: "hexops/mach-ecs", ZigCI: true},
	{Name: "hexops/mach-dusk", ZigCI: true},
	{Name: "hexops/mach-earcut", ZigCI: true},
	{Name: "hexops/mach-gamemode", ZigCI: true},
	{Name: "hexops/mach-model3d", ZigCI: true},
	{Name: "hexops/mach-sysjs", ZigCI: true},
	{Name: "hexops/mach-sysaudio", ZigCI: true},

	// Zig-packaged C libraries
	{Name: "hexops/brotli"},
	{Name: "hexops/harfbuzz"},
	{Name: "hexops/freetype"},
	{Name: "hexops/wayland-headers"},
	{Name: "hexops/x11-headers"},
	{Name: "hexops/vulkan-headers"},
	{Name: "hexops/linux-audio-headers"},
	{Name: "hexops/glfw"},
	{Name: "hexops/basisu"},
	{Name: "hexops/dawn"},
	{Name: "hexops/DirectXShaderCompiler"},

	// Language bindings for Mach
	{Name: "hexops/mach-rs"},

	// Examples
	{Name: "hexops/mach-glfw-vulkan-example", ZigCI: true},
	{Name: "hexops/mach-glfw-opengl-example", ZigCI: true},

	// Other useful libraries/tools
	{Name: "hexops/fastfilter", ZigCI: true},
	{Name: "hexops/zgo"},

	// Misc
	{Name: "hexops/mach-example-assets"},
	{Name: "hexops/font-assets"},
	{Name: "hexops/machengine.org"},
	{Name: "hexops/devlog"},
	{Name: "hexops/hexops.com"},
	{Name: "hexops/zigmonthly.org"},
	{Name: "hexops/wrench"},
	{Name: "hexops/media"},

	// Going away soon
	{Name: "hexops/sdk-linux-aarch64"},
	{Name: "hexops/sdk-linux-x86_64"},
	{Name: "hexops/sdk-windows-x86_64"},
	{Name: "hexops/sdk-macos-12.0"},
	{Name: "hexops/sdk-macos-11.3"},
}
