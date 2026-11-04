// @vitest-environment happy-dom
import { mount } from '@vue/test-utils';
import { describe, expect, it } from 'vitest';
import AppBadge from './AppBadge.vue';
import StatusLabel from './StatusLabel.vue';

describe('appBadge', () => {
  it('is a small badge with an optional hidden icon, xs and outline on request', () => {
    const w = mount(AppBadge, { props: { label: 'Web', icon: 'lucide:app-window' } });
    expect(w.classes()).toEqual(['badge', 'badge-sm']);
    expect(w.text()).toBe('Web');
    expect(w.get('[data-icon="lucide:app-window"]').attributes('aria-hidden')).toBe('true');
    expect(mount(AppBadge, { props: { label: 'Labs', size: 'xs', outline: true } }).classes()).toEqual(['badge', 'badge-xs', 'badge-outline']);
  });
});

describe('statusLabel', () => {
  it('never shows status by colour alone: a hidden dot, a label and an optional muted detail', () => {
    const w = mount(StatusLabel, { props: { tone: 'success', label: 'Serving', detail: 'checked 2 min ago' } });
    const dot = w.get('.status');
    expect(dot.classes()).toEqual(['status', 'status-success']);
    expect(dot.attributes('aria-hidden')).toBe('true');
    expect(w.get('.font-medium').text()).toBe('Serving');
    expect(w.text()).toContain('checked 2 min ago');
    expect(mount(StatusLabel, { props: { tone: 'error', label: 'Not reachable' } }).find('.text-base-content\\/70').exists()).toBe(false);
  });
});
