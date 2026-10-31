package api

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"anvil/internal/auth"
	"anvil/internal/errs"
	"anvil/internal/issues"
	"anvil/internal/pulls"
	"anvil/internal/store"
)

// labelJSON is a repository label.
type labelJSON struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

// issueJSON is an issue or pull request in lists and detail views.
type issueJSON struct {
	Number    int64       `json:"number"`
	Title     string      `json:"title"`
	Body      string      `json:"body"`
	State     string      `json:"state"`
	IsPull    bool        `json:"is_pull"`
	Author    string      `json:"author"`
	Comments  int         `json:"comments"`
	Labels    []labelJSON `json:"labels"`
	CreatedAt string      `json:"created_at"`
	UpdatedAt string      `json:"updated_at"`
	ClosedAt  string      `json:"closed_at,omitempty"`
}

func toIssueJSON(i *store.Issue) issueJSON {
	j := issueJSON{Number: i.Number, Title: i.Title, Body: i.Body, State: i.State, IsPull: i.IsPull, Author: i.AuthorName,
		Comments: i.Comments, Labels: []labelJSON{}, CreatedAt: rfc3339(i.CreatedAt), UpdatedAt: rfc3339(i.UpdatedAt), ClosedAt: rfc3339(i.ClosedAt)}
	for _, l := range i.Labels {
		j.Labels = append(j.Labels, labelJSON{l.ID, l.Name, l.Color})
	}
	return j
}

