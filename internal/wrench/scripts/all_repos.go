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
	Name                   string
	CI                     CIType
	Main                   string
	HasUpdateVerifyScripts bool
}

var AllRepos = calculateAllRepos()

func calculateAllRepos() []Repo {
	var all []Repo
	for _, category := range AllReposByCategory {
		all = append(all, category.Repos...)
	}
	return all
}

type RepoCategory struct {
	Name  string
	Repos []Repo
}

var AllReposByCategory = []RepoCategory{
	{Name: "Primary repositories", Repos: []Repo{
		{Name: "hexops/mach", CI: Zig},
		{Name: "hexops/mach-core", CI: Zig},
		{Name: "hexops/mach-examples", CI: Zig},
	}},
	{Name: "Standalone packages", Repos: []Repo{
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
		{Name: "hexops/fastfilter", CI: Zig},
	}},
	{Name: "Zig-packaged C libraries", Repos: []Repo{
		{Name: "hexops/brotli", CI: Zig, Main: "master", HasUpdateVerifyScripts: true},
		{Name: "hexops/harfbuzz", CI: Zig, HasUpdateVerifyScripts: true},
		{Name: "hexops/freetype", CI: Zig, Main: "master", HasUpdateVerifyScripts: true},
		{Name: "hexops/wayland-headers", CI: Zig, HasUpdateVerifyScripts: true},
		{Name: "hexops/x11-headers", CI: Zig, HasUpdateVerifyScripts: true},
		{Name: "hexops/vulkan-headers", CI: Zig, HasUpdateVerifyScripts: true},
		{Name: "hexops/linux-audio-headers", CI: Zig, HasUpdateVerifyScripts: true},
		{Name: "hexops/glfw", CI: Zig, Main: "master", HasUpdateVerifyScripts: true},
		{Name: "hexops/basisu", CI: Zig, Main: "master", HasUpdateVerifyScripts: true},
		{Name: "hexops/dawn", CI: None},
		{Name: "hexops/DirectXShaderCompiler", CI: None, Main: "master"},
		{Name: "hexops/vulkan-zig-generated", CI: Zig, HasUpdateVerifyScripts: true},
	}},
	{Name: "Mach language bindings", Repos: []Repo{
		{Name: "hexops/mach-rs", CI: Todo},
	}},
	{Name: "Other examples", Repos: []Repo{
		{Name: "hexops/mach-glfw-vulkan-example", CI: Zig},
		{Name: "hexops/mach-glfw-opengl-example", CI: Zig},
	}},
	{Name: "Go projects", Repos: []Repo{
		{Name: "hexops/zgo", CI: Todo},
		{Name: "hexops/wrench", CI: Go},
	}},
	{Name: "Websites", Repos: []Repo{
		{Name: "hexops/machengine.org", CI: Hugo},
		{Name: "hexops/devlog", CI: Hugo},
		{Name: "hexops/hexops.com", CI: Hugo},
		{Name: "hexops/zigmonthly.org", CI: Hugo},
	}},
	{Name: "Misc", Repos: []Repo{
		{Name: "hexops/font-assets", CI: Todo},
		{Name: "hexops/media", CI: None},
	}},
	{Name: "System SDKs", Repos: []Repo{
		{Name: "hexops/sdk-linux-aarch64", CI: None},
		{Name: "hexops/sdk-linux-x86_64", CI: None},
		{Name: "hexops/sdk-windows-x86_64", CI: None},
		{Name: "hexops/sdk-macos-12.0", CI: None},
		{Name: "hexops/sdk-macos-11.3", CI: None},
	}},
}