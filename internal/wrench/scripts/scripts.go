package scripts

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/hexops/wrench/internal/errors"
	"github.com/mholt/archiver/v4"
)

type Script struct {
	Command     string
	Args        []string
	Description string
	Execute     func(args ...string) error
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

func newCmd(name string, args []string, opt ...CmdOption) *exec.Cmd {
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
	fmt.Fprintf(os.Stderr, "$ %s%s\n", prefix, strings.Join(append([]string{name}, args...), " "))
	return cmd
}

func ExecArgs(name string, args []string, opt ...CmdOption) Cmd {
	return func() error {
		cmd := newCmd(name, args, opt...)
		cmd.Stderr = os.Stderr
		cmd.Stdout = os.Stdout
		if err := cmd.Run(); err != nil {
			if exitError, ok := err.(*exec.ExitError); ok {
				return fmt.Errorf("'%s': error: exit code: %v", name, exitError.ExitCode())
			}
		}
		return nil
	}
}

func Exec(cmdLine string, opt ...CmdOption) Cmd {
	split := strings.Fields(cmdLine)
	name, args := split[0], split[1:]
	return ExecArgs(name, args, opt...)
}

func OutputArgs(name string, args []string, opt ...CmdOption) (string, error) {
	var buf bytes.Buffer
	cmd := newCmd(name, args, opt...)
	cmd.Stderr = &buf
	cmd.Stdout = &buf
	if err := cmd.Run(); err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("'%s': error: exit code: %v", name, exitError.ExitCode())
		}
	}
	return strings.TrimSpace(buf.String()), nil
}

func Output(cmdLine string, opt ...CmdOption) (string, error) {
	split := strings.Fields(cmdLine)
	name, args := split[0], split[1:]
	return OutputArgs(name, args, opt...)
}

type Cmd func() error

func (cmd Cmd) IgnoreError() Cmd {
	return func() error {
		if err := cmd(); err != nil {
			fmt.Fprintf(os.Stderr, "ignoring error: %s\n", err)
		}
		return nil
	}
}

func Sequence(cmds ...Cmd) Cmd {
	return func() error {
		for _, cmd := range cmds {
			if err := cmd(); err != nil {
				return err
			}
		}
		return nil
	}
}

func DownloadFile(url string, filepath string) Cmd {
	return func() error {
		fmt.Fprintf(os.Stderr, "DownloadFile: %s > %s\n", url, filepath)
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

func ExtractArchive(archiveFilePath, dst string) Cmd {
	return func() error {
		fmt.Fprintf(os.Stderr, "ExtractArchive: %s > %s\n", archiveFilePath, dst)
		ctx := context.Background()
		handler := func(ctx context.Context, fi archiver.File) error {
			dstPath := filepath.Join(dst, fi.NameInArchive)
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
			return errors.Wrap(err, "Copy")
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
