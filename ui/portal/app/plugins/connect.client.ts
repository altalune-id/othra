import type { Interceptor } from '@connectrpc/connect';
import { createClient } from '@connectrpc/connect';
import { createConnectTransport } from '@connectrpc/connect-web';
import { errorCode } from '#base/lib/apperror/message';
import { createRedirectOnce, isUnauthenticated } from '#base/lib/auth/unauthenticated';
import { AccountService } from '#gen/account/v1/account_pb';
import { signInURL, termsURL } from '~/lib/hosted';

// One cookie-bearing transport to this project's account.v1; a signed-out session (ACC001) or owed terms (ACC010) go to the hosted page once and come back here.
export default defineNuxtPlugin({
  name: 'connect',
  setup() {
    const { public: config } = useRuntimeConfig();
    const redirectOnce = createRedirectOnce(url => window.location.assign(url));
    const toSignIn: Interceptor = next => async (req) => {
      try {
        return await next(req);
      } catch (err) {
        if (isUnauthenticated(err)) {
          redirectOnce(signInURL(config.hostedUrl, window.location));
        } else if (errorCode(err) === 'ACC010') {
          redirectOnce(termsURL(config.hostedUrl, window.location));
        }
        throw err;
      }
    };
    const transport = createConnectTransport({
      baseUrl: config.apiUrl,
      fetch: (input: RequestInfo | URL, init?: RequestInit) => globalThis.fetch(input, { ...init, credentials: 'same-origin' }),
      interceptors: [toSignIn],
    });
    return { provide: { accountClient: createClient(AccountService, transport) } };
  },
});
