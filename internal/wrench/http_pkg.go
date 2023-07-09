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
		handler("pkg", b.httpPkgPkg).ServeHTTP(w, r)
	})
	return mux
}

func (b *Bot) httpPkgRoot(w http.ResponseWriter, r *http.Request) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<h1>pkg.machengine.org</h1>`)
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
