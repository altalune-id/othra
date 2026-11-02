// @vitest-environment happy-dom
import { mount } from '@vue/test-utils';
import { describe, expect, it } from 'vitest';
import SidePanel from './SidePanel.vue';

describe('sidePanel', () => {
  it('is the design right drawer: a labelled dialog that takes focus, closes on Escape and returns focus', async () => {
    const opener = document.createElement('button');
    document.body.append(opener);
    opener.focus();
    const w = mount(SidePanel, { props: { 'title': 'Discovery document', 'closeLabel': 'Close Discovery document', 'open': false, 'onUpdate:open': (v: boolean) => w.setProps({ open: v }) }, slots: { default: '<pre>{}</pre>' }, attachTo: document.body });
    expect(w.classes()).toEqual(['drawer', 'drawer-end']);
    const panel = w.get('aside');
    expect(panel.attributes('role')).toBe('dialog');
    expect(w.get(`#${panel.attributes('aria-labelledby')}`).text()).toBe('Discovery document');
    expect(w.get('h2').classes()).toEqual(['text-sm', 'font-semibold']);
    expect(panel.classes()).toEqual(expect.arrayContaining(['w-96', 'max-w-[calc(100vw-2.5rem)]', 'border-s', 'border-base-300', 'bg-base-100', 'p-4']));
    await w.setProps({ open: true });
    await w.vm.$nextTick();
    expect((w.get('input.drawer-toggle').element as HTMLInputElement).checked).toBe(true);
    expect(document.activeElement?.getAttribute('aria-label')).toBe('Close Discovery document');
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));
    await w.vm.$nextTick();
    await w.vm.$nextTick();
    expect(w.props('open')).toBe(false);
    expect(document.activeElement).toBe(opener);
    w.unmount();
    opener.remove();
  });
});
