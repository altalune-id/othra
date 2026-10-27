// accountBackLink points the account sidebar back at the active org's overview, or at the org list when there is none.
export function accountBackLink(activeOrgSlug: string | undefined, orgs: { slug: string, name: string }[]): { to: string, org: string } {
  if (!activeOrgSlug) {
    return { to: '/orgs', org: '' };
  }
  return { to: `/o/${activeOrgSlug}`, org: orgs.find(o => o.slug === activeOrgSlug)?.name ?? activeOrgSlug };
}
