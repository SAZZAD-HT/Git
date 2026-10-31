package api

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"anvil/internal/auth"
	"anvil/internal/errs"
	"anvil/internal/repos"
	"anvil/internal/store"
)

// repoJSON is the API representation of a repository.
type repoJSON struct {
	ID            int64  `json:"id"`
	Owner         string `json:"owner"`
	Name          string `json:"name"`
	FullName      string `json:"full_name"`
	Description   string `json:"description"`
	Private       bool   `json:"private"`
	DefaultBranch string `json:"default_branch"`
	Empty         bool   `json:"empty"`
	Fork          bool   `json:"fork"`
	CloneURL      string `json:"clone_url"`
	Permission    string `json:"permission"` // the caller's access: none, read, write, admin
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

func (h *handlers) toRepoJSON(r *http.Request, repo *store.Repository) repoJSON {
	return repoJSON{ID: repo.ID, Owner: repo.OwnerName, Name: repo.Name, FullName: repo.FullName(), Description: repo.Description,
		Private: repo.IsPrivate(), DefaultBranch: repo.DefaultBranch, Empty: repo.IsEmpty, Fork: repo.ForkOfID != 0,
		CloneURL:   strings.TrimRight(h.PublicURL, "/") + "/" + repo.FullName() + ".git",
		Permission: h.Repos.Permission(r.Context(), auth.ActorFrom(r.Context()), repo).String(),
		CreatedAt:  rfc3339(repo.CreatedAt), UpdatedAt: rfc3339(repo.UpdatedAt)}
}

func (h *handlers) toRepoList(r *http.Request, list []store.Repository) []repoJSON {
	out := make([]repoJSON, len(list))
	for i := range list {
		out[i] = h.toRepoJSON(r, &list[i])
	}
	return out
}

func (h *handlers) mountRepos(r chi.Router) {
	r.Post("/repos", h.createRepo)
	r.Get("/user/repos", h.myRepos)
	r.Get("/users/{username}/repos", h.userRepos)
	r.Route("/repos/{owner}/{repo}", func(r chi.Router) {
		r.Get("/", h.getRepo)
		r.Patch("/", h.updateRepo)
		r.Delete("/", h.deleteRepo)
		r.Get("/collaborators", h.listCollaborators)
		r.Put("/collaborators/{username}", h.setCollaborator)
		r.Delete("/collaborators/{username}", h.removeCollaborator)
		r.Get("/protected-branches", h.listProtections)
		r.Post("/protected-branches", h.saveProtection)
		r.Patch("/protected-branches/{id}", h.saveProtection)
		r.Delete("/protected-branches/{id}", h.deleteProtection)
		r.Get("/branches", h.listBranches)
		r.Get("/tags", h.listTags)
		r.Get("/tree", h.getTree)
		r.Get("/tree/*", h.getTree)
		r.Get("/blob/*", h.getBlob)
		r.Get("/raw/*", h.getRaw)
		r.Get("/commits", h.listCommits)
	})
}

func ownerRepo(r *http.Request) (string, string) {
	return chi.URLParam(r, "owner"), chi.URLParam(r, "repo")
}

func repoPage(r *http.Request) repos.Page {
	p := page(r)
	return repos.Page{Page: p.Page, PerPage: p.PerPage}
}

// createRepo serves POST /api/v1/repos: {name, description, private, default_branch, init_readme}.
func (h *handlers) createRepo(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name          string `json:"name"`
		Description   string `json:"description"`
		Private       bool   `json:"private"`
		DefaultBranch string `json:"default_branch"`
		InitReadme    bool   `json:"init_readme"`
	}
	if err := decodeJSON(r, &in); err != nil {
		WriteError(w, r, err)
		return
	}
	repo, err := h.Repos.Create(r.Context(), auth.ActorFrom(r.Context()), repos.CreateRepo{Name: in.Name, Description: in.Description,
		Private: in.Private, DefaultBranch: in.DefaultBranch, InitReadme: in.InitReadme})
	if err != nil {
		WriteError(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusCreated, h.toRepoJSON(r, repo))
}

// myRepos serves GET /api/v1/user/repos: repositories you own or collaborate on.
func (h *handlers) myRepos(w http.ResponseWriter, r *http.Request) {
	list, err := h.Repos.ListMine(r.Context(), auth.ActorFrom(r.Context()), 100)
	if err != nil {
		WriteError(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusOK, h.toRepoList(r, list))
}

// userRepos serves GET /api/v1/users/{username}/repos: the repositories visible to the caller.
func (h *handlers) userRepos(w http.ResponseWriter, r *http.Request) {
	list, total, err := h.Repos.ListForUser(r.Context(), auth.ActorFrom(r.Context()), chi.URLParam(r, "username"), repoPage(r))
	if err != nil {
		WriteError(w, r, err)
		return
	}
	writePaged(w, r, h.toRepoList(r, list), page(r), total)
}

// getRepo serves GET /api/v1/repos/{o}/{r}.
func (h *handlers) getRepo(w http.ResponseWriter, r *http.Request) {
	o, n := ownerRepo(r)
	repo, err := h.Repos.Get(r.Context(), auth.ActorFrom(r.Context()), o, n)
	if err != nil {
		WriteError(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusOK, h.toRepoJSON(r, repo))
}

// updateRepo serves PATCH /api/v1/repos/{o}/{r}: {name?, description?, private?, default_branch?}.
func (h *handlers) updateRepo(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name          *string `json:"name"`
		Description   *string `json:"description"`
		Private       *bool   `json:"private"`
		DefaultBranch *string `json:"default_branch"`
	}
	if err := decodeJSON(r, &in); err != nil {
		WriteError(w, r, err)
		return
	}
	o, n := ownerRepo(r)
	repo, err := h.Repos.Update(r.Context(), auth.ActorFrom(r.Context()), o, n,
		repos.UpdateRepo{Name: in.Name, Description: in.Description, Private: in.Private, DefaultBranch: in.DefaultBranch})
	if err != nil {
		WriteError(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusOK, h.toRepoJSON(r, repo))
}

// deleteRepo serves DELETE /api/v1/repos/{o}/{r}: {confirm: "<owner>/<name>"}.
func (h *handlers) deleteRepo(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Confirm string `json:"confirm"`
	}
	if err := decodeJSON(r, &in); err != nil {
		WriteError(w, r, err)
		return
	}
	o, n := ownerRepo(r)
	if err := h.Repos.Delete(r.Context(), auth.ActorFrom(r.Context()), o, n, in.Confirm); err != nil {
		WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// listCollaborators serves GET /api/v1/repos/{o}/{r}/collaborators.
func (h *handlers) listCollaborators(w http.ResponseWriter, r *http.Request) {
	o, n := ownerRepo(r)
	list, err := h.Repos.Collaborators(r.Context(), auth.ActorFrom(r.Context()), o, n)
	if err != nil {
		WriteError(w, r, err)
		return
	}
	type collabJSON struct {
		Username string `json:"username"`
		Role     string `json:"role"`
	}
	out := make([]collabJSON, len(list))
	for i, c := range list {
		out[i] = collabJSON{c.Username, c.Role}
	}
	writeJSON(w, r, http.StatusOK, out)
}

// setCollaborator serves PUT /api/v1/repos/{o}/{r}/collaborators/{username}: {role}.
func (h *handlers) setCollaborator(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Role string `json:"role"`
	}
	if err := decodeJSON(r, &in); err != nil {
		WriteError(w, r, err)
		return
	}
	level, err := auth.ParseLevel(in.Role)
	if err != nil {
		WriteError(w, r, errs.FieldErrors{"role": "must be read, write or admin"})
		return
	}
	o, n := ownerRepo(r)
	if err := h.Repos.SetCollaborator(r.Context(), auth.ActorFrom(r.Context()), o, n, chi.URLParam(r, "username"), level); err != nil {
		WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// removeCollaborator serves DELETE /api/v1/repos/{o}/{r}/collaborators/{username}.
func (h *handlers) removeCollaborator(w http.ResponseWriter, r *http.Request) {
	o, n := ownerRepo(r)
	if err := h.Repos.RemoveCollaborator(r.Context(), auth.ActorFrom(r.Context()), o, n, chi.URLParam(r, "username")); err != nil {
		WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// protectionJSON is a branch protection rule.
type protectionJSON struct {
	ID                 int64    `json:"id"`
	Pattern            string   `json:"pattern"`
	AllowForcePush     bool     `json:"allow_force_push"`
	AllowDelete        bool     `json:"allow_delete"`
	RequirePullRequest bool     `json:"require_pull_request"`
	RequiredChecks     []string `json:"required_checks"`
}

func toProtectionJSON(p *store.ProtectedBranch) protectionJSON {
	return protectionJSON{p.ID, p.Pattern, p.AllowForcePush, p.AllowDelete, p.RequirePullRequest, p.RequiredChecks}
}

// listProtections serves GET /api/v1/repos/{o}/{r}/protected-branches.
func (h *handlers) listProtections(w http.ResponseWriter, r *http.Request) {
	o, n := ownerRepo(r)
	list, err := h.Repos.Protections(r.Context(), auth.ActorFrom(r.Context()), o, n)
	if err != nil {
		WriteError(w, r, err)
		return
	}
	out := make([]protectionJSON, len(list))
	for i := range list {
		out[i] = toProtectionJSON(&list[i])
	}
	writeJSON(w, r, http.StatusOK, out)
}

// saveProtection serves POST /protected-branches (create) and PATCH /protected-branches/{id}.
func (h *handlers) saveProtection(w http.ResponseWriter, r *http.Request) {
	var in protectionJSON
	if err := decodeJSON(r, &in); err != nil {
		WriteError(w, r, err)
		return
	}
	in.ID = 0
	if s := chi.URLParam(r, "id"); s != "" {
		id, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			WriteError(w, r, fmt.Errorf("rule %q: %w", s, errs.NotFound))
			return
		}
		in.ID = id
	}
	o, n := ownerRepo(r)
	p, err := h.Repos.SaveProtection(r.Context(), auth.ActorFrom(r.Context()), o, n, repos.ProtectionRule{ID: in.ID, Pattern: in.Pattern,
		AllowForcePush: in.AllowForcePush, AllowDelete: in.AllowDelete, RequirePullRequest: in.RequirePullRequest, RequiredChecks: in.RequiredChecks})
	if err != nil {
		WriteError(w, r, err)
		return
	}
	status := http.StatusOK
	if in.ID == 0 {
		status = http.StatusCreated
	}
	writeJSON(w, r, status, toProtectionJSON(p))
}

// deleteProtection serves DELETE /api/v1/repos/{o}/{r}/protected-branches/{id}.
func (h *handlers) deleteProtection(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		WriteError(w, r, fmt.Errorf("rule: %w", errs.NotFound))
		return
	}
	o, n := ownerRepo(r)
	if err := h.Repos.DeleteProtection(r.Context(), auth.ActorFrom(r.Context()), o, n, id); err != nil {
		WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// listBranches serves GET /api/v1/repos/{o}/{r}/branches.
func (h *handlers) listBranches(w http.ResponseWriter, r *http.Request) {
	o, n := ownerRepo(r)
	list, err := h.Repos.Branches(r.Context(), auth.ActorFrom(r.Context()), o, n)
	if err != nil {
		WriteError(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusOK, list)
}

// listTags serves GET /api/v1/repos/{o}/{r}/tags.
func (h *handlers) listTags(w http.ResponseWriter, r *http.Request) {
	o, n := ownerRepo(r)
	list, err := h.Repos.Tags(r.Context(), auth.ActorFrom(r.Context()), o, n)
	if err != nil {
		WriteError(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusOK, list)
}

// refPath splits the wildcard of /tree/* and /blob/* into a ref and a path.
func (h *handlers) refPath(r *http.Request) (ref, path string, err error) {
	o, n := ownerRepo(r)
	return h.Repos.SplitRefPath(r.Context(), auth.ActorFrom(r.Context()), o, n, chi.URLParam(r, "*"))
}

// getTree serves GET /api/v1/repos/{o}/{r}/tree/{ref}/{path...} ("/tree" alone is the
// default branch's root). The response includes the latest commit touching the directory.
func (h *handlers) getTree(w http.ResponseWriter, r *http.Request) {
	ref, path, err := h.refPath(r)
	if err != nil {
		WriteError(w, r, err)
		return
	}
	o, n := ownerRepo(r)
	actor := auth.ActorFrom(r.Context())
	tree, err := h.Repos.Tree(r.Context(), actor, o, n, ref, path)
	if err != nil {
		WriteError(w, r, err)
		return
	}
	out := map[string]any{"commit": tree.Commit, "ref": ref, "path": tree.Path, "entries": tree.Entries}
	if last, err := h.Repos.Commits(r.Context(), actor, o, n, tree.Commit, path, repos.Page{Page: 1, PerPage: 1}); err == nil && len(last) == 1 {
		out["last_commit"] = last[0]
	}
	writeJSON(w, r, http.StatusOK, out)
}

// getBlob serves GET /api/v1/repos/{o}/{r}/blob/{ref}/{path...}: file metadata and text.
func (h *handlers) getBlob(w http.ResponseWriter, r *http.Request) {
	ref, path, err := h.refPath(r)
	if err != nil {
		WriteError(w, r, err)
		return
	}
	o, n := ownerRepo(r)
	blob, err := h.Repos.Blob(r.Context(), auth.ActorFrom(r.Context()), o, n, ref, path)
	if err != nil {
		WriteError(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusOK, map[string]any{"ref": ref, "blob": blob})
}

// maxRaw is the largest file served by /raw (larger files arrive with releases and archives).
const maxRaw = 100 << 20

// getRaw serves GET /api/v1/repos/{o}/{r}/raw/{ref}/{path...} as a download. It is always an
// attachment with nosniff and a sandboxing CSP, so an uploaded .html or .svg cannot run
// scripts on Anvil's origin (docs/reference/10 §3).
func (h *handlers) getRaw(w http.ResponseWriter, r *http.Request) {
	ref, path, err := h.refPath(r)
	if err != nil {
		WriteError(w, r, err)
		return
	}
	o, n := ownerRepo(r)
	data, err := h.Repos.RawBlob(r.Context(), auth.ActorFrom(r.Context()), o, n, ref, path, maxRaw)
	if err != nil {
		WriteError(w, r, err)
		return
	}
	name := path[strings.LastIndex(path, "/")+1:]
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", name))
	w.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Write(data)
}

// listCommits serves GET /api/v1/repos/{o}/{r}/commits?ref=&path=&page=&per_page=.
func (h *handlers) listCommits(w http.ResponseWriter, r *http.Request) {
	o, n := ownerRepo(r)
	q := r.URL.Query()
	list, err := h.Repos.Commits(r.Context(), auth.ActorFrom(r.Context()), o, n, q.Get("ref"), q.Get("path"), repoPage(r))
	if err != nil {
		WriteError(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusOK, list)
}
