export interface WebhookEvent {
  name: string;
  labelKey: string;
}

// Keep in sync with the Go events catalog (internal/kernel/events).
export const webhookEvents: WebhookEvent[] = [
  { name: 'invite.created', labelKey: 'webhooks.event_invite_created' },
  { name: 'member.added', labelKey: 'webhooks.event_member_added' },
];
