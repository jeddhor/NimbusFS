package api

import (
	"net/http"

	"nimbusfs/internal/fsops"
)

type searchResultJSON struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	IsDir    bool   `json:"isDir"`
	Size     int64  `json:"size"`
	Modified string `json:"modified"`
	Owner    string `json:"owner"`
	Group    string `json:"group"`
}

func (a *API) Search(w http.ResponseWriter, r *http.Request) {
	if !a.cfg.Search.Enabled {
		writeError(w, http.StatusNotFound, "search is disabled")
		return
	}
	if !a.searchIndex.Ready() {
		writeJSON(w, http.StatusOK, map[string]any{"entries": []searchResultJSON{}, "indexing": true})
		return
	}

	username := usernameFrom(r.Context())
	id, err := fsops.LookupIdentity(username)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "user lookup failed")
		return
	}

	query := r.URL.Query().Get("q")
	hits, err := a.searchIndex.Search(query)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "search failed")
		return
	}

	results := make([]searchResultJSON, 0, len(hits))
	for _, h := range hits {
		if !fsops.CanRead(id, h.UID, h.GID, h.Mode) {
			continue
		}
		results = append(results, searchResultJSON{
			Name:     h.Name,
			Path:     h.Path,
			IsDir:    h.IsDir,
			Size:     h.Size,
			Modified: h.Modified.Format("2006-01-02T15:04:05Z07:00"),
			Owner:    fsops.LookupUserName(h.UID),
			Group:    fsops.LookupGroupName(h.GID),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": results, "indexing": false})
}

// ReindexSearch lets any authenticated user kick off a fresh index build,
// e.g. right after uploading a batch of files. It's just a tree walk as the
// server's own (root) process, so there's no extra privilege exposure here.
func (a *API) ReindexSearch(w http.ResponseWriter, r *http.Request) {
	if !a.cfg.Search.Enabled {
		writeError(w, http.StatusNotFound, "search is disabled")
		return
	}
	go a.searchIndex.Rebuild()
	writeJSON(w, http.StatusOK, map[string]string{"status": "reindexing"})
}
