package wrench

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/hexops/wrench/internal/errors"
	"github.com/hexops/wrench/internal/wrench/scripts"
	orderedmap "github.com/wk8/go-ordered-map/v2"
)

func (b *Bot) httpMuxPkgProxy(handler func(prefix string, handle handlerFunc) http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if b.Config.ModeType() == ModeZig {
			if r.URL.Path == "/" {
				handler("general", b.httpPkgZigRoot).ServeHTTP(w, r)
				return
			}
			if strings.HasPrefix(r.URL.Path, "/zig") {
				handler("zig", b.httpPkgZig).ServeHTTP(w, r)
				return
			}
			w.WriteHeader(http.StatusNotFound)
			return
		}

		// pkg mode
		if r.URL.Path == "/" {
			handler("general", b.httpPkgRoot).ServeHTTP(w, r)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/zig") {
			handler("zig", b.httpPkgZig).ServeHTTP(w, r)
			return
		}
		split := strings.Split(r.URL.Path, "/")
		if len(split) == 3 {
			// https://pkg.machengine.org/<project>/<file>
			handler("pkg", b.httpPkgPkg).ServeHTTP(w, r)
			return
		}
		if len(split) == 5 {
			// https://pkg.machengine.org/<project>/artifact/<version>/<file>
			handler("artifact", b.httpPkgArtifact).ServeHTTP(w, r)
			return
		}
		handler("pkg", b.httpPkgPkg).ServeHTTP(w, r)
	})

	// Every minute attempt to warm the cache with Zig versions we do not have. Note that this
	// is internally cached heavily (doesn't make requests to ziglang.org except every 15m) due to
	// index file caching.
	go func() {
		for {
			if err := b.httpPkgZigWarmCache(); err != nil {
				b.logf("failed to warm zig download cache: %s", err)
			}
			time.Sleep(1 * time.Minute)
		}
	}()
	return mux
}

func (b *Bot) httpPkgZigRoot(w http.ResponseWriter, r *http.Request) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	fmt.Fprintf(w, `
<h1 style="margin-bottom: 0.25rem;">Zig download mirror</h1>

<div>

<p>This site acts as a mirror of <a href="https://ziglang.org/download">ziglang.org/download</a></p>
<p>The rewrite logic is as follows:</p>
<div style="margin-left: 1rem;">
	<pre>
<strong>https://ziglang.org/builds/$FILE</strong> -> <strong>%s/zig/$FILE</strong>
</pre>
</div>
<p>Note: .tar.gz, .zip, and .minisig signatures are available for download. Signatures can also be downloaded from ziglang.org for verification purposes.</p>

</div>

<p>~ <a href="https://github.com/hexops/wrench">Wrench</a>.</p>
`, b.Config.ExternalURL)
	return nil
}
func (b *Bot) httpPkgRoot(w http.ResponseWriter, r *http.Request) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, "%s", `
<h1 style="margin-bottom: 0.25rem;">pkg.machengine.org</h1>
<strong style="font-weight: 16px; margin-top: 0; margin-left: 1rem;"><em>The <a href="https://machengine.org">Mach</a> package download server</em></strong>

<div style="margin-left: 2rem;">

<h3 style="margin-top: 2rem;">Zig downloads</h3>
<p>This site acts as a mirror of <a href="https://ziglang.org/download">ziglang.org/download</a></p>
<div style="margin-left: 1rem;">
	<p>The rewrite logic is as follows:</p>
	<pre>
<strong>https://ziglang.org/builds/$FILE</strong> -> <strong>https://pkg.machengine.org/zig/$FILE</strong>
</pre>
</div>
<p>Note: .tar.gz, .zip, and .minisig signatures are available for download. Signatures can also be downloaded from ziglang.org for verification purposes.</p>

<h3 style="margin-top: 2rem;">Mach downloads</h3>
<p>This site serves Zig packages for all <a href="https://wrench.machengine.org/projects/">Mach projects</a>.</p>
<div style="margin-left: 1rem;">
	<p>The rewrite logic is as follows:</p>
	<pre>
<strong>https://github.com/hexops/$PROJECT/archive/$FILE</strong> -> <strong>https://pkg.machengine.org/$PROJECT/$FILE</strong>
</pre>
</div>
<p>As well as binary release artifacts for some projects, built via our CI pipelines.</p>
<div style="margin-left: 1rem;">
	<p>The rewrite logic is as follows:</p>
	<pre>
<strong>https://github.com/hexops/$PROJECT/releases/download/$VERSION/$FILE</strong> -> <strong>https://pkg.machengine.org/$PROJECT/artifact/$VERSION/$FILE</strong>
</pre>
</div>

<h3 style="margin-top: 2rem;">Contact</h3>
<ul>
	<li><a href="https://github.com/hexops/mach/issues?q=is%3Aopen+is%3Aissue+label%3Awrench">Issue tracker</a></li>
	<li><a href="https://discord.gg/XNG3NZgCqp">Mach discord</a></li>
	<li><a href="https://github.com/hexops/wrench">Wrench source on GitHub</a></li>
</ul>

</div>
`)
	return nil
}

