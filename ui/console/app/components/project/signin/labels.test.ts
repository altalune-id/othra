import { describe, expect, it } from 'vitest';

import { t } from '../../../../test/nuxt-globals';
import { googleUpstreamCardLabels } from './labels';

describe('sign-in labels', () => {
  it('translates the Google card, with its own Remove lock line (MF4)', () => {
    expect(googleUpstreamCardLabels(t)).toMatchObject({
      title: 'signin.google',
      upstream: 'signin.upstream_label',
      redirectUri: 'signin.redirect_uri',
      removeLocked: 'signin.remove_locked',
      turnOffInstead: 'signin.turn_off_instead',
      save: 'common.save',
      copy: 'common.copy_label|{"label":"{label}"}',
    });
  });
});
