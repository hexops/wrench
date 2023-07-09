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
	"github.com/hexops/wrench/internal/wrench/api"
	"github.com/mholt/archiver/v4"
)

type Script struct {
	Command         string
	Args            []string
	Description     string
	Execute         func(args ...string) error
	ExecuteResponse func(args ...string) (*api.ScriptResponse, error)
}

func (s *Script) Run(args ...string) (*api.ScriptResponse, error) {
	if s.ExecuteResponse != nil {
		return s.ExecuteResponse(args...)
	}
	return &api.ScriptResponse{}, s.Execute(args...)
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

func CopyFile(src, dst string) Cmd {
	return func(w io.Writer) error {
		fmt.Fprintf(w, "cp %s %s\n", src, dst)

		fi, err := os.Stat(src)
		if err != nil {
			return errors.Wrap(err, "Stat")
		}
		contents, err := os.ReadFile(src)
		if err != nil {
			return errors.Wrap(err, "ReadFile")
		}
		err = os.WriteFile(dst, contents, fi.Mode().Perm())
		return errors.Wrap(err, "WriteFile")
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
	pathSep := "/"
	if strings.Contains(path, "\\") {
		pathSep = "\\"
	}
	elems := strings.Split(path, pathSep)
	if len(elems) >= n {
		elems = elems[n:]
	}
	return strings.Join(elems, pathSep)
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
	os.Setenv("GIT_TERMINAL_PROMPT", "0")
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
	err = ExecArgs(
		"git",
		[]string{"config", "credential.helper", ""},
		WorkDir(dir),
	)(w)
	if err != nil {
		return err
	}
	err = ExecArgs(
		"git",
		[]string{"config", "http.postBuffer", "157286400"},
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
	remoteURL = GitRemoteURLWithAuth(cleanGitURL(remoteURL))
	return ExecArgs("git", []string{"clone", remoteURL, dir})(w)
}

func GitCheckout(w io.Writer, dir, branchName string) error {
	return ExecArgs("git", []string{"checkout", branchName}, WorkDir(dir))(w)
}

func GitCheckoutRestore(w io.Writer, dir string, files ...string) error {
	return ExecArgs("git", append([]string{"checkout", "--"}, files...), WorkDir(dir))(w)
}

func GitCheckoutNewBranch(w io.Writer, dir, branchName string) error {
	return ExecArgs("git", []string{"checkout", "-B", branchName}, WorkDir(dir))(w)
}

func GitMerge(w io.Writer, dir, branchName string) error {
	return ExecArgs("git", []string{"merge", branchName}, WorkDir(dir))(w)
}

func GitFetch(w io.Writer, dir, remote string) error {
	return ExecArgs("git", []string{"fetch", remote}, WorkDir(dir))(w)
}

func GitResetHard(w io.Writer, dir, ref string) error {
	return ExecArgs("git", []string{"reset", "--hard", ref}, WorkDir(dir))(w)
}

func GitCleanFxd(w io.Writer, dir string) error {
	return ExecArgs("git", []string{"clean", "-fxd"}, WorkDir(dir))(w)
}

func GitRemoteAdd(w io.Writer, dir, remoteName, remoteURL string) error {
	return ExecArgs("git", []string{"remote", "add", remoteName, remoteURL}, WorkDir(dir))(w)
}

func GitRemoteURLWithAuth(remoteURL string) string {
	remoteURL = cleanGitURL(remoteURL)
	u, err := url.Parse(remoteURL)
	if err != nil {
		return remoteURL
	}
	u.User = url.UserPassword(
		os.Getenv("WRENCH_SECRET_GIT_PUSH_USERNAME"),
		os.Getenv("WRENCH_SECRET_GIT_PUSH_PASSWORD"),
	)
	return u.String()
}

func GitPush(w io.Writer, dir, remoteURL string, force bool) error {
	args := []string{"push", GitRemoteURLWithAuth(remoteURL)}
	if force {
		args = append(args, "--force")
	}
	return ExecArgs("git", args, WorkDir(dir))(w)
}

func GitBranches(w io.Writer, dir string) ([]string, error) {
	out, err := Output(w, `git branch -a --format %(refname:short)`, WorkDir(dir))
	if err != nil {
		return nil, errors.Wrap(err, "git branch")
	}
	var branches []string
	for _, line := range strings.Split(out, "\n") {
		branches = append(branches, strings.TrimSpace(line))
	}
	return branches, nil
}

func GitRevParse(w io.Writer, dir, rev string) (string, error) {
	out, err := Output(w, `git rev-parse `+rev, WorkDir(dir))
	if err != nil {
		return "", errors.Wrap(err, "git rev-parse")
	}
	return out, nil
}

func GitCloneOrUpdateAndClean(w io.Writer, workDir, repoURL string) error {
	_, err := os.Stat(workDir)
	if os.IsNotExist(err) {
		if err := GitClone(os.Stderr, workDir, repoURL); err != nil {
			return errors.Wrap(err, "GitClone")
		}
	} else {
		if err := GitFetch(os.Stderr, workDir, "origin"); err != nil {
			return errors.Wrap(err, "GitFetch")
		}
		if err := GitResetHard(os.Stderr, workDir, "origin/main"); err != nil {
			return errors.Wrap(err, "GitResetHard")
		}
		if err := GitCleanFxd(os.Stderr, workDir); err != nil {
			return errors.Wrap(err, "GitCleanFxd")
		}
	}
	return nil
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
				match = filepath.Join(dir, match)
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

func FindAndDelete(dir string, globs []string, delete func(name string) (bool, error)) Cmd {
	return func(w io.Writer) error {
		fmt.Fprintf(w, "FindAndDelete: dir=%s globs=%s\n", dir, globs)
		for _, glob := range globs {
			fsys := os.DirFS(dir)
			matches, err := doublestar.Glob(fsys, glob)
			if err != nil {
				return errors.Wrap(err, "Glob")
			}
			for _, match := range matches {
				match = filepath.Join(dir, match)
				shouldDelete, err := delete(match)
				if err != nil {
					return err
				}
				if shouldDelete {
					fmt.Fprintf(w, "$ rm -rf %s\n", match)
					if err := os.RemoveAll(match); err != nil {
						return errors.Wrap(err, "RemoveAll")
					}
				}
			}
		}
		return nil
	}
}

func Move(source, destination string) error {
	err := os.Rename(source, destination)
	if err != nil && strings.Contains(err.Error(), "invalid cross-device link") {
		return moveCrossDevice(source, destination)
	}
	return err
}

func moveCrossDevice(source, destination string) error {
	src, err := os.Open(source)
	if err != nil {
		return errors.Wrap(err, "Open(source)")
	}
	dst, err := os.Create(destination)
	if err != nil {
		src.Close()
		return errors.Wrap(err, "Create(destination)")
	}
	_, err = io.Copy(dst, src)
	src.Close()
	dst.Close()
	if err != nil {
		return errors.Wrap(err, "Copy")
	}
	fi, err := os.Stat(source)
	if err != nil {
		os.Remove(destination)
		return errors.Wrap(err, "Stat")
	}
	err = os.Chmod(destination, fi.Mode())
	if err != nil {
		os.Remove(destination)
		return errors.Wrap(err, "Stat")
	}
	os.Remove(source)
	return nil
}