var (
	zigVersionRegexp = regexp.MustCompile(`(\d+\.\d+\.\d+(-dev\.\d+\+[[:alnum:]]+)?)`)

	// From semver.org
	semverRegexp = regexp.MustCompile(`^(?P<major>0|[1-9]\d*)\.(?P<minor>0|[1-9]\d*)\.(?P<patch>0|[1-9]\d*)(?:-(?P<prerelease>(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?(?:\+(?P<buildmetadata>[0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?$`)
)

// https://pkg.machengine.org/zig/<file>
// -> https://ziglang.org/builds/<file>
func (b *Bot) httpPkgZig(w http.ResponseWriter, r *http.Request) error {
	// example URLs:
	//
	// https://pkg.machengine.org/zig/0.13.0/zig-0.13.0.tar.xz
	// https://pkg.machengine.org/zig/zig-0.14.0-dev.2+0884a4341.tar.xz
	// https://pkg.machengine.org/zig/index.json
	split := strings.Split(r.URL.Path, "/")
	if len(split) == 3 {
		// https://pkg.machengine.org/zig/zig-0.14.0-dev.2+0884a4341.tar.xz
		// ["" "zig" "zig-0.14.0-dev.2+0884a4341.tar.xz"]
	} else if len(split) == 4 {
		// https://pkg.machengine.org/zig/0.13.0/zig-0.13.0.tar.xz
		// ["" "zig" "0.13.0" "zig-0.13.0.tar.xz"]
		if !semverRegexp.MatchString(split[2]) {
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintf(w, "invalid /zig/<version>/file path - version is not semver\n")
			return nil
		}
	} else {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "invalid path\n")
		return nil
	}

	fname := split[len(split)-1]
	if fname == "index.json" {
		return b.httpPkgZigIndex(w, r)
	}

	// ensure the filename is not malicious in some way
	// valid filenames look like e.g.:
	//
	// zig-0.13.0.tar.xz
	// zig-0.14.0-dev.2+0884a4341.tar.xz
	// zig-bootstrap-0.14.0-dev.2+0884a4341.tar.xz
	// zig-macos-x86_64-0.14.0-dev.2+0884a4341.tar.xz
	// zig-windows-aarch64-0.14.0-dev.2+0884a4341.zip
	if fname != path.Clean(fname) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "invalid path: %q != %q\n", fname, path.Clean(fname))
		return nil
	}
	if path.Ext(fname) == "" {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "invalid extension\n")
		return nil
	}

	// Ensure we can parse the Zig version string from the filename
	version := zigVersionRegexp.FindString(fname)
	if version == "" {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "invalid Zig version: %q\n", version)
		return nil
	}

	indexFile, err := b.httpPkgZigIndexCached()
	if err != nil {
		return errors.Wrap(err, "httpPkgZigIndexCached")
	}
	var index map[string]map[string]any
	if err := json.Unmarshal(indexFile, &index); err != nil {
		return errors.Wrap(err, "unmarshalling index.json")
	}
	versionKind, err := b.zigVersionKind(version, index)
	if err != nil {
		return errors.Wrap(err, "unmarshalling index.json")
	}

	dirPath := path.Join("cache/zig/", versionKind, version)
	filePath := path.Join(dirPath, fname)
	serveCacheHit := func() error {
		w.Header().Set("cache-control", "public, max-age=31536000, immutable")
		f, err := os.Open(filePath)
		if err != nil {
			return err
		}
		defer f.Close()
		fi, err := f.Stat()
		if err != nil {
			return err
		}

		b.idLogf("zig", "serve %s", filePath)
		http.ServeContent(w, r, fname, fi.ModTime(), f)
		return nil
	}
	if _, err := os.Stat(filePath); err == nil {
		return serveCacheHit()
	}

	if err := b.httpPkgEnsureZigDownloadCached(version, versionKind, fname); err != nil {
		if !strings.Contains(err.Error(), "ignored") {
			b.idLogf("zig", "error downloading file: %s", err)
		}
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "unable to fetch\n")
		return nil
	}

	go func() {
		b.httpPkgEnsureZigVersionCached(version, versionKind)
	}()
	return serveCacheHit()
}

