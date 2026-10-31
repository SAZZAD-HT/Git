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

/** A repository. `permission` is the caller's access level. */
export interface Repo {
  id: number;
  owner: string;
  name: string;
  full_name: string;
  description: string;
  private: boolean;
  default_branch: string;
  empty: boolean;
  fork: boolean;
  clone_url: string;
  permission: 'none' | 'read' | 'write' | 'admin';
  created_at: string;
  updated_at: string;
}

/** One entry of a directory listing. */
export interface TreeEntry {
  name: string;
  path: string;
  type: 'blob' | 'tree' | 'commit';
  mode: string;
  sha: string;
  size: number;
}

/** A git identity with a timestamp. */
export interface Signature {
  name: string;
  email: string;
  when: string;
}

/** A commit in a history listing. */
export interface Commit {
  sha: string;
  parents: string[];
  author: Signature;
  committer: Signature;
  subject: string;
  body: string;
}

/** Body of GET /repos/{o}/{r}/tree/{ref}/{path}. */
export interface TreeResponse {
  commit: string;
  ref: string;
  path: string;
  entries: TreeEntry[];
  last_commit?: Commit;
}

/** A file at a commit. `content` is empty for binary or too-large files. */
export interface Blob {
  commit: string;
  path: string;
  sha: string;
  size: number;
  binary: boolean;
  too_large: boolean;
  content: string;
}

/** A branch with its head commit. */
export interface Branch {
  name: string;
  commit: string;
  protected: boolean;
  is_default: boolean;
}

/** A collaborator and their role. */
export interface Collaborator {
  username: string;
  role: 'read' | 'write' | 'admin';
}

/** A branch protection rule. */
export interface ProtectionRule {
  id: number;
  pattern: string;
  allow_force_push: boolean;
  allow_delete: boolean;
  require_pull_request: boolean;
  required_checks: string[];
}

/** One line of a diff hunk. */
export interface DiffLine {
  kind: 'context' | 'add' | 'del';
  old: number;
  new: number;
  text: string;
}

/** One "@@" section of a file diff. */
export interface Hunk {
  header: string;
  old_start: number;
  new_start: number;
  lines: DiffLine[];
}

/** The change to one file. */
export interface FileDiff {
  old_path: string;
  new_path: string;
  status: 'added' | 'deleted' | 'modified' | 'renamed' | 'copied';
  binary: boolean;
  additions: number;
  deletions: number;
  hunks: Hunk[];
  truncated: boolean;
}

/** A parsed diff. */
export interface Diff {
  files: FileDiff[];
  additions: number;
  deletions: number;
  truncated: boolean;
}

/** A commit with its diff. */
export interface CommitDetail extends Commit {
  diff?: Diff;
}

/** Body of GET /compare/{base}...{head}. */
export interface CompareView {
  base: string;
  head: string;
  base_sha: string;
  head_sha: string;
  merge_base: string;
  commits: Commit[];
  diff: Diff;
  identical: boolean;
  ahead_by: number;
  behind_base: boolean;
}
