package views

import (
	"context"
	"strconv"
	"strings"

	"altalune.id/othra/internal/web/i18n"
)

// MailData is the magic-link email: sign-up when the email has no end user yet, else sign-in.
type MailData struct {
	Project    string
	Link       string
	Minutes    int
	SignUp     bool
	HasReplyTo bool
	Primary    string
}

func (m MailData) key(base string) string {
	if m.SignUp {
		return base + "_sign_up"
	}
	return base + "_sign_in"
}

// MailSubject is the magic-link email's subject in ctx's locale.
func MailSubject(ctx context.Context, m MailData) string {
	return i18n.T(ctx, m.key("mail.subject"), "project", m.Project)
}

// MailText is the plain-text part of the magic-link email in ctx's locale.
func MailText(ctx context.Context, m MailData) string {
	var b strings.Builder
	b.WriteString(i18n.T(ctx, m.key("mail.intro"), "project", m.Project) + "\n\n")
	b.WriteString(i18n.T(ctx, m.key("mail.action"), "minutes", strconv.Itoa(m.Minutes)) + "\n")
	b.WriteString(m.Link + "\n\n")
	b.WriteString(i18n.T(ctx, m.key("mail.ignore")) + "\n\n")
	if m.HasReplyTo {
		b.WriteString(i18n.T(ctx, "mail.reply") + "\n")
	}
	b.WriteString(i18n.T(ctx, "mail.footer", "project", m.Project) + "\n")
	return b.String()
}
