// @vitest-environment happy-dom
import { mount } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import { beforeEach, describe, expect, it } from 'vitest';
import { defineComponent, h } from 'vue';

import { ProjectEnvironment } from '#gen/project/v1/project_pb';
import { setRoute } from '#test/nuxt-globals';
import ConsoleTopBar from '~/components/layout/ConsoleTopBar.vue';
import { useNavCrumbs } from '~/components/layout/useNavCrumbs';
import { useAuthStore } from '~/stores/auth';
import { useOrgsStore } from '~/stores/orgs';
import { useProjectsStore } from '~/stores/projects';
import { useUIStore } from '~/stores/ui';

beforeEach(() => {
  setActivePinia(createPinia());
  useAuthStore().me = { permissions: ['end_user:read'] } as never;
  useOrgsStore().items = [{ id: 'B6rTn1YkM8wQz3XpL5vGd', slug: 'acme', name: 'Acme', role: 'admin', permissions: [] }];
  useProjectsStore().items = [{ id: 'P1', orgId: 'B6rTn1YkM8wQz3XpL5vGd', slug: 'acme-prod', name: 'Acme', environment: ProjectEnvironment.LABS }] as never;
});

function crumbsAt(path: string, params: Record<string, string>) {
  setRoute({ path, params });
  let out!: ReturnType<typeof useNavCrumbs>;
  const probe = defineComponent({
    setup() {
      out = useNavCrumbs();
      return () => h('i');
    },
  });
  mount(probe);
  return out;
}

describe('nav crumbs', () => {
  it('trails org, mono project handle and the active section, and lists both navs in the palette', () => {
    const { crumbs, palette } = crumbsAt('/o/acme/projects/acme-prod/clients/k3V', { org: 'acme', project: 'acme-prod' });
    expect(crumbs.value).toEqual([
      { label: 'Acme', to: '/o/acme' },
      { label: 'acme-prod', to: '/o/acme/projects/acme-prod', mono: true },
      { label: 'nav.clients' },
    ]);
    expect(crumbsAt('/o/acme/projects/acme-prod', { org: 'acme', project: 'acme-prod' }).place.value).toEqual({ label: 'acme-prod', badge: 'environment.labs' });
    expect(palette.value).toContainEqual({ key: 'org-members', group: 'Acme', label: 'members.title', to: '/o/acme/settings/members' });
    expect(palette.value).toContainEqual({ key: 'project-clients', group: 'acme-prod', mono: true, label: 'nav.clients', to: '/o/acme/projects/acme-prod/clients' });
  });

  it('trails only the org and its page outside a project', () => {
    const { crumbs, palette } = crumbsAt('/o/acme/settings/members', { org: 'acme' });
    expect(crumbs.value).toEqual([{ label: 'Acme', to: '/o/acme' }, { label: 'members.title' }]);
    expect(crumbsAt('/o/acme', { org: 'acme' }).place.value).toEqual({ label: 'Acme' });
    expect(palette.value.every(p => p.group === 'Acme' && !p.mono)).toBe(true);
  });

  it('offers no org links where there is no org', () => {
    const { crumbs, palette } = crumbsAt('/orgs', {});
    expect(crumbs.value).toEqual([]);
    expect(palette.value).toEqual([]);
  });

  it('never links to an undefined org from the account pages', () => {
    const { palette } = crumbsAt('/account', {});
    expect(palette.value.some(p => p.to.includes('undefined'))).toBe(false);
  });

  it('trails Account and its page on the account pages', () => {
    const { crumbs, palette } = crumbsAt('/account/api-keys', {});
    expect(crumbs.value).toEqual([{ label: 'account.title', to: '/account' }, { label: 'apikeys.personal_title' }]);
    expect(crumbsAt('/account', {}).place.value).toEqual({ label: 'account.title' });
    expect(palette.value.map(p => p.to)).toEqual(['/account', '/account/api-keys']);
  });
});

describe('console top bar', () => {
  it('opens and closes the palette with Command or Control K', async () => {
    setRoute({ path: '/o/acme', params: { org: 'acme' } });
    const w = mount(ConsoleTopBar, { attachTo: document.body, global: { stubs: { ThemeFamilySwitcher: true, ThemeSwitcher: true } } });
    const dialog = () => w.get('dialog').element as HTMLDialogElement;
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'k', metaKey: true }));
    await w.vm.$nextTick();
    await w.vm.$nextTick();
    expect(dialog().open).toBe(true);
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'K', ctrlKey: true }));
    await w.vm.$nextTick();
    await w.vm.$nextTick();
    expect(dialog().open).toBe(false);
    useUIStore().navOpen = true;
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'k', ctrlKey: true }));
    await w.vm.$nextTick();
    await w.vm.$nextTick();
    expect(useUIStore().navOpen).toBe(false);
    expect(dialog().open).toBe(true);
    w.unmount();
  });
});