// commentJSON is one entry of an issue or pull request timeline.
type commentJSON struct {
	ID          int64  `json:"id"`
	Author      string `json:"author"`
	Body        string `json:"body"`
	Kind        string `json:"kind"`
	ReviewState string `json:"review_state,omitempty"`
	ReviewID    int64  `json:"review_id,omitempty"`
	CommitSHA   string `json:"commit_sha,omitempty"`
	Path        string `json:"path,omitempty"`
	Line        int    `json:"line,omitempty"`
	Side        string `json:"side,omitempty"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

func toCommentJSON(c *store.Comment) commentJSON {
	return commentJSON{c.ID, c.AuthorName, c.Body, c.Kind, c.ReviewState, c.ReviewID, c.CommitSHA, c.Path, c.Line, c.Side,
		rfc3339(c.CreatedAt), rfc3339(c.UpdatedAt)}
}

// pullJSON is a pull request: its issue fields plus branches and merge state.
type pullJSON struct {
	issueJSON
	Base           string   `json:"base"`
	Head           string   `json:"head"`
	HeadSHA        string   `json:"head_sha"`
	MergeStatus    string   `json:"merge_status"`
	ConflictFiles  []string `json:"conflict_files"`
	Merged         bool     `json:"merged"`
	MergedAt       string   `json:"merged_at,omitempty"`
	MergeCommitSHA string   `json:"merge_commit_sha,omitempty"`
}

func toPullJSON(v *pulls.View) pullJSON {
	files := v.Pull.ConflictFiles
	if files == nil {
		files = []string{}
	}
	return pullJSON{issueJSON: toIssueJSON(v.Issue), Base: v.Pull.BaseBranch, Head: v.Pull.HeadBranch, HeadSHA: v.Pull.HeadSHA,
		MergeStatus: v.Pull.MergeStatus, ConflictFiles: files, Merged: v.Pull.MergedAt != 0,
		MergedAt: rfc3339(v.Pull.MergedAt), MergeCommitSHA: v.Pull.MergeCommitSHA}
}

// mountIssues registers issue and pull request routes on r, which is already scoped to
// /repos/{owner}/{repo} by mountRepos (chi allows each pattern to be mounted only once).
func (h *handlers) mountIssues(r chi.Router) {
	{
		r.Get("/issues", h.listIssues)
		r.Post("/issues", h.createIssue)
		r.Get("/issues/{n}", h.getIssue)
		r.Patch("/issues/{n}", h.updateIssue)
		r.Get("/issues/{n}/comments", h.listComments)
		r.Post("/issues/{n}/comments", h.addComment)
		r.Patch("/comments/{id}", h.editComment)
		r.Delete("/comments/{id}", h.deleteComment)
		r.Get("/labels", h.listLabels)
		r.Post("/labels", h.saveLabel)
		r.Patch("/labels/{id}", h.saveLabel)
		r.Delete("/labels/{id}", h.deleteLabel)
		r.Get("/pulls", h.listPulls)
		r.Post("/pulls", h.openPull)
		r.Get("/pulls/{n}", h.getPull)
		r.Patch("/pulls/{n}", h.updateIssue) // title, body, state: same rules as issues
		r.Get("/pulls/{n}/compare", h.pullCompare)
		r.Post("/pulls/{n}/reviews", h.submitReview)
		r.Post("/pulls/{n}/merge", h.mergePull)
	}
}

// number reads {n}; a non-number is reported as not found.
func number(r *http.Request) (int64, error) {
	n, err := strconv.ParseInt(chi.URLParam(r, "n"), 10, 64)
	if err != nil || n < 1 {
		return 0, fmt.Errorf("#%s: %w", chi.URLParam(r, "n"), errs.NotFound)
	}
	return n, nil
}

func idParam(r *http.Request) (int64, error) {
	n, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("id %s: %w", chi.URLParam(r, "id"), errs.NotFound)
	}
	return n, nil
}

// listIssues serves GET /issues?state=open|closed|all&label=&author=&q=&type=issues|all.
func (h *handlers) listIssues(w http.ResponseWriter, r *http.Request) {
	o, n := ownerRepo(r)
	q := r.URL.Query()
	f := issues.Filter{State: q.Get("state"), Label: q.Get("label"), Author: q.Get("author"), Query: q.Get("q")}
	if q.Get("type") != "all" {
		no := false
		f.Pulls = &no
	}
	list, total, err := h.Issues.List(r.Context(), auth.ActorFrom(r.Context()), o, n, f, repoPage(r))
	if err != nil {
		WriteError(w, r, err)
		return
	}
	out := make([]issueJSON, len(list))
	for i := range list {
		out[i] = toIssueJSON(&list[i])
	}
	writePaged(w, r, out, page(r), total)
}

// createIssue serves POST /issues: {title, body, label_ids}.
func (h *handlers) createIssue(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Title    string  `json:"title"`
		Body     string  `json:"body"`
		LabelIDs []int64 `json:"label_ids"`
	}
	if err := decodeJSON(r, &in); err != nil {
		WriteError(w, r, err)
		return
	}
	o, n := ownerRepo(r)
	iss, err := h.Issues.Create(r.Context(), auth.ActorFrom(r.Context()), o, n, issues.CreateIssue{Title: in.Title, Body: in.Body, LabelIDs: in.LabelIDs})
	if err != nil {
		WriteError(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusCreated, toIssueJSON(iss))
}

// getIssue serves GET /issues/{n} (issues and pull requests).
func (h *handlers) getIssue(w http.ResponseWriter, r *http.Request) {
	num, err := number(r)
	if err != nil {
		WriteError(w, r, err)
		return
	}
	o, n := ownerRepo(r)
	_, iss, err := h.Issues.Get(r.Context(), auth.ActorFrom(r.Context()), o, n, num)
	if err != nil {
		WriteError(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusOK, toIssueJSON(iss))
}

// updateIssue serves PATCH /issues/{n} and /pulls/{n}: {title?, body?, state?, label_ids?}.
func (h *handlers) updateIssue(w http.ResponseWriter, r *http.Request) {
	num, err := number(r)
	if err != nil {
		WriteError(w, r, err)
		return
	}
	var in struct {
		Title    *string  `json:"title"`
		Body     *string  `json:"body"`
		State    *string  `json:"state"`
		LabelIDs *[]int64 `json:"label_ids"`
	}
	if err := decodeJSON(r, &in); err != nil {
		WriteError(w, r, err)
		return
	}
	o, n := ownerRepo(r)
	iss, err := h.Issues.Update(r.Context(), auth.ActorFrom(r.Context()), o, n, num, issues.UpdateIssue{Title: in.Title, Body: in.Body, State: in.State, LabelIDs: in.LabelIDs})
	if err != nil {
		WriteError(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusOK, toIssueJSON(iss))
}

// listComments serves GET /issues/{n}/comments: the whole timeline, oldest first.
func (h *handlers) listComments(w http.ResponseWriter, r *http.Request) {
	num, err := number(r)
	if err != nil {
		WriteError(w, r, err)
		return
	}
	o, n := ownerRepo(r)
	list, err := h.Issues.Comments(r.Context(), auth.ActorFrom(r.Context()), o, n, num)
	if err != nil {
		WriteError(w, r, err)
		return
	}
	out := make([]commentJSON, len(list))
	for i := range list {
		out[i] = toCommentJSON(&list[i])
	}
	writeJSON(w, r, http.StatusOK, out)
}

// addComment serves POST /issues/{n}/comments: {body}.
func (h *handlers) addComment(w http.ResponseWriter, r *http.Request) {
	num, err := number(r)
	if err != nil {
		WriteError(w, r, err)
		return
	}
	var in struct {
		Body string `json:"body"`
	}
	if err := decodeJSON(r, &in); err != nil {
		WriteError(w, r, err)
		return
	}
	o, n := ownerRepo(r)
	c, err := h.Issues.AddComment(r.Context(), auth.ActorFrom(r.Context()), o, n, num, in.Body)
	if err != nil {
		WriteError(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusCreated, toCommentJSON(c))
}

// editComment serves PATCH /comments/{id}: {body}.
func (h *handlers) editComment(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r)
	if err != nil {
		WriteError(w, r, err)
		return
	}
	var in struct {
		Body string `json:"body"`
	}
	if err := decodeJSON(r, &in); err != nil {
		WriteError(w, r, err)
		return
	}
	o, n := ownerRepo(r)
	c, err := h.Issues.EditComment(r.Context(), auth.ActorFrom(r.Context()), o, n, id, in.Body)
	if err != nil {
		WriteError(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusOK, toCommentJSON(c))
}

// deleteComment serves DELETE /comments/{id}.
func (h *handlers) deleteComment(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r)
	if err != nil {
		WriteError(w, r, err)
		return
	}
	o, n := ownerRepo(r)
	if err := h.Issues.DeleteComment(r.Context(), auth.ActorFrom(r.Context()), o, n, id); err != nil {
		WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// listLabels serves GET /labels.
func (h *handlers) listLabels(w http.ResponseWriter, r *http.Request) {
	o, n := ownerRepo(r)
	list, err := h.Issues.Labels(r.Context(), auth.ActorFrom(r.Context()), o, n)
	if err != nil {
		WriteError(w, r, err)
		return
	}
	out := make([]labelJSON, len(list))
	for i, l := range list {
		out[i] = labelJSON{l.ID, l.Name, l.Color}
	}
	writeJSON(w, r, http.StatusOK, out)
}

// saveLabel serves POST /labels and PATCH /labels/{id}: {name, color}.
func (h *handlers) saveLabel(w http.ResponseWriter, r *http.Request) {
	var in labelJSON
	if err := decodeJSON(r, &in); err != nil {
		WriteError(w, r, err)
		return
	}
	in.ID = 0
	if chi.URLParam(r, "id") != "" {
		id, err := idParam(r)
		if err != nil {
			WriteError(w, r, err)
			return
		}
		in.ID = id
	}
	o, n := ownerRepo(r)
	l, err := h.Issues.SaveLabel(r.Context(), auth.ActorFrom(r.Context()), o, n, store.Label{ID: in.ID, Name: in.Name, Color: in.Color})
	if err != nil {
		WriteError(w, r, err)
		return
	}
	status := http.StatusOK
	if in.ID == 0 {
		status = http.StatusCreated
	}
	writeJSON(w, r, status, labelJSON{l.ID, l.Name, l.Color})
}

// deleteLabel serves DELETE /labels/{id}.
func (h *handlers) deleteLabel(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r)
	if err != nil {
		WriteError(w, r, err)
		return
	}
	o, n := ownerRepo(r)
	if err := h.Issues.DeleteLabel(r.Context(), auth.ActorFrom(r.Context()), o, n, id); err != nil {
		WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// listPulls serves GET /pulls?state=open|closed|all.
func (h *handlers) listPulls(w http.ResponseWriter, r *http.Request) {
	o, n := ownerRepo(r)
	list, total, err := h.Pulls.List(r.Context(), auth.ActorFrom(r.Context()), o, n, r.URL.Query().Get("state"), repoPage(r))
	if err != nil {
		WriteError(w, r, err)
		return
	}
	out := make([]pullJSON, len(list))
	for i := range list {
		out[i] = toPullJSON(&list[i])
	}
	writePaged(w, r, out, page(r), total)
}

// openPull serves POST /pulls: {title, body, base, head}.
func (h *handlers) openPull(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Title string `json:"title"`
		Body  string `json:"body"`
		Base  string `json:"base"`
		Head  string `json:"head"`
	}
	if err := decodeJSON(r, &in); err != nil {
		WriteError(w, r, err)
		return
	}
	o, n := ownerRepo(r)
	v, err := h.Pulls.Open(r.Context(), auth.ActorFrom(r.Context()), o, n, pulls.OpenPull{Title: in.Title, Body: in.Body, Base: in.Base, Head: in.Head})
	if err != nil {
		WriteError(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusCreated, toPullJSON(v))
}

// getPull serves GET /pulls/{n}.
func (h *handlers) getPull(w http.ResponseWriter, r *http.Request) {
	num, err := number(r)
	if err != nil {
		WriteError(w, r, err)
		return
	}
	o, n := ownerRepo(r)
	v, err := h.Pulls.Get(r.Context(), auth.ActorFrom(r.Context()), o, n, num)
	if err != nil {
		WriteError(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusOK, toPullJSON(v))
}

// pullCompare serves GET /pulls/{n}/compare: commits and per-file diff (FR-COLLAB-06).
func (h *handlers) pullCompare(w http.ResponseWriter, r *http.Request) {
	num, err := number(r)
	if err != nil {
		WriteError(w, r, err)
		return
	}
	o, n := ownerRepo(r)
	v, err := h.Pulls.Compare(r.Context(), auth.ActorFrom(r.Context()), o, n, num)
	if err != nil {
		WriteError(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusOK, v)
}

// submitReview serves POST /pulls/{n}/reviews: {event, body, comments:[{path,line,side,body}]}.
func (h *handlers) submitReview(w http.ResponseWriter, r *http.Request) {
	num, err := number(r)
	if err != nil {
		WriteError(w, r, err)
		return
	}
	var in struct {
		Event    string `json:"event"`
		Body     string `json:"body"`
		Comments []struct {
			Path string `json:"path"`
			Line int    `json:"line"`
			Side string `json:"side"`
			Body string `json:"body"`
		} `json:"comments"`
	}
	if err := decodeJSON(r, &in); err != nil {
		WriteError(w, r, err)
		return
	}
	rv := pulls.Review{Event: in.Event, Body: in.Body}
	for _, c := range in.Comments {
		rv.Comments = append(rv.Comments, pulls.ReviewComment{Path: c.Path, Line: c.Line, Side: c.Side, Body: c.Body})
	}
	o, n := ownerRepo(r)
	c, err := h.Pulls.SubmitReview(r.Context(), auth.ActorFrom(r.Context()), o, n, num, rv)
	if err != nil {
		WriteError(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusCreated, toCommentJSON(c))
}

// mergePull serves POST /pulls/{n}/merge: {strategy, message, expected_head_sha}.
func (h *handlers) mergePull(w http.ResponseWriter, r *http.Request) {
	num, err := number(r)
	if err != nil {
		WriteError(w, r, err)
		return
	}
	var in struct {
		Strategy        string `json:"strategy"`
		Message         string `json:"message"`
		ExpectedHeadSHA string `json:"expected_head_sha"`
	}
	if err := decodeJSON(r, &in); err != nil {
		WriteError(w, r, err)
		return
	}
	o, n := ownerRepo(r)
	v, err := h.Pulls.Merge(r.Context(), auth.ActorFrom(r.Context()), o, n, num, pulls.MergeRequest{Strategy: in.Strategy, Message: in.Message, ExpectedHeadSHA: in.ExpectedHeadSHA})
	if err != nil {
		WriteError(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusOK, toPullJSON(v))
}
