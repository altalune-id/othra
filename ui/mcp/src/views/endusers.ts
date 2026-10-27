import type { Field, ListDescriptor, RowAction } from '../generic/descriptor';
import { t } from '../copy';
import { pick } from '../generic/descriptor';

export interface Section {
  title: string;
  help?: string;
  rows: string;
  key: string | string[];
  empty: string;
  columns: Field[];
  actions?: RowAction[];
}

const methodKinds = {
  SIGN_IN_METHOD_KIND_EMAIL_LINK: { label: t('methods.email'), tone: 'neutral' },
  SIGN_IN_METHOD_KIND_GOOGLE: { label: t('methods.google'), tone: 'neutral' },
} as const;

const tokenStates = {
  PERSONAL_TOKEN_STATE_ACTIVE: { label: t('tokens.status_active'), tone: 'success' },
  PERSONAL_TOKEN_STATE_EXPIRED: { label: t('tokens.status_expired'), tone: 'ghost' },
  PERSONAL_TOKEN_STATE_REVOKED: { label: t('tokens.status_revoked'), tone: 'ghost' },
} as const;

const who = (input: Record<string, unknown>) => pick(input, ['projectId', 'endUserId']);

export const endUserFields: Field[] = [
  { key: 'email', label: t('endusers.email') },
  { key: 'name', label: t('common.name'), hideEmpty: true },
  { key: 'id', label: t('endusers.sub'), kind: 'id' },
  { key: 'createdAt', label: t('endusers.created'), kind: 'date' },
  { key: 'lastSignInAt', label: t('endusers.last_sign_in'), kind: 'date', hideEmpty: true },
];

// endUserSections are the four parts of end_user_get, each with the revoke its rows allow.
export const endUserSections: Section[] = [
  {
    title: t('endusers.methods'),
    help: t('endusers.methods_help'),
    rows: 'methods',
    key: ['kind', 'identityId'],
    empty: t('mcp.empty_methods'),
    columns: [
      { key: 'kind', label: t('mcp.col_method'), kind: 'badge', badges: methodKinds },
      { key: 'linked', label: t('mcp.linked'), kind: 'bool' },
      { key: 'email', label: t('endusers.email') },
      { key: 'lastUsedAt', label: t('apikeys.col_last_used'), kind: 'date' },
    ],
  },
  {
    title: t('endusers.sessions'),
    rows: 'sessions',
    key: 'id',
    empty: t('endusers.sessions_empty'),
    columns: [
      { key: 'method', label: t('mcp.col_method'), kind: 'badge', badges: methodKinds },
      { key: 'userAgent', label: t('mcp.col_device') },
      { key: 'authTime', label: t('mcp.signed_in'), kind: 'date' },
      { key: 'lastSeenAt', label: t('mcp.last_seen'), kind: 'date' },
    ],
    actions: [{
      label: t('endusers.sign_out'),
      tool: 'end_user_session_revoke',
      args: (row, input) => ({ ...who(input), sessionId: row.id }),
      confirm: { title: () => t('mcp.sign_out_title'), body: () => t('endusers.sign_out_body'), button: t('endusers.sign_out') },
    }],
  },
  {
    title: t('endusers.apps'),
    rows: 'apps',
    key: 'clientId',
    empty: t('endusers.apps_empty'),
    columns: [
      { key: 'name', label: t('mcp.col_app') },
      { key: 'clientId', label: t('field.client_id'), kind: 'mono' },
      { key: 'connectedAt', label: t('mcp.col_connected'), kind: 'date' },
      { key: 'lastUsedAt', label: t('apikeys.col_last_used'), kind: 'date' },
    ],
    actions: [{
      label: t('apps.disconnect'),
      tool: 'end_user_app_disconnect',
      args: (row, input) => ({ ...who(input), clientId: row.clientId }),
      confirm: { title: row => t('apps.disconnect_title', { app: row.name ?? row.clientId }), body: () => t('endusers.disconnect_body'), button: t('apps.disconnect') },
    }],
  },
  {
    title: t('endusers.tokens'),
    rows: 'tokens',
    key: 'id',
    empty: t('endusers.tokens_empty'),
    columns: [
      { key: 'name', label: t('common.name') },
      { key: 'hint', label: t('mcp.col_ends_in'), format: v => (v ? `…${String(v)}` : '—') },
      { key: 'state', label: t('mcp.col_state'), kind: 'badge', badges: tokenStates },
      { key: 'server.name', label: t('mcp.col_for') },
      { key: 'expiresAt', label: t('mcp.col_expires'), kind: 'date' },
    ],
    actions: [{
      label: t('common.revoke'),
      tool: 'end_user_token_revoke',
      args: (row, input) => ({ ...who(input), tokenId: row.id }),
      confirm: { title: () => t('tokens.revoke_title'), body: row => t('tokens.revoke_body', { name: row.name ?? row.hint }), button: t('common.revoke') },
      when: row => row.state === 'PERSONAL_TOKEN_STATE_ACTIVE',
    }],
  },
];

export const endUserList: ListDescriptor = {
  kind: 'list',
  title: t('endusers.title'),
  rows: 'endUsers',
  key: 'id',
  empty: t('endusers.empty'),
  columns: endUserFields,
  refresh: { tool: 'end_user_list', args: ['projectId', 'query'] },
  more: { next: 'next', arg: 'after' },
};
