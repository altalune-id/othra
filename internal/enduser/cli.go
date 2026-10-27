package enduser

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"altalune.id/othra/internal/render"
)

// SignInLinkFn issues a printed /p/system sign-in link for a console account; the caller boots the server graph.
type SignInLinkFn func(ctx context.Context, email string) (IssuedLink, error)

// NewSignInLinkCmd builds `sign-in-link <email>`: a one-time console sign-in link written to stdout, never emailed.
func NewSignInLinkCmd(issue SignInLinkFn) *cobra.Command {
	return &cobra.Command{
		Use:   "sign-in-link <email>",
		Short: "Print a one-time console sign-in link for an existing console account (no email is sent)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			l, err := issue(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			expires := l.ExpiresAt.UTC().Format(time.RFC3339)
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "This link signs in as %s and works once, until %s. Give it only to them.\n", l.Email, expires)
			if f := render.Detect(cmd); f == render.FormatJSON || f == render.FormatNDJSON {
				return render.JSON(cmd.OutOrStdout(), map[string]any{"url": l.URL, "email": l.Email, "expires_at": expires})
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), l.URL)
			return nil
		},
	}
}
