import { useQuery } from '@tanstack/react-query';
import { api } from '../client';
import type { Blob, Branch, Commit, Repo, TreeResponse } from '../types';

/** Encodes each segment of a slash-separated path for use in a URL. */
export function encodePath(p: string): string {
  return p.split('/').map(encodeURIComponent).join('/');
}

/** Base API path for a repository. */
export function repoAPI(owner: string, name: string): string {
  return `/api/v1/repos/${encodeURIComponent(owner)}/${encodeURIComponent(name)}`;
}

/** Loads a repository. Query key `['repo', owner, name]`; invalidated by settings changes. */
export function useRepo(owner: string, name: string) {
  return useQuery({ queryKey: ['repo', owner, name], queryFn: () => api<Repo>(repoAPI(owner, name)) });
}

/**
 * Loads a directory listing. Query key `['tree', owner, name, ref, path]`. Disabled for
 * empty repositories (pass enabled=false).
 */
export function useTree(owner: string, name: string, ref: string, path: string, enabled = true) {
  const tail = ref ? `/tree/${encodePath(ref)}${path ? '/' + encodePath(path) : ''}` : '/tree';
  return useQuery({ queryKey: ['tree', owner, name, ref, path], queryFn: () => api<TreeResponse>(repoAPI(owner, name) + tail), enabled });
}

/** Loads one file. Query key `['blob', owner, name, ref, path]`. */
export function useBlob(owner: string, name: string, ref: string, path: string, enabled = true) {
  return useQuery({
    queryKey: ['blob', owner, name, ref, path],
    queryFn: () => api<{ ref: string; blob: Blob }>(`${repoAPI(owner, name)}/blob/${encodePath(ref)}/${encodePath(path)}`),
    enabled,
  });
}

/** Loads branches. Query key `['branches', owner, name]`. */
export function useBranches(owner: string, name: string, enabled = true) {
  return useQuery({ queryKey: ['branches', owner, name], queryFn: () => api<Branch[]>(`${repoAPI(owner, name)}/branches`), enabled });
}

/** Loads one page of history. Query key `['commits', owner, name, ref, path, page]`. */
export function useCommits(owner: string, name: string, ref: string, path: string, page: number) {
  const q = new URLSearchParams({ ref, path, page: String(page), per_page: '30' });
  return useQuery({ queryKey: ['commits', owner, name, ref, path, page], queryFn: () => api<Commit[]>(`${repoAPI(owner, name)}/commits?${q}`) });
}
