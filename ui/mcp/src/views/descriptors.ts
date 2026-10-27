import type { Descriptor, Field } from '../generic/descriptor';
import { t } from '../copy';
import { formatValue } from '../generic/descriptor';
import { endUserList } from './endusers';
import { roles } from './members';

const clientKinds = {
  CLIENT_KIND_WEB: { label: t('clients.kind_web'), tone: 'neutral' },
  CLIENT_KIND_CLI: { label: t('clients.kind_native'), tone: 'neutral' },
} as const;

const registrations = {
  CLIENT_REGISTRATION_REGISTERED: { label: '', tone: 'ghost' },
  CLIENT_REGISTRATION_CIMD: { label: t('mcp.metadata_document'), tone: 'warning' },
  CLIENT_REGISTRATION_SYSTEM: { label: t('common.system'), tone: 'info' },
} as const;

// secretHints sums up a client's secrets by their last characters, never the secrets themselves.
export function secretHints(value: unknown): string {
  const secrets = Array.isArray(value) ? (value as { hint?: string }[]) : [];
  if (secrets.length === 0) {
    return t('mcp.no_secrets');
  }
  return secrets.map(s => `…${s.hint ?? ''}`).join(', ');
}

const consoleLink: Field = { key: 'consoleUrl', label: t('mcp.manage_secrets'), kind: 'link', hideEmpty: true };

const clientFields: Field[] = [
  { key: 'name', label: t('common.name') },
  { key: 'id', label: t('field.client_id'), kind: 'id' },
  { key: 'kind', label: t('clients.kind'), kind: 'badge', badges: clientKinds },
  { key: 'registration', label: t('mcp.registered_by'), kind: 'badge', badges: registrations, hideEmpty: true },
  { key: 'redirectUris', label: t('field.redirect_uris'), kind: 'list' },
  { key: 'postLogoutRedirectUris', label: t('field.post_logout_redirect_uris'), kind: 'list', hideEmpty: true },
  { key: 'allowedScopes', label: t('rs.scopes'), kind: 'list' },
  { key: 'tokenEndpointAuthMethod', label: t('mcp.token_auth'), kind: 'mono' },
  { key: 'pkceRequired', label: t('mcp.pkce_required'), kind: 'bool' },
  { key: 'consentPolicy', label: t('field.consent_policy'), kind: 'badge', badges: {
    CONSENT_POLICY_REQUIRE: { label: t('mcp.consent_require'), tone: 'ghost' },
    CONSENT_POLICY_IMPLICIT: { label: t('mcp.consent_implicit'), tone: 'ghost' },
  } },
  { key: 'refreshTokens', label: t('clients.refresh'), kind: 'bool' },
  { key: 'secrets', label: t('secrets.title'), format: secretHints },
  { key: 'metadataUrl', label: t('mcp.metadata_document'), kind: 'url', hideEmpty: true },
  { key: 'createdAt', label: t('mcp.created'), kind: 'date' },
  consoleLink,
];

const rsStates = {
  RESOURCE_SERVER_STATE_PENDING: { label: t('rs.state_pending'), tone: 'warning' },
  RESOURCE_SERVER_STATE_VERIFIED: { label: t('rs.state_verified'), tone: 'success' },
  RESOURCE_SERVER_STATE_FAILED: { label: t('rs.state_failed'), tone: 'error' },
} as const;

const trustedBadge = { true: { label: t('rs.state_trusted'), tone: 'info' } } as const;

interface Failure {
  reason?: string;
  status?: number;
  resource?: string;
}

// failureText says why a resource server's metadata check failed, in the console's words.
export function failureText(failure: Failure | undefined, identifier: unknown): string {
  const reason = failure?.reason ?? 'unknown';
  const key = `rs.reason_${reason}`;
  const params = { status: failure?.status, got: failure?.resource, uri: identifier, issuer: t('mcp.this_issuer') };
  const text = t(key, params);
  return text === key ? t('rs.reason_unknown') : text;
}

