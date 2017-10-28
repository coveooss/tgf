package main

import (
	"os"
	"strings"

	"gopkg.in/alecthomas/kingpin.v2"
)

// ApplicationArguments allows proper management between managed and non managed arguments provided to kingpin
type ApplicationArguments struct {
	*kingpin.Application
	longs  map[string]bool
	shorts map[rune]bool
}

func (app ApplicationArguments) add(name, description string, isSwitch bool, shorts ...rune) *kingpin.FlagClause {
	flag := app.Application.Flag(name, description)
	switch len(shorts) {
	case 0:
		break
	case 1:
		flag = flag.Short(shorts[0])
		app.shorts[shorts[0]] = isSwitch
	default:
		panic("Maximum one short option should be specified")
	}

	app.longs[name] = isSwitch
	return flag
}

// Switch adds a switch argument to the application
// A switch is a boolean flag that do not require additional value
func (app ApplicationArguments) Switch(name, description string, shorts ...rune) *kingpin.FlagClause {
	return app.add(name, description, true, shorts...)
}

// Argument adds an argument to the application
// The argument requires additional argument to be complete
func (app ApplicationArguments) Argument(name, description string, shorts ...rune) *kingpin.FlagClause {
	return app.add(name, description, false, shorts...)
}

// SplitManaged splits the managed by kingpin and unmanaged argument to avoid error
func (app ApplicationArguments) SplitManaged() (managed []string, unmanaged []string) {
Arg:
	for i := 1; i < len(os.Args); i++ {
		arg := os.Args[i]
		if arg == "--" {
			unmanaged = append(unmanaged, os.Args[i+1:]...)
			break
		}
		if strings.HasPrefix(arg, "--") {
			argSplit := strings.Split(os.Args[i][2:], "=")
			if isSwitch, ok := app.longs[argSplit[0]]; ok {
				managed = append(managed, arg)
				if !isSwitch && len(argSplit) == 1 {
					// This is not a switch (bool flag) and there is no argument with
					// the flag, so the argument must be after and we add it to
					// the managed args if there is.
					i++
					if i < len(os.Args) {
						managed = append(managed, os.Args[i])
					}
				}
			} else {
				unmanaged = append(unmanaged, arg)
			}
		} else if strings.HasPrefix(arg, "-") {
			withArg := false
			for pos, opt := range arg[1:] {
				if isSwitch, ok := app.shorts[opt]; ok {
					if !isSwitch {
						// This is not a switch (bool flag), so we check if there are characters
						// following the current flag in the same word. If it is not the case,
						// then the argument must be after and we add it to the managed args
						// if there is. If it is the case, then, the argument is included in
						// the current flag and we consider the whole word as a managed argument.
						withArg = pos == len(arg[1:])-1
						break
					}
				} else {
					unmanaged = append(unmanaged, arg)
					continue Arg
				}
			}
			managed = append(managed, arg)
			if withArg {
				// The next argument must be an argument to the current flag
				i++
				if i < len(os.Args) {
					managed = append(managed, os.Args[i])
				}
			}
		} else {
			unmanaged = append(unmanaged, arg)
		}
	}
	return
}

// NewApplication returns an initialized copy of ApplicationArguments
func NewApplication(app *kingpin.Application) ApplicationArguments {
	return ApplicationArguments{
		Application: app,
		longs: map[string]bool{
			"help-man":               true,
			"help-long":              true,
			"completion-bash":        true,
			"completion-script-bash": true,
			"completion-script-zsh":  true,
		},
		shorts: map[rune]bool{},
	}
}
