import { describe, expect, it } from 'vitest';

import { mcpSnippet } from './mcpSnippet';

describe('mcpSnippet', () => {
  it('is a valid .mcp.json entry with the bearer key and the URL', () => {
    const snippet = mcpSnippet('othra', 'https://othra.example.com/app/mcp', 'othra_secret');
    const parsed = JSON.parse(snippet);
    expect(parsed).toEqual({
      mcpServers: {
        othra: {
          type: 'http',
          url: 'https://othra.example.com/app/mcp',
          headers: { Authorization: 'Bearer othra_secret' },
        },
      },
    });
    expect(snippet).toContain('\n  ');
  });
});