const rsFields: Field[] = [
  { key: 'name', label: t('common.name') },
  { key: 'id', label: t('common.id'), kind: 'id' },
  { key: 'identifier', label: t('field.identifier'), kind: 'url' },
  { key: 'state', label: t('mcp.col_state'), kind: 'badge', badges: rsStates },
  { key: 'trusted', label: t('mcp.col_trust'), kind: 'badge', badges: trustedBadge, hideEmpty: true },
  { key: 'scopes', label: t('rs.scopes'), kind: 'list' },
  { key: 'metadataUrl', label: t('rs.metadata_url'), kind: 'url' },
  { key: 'verifiedAt', label: t('rs.verified_at'), kind: 'date', hideEmpty: true },
  { key: 'lastCheckedAt', label: t('mcp.last_checked'), kind: 'date', hideEmpty: true },
  { key: 'acceptPersonalTokens', label: t('endusers.tokens'), kind: 'bool' },
  { key: 'createdAt', label: t('mcp.created'), kind: 'date' },
];

function failedNote(rs: Record<string, unknown>): string {
  return rs.state === 'RESOURCE_SERVER_STATE_FAILED' ? `${failureText(rs.lastError as Failure, rs.identifier)} ${t('rs.failed_body')}` : '';
}

const keyStatuses = {
  SIGNING_KEY_STATUS_NEXT: { label: t('keys.status_next'), tone: 'info' },
  SIGNING_KEY_STATUS_ACTIVE: { label: t('keys.status_active'), tone: 'success' },
  SIGNING_KEY_STATUS_RETIRING: { label: t('keys.status_retiring'), tone: 'warning' },
  SIGNING_KEY_STATUS_RETIRED: { label: t('keys.status_retired'), tone: 'ghost' },
} as const;

const keyFields: Field[] = [
  { key: 'kid', label: t('field.key_id'), kind: 'mono' },
  { key: 'status', label: t('keys.col_status'), kind: 'badge', badges: keyStatuses },
  { key: 'alg', label: t('keys.col_alg'), kind: 'mono' },
  { key: 'createdAt', label: t('mcp.created'), kind: 'date' },
  { key: 'activatesAt', label: t('mcp.starts_signing'), kind: 'date', hideEmpty: true },
  { key: 'retireAfter', label: t('keys.col_retires'), kind: 'date', hideEmpty: true },
];

// rotatedNote says when a newly rotated key starts signing, as the console does.
export function rotatedNote(key: Record<string, unknown>): string {
  if (key.status === 'SIGNING_KEY_STATUS_ACTIVE') {
    return t('keys.rotated_active', { kid: key.kid });
  }
  return t('keys.rotated', { kid: key.kid, time: formatValue({ key: 'activatesAt', label: '', kind: 'date' }, key.activatesAt) });
}

const upstreamKinds = { UPSTREAM_KIND_GOOGLE: { label: t('methods.google'), tone: 'neutral' } } as const;

const upstreamFields: Field[] = [
  { key: 'kind', label: t('signin.upstream_label'), kind: 'badge', badges: upstreamKinds },
  { key: 'enabled', label: t('signin.enabled'), kind: 'bool' },
  { key: 'clientId', label: t('field.client_id'), kind: 'mono' },
  { key: 'secretSet', label: t('mcp.secret_set'), kind: 'bool' },
  { key: 'redirectUri', label: t('mcp.redirect_uri'), kind: 'url' },
  { key: 'identitiesLinked', label: t('mcp.linked_end_users'), kind: 'bool' },
  { key: 'id', label: t('common.id'), kind: 'id' },
  { key: 'createdAt', label: t('mcp.created'), kind: 'date' },
];

const systemBadge = { true: { label: t('common.system'), tone: 'info' } } as const;

const orgFields: Field[] = [
  { key: 'name', label: t('common.name') },
  { key: 'slug', label: t('common.slug'), kind: 'mono' },
  { key: 'id', label: t('common.id'), kind: 'id' },
  { key: 'role', label: t('mcp.your_role'), kind: 'badge', badges: roles, hideEmpty: true },
  { key: 'system', label: t('clients.kind'), kind: 'badge', badges: systemBadge, hideEmpty: true },
  { key: 'createdAt', label: t('mcp.created'), kind: 'date' },
];

