package handlers

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	_ "image/gif"  // register GIF decoder
	_ "image/png"  // register PNG decoder
)

const thumbMaxDim = 256

// decodableThumb is the set of extensions the standard library can decode and
// downscale. Other image types are served as-is (still small enough to be fine
// as a thumbnail, and avoids a regression vs. the previous full-image preview).
var decodableThumb = map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".gif": true}

// Thumbnail serves a small cached JPEG for an image, generating it on first
// request. For image types the stdlib can't decode (webp/svg/bmp) it falls
// back to streaming the original.
func (h *FileHandler) Thumbnail(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Query().Get("path")
	if filePath == "" {
		http.Error(w, "path required", http.StatusBadRequest)
		return
	}
	if !h.checkAccess(r, filepath.ToSlash(filepath.Dir(filePath))) {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}
	absPath, err := h.safePath(filePath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	info, err := os.Stat(absPath)
	if err != nil || info.IsDir() {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	ext := strings.ToLower(filepath.Ext(absPath))
	if !decodableThumb[ext] {
		// Can't downscale this type — serve the original (with Range support).
		f, err := os.Open(absPath)
		if err != nil {
			http.Error(w, "Cannot open file", http.StatusInternalServerError)
			return
		}
		defer f.Close()
		w.Header().Set("Cache-Control", "private, max-age=86400")
		http.ServeContent(w, r, info.Name(), info.ModTime(), f)
		return
	}

	// Cache key includes size+modtime so an edited file regenerates.
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%d|%d|%d", filePath, info.Size(), info.ModTime().UnixNano(), thumbMaxDim)))
	cachePath := filepath.Join(h.thumbDir, hex.EncodeToString(sum[:])+".jpg")

	if data, err := os.ReadFile(cachePath); err == nil {
		writeThumb(w, data)
		return
	}

	data, err := makeThumbnail(absPath)
	if err != nil {
		// Decode failed (corrupt/unsupported) — fall back to the original.
		f, ferr := os.Open(absPath)
		if ferr != nil {
			http.Error(w, "Cannot open file", http.StatusInternalServerError)
			return
		}
		defer f.Close()
		w.Header().Set("Cache-Control", "private, max-age=86400")
		http.ServeContent(w, r, info.Name(), info.ModTime(), f)
		return
	}

	if err := os.MkdirAll(h.thumbDir, 0755); err == nil {
		_ = os.WriteFile(cachePath, data, 0600) // best effort
	}
	writeThumb(w, data)
}

func writeThumb(w http.ResponseWriter, data []byte) {
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "private, max-age=86400")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
	_, _ = w.Write(data)
}

func makeThumbnail(absPath string) ([]byte, error) {
	f, err := os.Open(absPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return nil, err
	}

	thumb := downscale(img, thumbMaxDim)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, thumb, &jpeg.Options{Quality: 82}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// downscale shrinks src so its longest side is at most maxDim, using area
// averaging (good anti-aliasing for thumbnails) in pure stdlib. Images already
// within bounds are returned unchanged.
func downscale(src image.Image, maxDim int) image.Image {
	b := src.Bounds()
	sw, sh := b.Dx(), b.Dy()
	if sw <= maxDim && sh <= maxDim {
		return src
	}
	dw, dh := sw, sh
	if sw >= sh {
		dw = maxDim
		dh = sh * maxDim / sw
	} else {
		dh = maxDim
		dw = sw * maxDim / sh
	}
	if dw < 1 {
		dw = 1
	}
	if dh < 1 {
		dh = 1
	}

	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
	for dy := 0; dy < dh; dy++ {
		sy0 := b.Min.Y + dy*sh/dh
		sy1 := b.Min.Y + (dy+1)*sh/dh
		if sy1 <= sy0 {
			sy1 = sy0 + 1
		}
		for dx := 0; dx < dw; dx++ {
			sx0 := b.Min.X + dx*sw/dw
			sx1 := b.Min.X + (dx+1)*sw/dw
			if sx1 <= sx0 {
				sx1 = sx0 + 1
			}
			var rs, gs, bs, as, n uint64
			for sy := sy0; sy < sy1; sy++ {
				for sx := sx0; sx < sx1; sx++ {
					cr, cg, cb, ca := src.At(sx, sy).RGBA() // 16-bit per channel
					rs += uint64(cr)
					gs += uint64(cg)
					bs += uint64(cb)
					as += uint64(ca)
					n++
				}
			}
			if n == 0 {
				n = 1
			}
			dst.SetRGBA(dx, dy, color.RGBA{
				R: uint8((rs / n) >> 8),
				G: uint8((gs / n) >> 8),
				B: uint8((bs / n) >> 8),
				A: uint8((as / n) >> 8),
			})
		}
	}
	return dst
}