func (b *Bot) zigVersionKind(version string, index map[string]map[string]any) (string, error) {
	if !strings.Contains(version, "-dev") {
		return "stable", nil
	}
	for indexVersion := range index {
		if !strings.Contains(indexVersion, "mach") {
			continue
		}
		// indexVersion is a mach-nominated version, e.g. "mach-latest", "2024.5.0-mach"
		info := index[indexVersion]
		nominatedZigVersion := info["version"].(string)
		if version == nominatedZigVersion {
			return "mach", nil
		}
	}
	return "dev", nil
}

func (b *Bot) httpPkgZigWarmCache() error {
	// Grab the latest index.json that we are aware of (cached in-memory for 15m)
	indexFile, err := b.httpPkgZigIndexCached()
	if err != nil {
		return errors.Wrap(err, "httpPkgZigIndexCached")
	}
	var index map[string]map[string]any
	if err := json.Unmarshal(indexFile, &index); err != nil {
		return errors.Wrap(err, "unmarshalling index.json")
	}
	for indexVersion, v := range index {
		ignored := map[string]struct{}{
			// Versions that nobody cares about pre-caching.
			"0.1.1":      struct{}{},
			"0.2.0":      struct{}{},
			"0.3.0":      struct{}{},
			"0.4.0":      struct{}{},
			"0.5.0":      struct{}{},
			"0.6.0":      struct{}{},
			"0.7.0":      struct{}{},
			"0.7.1":      struct{}{},
			"0.8.0":      struct{}{},
			"0.8.1":      struct{}{},
			"0.9.0":      struct{}{},
			"0.9.1":      struct{}{},
			"0.10.0":     struct{}{},
			"0.10.1":     struct{}{},
			"0.3.0-mach": struct{}{},

			// Do not warm the cache with master Zig versions, as these would fill the disk quickly.
			"master": struct{}{},
		}
		if _, ignore := ignored[indexVersion]; ignore {
			continue
		}

		version := indexVersion
		if aliasVersion, ok := v["version"]; ok {
			version = aliasVersion.(string)
		}
		versionKind, err := b.zigVersionKind(version, index)
		if err != nil {
			return errors.Wrap(err, "unmarshalling index.json")
		}

		b.httpPkgEnsureZigVersionCached(version, versionKind)
	}
	return nil
}

func (b *Bot) httpPkgEnsureZigVersionCached(version, versionKind string) {
	for _, tmpl := range []string{
		"zig-$VERSION.tar.xz",
		"zig-bootstrap-$VERSION.tar.xz",
		"zig-windows-x86_64-$VERSION.zip",
		"zig-windows-x86-$VERSION.zip",
		"zig-windows-aarch64-$VERSION.zip",
		"zig-macos-aarch64-$VERSION.tar.xz",
		"zig-macos-x86_64-$VERSION.tar.xz",
		"zig-linux-x86_64-$VERSION.tar.xz",
		"zig-linux-x86-$VERSION.tar.xz",
		"zig-linux-aarch64-$VERSION.tar.xz",
		"zig-linux-armv7a-$VERSION.tar.xz",
		"zig-linux-riscv64-$VERSION.tar.xz",
		"zig-linux-powerpc64le-$VERSION.tar.xz",
	} {
		for _, sub := range []string{"", ".minisig"} {
			fname := strings.Replace(tmpl+sub, "$VERSION", version, 1)
			if err := b.httpPkgEnsureZigDownloadCached(version, versionKind, fname); err != nil {
				if !strings.Contains(err.Error(), "ignored") {
					b.idLogf("zig", "error downloading file: %s", err)

				}
				continue
			}
		}
	}
}

var (
	cachedResponsesMu sync.Mutex
	cachedResponses   = map[string]error{}
	fsParallelismLock sync.Mutex
)

