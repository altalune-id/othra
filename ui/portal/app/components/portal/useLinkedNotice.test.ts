import { afterEach, describe, expect, it, vi } from 'vitest';
import { nextTick, ref } from 'vue';
import { useLinkedNotice } from './useLinkedNotice';

function mount(query: Record<string, string>) {
  const replace = vi.fn();
  const mounted: (() => void)[] = [];
  vi.stubGlobal('useRoute', () => ({ query }));
  vi.stubGlobal('useRouter', () => ({ replace }));
  vi.stubGlobal('onMounted', (fn: () => void) => mounted.push(fn));
  return { replace, mounted };
}

describe('useLinkedNotice (pm K-1)', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('shows once and drops ?linked=1 only after mount, keeping the rest of the query', () => {
    const { replace, mounted } = mount({ linked: '1', tab: 'x' });
    const linked = useLinkedNotice(ref(false));
    expect(linked.value).toBe(true);
    expect(replace).not.toHaveBeenCalled();
    mounted.forEach(fn => fn());
    expect(replace).toHaveBeenCalledWith({ query: { tab: 'x' } });
  });

  it('leaves the address alone without ?linked=1', () => {
    const { replace, mounted } = mount({});
    expect(useLinkedNotice(ref(false)).value).toBe(false);
    mounted.forEach(fn => fn());
    expect(replace).not.toHaveBeenCalled();
  });

  it('gives way to a later disconnect', async () => {
    mount({ linked: '1' });
    const unlinked = ref(false);
    const linked = useLinkedNotice(unlinked);
    unlinked.value = true;
    await nextTick();
    expect(linked.value).toBe(false);
  });

  it('keeps the notice when a disconnect is merely reset', async () => {
    mount({ linked: '1' });
    const unlinked = ref(true);
    const linked = useLinkedNotice(unlinked);
    unlinked.value = false;
    await nextTick();
    expect(linked.value).toBe(true);
  });
});
