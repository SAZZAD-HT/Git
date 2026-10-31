package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"anvil/internal/auth"
	"anvil/internal/users"
)

func (h *handlers) mountUsers(r chi.Router) {
	r.Get("/users/{username}", h.getUser)
}

// getUser serves GET /api/v1/users/{username}: the public profile. Email is included
// only for the user themself and site admins.
func (h *handlers) getUser(w http.ResponseWriter, r *http.Request) {
	u, err := h.Users.Get(r.Context(), chi.URLParam(r, "username"))
	if err != nil {
		WriteError(w, r, err)
		return
	}
	a := auth.ActorFrom(r.Context())
	writeJSON(w, r, http.StatusOK, toUserJSON(u, a.UserID == u.ID || a.IsSiteAdmin()))
}

func (h *handlers) mountAdmin(r chi.Router) {
	r.Get("/admin/users", h.adminListUsers)
	r.Post("/admin/users", h.adminCreateUser)
	r.Patch("/admin/users/{username}", h.adminUpdateUser)
}

// adminListUsers serves GET /api/v1/admin/users (site admins).
func (h *handlers) adminListUsers(w http.ResponseWriter, r *http.Request) {
	p := page(r)
	list, total, err := h.Users.List(r.Context(), auth.ActorFrom(r.Context()), p)
	if err != nil {
		WriteError(w, r, err)
		return
	}
	out := make([]userJSON, len(list))
	for i := range list {
		out[i] = toUserJSON(&list[i], true)
	}
	writePaged(w, r, out, p, total)
}

// adminCreateUser serves POST /api/v1/admin/users: {username, email, full_name, password, is_admin}.
func (h *handlers) adminCreateUser(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		FullName string `json:"full_name"`
		Password string `json:"password"`
		IsAdmin  bool   `json:"is_admin"`
	}
	if err := decodeJSON(r, &in); err != nil {
		WriteError(w, r, err)
		return
	}
	u, err := h.Users.Create(r.Context(), auth.ActorFrom(r.Context()), users.CreateUser{
		Username: in.Username, Email: in.Email, FullName: in.FullName, Password: in.Password, IsAdmin: in.IsAdmin})
	if err != nil {
		WriteError(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusCreated, toUserJSON(u, true))
}

// adminUpdateUser serves PATCH /api/v1/admin/users/{username}: {is_admin?, is_active?}.
func (h *handlers) adminUpdateUser(w http.ResponseWriter, r *http.Request) {
	var in struct {
		IsAdmin  *bool `json:"is_admin"`
		IsActive *bool `json:"is_active"`
	}
	if err := decodeJSON(r, &in); err != nil {
		WriteError(w, r, err)
		return
	}
	u, err := h.Users.SetFlags(r.Context(), auth.ActorFrom(r.Context()), chi.URLParam(r, "username"), users.AdminUpdate{IsAdmin: in.IsAdmin, IsActive: in.IsActive})
	if err != nil {
		WriteError(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusOK, toUserJSON(u, true))
}
