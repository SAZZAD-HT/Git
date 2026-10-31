import { useQuery } from '@tanstack/react-query';
import { api } from '../client';
import type { CompareView, Issue, IssueComment, Label, Pull } from '../types';
import { repoAPI } from './useRepo';

/** Items per page in issue and pull request lists. */
export const PER_PAGE = 30;

/** Filters for the issue list (GET /issues). */
export interface IssueQuery {
  state: 'open' | 'closed' | 'all';
  label?: string;
  author?: string;
  q?: string;
  page: number;
}

function params(o: Record<string, string | number | undefined>): string {
  const p = new URLSearchParams();
  for (const [k, v] of Object.entries(o)) if (v !== undefined && v !== '') p.set(k, String(v));
  return p.toString();
}

/** Lists issues (not pull requests). Query key `['issues', owner, name, query]`. */
export function useIssues(owner: string, name: string, query: IssueQuery) {
  return useQuery({
    queryKey: ['issues', owner, name, query],
    queryFn: () => api<Issue[]>(`${repoAPI(owner, name)}/issues?${params({ ...query, per_page: PER_PAGE })}`),
  });
}

/** Loads one issue. Query key `['issue', owner, name, number]`. */
export function useIssue(owner: string, name: string, number: number) {
  return useQuery({ queryKey: ['issue', owner, name, number], queryFn: () => api<Issue>(`${repoAPI(owner, name)}/issues/${number}`) });
}

/** Loads the timeline of an issue or pull request. Query key `['comments', owner, name, number]`. */
export function useComments(owner: string, name: string, number: number) {
  return useQuery({
    queryKey: ['comments', owner, name, number],
    queryFn: () => api<IssueComment[]>(`${repoAPI(owner, name)}/issues/${number}/comments`),
  });
}

/** Loads the repository's labels. Query key `['labels', owner, name]`. */
export function useLabels(owner: string, name: string) {
  return useQuery({ queryKey: ['labels', owner, name], queryFn: () => api<Label[]>(`${repoAPI(owner, name)}/labels`) });
}

/** Lists pull requests. Query key `['pulls', owner, name, state, page]`. */
export function usePulls(owner: string, name: string, state: string, page: number) {
  return useQuery({
    queryKey: ['pulls', owner, name, state, page],
    queryFn: () => api<Pull[]>(`${repoAPI(owner, name)}/pulls?${params({ state, page, per_page: PER_PAGE })}`),
  });
}

/**
 * Loads one pull request. Query key `['pull', owner, name, number]`. While the server is
 * still checking mergeability it polls every 2 seconds.
 */
export function usePull(owner: string, name: string, number: number) {
  return useQuery({
    queryKey: ['pull', owner, name, number],
    queryFn: () => api<Pull>(`${repoAPI(owner, name)}/pulls/${number}`),
    refetchInterval: (q) => (q.state.data && q.state.data.state === 'open' && (q.state.data.merge_status === 'checking' || q.state.data.merge_status === 'unknown') ? 2000 : false),
  });
}

/** Loads a pull request's commits and diff. Query key `['pull-compare', owner, name, number, headSHA]`. */
export function usePullCompare(owner: string, name: string, number: number, headSHA: string, enabled = true) {
  return useQuery({
    queryKey: ['pull-compare', owner, name, number, headSHA],
    queryFn: () => api<CompareView>(`${repoAPI(owner, name)}/pulls/${number}/compare`),
    enabled,
  });
}
