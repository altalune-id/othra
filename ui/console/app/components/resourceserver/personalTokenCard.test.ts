// @vitest-environment happy-dom
import { mount } from '@vue/test-utils';
import { describe, expect, it } from 'vitest';
import PersonalTokenCard from './PersonalTokenCard.vue';

const base = { accept: false, canEdit: true, endpoint: 'https://id.acme.test/p/acme/oauth/introspect', copyLabel: 'Copy {label}' };

describe('personalTokenCard', () => {
  it('keeps a locked toggle looking like itself, refuses the click and says why in the server\'s words', async () => {
    const w = mount(PersonalTokenCard, { props: { ...base, lockReason: 'no_secret' } });
    const toggle = w.get('input[type="checkbox"]');
    expect(toggle.attributes('aria-disabled')).toBe('true');
    expect(toggle.attributes('disabled')).toBeUndefined();
    await toggle.trigger('click');
    expect((toggle.element as HTMLInputElement).checked).toBe(false);
    expect(w.emitted('set')).toBeUndefined();
    expect(w.text()).toContain('apperror.RSV012');
    expect(mount(PersonalTokenCard, { props: { ...base, lockReason: 'unverified' } }).text()).toContain('rs.pat_locked_unverified');
  });

  it('asks to change the setting and leaves the toggle on the saved value until the server answers', async () => {
    const w = mount(PersonalTokenCard, { props: base });
    const toggle = w.get('input[type="checkbox"]');
    await toggle.setValue(true);
    expect(w.emitted('set')).toEqual([[true]]);
    expect((toggle.element as HTMLInputElement).checked).toBe(false);
  });

  it('shows the setting as text to people who cannot edit, with the introspection endpoint', () => {
    const w = mount(PersonalTokenCard, { props: { ...base, accept: true, canEdit: false } });
    expect(w.find('input[type="checkbox"]').exists()).toBe(false);
    expect(w.text()).toContain('rs.pat_on');
    expect(w.text()).toContain(base.endpoint);
  });
});
