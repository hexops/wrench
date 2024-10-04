package scripts

import "strings"

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

var isPrivateRepo = calculateIsPrivateRepo()

func calculateAllRepos() []Repo {
	var all []Repo
	for _, category := range AllReposByCategory {
		all = append(all, category.Repos...)
	}
	return all
}

const privateRepoCategory = "Private"

func IsPrivateRepo(ownerSlashRepo string) bool {
	if strings.Contains(ownerSlashRepo, "http:") || strings.Contains(ownerSlashRepo, "https:") || strings.Contains(ownerSlashRepo, "github.com") {
		panic("illegal input: " + ownerSlashRepo)
	}
	if _, private := isPrivateRepo[ownerSlashRepo]; private {
		return true
	}
	return false
}

func calculateIsPrivateRepo() map[string]struct{} {
	private := map[string]struct{}{}
	for _, category := range AllReposByCategory {
		if category.Name != privateRepoCategory {
			continue
		}
		for _, repo := range category.Repos {
			private[repo.Name] = struct{}{}
		}
	}
	return private
}

type RepoCategory struct {
	Name  string
	Repos []Repo
}

var AllReposByCategory = []RepoCategory{
	{Name: "Primary repositories", Repos: []Repo{
		{Name: "hexops/mach", CI: Zig},
	}},
	{Name: "Standalone packages", Repos: []Repo{
		{Name: "hexops/mach-dxcompiler", CI: Zig},
		{Name: "hexops/mach-freetype", CI: Zig},
		{Name: "hexops/mach-objc", CI: Zig},
		{Name: "hexops/mach-opus", CI: Zig},
		{Name: "hexops/mach-flac", CI: Zig},
		{Name: "hexops/fastfilter", CI: Zig},
	}},
	{Name: "Zig-packaged C libraries", Repos: []Repo{
		{Name: "hexops/brotli", CI: Zig, Main: "master", HasUpdateVerifyScripts: true},
		{Name: "hexops/harfbuzz", CI: Zig, HasUpdateVerifyScripts: true},
		{Name: "hexops/freetype", CI: Zig, Main: "master", HasUpdateVerifyScripts: true},
		{Name: "hexops/wayland-headers", CI: Zig, HasUpdateVerifyScripts: true},
		{Name: "hexops/x11-headers", CI: Zig, HasUpdateVerifyScripts: true},
		{Name: "hexops/vulkan-headers", CI: Zig, HasUpdateVerifyScripts: true},
		{Name: "hexops/opengl-headers", CI: Zig, HasUpdateVerifyScripts: true},
		{Name: "hexops/linux-audio-headers", CI: Zig, HasUpdateVerifyScripts: true},
		{Name: "hexops/xcode-frameworks", CI: Zig, HasUpdateVerifyScripts: true},
		{Name: "hexops/spirv-tools", CI: Zig, Main: "main", HasUpdateVerifyScripts: true},
		{Name: "hexops/spirv-cross", CI: Zig, Main: "main", HasUpdateVerifyScripts: true},
		{Name: "hexops/DirectXShaderCompiler", CI: None, Main: "master"},
		{Name: "hexops/vulkan-zig-generated", CI: Zig, HasUpdateVerifyScripts: true},
		{Name: "hexops/direct3d-headers", CI: Zig},
		{Name: "hexops/directx-headers", CI: Zig},
		{Name: "hexops/opus", Main: "master", CI: Zig},
		{Name: "hexops/opusfile", Main: "master", CI: Zig},
		{Name: "hexops/opusenc", Main: "master", CI: Zig},
		{Name: "hexops/flac", Main: "master", CI: Zig},
		{Name: "hexops/ogg", Main: "master", CI: Zig},
	}},
	{Name: "Mach language bindings", Repos: []Repo{
		{Name: "hexops/mach-rs", CI: Todo},
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
		{Name: "hexops/mach-example-assets", CI: Zig},
		{Name: "hexops/font-assets", CI: Zig},
		{Name: "hexops/media", CI: None},
	}},
	{Name: privateRepoCategory, Repos: []Repo{
		{Name: "hexops/reignfields", CI: Zig},
		{Name: "hexops/reignfields-assets", CI: None},
	}},
}
