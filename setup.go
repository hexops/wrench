package main

import (
	_ "embed"
	"flag"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/hexops/cmder"
	"github.com/hexops/wrench/internal/errors"
	"github.com/hexops/wrench/internal/wrench"
	"github.com/hexops/wrench/internal/wrench/scripts"
	"github.com/kardianos/service"
	"github.com/manifoldco/promptui"
)

func init() {
	const usage = `
Examples:

  Run the installation:

    $ wrench setup

`

	// Parse flags for our subcommand.
	flagSet := flag.NewFlagSet("setup", flag.ExitOnError)

	promptPath := func(fileName, defaultPath string) (string, bool) {
		validate := func(input string) error {
			return nil
		}

		prompt := promptui.Prompt{
			Label:    "Where shall I store the " + fileName + " file?",
			Validate: validate,
			Default:  defaultPath,
		}

		result, err := prompt.Run()
		if err != nil {
			fmt.Printf("Prompt failed %v\n", err)
			return "", false
		}

		u, err := user.Current()
		if err == nil {
			result = strings.Replace(result, "$HOME", u.HomeDir, -1)
		}
		return result, true
	}

	promptString := func(label, defaultValue string, isPassword bool) (string, bool) {
		validate := func(input string) error {
			return nil
		}

		prompt := promptui.Prompt{
			Label:       label,
			Validate:    validate,
			Default:     defaultValue,
			HideEntered: isPassword,
		}
		result, err := prompt.Run()
		if err != nil {
			fmt.Printf("Prompt failed %v\n", err)
			return "", false
		}
		return result, true
	}

	promptBool := func(label string, defaultValue bool) bool {
		def := "n"
		if defaultValue {
			def = "y"
		}
		prompt := promptui.Prompt{
			Label:     label,
			IsConfirm: true,
			Default:   def,
		}
		v, err := prompt.Run()
		if err != nil {
			return false
		}
		v = strings.ToLower(v)
		if strings.TrimSpace(v) == "" {
			return defaultValue
		}
		return v == "y" || v == "yes" || v == "true"
	}

	// Handles calls to our subcommand.
	handler := func(args []string) error {
		_ = flagSet.Parse(args)
		fmt.Printf("%s\n", logo)

		const (
			deployTypeRunner = "runner"
			deployTypeServer = "server"
		)
		prompt := promptui.Select{
			Label: "Deployment type",
			Items: []string{deployTypeRunner, deployTypeServer},
		}
		_, deployType, err := prompt.Run()
		if err != nil {
			fmt.Printf("Prompt failed %v\n", err)
			return nil
		}

		configFile, ok := promptPath("config", "$HOME/wrench/config.toml")
		if !ok {
			return nil
		}
		configFile, err = filepath.Abs(configFile)
		if err != nil {
			return errors.Wrap(err, "Abs")
		}

		var config wrench.Config
		if deployType == deployTypeRunner {
			writeConfig := false
			_, err := os.Stat(configFile)
			if err == nil {
				useExisting := promptBool("Use existing config file?", true)
				if !useExisting {
					writeConfig = true
				}
			} else {
				writeConfig = true
			}
			if writeConfig {
				config.Runner, ok = promptString("config: Runner ID (e.g. 'dev')", "", false)
				if !ok {
					return nil
				}
				if config.Runner == "" {
					fmt.Println("error: a runner ID must be specified")
					return nil
				}
				config.ExternalURL, ok = promptString("config: ExternalURL", "https://wrench.machengine.org", false)
				if !ok {
					return nil
				}
				config.Secret, ok = promptString("config: Secret (configured on server)", "", true)
				if !ok {
					return nil
				}
				fmt.Printf("wrench: writing config to disk..")
				if err := config.WriteTo(configFile); err != nil {
					fmt.Println(" error")
					return errors.Wrap(err, "WriteTo")
				}
				fmt.Println(" ok")
			}
		} else {
			_, err := os.Stat(configFile)
			if os.IsNotExist(err) {
				fmt.Println("Please create your own config file:", config)
				fmt.Println("Refer to: https://github.com/hexops/wrench/blob/main/internal/wrench/config.go")
				return nil
			}
			if err != nil {
				return errors.Wrap(err, "Stat")
			}
		}

		installService := promptBool("Install system service", true)
		if installService {
			fmt.Printf("wrench: checking permissions..")
			ok, err := isRoot()
			if err != nil {
				fmt.Println(" error")
				return errors.Wrap(err, "isRoot")
			}
			if !ok {
				fmt.Println(" error: please run as root")
				return nil
			}
			fmt.Println(" ok")

			exePath, err := os.Executable()
			if err != nil {
				return errors.Wrap(err, "Executable")
			}
			fmt.Println("wrench: binary path:", exePath)
			ok = promptBool("Encode this binary path", true)
			if !ok {
				fmt.Println("wrench: please move the binary and rerun")
				return nil
			}

			fmt.Printf("wrench: ensuring binary on system PATH..")
			err = scripts.EnsureOnPathPermanent(filepath.Dir(exePath))
			if err != nil {
				fmt.Println(" error")
				return errors.Wrap(err, "EnsureOnPathPermanent")
			}
			fmt.Println(" ok")

			fmt.Printf("wrench: installing system service..")
			svc, _ := newServiceBotWithConfig(&ServiceConfig{
				ConfigFile: configFile,
				Executable: exePath,
			})

			// Always attempt to uninstall the service. If it is already installed, the old version
			// would be out of date. If it's not installed, this will produce an error (Install will
			// fail with the same error below if it's permissions related.)
			_ = svc.Uninstall()
			if err := svc.Install(); err != nil {
				fmt.Println(" error")
				return errors.Wrap(err, "Install")
			}
			fmt.Println(" ok")

			fmt.Printf("wrench: launching system service..")
			_ = svc.Stop() // systemd will hand if trying to start an already-started service
			if err := svc.Start(); err != nil && !strings.Contains(err.Error(), "Warning: Expecting a LaunchAgents path") {
				fmt.Println(" error")
				return errors.Wrap(err, "Start")
			}
			fmt.Println(" ok")

			fmt.Printf("wrench: waiting for service to start.")
			running := time.Time{}
			for {
				fmt.Printf(".")
				time.Sleep(1 * time.Second)
				status, err := svc.Status()
				if err != nil {
					fmt.Println(" error")
					return errors.Wrap(err, "Status")
				}
				if status == service.StatusRunning {
					if running.IsZero() {
						running = time.Now()
					}
					if time.Since(running) > 5*time.Second {
						fmt.Println(" ok")
						break
					}
				} else if !running.IsZero() {
					fmt.Println(" ERROR")
					fmt.Println("Please debug using `wrench svc status`")
					return nil
				}
			}
		}

		return nil
	}

	// Register the command.
	commands = append(commands, &cmder.Command{
		FlagSet: flagSet,
		Aliases: []string{},
		Handler: handler,
		UsageFunc: func() {
			fmt.Fprintf(flag.CommandLine.Output(), "Usage of 'wrench %s':\n", flagSet.Name())
			flagSet.PrintDefaults()
			fmt.Printf("%s", usage)
		},
	})
}

