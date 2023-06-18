package scripts

type CIType string

const (
	None CIType = "none"
	Zig  CIType = "zig"
	Todo CIType = "todo"
	Hugo CIType = "hugo"
	Go   CIType = "go"
)

type Repo struct {
	Name string
	CI   CIType
}

var AllRepos = []Repo{
	// Critical repositories
	{Name: "hexops/mach", CI: Zig},
	{Name: "hexops/mach-core", CI: Zig},
	{Name: "hexops/mach-examples", CI: Zig},

	// Mach packages
	{Name: "hexops/mach-gpu", CI: Zig},
	{Name: "hexops/mach-gpu-dawn", CI: Zig},
	{Name: "hexops/mach-basisu", CI: Zig},
	{Name: "hexops/mach-freetype", CI: Zig},
	{Name: "hexops/mach-glfw", CI: Zig},
	{Name: "hexops/mach-ecs", CI: Todo},
	{Name: "hexops/mach-dusk", CI: Zig},
	{Name: "hexops/mach-earcut", CI: Zig},
	{Name: "hexops/mach-gamemode", CI: Todo},
	{Name: "hexops/mach-model3d", CI: Todo},
	{Name: "hexops/mach-sysjs", CI: Todo},
	{Name: "hexops/mach-sysaudio", CI: Todo},
	{Name: "hexops/mach-ggml", CI: Zig},

	// Zig-packaged C libraries
	{Name: "hexops/brotli", CI: None},
	{Name: "hexops/harfbuzz", CI: None},
	{Name: "hexops/freetype", CI: None},
	{Name: "hexops/wayland-headers", CI: None},
	{Name: "hexops/x11-headers", CI: None},
	{Name: "hexops/vulkan-headers", CI: None},
	{Name: "hexops/linux-audio-headers", CI: None},
	{Name: "hexops/glfw", CI: None},
	{Name: "hexops/basisu", CI: None},
	{Name: "hexops/dawn", CI: None},
	{Name: "hexops/DirectXShaderCompiler", CI: None},

	// Language bindings for Mach
	{Name: "hexops/mach-rs", CI: Todo},

	// Examples
	{Name: "hexops/mach-glfw-vulkan-example", CI: Zig},
	{Name: "hexops/mach-glfw-opengl-example", CI: Zig},

	// Other useful libraries
	{Name: "hexops/fastfilter", CI: Zig},

	// Go projects
	{Name: "hexops/zgo", CI: Todo},
	{Name: "hexops/wrench", CI: Go},

	// Hugo projects
	{Name: "hexops/machengine.org", CI: Hugo},
	{Name: "hexops/devlog", CI: Hugo},
	{Name: "hexops/hexops.com", CI: Hugo},
	{Name: "hexops/zigmonthly.org", CI: Hugo},

	// Misc
	{Name: "hexops/mach-example-assets", CI: Todo},
	{Name: "hexops/font-assets", CI: Todo},
	{Name: "hexops/media", CI: None},

	// Going away soon
	{Name: "hexops/sdk-linux-aarch64", CI: None},
	{Name: "hexops/sdk-linux-x86_64", CI: None},
	{Name: "hexops/sdk-windows-x86_64", CI: None},
	{Name: "hexops/sdk-macos-12.0", CI: None},
	{Name: "hexops/sdk-macos-11.3", CI: None},
}
