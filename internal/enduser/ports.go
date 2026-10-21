package enduser

import (
	"context"

	"github.com/google/uuid"

	"altalune.id/othra/mailer"
)

// MailQueue enqueues a message in the unit of work on ctx.
type MailQueue interface {
	Enqueue(ctx context.Context, m mailer.Message) (uuid.UUID, error)
}

// Projects runs fn once for every project, with ctx scoped to its org and the project.
type Projects interface {
	Each(ctx context.Context, fn func(ctx context.Context, orgID, projectID uuid.UUID) error) error
}

// ComposeFunc builds the sign-in email for a freshly issued link.
type ComposeFunc func(ctx context.Context, link IssuedLink) (mailer.Message, error)
