package api

import (
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"nimbusfs/internal/fsops"
	"nimbusfs/internal/thumbnail"
)

func (a *API) Thumbnail(w http.ResponseWriter, r *http.Request) {
	reqPath := r.URL.Query().Get("path")
	if reqPath == "" {
		writeError(w, http.StatusBadRequest, "path is required")
		return
	}
	size := thumbnail.DefaultSize
	if s := r.URL.Query().Get("size"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 && n <= 1024 {
			size = n
		}
	}
	kind := thumbnail.KindForExt(filepath.Ext(reqPath))
	if kind == thumbnail.KindNone {
		writeError(w, http.StatusNotFound, "no thumbnail available for this file type")
		return
	}
	if kind == thumbnail.KindVideo && !a.thumbnails.SupportsVideo() {
		writeError(w, http.StatusNotFound, "video thumbnails are unavailable (ffmpeg not installed)")
		return
	}
	if kind == thumbnail.KindPDF && !a.thumbnails.SupportsPDF() {
		writeError(w, http.StatusNotFound, "pdf thumbnails are unavailable (pdftoppm not installed)")
		return
	}

	username := usernameFrom(r.Context())
	id, err := fsops.LookupIdentity(username)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "user lookup failed")
		return
	}

	// The real access-control gate: actually open the file as the
	// requesting user, not just stat it. Stat only requires directory
	// traversal permission, not read permission on the entry itself, so
	// relying on it here would let anyone fetch a thumbnail — including a
	// cached one from the disk cache below — for a file they can't read.
	var entry fsops.Entry
	var absPath string
	err = fsops.As(id, func() error {
		f, info, statErr := a.sandbox.Open(reqPath)
		if statErr != nil {
			return statErr
		}
		defer f.Close()
		entry.IsDir = info.IsDir()
		entry.Size = info.Size()
		entry.Modified = info.ModTime()
		absPath, statErr = a.sandbox.AbsPath(reqPath)
		return statErr
	})
	if err != nil {
		writeFSError(w, err)
		return
	}
	if entry.IsDir {
		writeError(w, http.StatusBadRequest, "cannot thumbnail a directory")
		return
	}

	cachePath := a.thumbnails.CachePath(reqPath, size, entry.Modified.Unix(), entry.Size)
	if data, err := os.ReadFile(cachePath); err == nil {
		serveThumbnail(w, data)
		return
	}

	var data []byte
	switch kind {
	case thumbnail.KindImage:
		err = fsops.As(id, func() error {
			f, _, e := a.sandbox.Open(reqPath)
			if e != nil {
				return e
			}
			defer f.Close()
			data, e = thumbnail.FromImage(f, size)
			return e
		})
	case thumbnail.KindVideo:
		data, err = a.thumbnails.FromVideo(absPath, size, runAsFor(id))
	case thumbnail.KindPDF:
		data, err = a.thumbnails.FromPDF(absPath, size, runAsFor(id))
	}
	if err != nil || len(data) == 0 {
		writeError(w, http.StatusInternalServerError, "could not generate thumbnail")
		return
	}

	_ = os.WriteFile(cachePath, data, 0644)
	serveThumbnail(w, data)
}

func runAsFor(id *fsops.Identity) thumbnail.RunAsUser {
	return thumbnail.RunAsUser{UID: id.UID, GID: id.GID, Groups: id.Groups}
}

func serveThumbnail(w http.ResponseWriter, data []byte) {
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "private, max-age=86400")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}
