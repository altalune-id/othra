// @vitest-environment happy-dom
import { flushPromises, mount } from '@vue/test-utils';
import { afterEach, describe, expect, it, vi } from 'vitest';
import EmptyState from './EmptyState.vue';
import KeyValue from './KeyValue.vue';
import PageHeader from './PageHeader.vue';
import SecretReveal from './SecretReveal.vue';
import Section from './Section.vue';
import ToastHost from './ToastHost.vue';
import { useToast } from './useToast';

describe('pageHeader', () => {
  it('has one h1, crumbs as a labelled nav with the last one current, and a Preview badge on request', () => {
    const w = mount(PageHeader, {
      props: { title: 'https://id.acme.test/p/apps', mono: true, description: 'Issuer', crumbs: [{ label: 'Clients', to: '/c' }, { label: 'Store web' }], crumbsLabel: 'You are here', preview: true },
      slots: { actions: '<button>New</button>' },
    });
    expect(w.findAll('h1')).toHaveLength(1);
    expect(w.get('h1').classes()).toContain('font-mono');
    expect(w.get('nav').attributes('aria-label')).toBe('You are here');
    expect(w.get('[aria-current="page"]').text()).toBe('Store web');
    expect(w.findAll('nav a')).toHaveLength(1);
    expect(w.get('.badge').text()).toBe('Preview');
    expect(w.text()).toContain('New');
  });

  it('shows no nav or badge when not asked', () => {
    const w = mount(PageHeader, { props: { title: 'Clients' } });
    expect(w.find('nav').exists()).toBe(false);
    expect(w.find('.badge').exists()).toBe(false);
    expect(w.get('h1').classes()).toContain('text-2xl');
  });
});

describe('section', () => {
  it('is a region named by its h2, red-bordered for danger', () => {
    const w = mount(Section, { props: { title: 'Danger zone', tone: 'danger' }, slots: { default: '<p>Body</p>' } });
    const id = w.get('section').attributes('aria-labelledby');
    expect(w.get('h2').attributes('id')).toBe(id);
    expect(w.get('section').classes()).toContain('border-error');
  });
});

describe('emptyState', () => {
  it('uses the heading level it is given and shows its actions', () => {
    const w = mount(EmptyState, { props: { icon: 'lucide:app-window', title: 'No clients yet', body: 'A client is an app.', headingLevel: 3 }, slots: { actions: '<button>New client</button>' } });
    expect(w.get('h3').text()).toBe('No clients yet');
    expect(w.text()).toContain('A client is an app.');
    expect(w.text()).toContain('New client');
  });
});

describe('keyValue', () => {
  it('renders a real dl, a dash for empty values and a copy button only where asked', () => {
    const w = mount(KeyValue, { props: { items: [{ label: 'Client ID', value: 'abc', mono: true, copy: true }, { label: 'Logo' }] } });
    expect(w.findAll('dt').map(d => d.text())).toEqual(['Client ID', 'Logo']);
    expect(w.findAll('dd')[1]!.text()).toBe('—');
    expect(w.findAll('button')).toHaveLength(1);
    expect(w.get('button').attributes('aria-label')).toBe('Copy Client ID');
  });
});

describe('secretReveal', () => {
  const props = { value: 'othra_cs_x', warning: 'Shown once.', confirmLabel: 'I stored it' };

  it('shows the secret, a .env copy and a checkbox that reports the confirmation', async () => {
    const w = mount(SecretReveal, { props: { ...props, envName: 'OTHRA_CLIENT_SECRET' } });
    expect(w.text()).toContain('othra_cs_x');
    expect(w.text()).toContain('Shown once.');
    const copies = w.findAll('button');
    expect(copies).toHaveLength(2);
    await w.get('input[type="checkbox"]').setValue(true);
    expect(w.emitted('confirmed')).toEqual([[true]]);
  });

  it('copies every env line it is given', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    vi.stubGlobal('navigator', { clipboard: { writeText } });
    const w = mount(SecretReveal, { props: { ...props, env: { OTHRA_ISSUER: 'https://i', OTHRA_CLIENT_SECRET: 'othra_cs_x' } } });
    await w.findAll('button')[1]!.trigger('click');
    expect(writeText).toHaveBeenCalledWith('OTHRA_ISSUER=https://i\nOTHRA_CLIENT_SECRET=othra_cs_x');
    vi.unstubAllGlobals();
  });

  it('shows the error with its retry instead of a secret, and no env copy without a name', () => {
    const w = mount(SecretReveal, { props: { ...props, value: '', error: { message: 'Failed' } }, slots: { retry: '<button>Generate again</button>' } });
    expect(w.get('[role="alert"]').text()).toContain('Failed');
    expect(w.text()).toContain('Generate again');
    expect(w.find('input[type="checkbox"]').exists()).toBe(false);
    expect(mount(SecretReveal, { props }).findAll('button')).toHaveLength(1);
  });
});

describe('toast', () => {
  afterEach(() => {
    const { toasts } = useToast();
    toasts.splice(0);
    vi.useRealTimers();
  });

  it('shows pushed toasts in a polite live region, errors as alerts, and hides them after the lifetime', async () => {
    vi.useFakeTimers();
    const w = mount(ToastHost, { props: { lifetime: 1000 } });
    const { push } = useToast();
    push({ kind: 'success', text: 'Saved' });
    push({ kind: 'error', text: 'Failed' });
    await flushPromises();
    expect(w.get('.toast').attributes('aria-live')).toBe('polite');
    expect(w.findAll('[role="status"]')).toHaveLength(1);
    expect(w.get('[role="alert"]').text()).toContain('Failed');
    vi.advanceTimersByTime(1000);
    await flushPromises();
    expect(w.findAll('.alert')).toHaveLength(0);
  });

  it('pauses while hovered and closes on Dismiss', async () => {
    vi.useFakeTimers();
    const w = mount(ToastHost, { props: { lifetime: 1000 } });
    useToast().push({ kind: 'info', text: 'Copied' });
    await flushPromises();
    await w.get('.alert').trigger('mouseenter');
    vi.advanceTimersByTime(5000);
    await flushPromises();
    expect(w.findAll('.alert')).toHaveLength(1);
    await w.get('.alert').trigger('mouseleave');
    await w.get('.alert button').trigger('click');
    expect(w.findAll('.alert')).toHaveLength(0);
    w.unmount();
  });
});
