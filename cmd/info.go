package cmd

import (
	"fmt"
	"strings"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

var infoCmd = &cobra.Command{
	Use:   "info <tool>",
	Short: "Show detailed info about a tool",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		reg, _, _, err := loadContext()
		if err != nil {
			return err
		}

		t, ok := reg.FindTool(args[0])
		if !ok {
			return fmt.Errorf("unknown tool: %s", args[0])
		}

		pterm.DefaultHeader.WithFullWidth().WithBackgroundStyle(pterm.NewStyle(pterm.BgDefault)).
			WithTextStyle(pterm.NewStyle(pterm.Bold, pterm.FgCyan)).Println(t.Name)
		pterm.Println()

		rows := pterm.TableData{
			{"Description", t.Description},
			{"Installer", t.Installer},
			{"Categories", strings.Join(t.Category, ", ")},
		}
		if t.Steps != nil {
			rows = append(rows, []string{"Install cmd", "Custom Steps are specified"})
		}
		if len(t.Profiles) > 0 {
			rows = append(rows, []string{"Profiles", strings.Join(t.Profiles, ", ")})
		}
		if len(t.Aliases) > 0 {
			rows = append(rows, []string{"Aliases", strings.Join(t.Aliases, ", ")})
		}
		if t.Repo != "" {
			rows = append(rows, []string{"Repo", "https://github.com/" + t.Repo})
		}
		if t.Package != "" {
			rows = append(rows, []string{"Package", t.Package})
		}
		if t.AssetPattern != "" {
			rows = append(rows, []string{"Asset", t.AssetPattern})
		}
		if t.PythonVersion != "" {
			rows = append(rows, []string{"Python", t.PythonVersion})
		}
		if t.ArchPackage != "" {
			rows = append(rows, []string{"Arch pkg", t.ArchPackage})
		}
		if t.DebianPackage != "" {
			rows = append(rows, []string{"Debian pkg", t.DebianPackage})
		}

		return pterm.DefaultTable.WithBoxed().WithLeftAlignment().WithData(rows).Render()
	},
}

func init() {
	rootCmd.AddCommand(infoCmd)
}
