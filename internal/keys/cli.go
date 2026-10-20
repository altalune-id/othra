package keys

import (
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/types/known/timestamppb"

	keysv1 "altalune.id/othra/gen/go/keys/v1"
	keysv1connect "altalune.id/othra/gen/go/keys/v1/keysv1connect"
	"altalune.id/othra/internal/render"
)

// ClientFn returns the SigningKey Connect client for a command; the caller resolves URL and token.
type ClientFn func(cmd *cobra.Command) (keysv1connect.SigningKeyServiceClient, error)

// NewCmd builds the `keys` command tree, mounted under `project`.
func NewCmd(clientFn ClientFn) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "keys",
		Short: "List and rotate a project's signing keys",
	}
	cmd.AddCommand(newListCmd(clientFn), newRotateCmd(clientFn))
	return cmd
}

func newListCmd(clientFn ClientFn) *cobra.Command {
	return &cobra.Command{
		Use:   "list <project-id>",
		Short: "List a project's signing keys",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFn(cmd)
			if err != nil {
				return err
			}
			resp, err := client.ListSigningKeys(cmd.Context(), connect.NewRequest(&keysv1.ListSigningKeysRequest{ProjectId: args[0]}))
			if err != nil {
				return err
			}
			keys := resp.Msg.GetKeys()
			if isJSON(cmd) {
				out := make([]map[string]any, 0, len(keys))
				for _, k := range keys {
					out = append(out, keyMap(k))
				}
				return render.JSON(cmd.OutOrStdout(), out)
			}
			rows := make([][]string, 0, len(keys))
			for _, k := range keys {
				rows = append(rows, []string{k.GetPublicId(), k.GetKid(), k.GetAlg(), statusName(k.GetStatus()),
					stamp(k.GetCreatedAt()), stamp(k.GetActivatesAt()), stamp(k.GetRetireAfter())})
			}
			return render.Table(cmd.OutOrStdout(), []string{"ID", "KID", "ALG", "STATUS", "CREATED", "ACTIVATES", "RETIRES"}, rows)
		},
	}
}

func newRotateCmd(clientFn ClientFn) *cobra.Command {
	var now bool
	cmd := &cobra.Command{
		Use:   "rotate <project-id>",
		Short: "Rotate a project's signing key",
		Long: "Publish a new signing key; it starts signing after the pre-publication delay (10 minutes by default).\n" +
			"--now makes it sign at once: verifiers holding a cached key set reject new tokens until they fetch it again.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFn(cmd)
			if err != nil {
				return err
			}
			req := connect.NewRequest(&keysv1.RotateSigningKeyRequest{ProjectId: args[0], Immediate: now})
			resp, err := client.RotateSigningKey(cmd.Context(), req)
			if err != nil {
				return err
			}
			k := resp.Msg.GetKey()
			if isJSON(cmd) {
				return render.JSON(cmd.OutOrStdout(), keyMap(k))
			}
			if k.GetStatus() == keysv1.SigningKeyStatus_SIGNING_KEY_STATUS_NEXT {
				cmd.Printf("Published key %s; it starts signing at %s\n", k.GetKid(), stamp(k.GetActivatesAt()))
				return nil
			}
			cmd.Printf("Key %s signs now\n", k.GetKid())
			return nil
		},
	}
	cmd.Flags().BoolVar(&now, "now", false, "Make the new key sign at once (emergency)")
	return cmd
}

func keyMap(k *keysv1.SigningKey) map[string]any {
	return map[string]any{
		"id": k.GetPublicId(), "kid": k.GetKid(), "alg": k.GetAlg(), "status": statusName(k.GetStatus()),
		"createdAt": stamp(k.GetCreatedAt()), "activatesAt": stamp(k.GetActivatesAt()),
		"activatedAt": stamp(k.GetActivatedAt()), "retireAfter": stamp(k.GetRetireAfter()),
	}
}

func statusName(s keysv1.SigningKeyStatus) string {
	return strings.ToLower(strings.TrimPrefix(s.String(), "SIGNING_KEY_STATUS_"))
}

func stamp(t *timestamppb.Timestamp) string {
	if t == nil {
		return ""
	}
	return t.AsTime().UTC().Format(time.RFC3339)
}

func isJSON(cmd *cobra.Command) bool {
	switch render.Detect(cmd) {
	case render.FormatJSON, render.FormatNDJSON:
		return true
	default:
		return false
	}
}
