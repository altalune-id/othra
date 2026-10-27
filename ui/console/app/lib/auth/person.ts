// isPerson reports a signed-in person with a console session (a password or Continue with Othra), never an API key or basic auth.
export function isPerson(me: { source?: string, userId?: string } | null | undefined): boolean {
  return !!me && (me.source === 'local' || me.source === 'oidc') && !!me.userId;
}
