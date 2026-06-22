// Package thumbnail generates and caches small preview images for files.
// Image formats are decoded/resized/encoded in pure Go. Video and PDF
// thumbnails are opportunistic: if ffmpeg/pdftoppm are present on PATH at
// startup they're used to extract a frame/page, otherwise those formats
// simply have no thumbnail (the UI falls back to a generic type icon) —
// keeping the core binary free of cgo or hard external dependencies.
package thumbnail

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp" // decode-only registration

	_ "image/gif"
	_ "image/png"
)

const DefaultSize = 256

var imageExts = map[string]bool{"jpg": true, "jpeg": true, "png": true, "gif": true, "webp": true}
var videoExts = map[string]bool{"mp4": true, "webm": true, "mov": true, "mkv": true}
var pdfExts = map[string]bool{"pdf": true}

type Kind int

const (
	KindNone Kind = iota
	KindImage
	KindVideo
	KindPDF
)

func KindForExt(ext string) Kind {
	ext = strings.ToLower(strings.TrimPrefix(ext, "."))
	switch {
	case imageExts[ext]:
		return KindImage
	case videoExts[ext]:
		return KindVideo
	case pdfExts[ext]:
		return KindPDF
	default:
		return KindNone
	}
}

// Generator produces thumbnail JPEG bytes, caching them on disk.
type Generator struct {
	cacheDir string
	tmpDir   string
	ffmpeg   string // resolved path, empty if unavailable
	pdftoppm string
}

func New(cacheDir string) *Generator {
	g := &Generator{cacheDir: cacheDir, tmpDir: filepath.Join(filepath.Dir(cacheDir), "tmp")}
	if p, err := exec.LookPath("ffmpeg"); err == nil {
		g.ffmpeg = p
	} else {
		log.Printf("thumbnail: ffmpeg not found on PATH, video thumbnails disabled")
	}
	if p, err := exec.LookPath("pdftoppm"); err == nil {
		g.pdftoppm = p
	} else {
		log.Printf("thumbnail: pdftoppm not found on PATH, PDF thumbnails disabled")
	}
	_ = os.MkdirAll(cacheDir, 0750)
	// World-writable + sticky, like /tmp: pdftoppm runs with the requesting
	// user's real uid/gid dropped via dropPrivileges, so it needs a
	// directory it can create an output file in even though this process
	// (and the directory's owner) is root.
	_ = os.MkdirAll(g.tmpDir, 0777)
	_ = os.Chmod(g.tmpDir, 0777|os.ModeSticky)
	return g
}

func (g *Generator) SupportsVideo() bool { return g.ffmpeg != "" }
func (g *Generator) SupportsPDF() bool   { return g.pdftoppm != "" }

// CachePath returns where a thumbnail for the given content-identity
// (relative path + size + mtime, so it invalidates automatically when the
// source file changes) would live, without generating it.
func (g *Generator) CachePath(relPath string, size int, modUnix int64, fileSize int64) string {
	sum := sha256.Sum256([]byte(cacheKey(relPath, size, modUnix, fileSize)))
	return filepath.Join(g.cacheDir, hex.EncodeToString(sum[:])+".jpg")
}

// FromImage decodes, resizes, and JPEG-encodes an already-opened image file.
// The caller is responsible for opening it with the correct (impersonated)
// permissions — this function does no filesystem access of its own.
func FromImage(r io.Reader, maxDim int) ([]byte, error) {
	src, _, err := image.Decode(r)
	if err != nil {
		return nil, err
	}
	dst := resize(src, maxDim)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: 82}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func resize(src image.Image, maxDim int) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= maxDim && h <= maxDim {
		return src
	}
	var nw, nh int
	if w > h {
		nw = maxDim
		nh = h * maxDim / w
	} else {
		nh = maxDim
		nw = w * maxDim / h
	}
	if nw < 1 {
		nw = 1
	}
	if nh < 1 {
		nh = 1
	}
	dst := image.NewRGBA(image.Rect(0, 0, nw, nh))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, b, draw.Over, nil)
	return dst
}

// RunAsUser is the credential a privilege-dropped subprocess should run
// with, so external tools (ffmpeg, pdftoppm) only ever see what the
// requesting Linux user could see themselves — never the server's root.
type RunAsUser struct {
	UID    int
	GID    int
	Groups []int
}

// FromVideo extracts a frame near the start of absPath via ffmpeg, running
// ffmpeg itself dropped to runAs's uid/gid so it can't read anything the
// requesting user couldn't read directly.
func (g *Generator) FromVideo(absPath string, maxDim int, runAs RunAsUser) ([]byte, error) {
	if g.ffmpeg == "" {
		return nil, fmt.Errorf("ffmpeg not available")
	}
	scale := fmt.Sprintf("scale='if(gt(iw,ih),%d,-1)':'if(gt(iw,ih),-1,%d)'", maxDim, maxDim)
	cmd := exec.Command(g.ffmpeg,
		"-ss", "1",
		"-i", absPath,
		"-frames:v", "1",
		"-vf", scale,
		"-f", "image2pipe",
		"-vcodec", "mjpeg",
		"-loglevel", "error",
		"-",
	)
	dropPrivileges(cmd, runAs)
	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffmpeg: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return out.Bytes(), nil
}

// FromPDF rasterizes the first page of absPath via pdftoppm, dropped to
// runAs's uid/gid for the same reason as FromVideo.
//
// Unlike ffmpeg, this version of pdftoppm has no stdout-pipe convention —
// passing "-" as the output prefix just creates a file literally named
// "-.jpg". So it writes to a real file in a sticky, world-writable temp
// directory (the dropped-privilege subprocess needs to be able to create
// it even though the directory itself is owned by root), which this
// process then reads back and removes.
func (g *Generator) FromPDF(absPath string, maxDim int, runAs RunAsUser) ([]byte, error) {
	if g.pdftoppm == "" {
		return nil, fmt.Errorf("pdftoppm not available")
	}
	outPrefix := filepath.Join(g.tmpDir, "pdf-"+randomHex())
	outFile := outPrefix + ".jpg"
	defer os.Remove(outFile)

	cmd := exec.Command(g.pdftoppm,
		"-jpeg",
		"-singlefile",
		"-f", "1", "-l", "1",
		"-scale-to", strconv.Itoa(maxDim),
		absPath,
		outPrefix,
	)
	dropPrivileges(cmd, runAs)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("pdftoppm: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return os.ReadFile(outFile)
}

func randomHex() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// dropPrivileges sets the subprocess's real credentials (not just fsuid) to
// runAs, which the kernel preserves across execve — unlike fsuid, which
// execve resets to match the (still-root) effective uid of the calling
// process. This is what actually makes ffmpeg/pdftoppm see the filesystem
// as the requesting user rather than as root.
func dropPrivileges(cmd *exec.Cmd, runAs RunAsUser) {
	groups := make([]uint32, len(runAs.Groups))
	for i, g := range runAs.Groups {
		groups[i] = uint32(g)
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Credential: &syscall.Credential{
			Uid:    uint32(runAs.UID),
			Gid:    uint32(runAs.GID),
			Groups: groups,
		},
	}
}

func cacheKey(relPath string, size int, modUnix int64, fileSize int64) string {
	return fmt.Sprintf("%s-%d-%d-%d", relPath, size, modUnix, fileSize)
}
