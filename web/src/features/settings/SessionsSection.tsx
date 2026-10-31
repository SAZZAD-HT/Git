import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '../../api/client';
import type { BrowserSession } from '../../api/types';
import { formatTime } from '../../components/formErrors';

/**
 * Your browser sessions at `/settings/sessions` (FR-AUTH-10). Query key `['user-sessions']`,
 * invalidated after revoking a session or changing the password.
 */
export function SessionsSection() {
  const queryClient = useQueryClient();
  const sessions = useQuery({ queryKey: ['user-sessions'], queryFn: () => api<BrowserSession[]>('/api/v1/user/sessions') });
  const revoke = useMutation({
    mutationFn: (id: string) => api(`/api/v1/user/sessions/${encodeURIComponent(id)}`, { method: 'DELETE' }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['user-sessions'] }),
  });

  return (
    <section className="panel">
      <h2>Sessions</h2>
      <p className="muted">Browsers signed in to your account. Sign out any you do not recognise.</p>
      {sessions.isPending && <p className="muted">Loading…</p>}
      {sessions.isError && <p className="error">{sessions.error.message}</p>}
      <ul className="session-list">
        {sessions.data?.map((s) => (
          <li key={s.id}>
            <div>
              <div className="session-agent">{s.user_agent || 'Unknown browser'} {s.current && <span className="badge badge-ok">This browser</span>}</div>
              <div className="muted">{s.ip || 'unknown IP'} · signed in {formatTime(s.created_at)} · last active {formatTime(s.last_seen_at)}</div>
            </div>
            {!s.current && (
              <button type="button" className="button button-danger" disabled={revoke.isPending} onClick={() => revoke.mutate(s.id)}>Sign out</button>
            )}
          </li>
        ))}
      </ul>
    </section>
  );
}
