import { readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';

const read = (path: string) => readFileSync(new URL(path, import.meta.url), 'utf8');
const page = (path: string) => ({ path, source: read(`../../pages/o/[org]/projects/[project]/${path}.vue`) });

const lists = [page('clients/index'), page('resource-servers/index')];
const details = [page('clients/[client]'), page('resource-servers/[rs]')];
const creates = [page('clients/new'), page('resource-servers/new')];

describe('clients and resource server pages (M3)', () => {
  it.each([...lists, ...details])('$path tells members why they see no write controls', ({ source }) => {
    expect(source).toContain('useCanEditProject');
    expect(source).toContain('<ProjectReadOnlyNote v-if="!canEdit"');
  });

  it.each(lists)('$path offers New only to editors', ({ source }) => {
    for (const link of source.match(/<NuxtLink[^>]*\/new`[^>]*>/g) ?? []) {
      expect(link).toContain('v-if="canEdit"');
    }
  });

  it.each(details)('$path gates Delete and every write form on canEdit', ({ source }) => {
    expect(source).toMatch(/<button v-if="canEdit"[^>]*@click="page\.askDelete\(\)"/);
    for (const form of source.match(/<form\s[^>]*>/g) ?? []) {
      expect(form).toContain('v-if="canEdit"');
    }
    expect(source).toContain(':can-edit="canEdit"');
  });

  it.each(creates)('$path sends members back to the list', ({ source }) => {
    expect(source).toMatch(/if \(!canEdit\.value\) \{\s+await navigateTo\(base\.value, \{ replace: true \}\);/);
  });

  it('holds the page while a new secret is shown and drops it once saved', () => {
    for (const { source } of [creates[0]!, details[0]!]) {
      expect(source).toContain('onBeforeRouteLeave(() => !revealed.value);');
      expect(source).toContain('<ClientSecretDialog');
    }
  });

  it('checks a new resource server once, right after create', () => {
    expect(creates[1]!.source).toContain('query: { verify: \'1\' }');
    expect(details[1]!.source).toMatch(/route\.query\.verify !== '1'[\s\S]+navigateTo\(\{ query: \{\} \}, \{ replace: true \}\);\s+if \(canEdit\.value && !server\.value\.trusted\) \{\s+await page\.verify\(\);/);
  });

  it('keeps the secret dialog open until the box is ticked', () => {
    const dialog = read('../client/ClientSecretDialog.vue');
    expect(dialog).toContain(':dismissible="stored"');
    expect(dialog).toContain(':disabled="!stored"');
    expect(dialog).toMatch(/function done\(\) \{\s+if \(stored\.value\) \{\s+emit\('done'\);/);
  });

  it('lists Clients and Resource servers in the project sidebar above Webhooks', () => {
    const sidebar = read('./ProjectSidebar.vue');
    const order = ['nav.overview', 'nav.clients', 'nav.resource_servers', 'webhooks.title', 'nav.settings'].map(k => sidebar.indexOf(`t('${k}')`));
    expect(order.every(i => i >= 0)).toBe(true);
    expect([...order].sort((a, b) => a - b)).toEqual(order);
  });

  it('links list rows with NuxtLink so new tabs and copied links keep the console base', () => {
    for (const list of ['../client/ClientList.vue', '../resourceserver/ResourceServerList.vue']) {
      const source = read(list);
      expect(source, list).toContain('<NuxtLink :to="row.to"');
      expect(source, list).not.toContain('<a :href');
    }
  });

  it('never changes a field message on blur, so the next click lands where it was aimed', () => {
    const sources = ['../client/RedirectUriList.vue', '../client/ClientForm.vue', '../resourceserver/ScopeTable.vue'].map(read).concat([...creates, ...details].map(p => p.source));
    for (const source of sources) {
      expect(source).not.toContain('@blur');
    }
    expect(read('../client/RedirectUriList.vue')).toMatch(/function set\(index: number, value: string\) \{[^}]+emit\('edit', index\);/);
  });

  it('puts focus back after the secret is saved on create and after Verify now', () => {
    expect(creates[0]!.source).toContain('query: { focus: \'secrets\' }');
    expect(details[0]!.source).toMatch(/route\.query\.focus !== 'secrets'[\s\S]+secretsCard\.value\?\.focusTitle\(\);/);
    expect(details[1]!.source).toMatch(/async function verifyNow\(\) \{\s+await page\.verify\(\);\s+await nextTick\(\);\s+verifyCard\.value\?\.focusVerify\(\);/);
    expect(read('../resourceserver/VerificationCard.vue')).toContain('defineExpose({ focusVerify: () => verifyButton.value?.focus() });');
  });

  it('shows members no trust divider unless the server is trusted, and read-only link copy', () => {
    expect(read('../resourceserver/VerificationCard.vue')).toContain('<template v-if="trustAvailable && (canEdit || trusted())">');
    expect(read('../link/LinkList.vue')).toContain('{{ canEdit ? labels.empty : labels.emptyReadOnly }}');
  });

  it('notes on require-consent clients that end users cannot sign in yet (M5 D3)', () => {
    expect(read('../client/ClientForm.vue')).toMatch(/<p v-if="fields\.consent === 'require'" :id="ids\.consentNote"[^>]*>\s*<Icon name="lucide:triangle-alert"[^>]*class="[^"]*text-warning"[^>]*\/>\s*\{\{ labels\.consentRequireNote \}\}/);
    expect(details[0]!.source).toMatch(/<p v-if="client\.consentPolicy !== ConsentPolicy\.IMPLICIT"[^>]*data-testid="consent-require-note"[^>]*>\s*<Icon name="lucide:triangle-alert"[^>]*class="[^"]*text-warning"/);
    expect(read('../client/ClientForm.vue')).not.toContain('lucide:info');
  });

  it('offers the secret method and the PKCE switch on web apps only, with a warning when PKCE is off (M5 G11)', () => {
    const form = read('../client/ClientForm.vue');
    expect(form).toMatch(/<template v-if="kind\?\.secret">[\s\S]*name="client-auth-method"[\s\S]*labels\.pkceRequired[\s\S]*<\/template>/);
    expect(form).toMatch(/<p v-if="!fields\.pkceRequired" :id="ids\.pkceWarn"[^>]*>\s*<Icon name="lucide:triangle-alert"/);
    expect(details[0]!.source).toMatch(/<template v-if="kind\.secret">[\s\S]*clients\.auth_method[\s\S]*data-testid="client-pkce-off"/);
  });
});
