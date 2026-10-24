// Command tooltrack shows the installed and latest version of your
// development tools and updates them from the terminal.
package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/JoseDev155/tooltrack/internal/app"
	"github.com/JoseDev155/tooltrack/internal/config"
	"github.com/JoseDev155/tooltrack/internal/ui"
)

// Version is set at build time with -ldflags "-X main.Version=...".
var Version = "dev"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "tooltrack:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("tooltrack", flag.ContinueOnError)
	showVersion := fs.Bool("version", false, "print the version and exit")
	home := fs.String("home", "", "directory for data/ and logs/ (default: user config dir, or $"+config.EnvHome+")")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *showVersion {
		fmt.Println("tooltrack", Version)
		return nil
	}

	dir := *home
	if dir == "" {
		var err error
		if dir, err = config.Dir(); err != nil {
			return err
		}
	}
	a, err := app.New(config.PathsFor(dir))
	if err != nil {
		return err
	}
	defer a.Close()

	_, err = tea.NewProgram(ui.New(a)).Run()
	return err
}
