import type { GoogleUpstreamCardLabels } from './GoogleUpstreamCard.vue';

type Translate = (key: string, params?: Record<string, unknown>) => string;

// googleUpstreamCardLabels translates the Google upstream card.
export function googleUpstreamCardLabels(t: Translate): GoogleUpstreamCardLabels {
  return {
    title: t('signin.google'),
    upstream: t('signin.upstream_label'),
    intro: t('signin.google_intro'),
    setup: t('signin.google_setup'),
    redirectUri: t('signin.redirect_uri'),
    redirectHelp: t('signin.redirect_help'),
    copy: t('common.copy_label', { label: '{label}' }),
    clientId: t('signin.client_id'),
    clientIdHelp: t('signin.client_id_help'),
    clientSecret: t('signin.client_secret'),
    secretHelp: t('signin.secret_help'),
    secretSet: t('signin.secret_set'),
    secretReplace: t('signin.secret_replace'),
    enabled: t('signin.enabled'),
    enabledHelp: t('signin.enabled_help'),
    locked: t('signin.google_locked'),
    save: t('common.save'),
    saving: t('common.saving'),
    saved: t('common.saved'),
    cancel: t('common.cancel'),
    remove: t('signin.google_remove'),
    removeLocked: t('signin.remove_locked'),
    turnOffInstead: t('signin.turn_off_instead'),
    on: t('signin.on'),
    off: t('signin.off'),
  };
}
