/**
 * Error thrown by {@link api} for any non-2xx response. `code` comes from the server's
 * error envelope (docs/reference/06-api-spec.md §1), e.g. `not_found`, `validation_failed`.
 */
export class ApiError extends Error {
  constructor(
    /** HTTP status code. */
    readonly status: number,
    /** Machine-readable error code from the server, or `network` / `bad_response`. */
    readonly code: string,
    message: string,
    /** Server request id; show it to users so they can quote it in bug reports. */
    readonly requestId = '',
    /** Extra details, e.g. `fields` for validation errors. */
    readonly details: Record<string, unknown> = {},
  ) {
    super(message);
    this.name = 'ApiError';
  }
}

const UNSAFE = new Set(['POST', 'PUT', 'PATCH', 'DELETE']);
let csrfToken = '';

/** Sets the CSRF token sent with unsafe requests (loaded with the session from P2 on). */
export function setCsrfToken(token: string): void {
  csrfToken = token;
}

/**
 * Calls the Anvil REST API and returns parsed JSON.
 * Adds the CSRF header to unsafe methods and converts error bodies into ApiError.
 * @param path - path under the site root, e.g. `/api/v1/version`.
 * @throws ApiError with `code` from the server's error envelope, or `network` when the
 *   server could not be reached.
 */
export async function api<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers);
  headers.set('Accept', 'application/json');
  if (init.body && !headers.has('Content-Type')) headers.set('Content-Type', 'application/json');
  if (UNSAFE.has((init.method ?? 'GET').toUpperCase()) && csrfToken) headers.set('X-CSRF-Token', csrfToken);

  let res: Response;
  try {
    res = await fetch(path, { ...init, headers, credentials: 'same-origin' });
  } catch (e) {
    throw new ApiError(0, 'network', `Cannot reach the server: ${(e as Error).message}`);
  }
  const text = await res.text();
  let body: unknown = undefined;
  if (text) {
    try {
      body = JSON.parse(text);
    } catch {
      if (res.ok) throw new ApiError(res.status, 'bad_response', 'The server sent a response that is not JSON');
    }
  }
  if (!res.ok) {
    const env = body as { error?: { code?: string; message?: string; details?: Record<string, unknown> }; request_id?: string } | undefined;
    throw new ApiError(
      res.status,
      env?.error?.code ?? 'http_' + res.status,
      env?.error?.message ?? res.statusText,
      env?.request_id ?? res.headers.get('X-Request-ID') ?? '',
      env?.error?.details ?? {},
    );
  }
  return body as T;
}
