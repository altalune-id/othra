import { describe, expect, it } from 'vitest';

import { accountBackLink } from './backLink';

const orgs = [{ id: 'V3StGXR8Z5jdHj6BmyT2a', slug: 'acme', name: 'Acme' }];

describe('accountBackLink', () => {
  it('goes back to the active org overview, by name', () => {
    expect(accountBackLink('acme', orgs)).toEqual({ to: '/o/acme', org: 'Acme' });
  });

  it('falls back to the slug when the org list has not loaded the active org', () => {
    expect(accountBackLink('globex', orgs)).toEqual({ to: '/o/globex', org: 'globex' });
  });

  it('goes to the org list when there is no active org', () => {
    expect(accountBackLink('', orgs)).toEqual({ to: '/orgs', org: '' });
    expect(accountBackLink(undefined, [])).toEqual({ to: '/orgs', org: '' });
  });
});
