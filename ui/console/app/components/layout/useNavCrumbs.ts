import type { Crumb, NavGroup, PaletteItem, TopBarPlace } from '#base/components/ui/types';
import { useAccountNav } from '~/components/account/useAccountNav';
import { useOrgNav } from '~/components/layout/useOrgNav';
import { environmentLabelKey } from '~/components/project/environment';
import { useProjectNav } from '~/components/project/useProjectNav';
import { useOrgsStore } from '~/stores/orgs';
import { useProjectsStore } from '~/stores/projects';

// useNavCrumbs derives the top bar trail and the palette entries from the sidebar nav, so a page needs no crumbs of its own.
export function useNavCrumbs() {
  const route = useRoute();
  const { t } = useI18n();
  const orgs = useOrgsStore();
  const projects = useProjectsStore();
  const orgNav = useOrgNav();
  const projectNav = useProjectNav();
  const accountNav = useAccountNav();
  const inAccount = computed(() => route.path.startsWith('/account'));

  const org = computed(() => orgs.bySlug(route.params.org as string));
  const project = computed(() => route.params.project as string | undefined);
  const groups = computed<NavGroup[]>(() => (project.value ? projectNav.value : [orgNav.value]));

  const crumbs = computed<Crumb[]>(() => {
    if (inAccount.value) {
      const active = accountNav.value.items.find(i => i.active);
      return [{ label: t('account.title'), to: '/account' }, ...(active ? [{ label: active.label }] : [])];
    }
    if (!org.value) {
      return [];
    }
    const trail: Crumb[] = [{ label: org.value.name, to: `/o/${org.value.slug}` }];
    if (project.value) {
      trail.push({ label: project.value, to: `/o/${org.value.slug}/projects/${project.value}`, mono: true });
    }
    const active = groups.value.flatMap(g => g.items).find(i => i.active);
    if (active) {
      trail.push({ label: active.label });
    }
    return trail;
  });

  const palette = computed<PaletteItem[]>(() => {
    if (inAccount.value) {
      return accountNav.value.items.map(i => ({ key: `account-${i.key}`, group: t('account.title'), label: i.label, to: i.to }));
    }
    const name = org.value?.name;
    if (!name) {
      return [];
    }
    const items: PaletteItem[] = orgNav.value.items.map(i => ({ key: `org-${i.key}`, group: name, label: i.label, to: i.to }));
    if (project.value) {
      items.push(...projectNav.value.flatMap(g => g.items).map(i => ({ key: `project-${i.key}`, group: project.value!, mono: true, label: i.label, to: i.to })));
    }
    return items;
  });

  const place = computed<TopBarPlace | undefined>(() => {
    if (inAccount.value) {
      return { label: t('account.title') };
    }
    if (!org.value) {
      return undefined;
    }
    if (!project.value) {
      return { label: org.value.name };
    }
    const env = projects.items.find(p => p.orgId === org.value!.id && p.slug === project.value)?.environment;
    const key = environmentLabelKey(env);
    return { label: project.value, badge: key === 'environment.none' ? undefined : t(key) };
  });

  return { crumbs, place, palette };
}