func isRoot() (bool, error) {
	u, err := user.Current()
	if err != nil {
		return false, err
	}
	if runtime.GOOS != "windows" {
		return u.Uid == "0", nil
	}
	ids, err := u.GroupIds()
	if err != nil {
		return false, err
	}
	for i := range ids {
		if ids[i] == "S-1-5-32-544" { // SID for the built-in Administrators group
			return true, nil
		}
	}
	return false, nil
}

const logo = `
@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@
@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@&@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@
@@@@@@@@@@@@@@@@@@@@@@@@@@@#&@GG&&&&&&&&&&&&&&&@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@
@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@#BPPPP555P555PPPGGGBBB&@@@@@@@@@@@@@@@@@@@@@@@@@@@
@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@#????????????JJJJJP#&@@@@@@@@@@@@@@@@@@@@@@@@@@@
@@@@@@@@@@@@@@@&#@@@@@@@@@@@@@@@B777777777777777!5@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@
@@@@@@@@@@@@@@B7G@@@@@@@@@@@@@@P7777777777777777!#@@@@@@@@@@@@@#B@@@@@@@@@@@@@@@
@@@@@@@@@@@@@#77Y&@@@@@@@@@&#PJ7????????7???77??7?G&@@@@@@@@@#P7!?B@@@@@@@@@@@@@
@@@@@@@@@@@@@?777?5PB###BBPYJ?????7777?PBGPPGG5????J5GGBBGP5?7!7J77&@@@@@@@@@@@@
@@@@@@#5J?JBG!7??YY?77!!~:..............!PPG?^....:^~~!77!7JJ??7PYYG@@@@@@@@@@@@
@@@@&J:.::..5PPPP#~......:^^~~~~^^.......:P^.......^^^^~!?~^:~!?5BPP&@@@@@@@@@@@
@@@B^ !?!77^B555B!....^!77!~^^^^~!:...............:?P5!...~J!...YGGP5B@BGGG#@@@@
@@#:.??...:G5555B....77~:.........................B@@@@?....7...?P5P#?~....:J@@@
@@^.:5....YG5555B:...:....::::..:^^..............7@@@@@@:.......G555& .^~:...?@@
@G..7!~7~^#55555PP^..^7Y5GBBBBG5YJ~..............Y@@@@@@!......YG555#.?7~J!.. B@
@J..^Y^?7G5555555PGJ.P5J7~^^^^~!J5G?.............7@@@@@@:....!PP5555&.^...B...P@
@B...^~^^#555555PPJ!.:............::.....~~:......B@@@@?..:7PGP55555# !!..B...#@
@@G!... YP55555G?:.......................!~^.......JPY7....:~75P5555G^J!!J!..Y@@
@@@@G5YP#55555B^.........................^!:...................G55555.!!!~ :Y@@@
@@@@@@@@@B555PP.................7J7777J5B@@#5?777!:........... P555B^....^?#@@@@
@@@@@@@@@@G555B..................Y@@@@@@@@@@@@@&^.............7G55GB77?YG&@@@@@@
@@@@@@@@@@@B55P5^.................Y@@@@@@@@@@@#~.............7G55G@@@@@@@@@@@@@@
@@@@@@@@@@@@&G5PP?^................~P@@@@@@@&Y:............~5P5P#@@@@@@@@@@@@@@@
@@@@@@@@@@@@@@&BPPPY?!::.............^?5GPY7:.........::~?5PPG#@@@@@@@@@@@@@@@@@
@@@@@@@@@@@@@@@@@&BGGPP5Y?7!~^^:::...........::^^~!7?Y5PPGG#@@@@@@@@@@@@@@@@@@@@
@@@@@@@@@@@@@@@@@@@@@&###BBGGGGP55YYYYYJYYYY5PPGBBBB####&@@@@@@@@@@@@@@@@@@@@@@@
@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@&&&&&&&&&&&&&@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@
@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@
@@@@@@@@@@@@@@@@@@ @@@@@@@@@@@@@@@@@@ __@_@@@@@@@@@@@@ @@@@ @@@@@@@_@@@@@@@@@@@@
@.@@@@@@@@@@@@@@@@.@@@@.@@@@@@@@@@@@.:@@@@@@@@@@@@@@@@.@@@@.@@@@@@@@@@@@@@@@@.@@
@.@@@@@@@@@.___.@@..__@.@@.__..@@@@@..__@.@.@@@@@.@@@@..__@..___..@.@@.___.@@.@@
@.@@@@@@@@..@@@..@._@@@@@..@@___@@@@._@@@.@@.@@@.@@@@@._@@@._@@@_.@.@..@@@__@.@@
@.@@@@@@@@..@@@..@.@@@@@@..@@@@@@@@@.@@@@.@@@_._@@@@@@.@@@@.@@@@@.@.@_.@@@@@@.@@
@.@@@@@@@@.._____@.@@@@@@@____.@@@@@.@@@@.@@@@.@@@@@@@.@@@@.@@@@@.@.@@_____.@.@@
@.@@@@@@@@..@@@@@@.@@@@@@@@@@@._@@@@.@@@@.@@@.@.@@@@@@.@@@@.@@@@@.@.@@@@@@@._@@@
@________@@______@____@@@______@@@@@_@@@@_@@_@@@_@@@@@____@_@@@@@_@_@_______@_@@
@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@
`
