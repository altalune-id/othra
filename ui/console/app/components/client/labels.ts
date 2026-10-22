import type { ClientFormLabels, ClientKindOption } from './ClientForm.vue';
import type { ClientSecretDialogLabels } from './ClientSecretDialog.vue';
import type { ClientSecretsLabels } from './ClientSecrets.vue';
import type { RotateSecretDialogLabels } from './RotateSecretDialog.vue';
import { clientKindNames, clientKinds } from './kinds';

type Translate = (key: string, params?: Record<string, unknown>) => string;

function uriList(t: Translate, field: string) {
  return {
    uri: t(`clients.${field}_n`),
    remove: t(`clients.${field}_remove`),
    add: t('clients.add_uri'),
    localhostWarning: t('clients.localhost_warning'),
  };
}

// clientFormLabels translates the create and edit form of an OAuth client.
export function clientFormLabels(t: Translate, mode: 'create' | 'edit'): ClientFormLabels {
  return {
    kind: t('clients.kind'),
    kindFixed: t('clients.kind_fixed'),
    name: t('common.name'),
    nameHelp: t('clients.name_help'),
    redirectUris: t('clients.redirect_uris'),
    postLogoutUris: t('clients.post_logout_uris'),
    postLogoutHelp: t('clients.post_logout_help'),
    refresh: t('clients.refresh'),
    refreshHelp: t('clients.refresh_help'),
    consent: t('clients.consent'),
    consentRequire: t('clients.consent_require'),
    consentImplicit: t('clients.consent_implicit'),
    consentImplicitWarning: t('clients.consent_implicit_warning'),
    consentRequireNote: t('clients.consent_require_note'),
    authMethod: t('clients.auth_method'),
    authMethodBasic: t('clients.auth_method_basic'),
    authMethodPost: t('clients.auth_method_post'),
    authMethodHelp: t('clients.auth_method_help'),
    pkce: t('clients.pkce'),
    pkceRequired: t('clients.pkce_required'),
    pkceOffWarning: t('clients.pkce_off_warning'),
    submit: mode === 'create' ? t('clients.create') : t('common.save'),
    submitting: mode === 'create' ? t('common.creating') : t('common.saving'),
    cancel: t('common.cancel'),
    uriList: uriList(t, 'redirect_uri'),
    postLogoutList: uriList(t, 'post_logout_uri'),
  };
}

// kindOptions lists every client kind for the picker, with its copy and redirect URI help.
export function kindOptions(t: Translate): ClientKindOption[] {
  return clientKindNames.map((value) => {
    const k = clientKinds[value];
    return { value, icon: k.icon, label: t(k.labelKey), help: t(k.helpKey), redirectHelp: t(k.redirectHelpKey), placeholder: k.placeholder, postLogout: k.postLogout, secret: k.secret };
  });
}

export function secretDialogLabels(t: Translate): ClientSecretDialogLabels {
  return {
    title: t('secret.title'),
    body: t('secret.body'),
    clientId: t('clients.client_id'),
    secret: t('secret.label'),
    copyClientId: t('secret.copy_client_id'),
    copySecret: t('secret.copy_secret'),
    confirm: t('secret.confirm'),
    closeDisabled: t('secret.close_disabled'),
    done: t('common.done'),
  };
}

export function secretsLabels(t: Translate): ClientSecretsLabels {
  return {
    title: t('secrets.title'),
    intro: t('secrets.intro'),
    current: t('secrets.status_current'),
    old: t('secrets.status_old', { date: '{date}' }),
    created: t('secrets.created', { date: '{date}' }),
    noSecret: t('clients.no_secret'),
    generate: t('clients.generate_secret'),
    rotate: t('secrets.rotate'),
    revoke: t('secrets.revoke_old'),
    personOnly: t('secrets.person_only'),
  };
}

export function rotateLabels(t: Translate): RotateSecretDialogLabels {
  return {
    title: t('secrets.rotate_title'),
    body: t('secrets.rotate_body'),
    grace: t('secrets.grace'),
    nowWarning: t('secrets.now_warning'),
    endsOld: t('secrets.rotate_ends_old', { hint: '{hint}' }),
    cancel: t('common.cancel'),
    confirm: t('secrets.rotate_confirm'),
  };
}
