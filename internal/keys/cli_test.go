package keys_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	keysv1 "altalune.id/othra/gen/go/keys/v1"
	keysv1connect "altalune.id/othra/gen/go/keys/v1/keysv1connect"
	"altalune.id/othra/internal/keys"
)

type stubHandler struct {
	keysv1connect.UnimplementedSigningKeyServiceHandler
	immediate bool
}

var created = timestamppb.New(time.Date(2026, 10, 18, 9, 0, 0, 0, time.UTC))

func (s *stubHandler) ListSigningKeys(context.Context, *connect.Request[keysv1.ListSigningKeysRequest]) (*connect.Response[keysv1.ListSigningKeysResponse], error) {
	return connect.NewResponse(&keysv1.ListSigningKeysResponse{Keys: []*keysv1.SigningKey{{
		PublicId: "pub1", Kid: "kid-1", Alg: "RS256", Status: keysv1.SigningKeyStatus_SIGNING_KEY_STATUS_ACTIVE, CreatedAt: created,
	}}}), nil
}

func (s *stubHandler) RotateSigningKey(_ context.Context, req *connect.Request[keysv1.RotateSigningKeyRequest]) (*connect.Response[keysv1.RotateSigningKeyResponse], error) {
	s.immediate = req.Msg.GetImmediate()
	if req.Msg.GetProjectId() == "missing" {
		return nil, connect.NewError(connect.CodeNotFound, nil)
	}
	k := &keysv1.SigningKey{PublicId: "pub2", Kid: "kid-2", Alg: "RS256", Status: keysv1.SigningKeyStatus_SIGNING_KEY_STATUS_NEXT,
		CreatedAt: created, ActivatesAt: timestamppb.New(created.AsTime().Add(10 * time.Minute))}
	if s.immediate {
		k.Status, k.ActivatesAt = keysv1.SigningKeyStatus_SIGNING_KEY_STATUS_ACTIVE, nil
	}
	return connect.NewResponse(&keysv1.RotateSigningKeyResponse{Key: k}), nil
}

func run(t *testing.T, h *stubHandler, args ...string) (*bytes.Buffer, error) {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle(keysv1connect.NewSigningKeyServiceHandler(h))
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	client := keysv1connect.NewSigningKeyServiceClient(ts.Client(), ts.URL)
	buf := new(bytes.Buffer)
	root := &cobra.Command{Use: "x", SilenceUsage: true, SilenceErrors: true}
	root.PersistentFlags().String("output", "", "")
	root.AddCommand(keys.NewCmd(func(*cobra.Command) (keysv1connect.SigningKeyServiceClient, error) { return client, nil }))
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs(append([]string{"keys"}, args...))
	return buf, root.ExecuteContext(t.Context())
}

func TestKeysList(t *testing.T) {
	buf, err := run(t, &stubHandler{}, "list", "prj1", "--output", "text")
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "kid-1")
	assert.Contains(t, buf.String(), "active")
	assert.Contains(t, buf.String(), "2026-10-18T09:00:00Z")

	buf, err = run(t, &stubHandler{}, "list", "prj1", "--output", "json")
	require.NoError(t, err)
	assert.Contains(t, buf.String(), `"kid": "kid-1"`)
	assert.Contains(t, buf.String(), `"status": "active"`)
}

func TestKeysRotate(t *testing.T) {
	h := &stubHandler{}
	buf, err := run(t, h, "rotate", "prj1", "--output", "text")
	require.NoError(t, err)
	assert.False(t, h.immediate)
	assert.Contains(t, buf.String(), "Published key kid-2; it starts signing at 2026-10-18T09:10:00Z")

	buf, err = run(t, h, "rotate", "prj1", "--now", "--output", "text")
	require.NoError(t, err)
	assert.True(t, h.immediate)
	assert.Contains(t, buf.String(), "Key kid-2 signs now")

	buf, err = run(t, h, "rotate", "prj1", "--output", "json")
	require.NoError(t, err)
	assert.Contains(t, buf.String(), `"status": "next"`)

	_, err = run(t, h, "rotate", "missing")
	assert.Error(t, err)
	_, err = run(t, h, "list")
	assert.Error(t, err, "the project id is required")
}