func (b *Bot) httpPkgEnsureZigDownloadCached(version, versionKind, fname string) error {
	dirPath := path.Join("cache/zig/", versionKind, version)
	filePath := path.Join(dirPath, fname)
	if _, err := os.Stat(filePath); err == nil {
		return nil
	}

	url := ""
	if versionKind == "mach" {
		url = "https://pkg.machengine.org" + path.Join("/zig/", fname)
	} else if versionKind == "stable" {
		url = "https://ziglang.org" + path.Join("/download/", version, fname)
	} else {
		url = "https://ziglang.org" + path.Join("/builds/", fname)
	}

	// URLs that we know do not exist
	ignored := map[string]struct{}{}
	// minisig files for these two versions no longer exist anywhere.
	for _, version := range []string{"0.12.0-dev.2063+804cee3b9", "0.12.0-dev.3180+83e578a18"} {
		for _, tmpl := range []string{
			"zig-$VERSION.tar.xz",
			"zig-bootstrap-$VERSION.tar.xz",
			"zig-windows-x86_64-$VERSION.zip",
			"zig-windows-x86-$VERSION.zip",
			"zig-windows-aarch64-$VERSION.zip",
			"zig-macos-aarch64-$VERSION.tar.xz",
			"zig-macos-x86_64-$VERSION.tar.xz",
			"zig-linux-x86_64-$VERSION.tar.xz",
			"zig-linux-x86-$VERSION.tar.xz",
			"zig-linux-aarch64-$VERSION.tar.xz",
			"zig-linux-armv7a-$VERSION.tar.xz",
			"zig-linux-riscv64-$VERSION.tar.xz",
			"zig-linux-powerpc64le-$VERSION.tar.xz",
		} {
			for _, sub := range []string{".minisig"} {
				fname := strings.Replace(tmpl+sub, "$VERSION", version, 1)
				ignored["https://pkg.machengine.org/zig/"+fname] = struct{}{}
			}
		}
	}

	if _, ignore := ignored[url]; ignore {
		return errors.New("ignored")
	}

	logWriter := b.idWriter("zig")

	cachedResponsesMu.Lock()
	cachedError, isCachedError := cachedResponses[url]
	cachedResponsesMu.Unlock()
	if isCachedError {
		fmt.Fprintf(logWriter, "not fetching: %s (cached error %s)\n", url, cachedError)
		return cachedError
	}
	fmt.Fprintf(logWriter, "fetch: %s > %s\n", url, filePath)

	resp, err := httpGet(url, 60*time.Second)
	if err != nil {
		return errors.Wrap(err, "Get")
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 && resp.StatusCode <= 500 {
		// 404 not found, 403 forbidden, etc.
		err := fmt.Errorf("bad response status: %s", resp.Status)
		cachedResponsesMu.Lock()
		cachedResponses[url] = err
		cachedResponsesMu.Unlock()
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad response status: %s", resp.Status)
	}

	// Two goroutines may have fetched this file at the same time (assuming neither hit the cache,
	// new file download) - which is fine - but we can't have them write to disk at the same time.
	fsParallelismLock.Lock()
	defer fsParallelismLock.Unlock()

	if err := os.MkdirAll(dirPath, os.ModePerm); err != nil {
		return errors.Wrap(err, "MkdirAll "+dirPath)
	}
	out, err := os.Create(filePath)
	if err != nil {
		return errors.Wrap(err, "Create "+filePath)
	}
	defer out.Close()
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return errors.Wrap(err, "Copy")
	}
	return nil
}

