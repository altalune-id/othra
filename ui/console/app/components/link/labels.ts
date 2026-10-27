import type { LinkDialogLabels } from './LinkDialog.vue';
import type { LinkListLabels } from './LinkList.vue';

type Translate = (key: string, params?: Record<string, unknown>) => string;

// linkListLabels translates the links card; on a client it lists resource servers, on a resource server it lists clients.
export function linkListLabels(t: Translate, page: 'client' | 'resourceServer'): LinkListLabels {
  return {
    title: page === 'client' ? t('links.title_resource_servers') : t('links.title_clients'),
    link: page === 'client' ? t('links.link_resource_server') : t('links.link_client'),
    empty: t('links.empty'),
    emptyReadOnly: t('links.empty_readonly'),
    edit: t('common.edit'),
    unlink: t('links.unlink'),
  };
}

export function linkDialogLabels(t: Translate, page: 'client' | 'resourceServer', edit: boolean): LinkDialogLabels {
  let title = page === 'client' ? t('links.dialog_resource_server') : t('links.dialog_client');
  if (edit) {
    title = t('links.edit_title');
  }
  return {
    title,
    target: page === 'client' ? t('links.resource_server') : t('links.client'),
    scopes: t('links.scopes'),
    scopesHelp: t('links.scopes_help'),
    pickScope: t('apperror.RSV009_empty'),
    unverified: t('links.unverified_note'),
    noScopes: t('links.no_scopes'),
    noneLeft: page === 'client' ? t('links.none_left_resource_servers') : t('links.none_left_clients'),
    cancel: t('common.cancel'),
    confirm: edit ? t('common.save') : t('links.link'),
  };
}