const projectFields: Field[] = [
  { key: 'name', label: t('common.name') },
  { key: 'slug', label: t('common.slug'), kind: 'mono' },
  { key: 'id', label: t('common.id'), kind: 'id' },
  { key: 'issuerUrl', label: t('clients.issuer'), kind: 'url' },
  { key: 'createdAt', label: t('mcp.created'), kind: 'date' },
];

const webhookFields: Field[] = [
  { key: 'url', label: t('field.url'), kind: 'url' },
  { key: 'description', label: t('common.description'), hideEmpty: true },
  { key: 'eventTypes', label: t('field.event_types'), kind: 'list' },
  { key: 'active', label: t('tokens.status_active'), kind: 'bool' },
  { key: 'id', label: t('common.id'), kind: 'id' },
];

const deliveryStatuses = {
  pending: { label: t('mcp.delivery_pending'), tone: 'warning' },
  delivered: { label: t('mcp.delivery_delivered'), tone: 'success' },
  failed: { label: t('mcp.delivery_failed'), tone: 'error' },
} as const;

// descriptors describes each tool a generic view renders: where its data is and how each field shows.
export const descriptors: Record<string, Descriptor> = {
  org_list: {
    kind: 'list',
    title: t('org.title'),
    rows: 'orgs',
    key: 'id',
    empty: t('mcp.empty_orgs'),
    columns: [
      { key: 'name', label: t('common.name') },
      { key: 'slug', label: t('common.slug'), kind: 'mono' },
      { key: 'role', label: t('mcp.your_role'), kind: 'badge', badges: roles },
      { key: 'id', label: t('common.id'), kind: 'id' },
    ],
    refresh: { tool: 'org_list', args: [] },
  },
  client_list: {
    kind: 'list',
    title: t('mcp.title_clients'),
    rows: 'clients',
    key: 'id',
    empty: t('mcp.empty_clients'),
    columns: [
      { key: 'name', label: t('common.name') },
      { key: 'kind', label: t('clients.kind'), kind: 'badge', badges: clientKinds },
      { key: 'registration', label: t('mcp.registered_by'), kind: 'badge', badges: registrations },
      { key: 'id', label: t('field.client_id'), kind: 'id' },
      { key: 'secrets', label: t('secrets.title'), format: secretHints },
    ],
    refresh: { tool: 'client_list', args: ['projectId'] },
  },
  client_get: {
    kind: 'detail',
    title: t('mcp.title_client'),
    object: 'client',
    fields: clientFields,
  },
  client_create: {
    kind: 'result',
    title: t('mcp.client_created'),
    object: 'client',
    fields: clientFields.filter(f => ['name', 'id', 'kind', 'redirectUris', 'allowedScopes', 'tokenEndpointAuthMethod', 'consoleUrl'].includes(f.key)),
    note: client => (client.kind === 'CLIENT_KIND_WEB'
      ? t('mcp.client_created_web')
      : t('mcp.client_created_cli')),
  },
  resource_server_list: {
    kind: 'list',
    title: t('rs.title'),
    rows: 'resourceServers',
    key: 'id',
    empty: t('mcp.empty_rs'),
    columns: [
      { key: 'name', label: t('common.name') },
      { key: 'identifier', label: t('field.identifier'), kind: 'url' },
      { key: 'state', label: t('mcp.col_state'), kind: 'badge', badges: rsStates },
      { key: 'trusted', label: t('mcp.col_trust'), kind: 'badge', badges: trustedBadge },
      { key: 'scopes', label: t('rs.scopes'), kind: 'list' },
    ],
    refresh: { tool: 'resource_server_list', args: ['projectId'] },
  },
  resource_server_get: {
    kind: 'detail',
    title: t('links.resource_server'),
    object: 'resourceServer',
    fields: rsFields,
    note: failedNote,
  },
  resource_server_create: {
    kind: 'result',
    title: t('mcp.rs_created'),
    object: 'resourceServer',
    fields: rsFields.filter(f => ['name', 'id', 'identifier', 'state', 'scopes', 'metadataUrl'].includes(f.key)),
    note: rs => (rs.state === 'RESOURCE_SERVER_STATE_VERIFIED' ? '' : t('mcp.rs_verify_next')),
  },
  resource_server_verify: {
    kind: 'detail',
    title: t('mcp.title_verification'),
    fields: [
      { key: 'passed', label: t('mcp.result'), kind: 'badge', badges: { 'true': { label: t('mcp.verify_passed'), tone: 'success' }, '': { label: t('mcp.verify_failed'), tone: 'error' } } },
      { key: 'resourceServer.name', label: t('common.name') },
      { key: 'resourceServer.identifier', label: t('field.identifier'), kind: 'url' },
      { key: 'resourceServer.metadataUrl', label: t('rs.metadata_url'), kind: 'url' },
      { key: 'resourceServer.state', label: t('mcp.col_state'), kind: 'badge', badges: rsStates },
    ],
    note: out => (out.passed ? '' : `${failureText(out.error as Failure, (out.resourceServer as Record<string, unknown> | undefined)?.identifier)} ${t('rs.failed_body')}`),
  },
  resource_server_link_list: {
    kind: 'list',
    title: t('mcp.title_links'),
    rows: 'links',
    key: ['clientId', 'resourceServerId'],
    empty: t('mcp.empty_links'),
    columns: [
      { key: 'clientName', label: t('mcp.title_client') },
      { key: 'resourceServerName', label: t('links.resource_server') },
      { key: 'resourceServerIdentifier', label: t('field.identifier'), kind: 'url' },
      { key: 'resourceServerState', label: t('mcp.col_state'), kind: 'badge', badges: rsStates },
      { key: 'scopes', label: t('rs.scopes'), kind: 'list' },
    ],
    refresh: { tool: 'resource_server_link_list', args: ['projectId', 'clientId', 'resourceServerId'] },
  },
  signing_key_list: {
    kind: 'list',
    title: t('keys.title'),
    rows: 'keys',
    key: 'publicId',
    empty: t('mcp.empty_keys'),
    columns: keyFields,
    refresh: { tool: 'signing_key_list', args: ['projectId'] },
  },
  signing_key_rotate: {
    kind: 'result',
    title: t('mcp.key_rotated'),
    object: 'key',
    fields: keyFields,
    note: rotatedNote,
  },
  upstream_list: {
    kind: 'list',
    title: t('mcp.title_upstreams'),
    rows: 'upstreams',
    key: 'id',
    empty: t('mcp.empty_upstreams'),
    columns: upstreamFields.filter(f => ['kind', 'enabled', 'clientId', 'redirectUri', 'identitiesLinked'].includes(f.key)),
    refresh: { tool: 'upstream_list', args: ['projectId'] },
  },
  upstream_get: {
    kind: 'detail',
    title: t('signin.upstream_label'),
    object: 'upstream',
    fields: upstreamFields,
    note: up => (up.secretSet ? '' : t('mcp.upstream_secret_missing')),
  },
  end_user_list: endUserList,
  org_get: { kind: 'detail', title: t('mcp.title_org'), object: 'org', fields: orgFields },
  org_create: { kind: 'result', title: t('mcp.org_created'), object: 'org', fields: orgFields },
  project_list: {
    kind: 'list',
    title: t('project.title'),
    rows: 'projects',
    key: 'id',
    empty: t('mcp.empty_projects'),
    columns: [
      { key: 'name', label: t('common.name') },
      { key: 'slug', label: t('common.slug'), kind: 'mono' },
      { key: 'system', label: t('clients.kind'), kind: 'badge', badges: systemBadge },
      { key: 'id', label: t('common.id'), kind: 'id' },
    ],
    refresh: { tool: 'project_list', args: ['orgId'] },
  },
  project_create: { kind: 'result', title: t('mcp.project_created'), object: 'project', fields: projectFields },
  member_list: {
    kind: 'list',
    title: t('members.title'),
    rows: 'members',
    key: 'userId',
    empty: t('mcp.empty_members'),
    columns: [
      { key: 'name', label: t('common.name') },
      { key: 'email', label: t('field.email') },
      { key: 'role', label: t('field.role'), kind: 'badge', badges: roles },
      { key: 'joinedAt', label: t('mcp.col_joined'), kind: 'date' },
    ],
    refresh: { tool: 'member_list', args: ['orgId'] },
  },
  webhook_list: {
    kind: 'list',
    title: t('webhooks.title'),
    rows: 'webhooks',
    key: 'id',
    empty: t('mcp.empty_webhooks'),
    columns: webhookFields.filter(f => f.key !== 'description'),
    refresh: { tool: 'webhook_list', args: ['orgId', 'projectId'] },
  },
  webhook_create: {
    kind: 'result',
    title: t('mcp.webhook_created'),
    fields: [
      ...webhookFields.map(f => ({ ...f, key: `webhook.${f.key}` })),
      { key: 'secret', label: t('mcp.signing_secret'), kind: 'secret' },
    ],
    note: () => t('mcp.webhook_secret_once'),
  },
  webhook_deliveries: {
    kind: 'list',
    title: t('webhooks.deliveries'),
    rows: 'deliveries',
    key: 'id',
    empty: t('webhooks.no_deliveries'),
    columns: [
      { key: 'eventType', label: t('webhooks.col_event'), kind: 'mono' },
      { key: 'status', label: t('keys.col_status'), kind: 'badge', badges: deliveryStatuses },
      { key: 'attempt', label: t('webhooks.col_attempt') },
      { key: 'lastError', label: t('mcp.last_error') },
      { key: 'createdAt', label: t('mcp.created'), kind: 'date' },
    ],
    refresh: { tool: 'webhook_deliveries', args: ['orgId', 'projectId', 'id'] },
  },
  webhook_retry_failed: {
    kind: 'result',
    title: t('mcp.retry_title'),
    fields: [{ key: 'requeued', label: t('mcp.requeued'), format: v => String(v ?? 0) }],
  },
  member_invite_confirm: {
    kind: 'result',
    title: t('mcp.invite_sent'),
    object: 'invite',
    fields: [
      { key: 'email', label: t('field.email') },
      { key: 'role', label: t('field.role'), kind: 'badge', badges: roles },
      { key: 'expiresAt', label: t('mcp.col_expires'), kind: 'date' },
    ],
  },
  member_invite_discard: {
    kind: 'result',
    title: t('mcp.invite_discarded'),
    fields: [],
    note: () => t('mcp.invite_discarded_body'),
  },
  project_get: {
    kind: 'detail',
    title: t('mcp.title_project'),
    object: 'project',
    fields: [
      { key: 'name', label: t('common.name') },
      { key: 'slug', label: t('common.slug'), kind: 'mono' },
      { key: 'id', label: t('common.id'), kind: 'id' },
      { key: 'system', label: t('clients.kind'), kind: 'badge', badges: { true: { label: t('common.system'), tone: 'info' } }, hideEmpty: true },
      { key: 'issuerUrl', label: t('clients.issuer'), kind: 'url' },
      { key: 'issuerHandleLocked', label: t('mcp.handle_locked'), kind: 'bool' },
      { key: 'displayName', label: t('field.display_name'), hideEmpty: true },
      { key: 'supportEmail', label: t('field.support_email'), hideEmpty: true },
      { key: 'termsUrl', label: t('mcp.terms'), kind: 'url', hideEmpty: true },
      { key: 'privacyUrl', label: t('mcp.privacy'), kind: 'url', hideEmpty: true },
      { key: 'signinEmailLink', label: t('mcp.email_link_signin'), kind: 'bool' },
      { key: 'createdAt', label: t('mcp.created'), kind: 'date' },
    ],
  },
};
