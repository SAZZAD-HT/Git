import { Link, useParams } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { api, ApiError } from '../../api/client';
import type { Repo, User } from '../../api/types';
import { NotFoundPage } from '../../components/NotFoundPage';
import { RepoList } from '../../components/RepoList';

/** A user's profile and the repositories you can see, at `/:owner`. Query keys `['user', name]`, `['user-repos', name]`. */
export function ProfilePage() {
  const { owner = '' } = useParams();
  const user = useQuery({ queryKey: ['user', owner], queryFn: () => api<User>(`/api/v1/users/${encodeURIComponent(owner)}`) });
  const repos = useQuery({ queryKey: ['user-repos', owner], queryFn: () => api<Repo[]>(`/api/v1/users/${encodeURIComponent(owner)}/repos?per_page=100`), enabled: !!user.data });

  if (user.isError) return user.error instanceof ApiError && user.error.status === 404 ? <NotFoundPage /> : <p className="error">{user.error.message}</p>;
  if (!user.data) return <p className="muted">Loading…</p>;
  const u = user.data;
  return (
    <div className="profile">
      <aside className="profile-card">
        <div className="avatar avatar-lg" aria-hidden="true">{u.username.slice(0, 1).toUpperCase()}</div>
        <h1>{u.full_name || u.username}</h1>
        {u.full_name && <p className="muted">{u.username}</p>}
        {u.is_admin && <span className="badge badge-ok">Site admin</span>}
      </aside>
      <section>
        <h2>Repositories</h2>
        {repos.data && <RepoList repos={repos.data} empty={<p className="muted">No repositories you can see. <Link to="/new">Create one</Link>?</p>} />}
      </section>
    </div>
  );
}
