package wrench

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"strings"

	"github.com/hexops/wrench/internal/wrench/scripts"
)

func (b *Bot) httpMuxPkgProxy(handler func(prefix string, handle handlerFunc) http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
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
	return mux
}

func (b *Bot) httpPkgRoot(w http.ResponseWriter, r *http.Request) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, "%s", `
<h1 style="margin-bottom: 0.25rem;">pkg.machengine.org</h1>
<strong style="font-weight: 16px; margin-top: 0; margin-left: 1rem;"><em>The <a href="https://machengine.org">Mach</a> package download server</em></strong>

<div style="margin-left: 2rem;"">

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

// https://pkg.machengine.org/zig/<file>
// -> https://ziglang.org/builds/<file>
func (b *Bot) httpPkgZig(w http.ResponseWriter, r *http.Request) error {
	split := strings.Split(r.URL.Path, "/")
	if len(split) == 0 {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "invalid path\n")
		return nil
	}
	fname := split[len(split)-1]

	// Validate this is an allowed file
	validate := fname
	validate = strings.TrimSuffix(validate, ".tar.xz")
	validate = strings.TrimSuffix(validate, ".tar.xz.minisig")
	validate = strings.TrimSuffix(validate, ".zip")
	validate = strings.TrimSuffix(validate, ".zip.minisig")
	zigVersionRegexp := regexp.MustCompile(`(\d\.?)+-[[:alnum:]]+.\d+\+[[:alnum:]]+`)
	if !zigVersionRegexp.MatchString(validate) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "invalid filename\n")
		return nil
	}

	path := path.Join("cache/zig/", fname)
	serveCacheHit := func() error {
		w.Header().Set("cache-control", "public, max-age=31536000, immutable")
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		fi, err := f.Stat()
		if err != nil {
			return err
		}

		b.idLogf("zig", "serve %s", path)
		http.ServeContent(w, r, fname, fi.ModTime(), f)
		return nil
	}
	if _, err := os.Stat(path); err == nil {
		return serveCacheHit()
	}

	url := "https://ziglang.org/builds/" + fname
	logWriter := b.idWriter("zig")
	_ = os.MkdirAll("cache/zig/", os.ModePerm)
	if err := scripts.DownloadFile(url, path)(logWriter); err != nil {
		b.idLogf("zig", "error downloading file: %s url=%s", err, url)
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "unable to fetch\n")
		return nil
	}
	return serveCacheHit()
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
