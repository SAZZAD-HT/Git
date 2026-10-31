import { createContext, useCallback, useContext, useEffect, useState, type ReactNode } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { api, ApiError, setCsrfToken } from '../api/client';
import type { SessionInfo, User } from '../api/types';

/** The signed-in user (or null) plus helpers to change it. */
export interface SessionValue {
  /** The current user, null when signed out, undefined while loading. */
  user: User | null | undefined;
  /** Records a fresh sign-in (from login or register responses). */
  signedIn: (info: SessionInfo) => void;
  /** Ends the session on the server and clears all cached data. */
  signOut: () => Promise<void>;
  /** Reloads the current user, e.g. after a profile change. */
  refresh: () => Promise<void>;
}

const SessionContext = createContext<SessionValue | null>(null);

/**
 * Loads the current user once from GET /api/v1/user and provides it to the app
 * (docs/reference/09 §3). A 401 simply means "signed out".
 */
export function SessionProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null | undefined>(undefined);
  const queryClient = useQueryClient();

  const refresh = useCallback(async () => {
    try {
      const info = await api<SessionInfo>('/api/v1/user');
      setCsrfToken(info.csrf_token ?? '');
      setUser(info.user);
    } catch (e) {
      if (e instanceof ApiError && e.status === 401) setUser(null);
      else throw e;
    }
  }, []);

  useEffect(() => {
    refresh().catch(() => setUser(null));
  }, [refresh]);

  const signedIn = useCallback((info: SessionInfo) => {
    setCsrfToken(info.csrf_token ?? '');
    setUser(info.user);
  }, []);

  const signOut = useCallback(async () => {
    try {
      await api('/api/v1/auth/logout', { method: 'POST' });
    } finally {
      setCsrfToken('');
      setUser(null);
      queryClient.clear();
    }
  }, [queryClient]);

  return <SessionContext.Provider value={{ user, signedIn, signOut, refresh }}>{children}</SessionContext.Provider>;
}

/** Returns the session context. Must be used inside SessionProvider. */
export function useSession(): SessionValue {
  const v = useContext(SessionContext);
  if (!v) throw new Error('useSession must be used inside SessionProvider');
  return v;
}

/**
 * Returns `next` if it is a safe same-site path, otherwise `/`. Prevents open redirects
 * after sign-in: it must start with `/` but not `//` or `/\` (docs/reference/10 §3).
 */
export function safeNext(next: string | null): string {
  if (!next || !next.startsWith('/') || next.startsWith('//') || next.startsWith('/\\')) return '/';
  return next;
}
