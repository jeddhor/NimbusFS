// Package search maintains an in-memory bleve index over the sandboxed
// filesystem tree, rebuilt periodically, supporting filename/extension/
// wildcard search with permission-aware result filtering.
package search

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/mapping"
	"github.com/blevesearch/bleve/v2/search/query"

	"nimbusfs/internal/fsops"
)

// doc is the bleve document shape for one filesystem entry.
type doc struct {
	Path      string `json:"path"`
	Name      string `json:"name"`
	NameLower string `json:"nameLower"`
	Ext       string `json:"ext"`
	IsDir     bool   `json:"isDir"`
	UID       uint32 `json:"uid"`
	GID       uint32 `json:"gid"`
	Mode      uint32 `json:"mode"`
	Size      int64  `json:"size"`
	Modified  int64  `json:"modified"`
}

type Hit struct {
	Path     string
	Name     string
	IsDir    bool
	UID      uint32
	GID      uint32
	Mode     os.FileMode
	Size     int64
	Modified time.Time
}

// Index wraps a bleve index with the rebuild lifecycle this package needs.
// Swapping the underlying bleve.Index on rebuild (rather than mutating one
// in place) keeps concurrent searches simple: they just snapshot the
// current pointer once and never see a half-rebuilt index.
type Index struct {
	sandbox *fsops.Sandbox

	mu    sync.RWMutex
	bleve bleve.Index

	ready    atomic.Bool
	building atomic.Bool
}

func New(sandbox *fsops.Sandbox) *Index {
	return &Index{sandbox: sandbox}
}

func (idx *Index) Ready() bool { return idx.ready.Load() }

// Rebuild walks the sandbox root and replaces the index. Safe to call
// concurrently with Search, and safe to call again while a previous
// rebuild is still running (the later call is dropped — a rebuild already
// in flight will pick up a reasonably fresh view anyway).
func (idx *Index) Rebuild() {
	if !idx.building.CompareAndSwap(false, true) {
		return
	}
	defer idx.building.Store(false)

	start := time.Now()
	next, err := bleve.NewMemOnly(buildMapping())
	if err != nil {
		log.Printf("search: could not create index: %v", err)
		return
	}

	// Indexing one document at a time (next.Index) commits a transaction
	// per call, which dominates cost at scale — batching is the documented
	// way to bulk-load bleve and is roughly two orders of magnitude faster
	// (50k tiny files: ~100s individually vs ~1s batched in testing).
	const batchSize = 1000
	batch := next.NewBatch()
	count := 0

	flush := func() error {
		if batch.Size() == 0 {
			return nil
		}
		if err := next.Batch(batch); err != nil {
			return err
		}
		batch.Reset()
		return nil
	}

	root := idx.sandbox.Root()
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			// Permission errors etc on individual entries shouldn't abort the walk.
			return nil
		}
		if path == root {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		rel, err := idx.sandbox.RelPath(path)
		if err != nil {
			return nil
		}
		uid, gid, _ := fsops.StatT(info)
		entry := doc{
			Path:      rel,
			Name:      info.Name(),
			NameLower: strings.ToLower(info.Name()),
			Ext:       strings.ToLower(strings.TrimPrefix(filepath.Ext(info.Name()), ".")),
			IsDir:     info.IsDir(),
			UID:       uid,
			GID:       gid,
			Mode:      uint32(info.Mode().Perm()),
			Size:      info.Size(),
			Modified:  info.ModTime().Unix(),
		}
		if err := batch.Index(rel, entry); err != nil {
			return nil
		}
		count++
		if batch.Size() >= batchSize {
			if err := flush(); err != nil {
				log.Printf("search: batch index error: %v", err)
			}
		}
		return nil
	})
	if err != nil {
		log.Printf("search: walk error: %v", err)
	}
	if err := flush(); err != nil {
		log.Printf("search: final batch index error: %v", err)
	}

	idx.mu.Lock()
	old := idx.bleve
	idx.bleve = next
	idx.mu.Unlock()
	if old != nil {
		_ = old.Close()
	}
	idx.ready.Store(true)

	log.Printf("search: indexed %d entries in %s", count, time.Since(start).Round(time.Millisecond))
}

