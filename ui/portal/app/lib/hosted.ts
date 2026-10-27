// signInURL is the hosted sign-in page that returns here after it.
export function signInURL(hosted: string, here: Pick<Location, 'pathname' | 'search'>): string {
  return `${hosted}/login?return_to=${encodeURIComponent(here.pathname + here.search)}`;
}

// termsURL is the hosted terms page, coming back to the same portal page once they are accepted.
export function termsURL(hosted: string, here: Pick<Location, 'pathname' | 'search'>): string {
  return `${hosted}/terms?return_to=${encodeURIComponent(here.pathname + here.search)}`;
}

// switchAccountURL opens the hosted account picker's "add an account" form, coming back to home, the portal's own URL.
export function switchAccountURL(hosted: string, home: string): string {
  return `${hosted}/login?add=1&return_to=${encodeURIComponent(home)}`;
}

// reauthURL asks the hosted sign-in for a fresh sign-in, then comes back to path.
export function reauthURL(hosted: string, path: string): string {
  return `${hosted}/login?prompt=login&return_to=${encodeURIComponent(path)}`;
}
