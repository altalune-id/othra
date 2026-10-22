import { describe, expect, it } from 'vitest';

import { t } from '../../../test/nuxt-globals';
import { clientFormLabels, kindOptions, rotateLabels, secretDialogLabels, secretsLabels } from './labels';

describe('client labels', () => {
  it('names the submit button by mode and each URI list by its field', () => {
    const create = clientFormLabels(t, 'create');
    expect([create.submit, create.submitting]).toEqual(['clients.create', 'common.creating']);
    expect(create.uriList.uri).toBe('clients.redirect_uri_n');
    expect(create.postLogoutList.remove).toBe('clients.post_logout_uri_remove');
    const edit = clientFormLabels(t, 'edit');
    expect([edit.submit, edit.submitting]).toEqual(['common.save', 'common.saving']);
    expect(create.consentRequireNote).toBe('clients.consent_require_note');
    expect([create.authMethodPost, create.pkceOffWarning]).toEqual(['clients.auth_method_post', 'clients.pkce_off_warning']);
  });

  it('offers every kind from the kind map with its copy', () => {
    expect(kindOptions(t)).toEqual([
      { value: 'web', icon: 'lucide:globe', label: 'clients.kind_web', help: 'clients.kind_web_help', redirectHelp: 'clients.redirect_help_web', placeholder: 'https://app.example.com/oauth/callback', postLogout: true, secret: true },
      { value: 'cli', icon: 'lucide:terminal', label: 'clients.kind_native', help: 'clients.kind_native_help', redirectHelp: 'clients.redirect_help_native', placeholder: 'http://127.0.0.1/callback', postLogout: false, secret: false },
    ]);
  });

  it('keeps the placeholders the components fill in', () => {
    expect(secretsLabels(t).old).toBe('secrets.status_old|{"date":"{date}"}');
    expect(rotateLabels(t).endsOld).toBe('secrets.rotate_ends_old|{"hint":"{hint}"}');
    expect(secretDialogLabels(t).copySecret).toBe('secret.copy_secret');
  });
});