// https://pkg.machengine.org/<project>/<file>
// -> https://github.com/hexops/<project>/archive/<file>
//
// e.g. https://pkg.machengine.org/mach-ecs/83a3ed801008a976dd79e10068157b02c3b76a36.tar.gz
func (b *Bot) httpPkgPkg(w http.ResponseWriter, r *http.Request) error {
	split := strings.Split(r.URL.Path, "/")
	if len(split) != 3 {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "invalid path\n")
		return nil
	}
	project, fname := split[1], split[2]
	if project != path.Clean(project) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "illegal project name\n")
		return nil
	}
	if fname != path.Clean(fname) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "illegal file name\n")
		return nil
	}
	if !strings.HasSuffix(fname, ".tar.gz") {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "illegal file extension\n")
		return nil
	}
	validate := strings.TrimSuffix(fname, ".tar.gz")
	isCommitHash := len(validate) == 40
	isVersion := strings.HasPrefix(validate, "v") && semverRegexp.MatchString(strings.TrimPrefix(validate, "v"))
	if !isCommitHash && !isVersion {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "filename must be <commit hash>.tar.gz or <semver>.tar.gz\n")
		return nil
	}

	cachePath := path.Join("cache/pkg/", project, fname)
	serveCacheHit := func() error {
		w.Header().Set("cache-control", "public, max-age=31536000, immutable")
		f, err := os.Open(cachePath)
		if err != nil {
			return err
		}
		defer f.Close()
		fi, err := f.Stat()
		if err != nil {
			return err
		}

		b.idLogf("pkg", "serve %s", cachePath)
		http.ServeContent(w, r, fname, fi.ModTime(), f)
		return nil
	}
	if _, err := os.Stat(cachePath); err == nil {
		return serveCacheHit()
	}

	u := &url.URL{
		Scheme: "https",
		Host:   "github.com",
		Path:   path.Join("hexops", project, "archive", fname),
	}
	url := u.String()
	logWriter := b.idWriter("pkg")
	_ = os.MkdirAll(path.Join("cache/pkg/", project), os.ModePerm)
	if err := scripts.DownloadFile(url, cachePath)(logWriter); err != nil {
		b.idLogf("pkg", "error downloading file: %s url=%s", err, url)
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "unable to fetch\n")
		return nil
	}
	return serveCacheHit()
}

// https://pkg.machengine.org/<project>/artifact/<version>/<file>
// -> https://github.com/hexops/<project>/releases/download/<version>/<file>
//
// e.g. https://pkg.machengine.org/mach-dxcompiler/artifact/2024.02.10+4ccd240.1/aarch64-linux-gnu_ReleaseFast_lib.tar.zst
func (b *Bot) httpPkgArtifact(w http.ResponseWriter, r *http.Request) error {
	split := strings.Split(r.URL.Path, "/")
	if len(split) != 5 {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "invalid path, found %v elements expected 5\n", len(split))
		return nil
	}
	project, version, fname := split[1], split[3], split[4]
	if project != path.Clean(project) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "illegal project name\n")
		return nil
	}
	if version != path.Clean(version) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "illegal version\n")
		return nil
	}
	if fname != path.Clean(fname) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "illegal file name\n")
		return nil
	}

	cachePath := path.Join("cache/pkg-artifact/", project, version, fname)
	serveCacheHit := func() error {
		w.Header().Set("cache-control", "public, max-age=31536000, immutable")
		f, err := os.Open(cachePath)
		if err != nil {
			return err
		}
		defer f.Close()
		fi, err := f.Stat()
		if err != nil {
			return err
		}

		b.idLogf("artifact", "serve %s", cachePath)
		http.ServeContent(w, r, fname, fi.ModTime(), f)
		return nil
	}
	if _, err := os.Stat(cachePath); err == nil {
		return serveCacheHit()
	}

	// e.g. https://github.com/hexops/mach-dxcompiler/releases/download/2024.02.10+4ccd240.1/aarch64-linux-gnu_ReleaseFast_lib.tar.zst
	u := &url.URL{
		Scheme: "https",
		Host:   "github.com",
		Path:   path.Join("hexops", project, "releases/download", version, fname),
	}
	url := u.String()
	logWriter := b.idWriter("pkg")
	_ = os.MkdirAll(path.Join("cache/pkg-artifact/", project, version), os.ModePerm)
	if err := scripts.DownloadFile(url, cachePath)(logWriter); err != nil {
		b.idLogf("artifact", "error downloading file: %s url=%s", err, url)
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "unable to fetch\n")
		return nil
	}
	return serveCacheHit()
}

// https://pkg.machengine.org/zig/index.json - a strict superset of https://ziglang.org/builds/index.json
// updated every 15 minutes.
//
// Serves a memory-cached version of https://ziglang.org/builds/index.json (updated every 15 minutes)
// with any keys not present in that file from https://machengine.org/zig/index.json added at the end.
func (b *Bot) httpPkgZigIndex(w http.ResponseWriter, r *http.Request) error {
	cachedFile, err := b.httpPkgZigIndexCached()
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	fmt.Fprintf(w, "%s", cachedFile)
	return nil
}

var (
	httpPkgZigIndexMu        sync.RWMutex
	httpPkgZigIndexFetchedAt time.Time
	httpPkgZigIndexCached    []byte
)

