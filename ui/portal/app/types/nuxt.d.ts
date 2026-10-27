import type { Client } from '@connectrpc/connect';
import type { AccountService } from '#gen/account/v1/account_pb';

declare module '#app' {
  interface NuxtApp {
    $accountClient: Client<typeof AccountService>;
  }
}
