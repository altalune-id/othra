import type { Ref } from 'vue';

// useLinkedNotice shows the "Google is connected" notice once after ?linked=1, drops it from the address after mount, and hides it after a disconnect.
export function useLinkedNotice(unlinked: Ref<boolean>) {
  const route = useRoute();
  const router = useRouter();
  const linked = ref(route.query.linked === '1');

  watch(unlinked, (now) => {
    if (now) {
      linked.value = false;
    }
  });

  onMounted(() => {
    if (linked.value) {
      const { linked: _shown, ...rest } = route.query;
      router.replace({ query: rest });
    }
  });

  return linked;
}
