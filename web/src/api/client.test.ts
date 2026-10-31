import { afterEach, describe, expect, it, vi } from 'vitest';
import { api, ApiError, setCsrfToken } from './client';

function mockFetch(status: number, body: string, headers: Record<string, string> = {}) {
  const fn = vi.fn(async () => new Response(body, { status, headers }));
  vi.stubGlobal('fetch', fn);
  return fn;
}

afterEach(() => {
  vi.unstubAllGlobals();
  setCsrfToken('');
});

describe('api', () => {
  it('returns parsed JSON', async () => {
    mockFetch(200, '{"version":"dev"}');
    await expect(api<{ version: string }>('/api/v1/version')).resolves.toEqual({ version: 'dev' });
  });

  it('turns the error envelope into ApiError', async () => {
    mockFetch(404, '{"error":{"code":"not_found","message":"nope","details":{}},"request_id":"01ABC"}');
    const err = await api('/api/v1/x').catch((e) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect(err).toMatchObject({ status: 404, code: 'not_found', message: 'nope', requestId: '01ABC' });
  });

  it('sends the CSRF token only on unsafe methods', async () => {
    const fetch = mockFetch(200, '{}');
    setCsrfToken('tok');
    await api('/a');
    await api('/b', { method: 'POST', body: '{}' });
    const headers = (i: number) => (fetch.mock.calls[i] as unknown as [string, RequestInit])[1].headers as Headers;
    expect(headers(0).get('X-CSRF-Token')).toBeNull();
    expect(headers(1).get('X-CSRF-Token')).toBe('tok');
  });

  it('reports network failures', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => { throw new TypeError('offline'); }));
    await expect(api('/a')).rejects.toMatchObject({ code: 'network' });
  });
});
