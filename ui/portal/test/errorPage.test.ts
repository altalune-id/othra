// @vitest-environment happy-dom
import { flushPromises, mount } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import { describe, expect, it, vi } from 'vitest';
import { defineComponent } from 'vue';

import { runtimeConfig, setClient } from '#test/nuxt-globals';
import ErrorView from '~/error.vue';
import DefaultLayout from '~/layouts/default.vue';

describe('portal error page', () => {
  it('keeps the portal layout and goes back to the profile', async () => {
    const clearError = vi.fn();
    vi.stubGlobal('clearError', clearError);
    const w = mount(ErrorView, { props: { error: { statusCode: 404 } as never }, global: { stubs: { NuxtLayout: { template: '<div data-testid="layout"><slot /></div>' } } } });
    expect(w.find('[data-testid="layout"] h1').text()).toBe('error_page.not_found_title');
    expect(w.text()).toContain('error_page.not_found_body');
    await w.get('button').trigger('click');
    expect(clearError).toHaveBeenCalledWith({ redirect: '/' });
  });

  it('paints the theme the viewer picked, since Nuxt renders it instead of app.vue', async () => {
    setActivePinia(createPinia());
    runtimeConfig.public = { ...runtimeConfig.public, project: 'Acme Store' };
    setClient('$accountClient', { getProfile: vi.fn().mockRejectedValue(new Error('offline')) });
    localStorage.setItem('othra_theme', JSON.stringify({ family: 'retro', mode: 'light' }));
    document.documentElement.removeAttribute('data-theme');
    const host = defineComponent({ components: { ErrorView }, template: '<Suspense><ErrorView :error="{ statusCode: 502 }" /></Suspense>' });
    mount(host, { global: { stubs: { NuxtLayout: DefaultLayout, ToastHost: true, LanguageSwitcher: true, ThemeSwitcher: true } } });
    await flushPromises();
    expect(document.documentElement.getAttribute('data-theme')).toBe('retro-light');
  });
});
