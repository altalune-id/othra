import type { InjectionKey } from 'vue';

export interface ShellHold {
  hold: () => void;
  release: () => void;
}

// shellHoldKey lets a panel above the page ask AppShell to make its content and sidebar inert while it is open.
export const shellHoldKey: InjectionKey<ShellHold> = Symbol('shellHold');
