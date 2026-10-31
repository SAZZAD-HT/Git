import { useQuery } from '@tanstack/react-query';
import { api, ApiError } from '../client';

/** Body of GET /api/v1/version. Replaced by the generated schema.d.ts once api/openapi.yaml exists. */
export interface VersionInfo {
  version: string;
  commit: string;
  build_date: string;
  go_version: string;
  git_version: string;
}

/** Body of GET /readyz. */
export interface Readiness {
  status: 'ready' | 'not_ready';
  checks: Record<string, string>;
}

/**
 * Loads the server version. Query key `['version']`; never invalidated (it changes only on restart).
 */
export function useVersion() {
  return useQuery({ queryKey: ['version'], queryFn: () => api<VersionInfo>('/api/v1/version'), staleTime: Infinity });
}

/**
 * Loads server readiness, refreshing every 10 seconds. Query key `['readyz']`.
 * A 503 response still carries the check results, so it is returned instead of thrown.
 */
export function useReadiness() {
  return useQuery({
    queryKey: ['readyz'],
    refetchInterval: 10_000,
    queryFn: async () => {
      try {
        return await api<Readiness>('/readyz');
      } catch (e) {
        if (e instanceof ApiError && e.status === 503) return { status: 'not_ready', checks: { server: e.message } } as Readiness;
        throw e;
      }
    },
  });
}
