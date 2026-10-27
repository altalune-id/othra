package views

import (
	"context"
	"strconv"
	"strings"

	"github.com/altalune-id/othra/internal/web/i18n"
)

// MailData is the magic-link email: adding Google when GoogleEmail is set, sign-up when the email has no end user yet, else sign-in.
type MailData struct {
	Project     string
	Link        string
	Minutes     int
	SignUp      bool
	HasReplyTo  bool
	Primary     string
	GoogleEmail string
	// GoogleSignUp is a first Google sign-in whose inbox the link proves; it needs GoogleEmail.
	GoogleSignUp bool
}

func (m MailData) key(base string) string {
	if m.GoogleSignUp {
		return base + "_google_sign_up"
	}
	if m.GoogleEmail != "" {
		return base + "_link"
	}
	if m.SignUp {
		return base + "_sign_up"
	}
	return base + "_sign_in"
}

// MailSubject is the magic-link email's subject in ctx's locale.
func MailSubject(ctx context.Context, m MailData) string {
	return i18n.T(ctx, m.key("mail.subject"), "project", m.Project)
}

func (m MailData) intro(ctx context.Context) string {
	return i18n.T(ctx, m.key("mail.intro"), "project", m.Project, "google_email", m.GoogleEmail)
}

// MailText is the plain-text part of the magic-link email in ctx's locale.
func MailText(ctx context.Context, m MailData) string {
	var b strings.Builder
	b.WriteString(m.intro(ctx) + "\n\n")
	b.WriteString(i18n.T(ctx, m.key("mail.action"), "minutes", strconv.Itoa(m.Minutes)) + "\n")
	b.WriteString(m.Link + "\n\n")
	b.WriteString(i18n.T(ctx, m.key("mail.ignore")) + "\n\n")
	if m.HasReplyTo {
		b.WriteString(i18n.T(ctx, "mail.reply") + "\n")
	}
	b.WriteString(i18n.T(ctx, "mail.footer", "project", m.Project) + "\n")
	return b.String()
}
