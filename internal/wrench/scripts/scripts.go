package scripts

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/hexops/wrench/internal/errors"
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
