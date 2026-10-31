import { describe, expect, it } from 'vitest';
import { safeNext } from './session';

describe('safeNext', () => {
  it('keeps local paths', () => {
    expect(safeNext('/settings/tokens?x=1')).toBe('/settings/tokens?x=1');
  });
  it('rejects open redirects', () => {
    for (const bad of ['//evil.example', '/\\evil.example', 'https://evil.example', 'javascript:alert(1)', '', null]) {
      expect(safeNext(bad)).toBe('/');
    }
  });
});
