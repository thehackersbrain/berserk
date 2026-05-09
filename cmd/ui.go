package cmd

import (
	"os"

	"github.com/pterm/pterm"
)

// pterm auto-detects NO_COLOR, TERM=dumb, and non-TTY stdout, so the manual
// ANSI suppression logic the package used to carry is no longer needed.

func init() {
	// pterm's default writer is stdout for every printer, including Error and
	// Warning. Send those to stderr so `berserk install foo 2>errors.log`
	// captures diagnostics the way users expect.
	pterm.Error.Writer = os.Stderr
	pterm.Warning.Writer = os.Stderr
}

func printOK(format string, a ...any) {
	pterm.Success.Printfln(format, a...)
}

func printInfo(format string, a ...any) {
	pterm.Info.Printfln(format, a...)
}

func printWarn(format string, a ...any) {
	pterm.Warning.Printfln(format, a...)
}

func printErr(format string, a ...any) {
	pterm.Error.Printfln(format, a...)
}

// printProgress is a transient-looking status line. Not a spinner — subprocess
// output below would clobber a live spinner under parallel installs.
var progressPrinter = pterm.PrefixPrinter{
	MessageStyle: pterm.NewStyle(pterm.FgDefault),
	Prefix: pterm.Prefix{
		Style: pterm.NewStyle(pterm.FgYellow),
		Text:  " WORKING ",
	},
}

func printProgress(format string, a ...any) {
	progressPrinter.Printfln(format, a...)
}
