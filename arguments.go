package main

import (
	"gopkg.in/alecthomas/kingpin.v2"
	"os"
	"strings"
)

// ApplicationArguments allows proper management between managed and non managed arguments provided to kingpin
type ApplicationArguments struct {
	*kingpin.Application
	longs  map[string]bool
	shorts map[byte]bool
}

func (app ApplicationArguments) add(name, description string, isSwitch bool, shorts ...byte) *kingpin.FlagClause {
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
func (app ApplicationArguments) Switch(name, description string, shorts ...byte) *kingpin.FlagClause {
	return app.add(name, description, true, shorts...)
}

// Argument adds an argument to the application
// The argument requires additional argument to be complete
func (app ApplicationArguments) Argument(name, description string, shorts ...byte) *kingpin.FlagClause {
	return app.add(name, description, false, shorts...)
}

// SplitManaged splits the managed by kingpin and unmanaged argument to avoid error
func (app ApplicationArguments) SplitManaged() (managed []string, unmanaged []string) {
Arg:
	for i := 1; i < len(os.Args); i++ {
		arg := os.Args[i]
		if strings.HasPrefix(arg, "--") {
			argSplit := strings.Split(os.Args[i][2:], "=")
			if isSwitch, ok := app.longs[argSplit[0]]; ok {
				managed = append(managed, arg)
				if !isSwitch && len(argSplit) == 1 {
					i++
					managed = append(managed, os.Args[i])
				}
			} else {
				unmanaged = append(unmanaged, arg)
			}
		} else if strings.HasPrefix(arg, "-") {
			for _, c := range arg[1:] {
				if isSwitch, ok := app.shorts[byte(c)]; ok {
					if !isSwitch {
						break
					}
				} else {
					unmanaged = append(unmanaged, arg)
					continue Arg
				}
			}
			managed = append(managed, arg)
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
		longs:       map[string]bool{"help": true},
		shorts:      map[byte]bool{'h': true},
	}
}
