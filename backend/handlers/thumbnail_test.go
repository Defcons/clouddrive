package handlers

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func writePNG(t *testing.T, path string, w, h int) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, color.RGBA{uint8(x % 256), uint8(y % 256), 128, 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}
}

func decodeDims(t *testing.T, data []byte) (int, int) {
	t.Helper()
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode thumb: %v", err)
	}
	return cfg.Width, cfg.Height
}

func TestThumbnailDownscalesAndCaches(t *testing.T) {
	root := t.TempDir()
	writePNG(t, filepath.Join(root, "big.png"), 1000, 500)
	h := NewFileHandler(root, nil, nil, nil, nil, nil)

	rec := serve(h.Thumbnail, http.MethodGet, "/api/files/thumbnail?path=/big.png", "", sessionToken(t, "admin", "admin", "/"))
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "image/jpeg" {
		t.Errorf("expected image/jpeg, got %q", ct)
	}
	w, hh := decodeDims(t, rec.Body.Bytes())
	if w != thumbMaxDim || hh != thumbMaxDim/2 {
		t.Errorf("thumb dims = %dx%d, want %dx%d", w, hh, thumbMaxDim, thumbMaxDim/2)
	}
	// Second request should be served from the on-disk cache.
	if _, err := os.Stat(h.thumbDir); err != nil {
		t.Errorf("expected thumb cache dir: %v", err)
	}
	rec2 := serve(h.Thumbnail, http.MethodGet, "/api/files/thumbnail?path=/big.png", "", sessionToken(t, "admin", "admin", "/"))
	if rec2.Code != http.StatusOK || rec2.Body.Len() != rec.Body.Len() {
		t.Errorf("cached thumb mismatch: code %d len %d vs %d", rec2.Code, rec2.Body.Len(), rec.Body.Len())
	}
}

func TestThumbnailSmallImageNotUpscaled(t *testing.T) {
	root := t.TempDir()
	writePNG(t, filepath.Join(root, "small.png"), 64, 48)
	h := NewFileHandler(root, nil, nil, nil, nil, nil)
	rec := serve(h.Thumbnail, http.MethodGet, "/api/files/thumbnail?path=/small.png", "", sessionToken(t, "admin", "admin", "/"))
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d", rec.Code)
	}
	w, hh := decodeDims(t, rec.Body.Bytes())
	if w != 64 || hh != 48 {
		t.Errorf("small image should not be upscaled, got %dx%d", w, hh)
	}
}
