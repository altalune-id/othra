// @vitest-environment happy-dom
import { mount } from '@vue/test-utils';
import { describe, expect, it, vi } from 'vitest';

import ErrorView from '~/error.vue';

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
});
