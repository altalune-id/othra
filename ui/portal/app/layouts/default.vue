<script setup lang="ts">
import { provide } from 'vue';
import LanguageSwitcher from '#base/components/layout/LanguageSwitcher.vue';
import ThemeSwitcher from '#base/components/layout/ThemeSwitcher.vue';
import { useTheme } from '#base/components/layout/useTheme';
import AppButton from '#base/components/ui/AppButton.vue';
import AppDropdown from '#base/components/ui/AppDropdown.vue';
import AppShell from '#base/components/ui/AppShell.vue';
import { copiedLabelKey } from '#base/components/ui/copy';
import NavbarShell from '#base/components/ui/NavbarShell.vue';
import SidebarNav from '#base/components/ui/SidebarNav.vue';
import ToastHost from '#base/components/ui/ToastHost.vue';
import { switchAccountURL } from '~/lib/hosted';
import { navItems } from '~/lib/nav';
import { useAccountStore } from '~/stores/account';

const route = useRoute();
const router = useRouter();
const { t, locale } = useI18n();
const config = useRuntimeConfig().public;
const account = useAccountStore();
const navOpen = ref(false);
provide(copiedLabelKey, () => t('common.copied'));
onMounted(useTheme().init);

useHead({
  htmlAttrs: { lang: locale },
  bodyAttrs: { class: 'text-sm leading-[normal]' },
  titleTemplate: title => (title ? `${title} · ${config.project}` : t('portal.title', { project: config.project })),
});

const groups = computed(() => [{ items: navItems(route.path, t) }]);
const initial = computed(() => [...config.project][0]?.toUpperCase() ?? '');
const switchURL = computed(() => switchAccountURL(config.hostedUrl, router.resolve('/').href));

watch(() => route.fullPath, () => {
  navOpen.value = false;
});

await account.ensureLoaded();
</script>

<template>
  <div class="flex min-h-dvh flex-col bg-base-100 text-base-content">
    <NavbarShell>
      <template #brand>
        <AppButton variant="ghost" size="sm" square class="md:hidden" :aria-label="t('menu.open')" :aria-expanded="navOpen" aria-controls="app-sidebar" @click="navOpen = !navOpen">
          <Icon name="lucide:menu" size="18" />
        </AppButton>
        <span class="flex min-w-0 items-center gap-2">
          <img v-if="config.logoUrl" :src="config.logoUrl" alt="" class="size-8 shrink-0 rounded-box object-contain">
          <span v-else class="flex size-8 shrink-0 items-center justify-center rounded-box bg-base-200 text-sm font-semibold" aria-hidden="true">{{ initial }}</span>
          <span class="truncate text-sm font-semibold" :title="config.project">{{ config.project }}</span>
        </span>
      </template>
      <template #end>
        <LanguageSwitcher />
        <ThemeSwitcher />
        <AppDropdown placement="end" content-class="w-60 p-2" :label="t('menu.account')" trigger-class="max-w-48">
          <template #trigger>
            <span class="truncate max-sm:hidden">{{ account.profile?.email }}</span>
            <Icon name="lucide:circle-user-round" size="18" class="sm:hidden" />
            <Icon name="lucide:chevron-down" size="14" />
          </template>
          <ul class="menu w-full p-0">
            <li class="menu-title break-all">
              {{ account.profile?.email }}
            </li>
            <li>
              <a :href="switchURL">
                <Icon name="lucide:users-round" size="16" />
                {{ t('menu.switch') }}
              </a>
            </li>
            <li>
              <form method="post" :action="`${config.hostedUrl}/login/signout`" class="p-0">
                <input type="hidden" name="csrf" :value="account.signOutToken">
                <input type="hidden" name="user_session" :value="account.currentSessionId">
                <input type="hidden" name="next" value="signed_out">
                <button type="submit" class="flex w-full items-center gap-2 px-3 py-1.5">
                  <Icon name="lucide:log-out" size="16" />
                  {{ t('menu.sign_out') }}
                </button>
              </form>
            </li>
          </ul>
        </AppDropdown>
      </template>
    </NavbarShell>
    <AppShell v-model:open="navOpen" :close-label="t('menu.close')">
      <template #sidebar>
        <SidebarNav :label="t('portal.nav_label')" :groups="groups" />
      </template>
      <main class="min-w-0 flex-1 p-4 sm:p-6">
        <slot />
        <footer v-if="config.termsUrl || config.privacyUrl" class="mt-10 flex gap-4 text-xs text-base-content/70">
          <a v-if="config.termsUrl" class="link" :href="config.termsUrl" target="_blank" rel="noopener noreferrer">{{ t('portal.terms') }}</a>
          <a v-if="config.privacyUrl" class="link" :href="config.privacyUrl" target="_blank" rel="noopener noreferrer">{{ t('portal.privacy') }}</a>
        </footer>
      </main>
    </AppShell>
    <ToastHost :close-label="t('common.close')" />
  </div>
</template>
