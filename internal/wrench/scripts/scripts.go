package scripts

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/hexops/wrench/internal/errors"
	"github.com/mholt/archiver/v4"
)

type Script struct {
	Command         string
	Args            []string
	Description     string
	Execute         func(args ...string) error
	ExecuteResponse func(args ...string) (*Response, error)
}

func (s *Script) Run(args ...string) (*Response, error) {
	if s.ExecuteResponse != nil {
		return s.ExecuteResponse(args...)
	}
	return nil, s.Execute(args...)
}

type Response struct {
	PushedRepos []string
}

var Scripts = []Script{}

type CmdOption func(c *exec.Cmd)

func WorkDir(dir string) CmdOption {
	return func(c *exec.Cmd) {
		c.Dir = dir
	}
}

func Env(key, value string) CmdOption {
	return func(c *exec.Cmd) {
		if c.Env == nil {
			c.Env = os.Environ()
		}
		c.Env = append(c.Env, key+"="+value)
	}
}

func NewCmd(w io.Writer, name string, args []string, opt ...CmdOption) *exec.Cmd {
	cmd := exec.Command(name, args...)
	for _, opt := range opt {
		opt(cmd)
	}
	prefix := ""
	if cmd.Dir != "" {
		prefix = fmt.Sprintf("cd %s/ && ", cmd.Dir)
	}
	if cmd.Env != nil {
		for _, envKeyValue := range cmd.Env[len(os.Environ()):] {
			prefix = fmt.Sprintf("%s %s ", prefix, envKeyValue)
		}
	}
	fmt.Fprintf(w, "$ %s%s\n", prefix, strings.Join(append([]string{name}, args...), " "))
	return cmd
}

func ExecArgs(name string, args []string, opt ...CmdOption) Cmd {
	return func(w io.Writer) error {
		cmd := NewCmd(w, name, args, opt...)
		cmd.Stderr = w
		cmd.Stdout = w
		if err := cmd.Run(); err != nil {
			if exitError, ok := err.(*exec.ExitError); ok {
				return fmt.Errorf("'%s': error: exit code: %v", name, exitError.ExitCode())
			}
			return err
		}
		return nil
	}
}

func Exec(cmdLine string, opt ...CmdOption) Cmd {
	split := strings.Fields(cmdLine)
	name, args := split[0], split[1:]
	return ExecArgs(name, args, opt...)
}

func OutputArgs(w io.Writer, name string, args []string, opt ...CmdOption) (string, error) {
	var buf bytes.Buffer
	cmd := NewCmd(w, name, args, opt...)
	cmd.Stderr = &buf
	cmd.Stdout = &buf
	if err := cmd.Run(); err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("'%s': error: exit code: %v", name, exitError.ExitCode())
		}
		return "", err
	}
	return strings.TrimSpace(buf.String()), nil
}

func Output(w io.Writer, cmdLine string, opt ...CmdOption) (string, error) {
	split := strings.Fields(cmdLine)
	name, args := split[0], split[1:]
	return OutputArgs(w, name, args, opt...)
}

type Cmd func(w io.Writer) error

func (cmd Cmd) IgnoreError() Cmd {
	return func(w io.Writer) error {
		if err := cmd(w); err != nil {
			fmt.Fprintf(w, "ignoring error: %s\n", err)
		}
		return nil
	}
}

func Sequence(cmds ...Cmd) Cmd {
	return func(w io.Writer) error {
		for _, cmd := range cmds {
			if err := cmd(w); err != nil {
				return err
			}
		}
		return nil
	}
}

func DownloadFile(url string, filepath string) Cmd {
	return func(w io.Writer) error {
		fmt.Fprintf(w, "DownloadFile: %s > %s\n", url, filepath)
		out, err := os.Create(filepath)
		if err != nil {
			return errors.Wrap(err, "Create")
		}
		defer out.Close()

		resp, err := http.Get(url)
		if err != nil {
			return errors.Wrap(err, "Get")
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("bad response status: %s", resp.Status)
		}

		_, err = io.Copy(out, resp.Body)
		if err != nil {
			return errors.Wrap(err, "Copy")
		}
		return nil
	}
}

