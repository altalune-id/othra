import { describe, expect, it } from 'vitest';

import { t } from '#test/nuxt-globals';
import { linkDialogLabels, linkListLabels } from './labels';

describe('link labels', () => {
  it('names the other side of the link on each page', () => {
    expect(linkListLabels(t, 'client').title).toBe('links.title_resource_servers');
    expect(linkListLabels(t, 'resourceServer').link).toBe('links.link_client');
    expect(linkListLabels(t, 'client').emptyReadOnly).toBe('links.empty_readonly');
  });

  it('titles the dialog by page and mode', () => {
    expect(linkDialogLabels(t, 'client', false)).toMatchObject({ title: 'links.dialog_resource_server', target: 'links.resource_server', confirm: 'links.link', noneLeft: 'links.none_left_resource_servers' });
    expect(linkDialogLabels(t, 'resourceServer', false)).toMatchObject({ title: 'links.dialog_client', target: 'links.client', noneLeft: 'links.none_left_clients' });
    expect(linkDialogLabels(t, 'resourceServer', true)).toMatchObject({ title: 'links.edit_title', confirm: 'common.save' });
  });
});