func (b *Bot) httpPkgZigIndexCached() ([]byte, error) {
	httpPkgZigIndexMu.RLock()
	if time.Since(httpPkgZigIndexFetchedAt) < 15*time.Minute {
		defer httpPkgZigIndexMu.RUnlock()
		return httpPkgZigIndexCached, nil
	}

	// Cache needs updating
	httpPkgZigIndexMu.RUnlock()
	httpPkgZigIndexMu.Lock()
	defer httpPkgZigIndexMu.Unlock()

	if time.Since(httpPkgZigIndexFetchedAt) < 15*time.Minute {
		// Someone else beat us to the update.
		return httpPkgZigIndexCached, nil
	}

	// Fetch the latest upstream Zig index.json
	resp, err := httpGet("https://ziglang.org/download/index.json", 60*time.Second)
	if err != nil {
		return nil, errors.Wrap(err, "fetching upstream https://ziglang.org/download/index.json")
	}
	defer resp.Body.Close()
	latestIndex := orderedmap.New[string, *orderedmap.OrderedMap[string, any]]()
	if err := json.NewDecoder(resp.Body).Decode(&latestIndex); err != nil {
		return nil, errors.Wrap(err, "parsing upstream https://ziglang.org/builds/index.json")
	}

	// Fetch the Mach index.json which contains Mach nominated versions, but is otherwise not as
	// up-to-date as ziglang.org's version.
	resp, err = httpGet("https://machengine.org/zig/index.json", 60*time.Second)
	if err != nil {
		return nil, errors.Wrap(err, "fetching mach https://machengine.org/zig/index.json")
	}
	defer resp.Body.Close()
	machIndex := orderedmap.New[string, *orderedmap.OrderedMap[string, any]]()
	if err := json.NewDecoder(resp.Body).Decode(&machIndex); err != nil {
		return nil, errors.Wrap(err, "parsing mach https://machengine.org/zig/index.json")
	}

	// "master", "0.13.0", etc.
	for version := latestIndex.Oldest(); version != nil; version = version.Next() {
		// "src", "x86_64-macos", etc.
		for versionField := version.Value.Oldest(); versionField != nil; versionField = versionField.Next() {
			// "version", "date", "src", "x86_64-macos", etc.
			download, ok := versionField.Value.(map[string]any)
			if ok {
				newDownload := map[string]any{}
				for key, value := range download {
					newDownload[key] = value
					if key == "tarball" {
						newDownload["zigTarball"] = value.(string)

						newTarball := strings.Replace(value.(string), "https://ziglang.org/builds/", b.Config.ExternalURL+"/zig/", 1)
						newTarball = strings.Replace(newTarball, "https://ziglang.org/download/", b.Config.ExternalURL+"/zig/", 1)
						newDownload["tarball"] = newTarball
					}
				}
				version.Value.Set(versionField.Key, newDownload)
			}
		}
	}

	// "master", "0.13.0", etc.
	for version := machIndex.Oldest(); version != nil; version = version.Next() {
		if _, present := latestIndex.Get(version.Key); present {
			// Always use the upstream index.json in the event of a collision
			continue
		}

		// "src", "x86_64-macos", etc.
		for versionField := version.Value.Oldest(); versionField != nil; versionField = versionField.Next() {
			// "version", "date", "src", "x86_64-macos", etc.
			download, ok := versionField.Value.(map[string]any)
			if ok {
				newDownload := map[string]any{}
				for key, value := range download {
					newDownload[key] = value
					if key == "tarball" {
						newDownload["tarball"] = strings.Replace(value.(string), "https://pkg.machengine.org/zig/", b.Config.ExternalURL+"/zig/", 1)
					}
				}
				version.Value.Set(versionField.Key, newDownload)
			}
		}
		latestIndex.Set(version.Key, version.Value)
	}

	httpPkgZigIndexCached, err = json.MarshalIndent(latestIndex, "", "  ")
	if err != nil {
		return nil, errors.Wrap(err, "marshalling index.json")
	}
	httpPkgZigIndexFetchedAt = time.Now()

	return httpPkgZigIndexCached, nil
}

// Like http.Get, but actually respects a timeout instead of leaking a goroutine to forever run.
func httpGet(url string, timeout time.Duration) (*http.Response, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	return http.DefaultClient.Do(req)
}