// StartPeriodicRebuild runs an initial build in the background and then
// rebuilds on the given interval until ctx-like stop isn't needed — this
// runs for the lifetime of the process, same as the server itself.
func (idx *Index) StartPeriodicRebuild(interval time.Duration) {
	go idx.Rebuild()
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			idx.Rebuild()
		}
	}()
}

const maxResults = 200

// Search runs queryStr against the index and returns up to maxResults hits,
// unfiltered by permission — callers are responsible for filtering with
// fsops.CanRead before returning results to a user.
func (idx *Index) Search(queryStr string) ([]Hit, error) {
	idx.mu.RLock()
	index := idx.bleve
	idx.mu.RUnlock()
	if index == nil {
		return nil, nil
	}

	q := buildQuery(queryStr)
	req := bleve.NewSearchRequest(q)
	req.Size = maxResults
	req.Fields = []string{"path", "name", "isDir", "uid", "gid", "mode", "size", "modified"}

	result, err := index.Search(req)
	if err != nil {
		return nil, err
	}

	hits := make([]Hit, 0, len(result.Hits))
	for _, h := range result.Hits {
		hits = append(hits, Hit{
			Path:     fieldString(h.Fields, "path"),
			Name:     fieldString(h.Fields, "name"),
			IsDir:    fieldBool(h.Fields, "isDir"),
			UID:      uint32(fieldNumber(h.Fields, "uid")),
			GID:      uint32(fieldNumber(h.Fields, "gid")),
			Mode:     os.FileMode(uint32(fieldNumber(h.Fields, "mode"))),
			Size:     int64(fieldNumber(h.Fields, "size")),
			Modified: time.Unix(int64(fieldNumber(h.Fields, "modified")), 0),
		})
	}
	return hits, nil
}

func buildMapping() mapping.IndexMapping {
	keywordField := bleve.NewTextFieldMapping()
	keywordField.Analyzer = "keyword"

	unindexedText := bleve.NewTextFieldMapping()
	unindexedText.Index = false
	unindexedNumeric := bleve.NewNumericFieldMapping()
	unindexedNumeric.Index = false
	unindexedBool := bleve.NewBooleanFieldMapping()
	unindexedBool.Index = false

	// Only nameLower/ext are ever queried (see buildQuery) — everything else
	// here is metadata we just want stored for display and for the
	// permission filter in the API layer, not searched. Skipping analysis
	// on it is most of the difference between indexing 50k tiny files in
	// ~70s (analyzing every field) and under a second (storing only what's
	// actually queried).
	doc := bleve.NewDocumentMapping()
	doc.AddFieldMappingsAt("nameLower", keywordField)
	doc.AddFieldMappingsAt("ext", keywordField)
	doc.AddFieldMappingsAt("name", unindexedText)
	doc.AddFieldMappingsAt("path", unindexedText)
	doc.AddFieldMappingsAt("isDir", unindexedBool)
	doc.AddFieldMappingsAt("uid", unindexedNumeric)
	doc.AddFieldMappingsAt("gid", unindexedNumeric)
	doc.AddFieldMappingsAt("mode", unindexedNumeric)
	doc.AddFieldMappingsAt("size", unindexedNumeric)
	doc.AddFieldMappingsAt("modified", unindexedNumeric)

	im := bleve.NewIndexMapping()
	im.DefaultMapping = doc
	return im
}

// buildQuery interprets the user's search text: a literal wildcard pattern
// (containing * or ?) is used as-is against the lowercased name, anything
// else is wrapped in "*...*" for substring/instant-filter behavior. This
// single mechanism covers filename, extension (e.g. "*.pdf"), and wildcard
// search all at once.
func buildQuery(raw string) query.Query {
	q := strings.ToLower(strings.TrimSpace(raw))
	if q == "" {
		q = "*"
	} else if !strings.ContainsAny(q, "*?") {
		q = "*" + q + "*"
	}
	wq := bleve.NewWildcardQuery(q)
	wq.SetField("nameLower")
	return wq
}

func fieldString(fields map[string]interface{}, key string) string {
	s, _ := fields[key].(string)
	return s
}

func fieldBool(fields map[string]interface{}, key string) bool {
	b, _ := fields[key].(bool)
	return b
}

func fieldNumber(fields map[string]interface{}, key string) float64 {
	n, _ := fields[key].(float64)
	return n
}