func ExtractArchive(archiveFilePath, dst string, stripPathComponents int) Cmd {
	return func(w io.Writer) error {
		fmt.Fprintf(w, "ExtractArchive: %s > %s\n", archiveFilePath, dst)
		ctx := context.Background()
		handler := func(ctx context.Context, fi archiver.File) error {
			dstPath := filepath.Join(dst, stripComponents(fi.NameInArchive, stripPathComponents))
			if fi.IsDir() {
				err := os.MkdirAll(dstPath, os.ModePerm)
				return errors.Wrap(err, "MkdirAll")
			}

			src, err := fi.Open()
			if err != nil {
				return errors.Wrap(err, "Open")
			}
			defer src.Close()
			dst, err := os.Create(dstPath)
			if err != nil {
				return errors.Wrap(err, "Create")
			}
			_, err = io.Copy(dst, src)
			if err != nil {
				return errors.Wrap(err, "Copy")
			}
			err = os.Chmod(dstPath, fi.Mode().Perm())
			return errors.Wrap(err, "Chmod")
		}
		archiveFile, err := os.Open(archiveFilePath)
		if err != nil {
			return errors.Wrap(err, "Open(archiveFilePath)")
		}
		defer archiveFile.Close()

		type Format interface {
			Extract(
				ctx context.Context,
				sourceArchive io.Reader,
				pathsInArchive []string,
				handleFile archiver.FileHandler,
			) error
		}
		var format Format
		if strings.HasSuffix(archiveFilePath, ".tar.gz") {
			format = archiver.CompressedArchive{
				Compression: archiver.Gz{},
				Archival:    archiver.Tar{},
			}
		} else if strings.HasSuffix(archiveFilePath, ".tar.xz") {
			format = archiver.CompressedArchive{
				Compression: archiver.Xz{},
				Archival:    archiver.Tar{},
			}
		} else if strings.HasSuffix(archiveFilePath, ".zip") {
			format = archiver.Zip{}
		} else {
			return errors.Wrap(err, "unsupported archive format")
		}

		err = format.Extract(ctx, archiveFile, nil, handler)
		if err != nil {
			return errors.Wrap(err, "Extract")
		}
		return nil
	}
}

func stripComponents(path string, n int) string {
	elems := strings.Split(path, string(os.PathSeparator))
	if len(elems) >= n {
		elems = elems[n:]
	}
	return strings.Join(elems, string(os.PathSeparator))
}

func AppendToFile(file, format string, v ...any) Cmd {
	return func(w io.Writer) error {
		fmt.Fprintf(w, "AppendToFile: %s >> %s\n", fmt.Sprintf(format, v...), file)
		f, err := os.OpenFile(file, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return errors.Wrap(err, "OpenFile")
		}
		defer f.Close()
		_, err = fmt.Fprintf(f, format, v...)
		return errors.Wrap(err, "Fprintf")
	}
}

func GitChangesExist(w io.Writer, dir string) (bool, error) {
	output, err := Output(w, "git status --porcelain", WorkDir(dir))
	if err != nil {
		return false, err
	}
	output = strings.TrimSpace(output)
	return len(output) > 0, nil
}

func GitConfigureRepo(w io.Writer, dir string) error {
	err := ExecArgs(
		"git",
		[]string{"config", "user.name", os.Getenv("WRENCH_SECRET_GIT_CONFIG_USER_NAME")},
		WorkDir(dir),
	)(w)
	if err != nil {
		return err
	}
	err = ExecArgs(
		"git",
		[]string{"config", "user.email", os.Getenv("WRENCH_SECRET_GIT_CONFIG_USER_EMAIL")},
		WorkDir(dir),
	)(w)
	if err != nil {
		return err
	}
	return nil
}

func GitCommit(w io.Writer, dir, message string) error {
	err := ExecArgs("git", []string{"add", "."}, WorkDir(dir))(w)
	if err != nil {
		return errors.Wrap(err, "git add")
	}
	return ExecArgs("git", []string{"commit", "-s", "-m", message}, WorkDir(dir))(w)
}

func GitClone(w io.Writer, dir, remoteURL string) error {
	remoteURL = cleanGitURL(remoteURL)
	return ExecArgs("git", []string{"clone", remoteURL, dir})(w)
}

func GitCheckoutNewBranch(w io.Writer, dir, branchName string) error {
	return ExecArgs("git", []string{"checkout", "-B", branchName}, WorkDir(dir))(w)
}

func GitPush(w io.Writer, dir, remoteURL string, force bool) error {
	remoteURL = cleanGitURL(remoteURL)
	u, err := url.Parse(remoteURL)
	if err != nil {
		return errors.Wrap(err, "Parse")
	}
	u.User = url.UserPassword(
		os.Getenv("WRENCH_SECRET_GIT_PUSH_USERNAME"),
		os.Getenv("WRENCH_SECRET_GIT_PUSH_PASSWORD"),
	)

	args := []string{"push", u.String(), "--all"}
	if force {
		args = append(args, "--force")
	}
	return ExecArgs("git", args, WorkDir(dir))(w)
}

func cleanGitURL(remoteURL string) string {
	if !strings.HasPrefix(remoteURL, "https://") && !strings.HasPrefix(remoteURL, "http://") {
		return "https://" + remoteURL
	}
	return remoteURL
}

func FindAndReplace(dir string, globs []string, replacer func(name string, contents []byte) ([]byte, error)) Cmd {
	return func(w io.Writer) error {
		fmt.Fprintf(w, "FindAndReplace: dir=%s globs=%s\n", dir, globs)
		for _, glob := range globs {
			fsys := os.DirFS(dir)
			matches, err := doublestar.Glob(fsys, glob)
			if err != nil {
				return errors.Wrap(err, "Glob")
			}
			for _, match := range matches {
				if strings.Contains(match, ".git/") {
					continue
				}
				data, err := os.ReadFile(match)
				if err != nil {
					return errors.Wrap(err, "ReadFile")
				}
				replacement, err := replacer(match, data)
				if err != nil {
					return err
				}
				err = os.WriteFile(match, replacement, 0o655) // perms will not change as file exists already
				if err != nil {
					return errors.Wrap(err, "WriteFile")
				}
			}
		}
		return nil
	}
}
