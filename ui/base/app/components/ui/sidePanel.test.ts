// @vitest-environment happy-dom
import { mount } from '@vue/test-utils';
import { afterEach, describe, expect, it } from 'vitest';
import { defineComponent, h, nextTick, ref } from 'vue';
import AppShell from './AppShell.vue';
import SidePanel from './SidePanel.vue';

afterEach(() => {
  document.body.innerHTML = '';
});

const panel = () => document.body.querySelector('body > .drawer-end') as HTMLElement;

function inShell(open = ref(false), shown = ref(true)) {
  const Page = defineComponent({
    setup: () => () => [
      h('button', { id: 'trigger' }, 'Open'),
      shown.value ? h(SidePanel, { 'title': 'Discovery document', 'closeLabel': 'Close Discovery document', 'open': open.value, 'onUpdate:open': (v: boolean) => (open.value = v) }, () => h('pre', { tabindex: 0, ariaLabel: 'Discovery document' }, '{}')) : null,
    ],
  });
  const host = defineComponent({ setup: () => () => [h(AppShell, { full: true, breakpoint: 'lg' }, { default: () => h(Page), sidebar: () => h('nav', [h('a', { href: '/one' }, 'One')]) }), h('div', { id: 'toasts', role: 'status' })] });
  return mount(host, { attachTo: document.body });
}

describe('sidePanel', () => {
  it('is the design right drawer, a labelled modal dialog placed on body', () => {
    mount(SidePanel, { props: { title: 'Discovery document' }, attachTo: document.body });
    expect(panel().className).toBe('drawer drawer-end');
    const aside = panel().querySelector('aside')!;
    expect(aside.getAttribute('role')).toBe('dialog');
    expect(aside.getAttribute('aria-modal')).toBe('true');
    expect(document.getElementById(aside.getAttribute('aria-labelledby')!)!.textContent!.trim()).toBe('Discovery document');
    expect(panel().querySelector('h2')!.className).toBe('text-sm font-semibold');
    expect(aside.classList).toContain('w-96');
    expect(panel().querySelector('input.drawer-toggle')!.hasAttribute('aria-label')).toBe(false);
  });

  // NOTE: a visible toast's Dismiss button stays tabbable while a panel is open; accepted, as toasts are transient status.
  it('makes the shell content and sidebar inert while open, keeps toasts live, and restores on close with focus back', async () => {
    const open = ref(false);
    inShell(open);
    const trigger = document.getElementById('trigger') as HTMLButtonElement;
    trigger.focus();
    open.value = true;
    await nextTick();
    await nextTick();
    expect(document.querySelector('.drawer-content')!.hasAttribute('inert')).toBe(true);
    expect(document.querySelector('#app-sidebar')!.hasAttribute('inert')).toBe(true);
    expect(panel().closest('[inert]')).toBeNull();
    expect(document.getElementById('toasts')!.closest('[inert]')).toBeNull();
    expect(document.activeElement?.getAttribute('aria-label')).toBe('Close Discovery document');
    expect((panel().querySelector('pre') as HTMLElement).tabIndex).toBe(0);
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));
    await nextTick();
    await nextTick();
    expect(open.value).toBe(false);
    expect(document.querySelector('.drawer-content')!.hasAttribute('inert')).toBe(false);
    expect(document.querySelector('#app-sidebar')!.hasAttribute('inert')).toBe(false);
    expect(document.activeElement).toBe(trigger);
  });

  it('moves focus to Close only once the drawer side is visible, as the drawer fades in', async () => {
    const open = ref(false);
    mount(defineComponent({ setup: () => () => h(SidePanel, { 'title': 'Discovery document', 'closeLabel': 'Close Discovery document', 'open': open.value, 'onUpdate:open': (v: boolean) => (open.value = v) }) }), { attachTo: document.body });
    const side = panel().querySelector('.drawer-side') as HTMLElement;
    side.style.visibility = 'hidden';
    open.value = true;
    await nextTick();
    await nextTick();
    expect(document.activeElement?.getAttribute('aria-label')).not.toBe('Close Discovery document');
    side.style.visibility = 'visible';
    await new Promise(resolve => setTimeout(resolve, 60));
    expect(document.activeElement?.getAttribute('aria-label')).toBe('Close Discovery document');
  });

  it('releases the shell when the panel unmounts while open, as on a route change', async () => {
    const open = ref(true);
    const shown = ref(true);
    inShell(open, shown);
    await nextTick();
    await nextTick();
    expect(document.querySelector('.drawer-content')!.hasAttribute('inert')).toBe(true);
    shown.value = false;
    await nextTick();
    await nextTick();
    expect(document.querySelector('.drawer-content')!.hasAttribute('inert')).toBe(false);
    expect(document.querySelector('#app-sidebar')!.hasAttribute('inert')).toBe(false);
  });
});
