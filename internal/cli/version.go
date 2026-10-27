package cli

import (
	"github.com/spf13/cobra"

	"github.com/altalune-id/othra/version"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "version",
		Short:   "Print Othra version info",
		GroupID: "meta",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cmd.Println(version.String())
			return nil
		},
	}
}
