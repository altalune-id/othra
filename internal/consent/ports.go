package consent

import "context"

// Grants revokes what a client holds for a consent once it is withdrawn.
type Grants interface {
	RevokeConsent(ctx context.Context, p Project, key Key) error
}
