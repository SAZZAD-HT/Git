// API response types. Hand-written until api/openapi.yaml exists; then replaced by the
// generated schema.d.ts (docs/reference/09 §7).

/** A user as returned by the API. `email` and `is_active` appear only for yourself and site admins. */
export interface User {
  id: number;
  username: string;
  full_name: string;
  email?: string;
  is_admin: boolean;
  is_active?: boolean;
  created_at: string;
}

/** Body of POST /auth/login, POST /auth/register and GET /user. */
export interface SessionInfo {
  user: User;
  csrf_token?: string;
}

/** A personal access token (never includes the secret, except `token` right after creation). */
export interface AccessToken {
  id: number;
  name: string;
  token_prefix: string;
  scopes: string[];
  expires_at?: string;
  last_used_at?: string;
  created_at: string;
  token?: string;
}

/** One of your browser sessions. */
export interface BrowserSession {
  id: string;
  ip: string;
  user_agent: string;
  created_at: string;
  last_seen_at: string;
  current: boolean;
}

/** Body of GET /api/v1/meta. */
export interface Meta {
  allow_registration: boolean;
}
