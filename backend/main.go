package main

import (
	"clouddrive/handlers"
	"clouddrive/middleware"
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"

	"github.com/rs/cors"
)

//go:embed static/*
var staticFiles embed.FS

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	storageRoot := os.Getenv("STORAGE_ROOT")
	if storageRoot == "" {
		storageRoot = "./data"
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "change-me-in-production"
	}

	username := os.Getenv("USERNAME")
	if username == "" {
		username = "admin"
	}
	password := os.Getenv("PASSWORD")
	if password == "" {
		password = "admin"
	}

	fileHandler := handlers.NewFileHandler(storageRoot)
	authHandler := handlers.NewAuthHandler(username, password, jwtSecret)
	authMiddleware := middleware.NewAuthMiddleware(jwtSecret)
	diskHandler := handlers.NewDiskHandler(storageRoot)

	mux := http.NewServeMux()

	// Auth
	mux.HandleFunc("POST /api/auth/login", authHandler.Login)
	mux.HandleFunc("GET /api/auth/check", authMiddleware.Wrap(authHandler.Check))

	// Files (protected)
	mux.HandleFunc("GET /api/files", authMiddleware.Wrap(fileHandler.List))
	mux.HandleFunc("GET /api/files/download", authMiddleware.Wrap(fileHandler.Download))
	mux.HandleFunc("GET /api/files/preview", authMiddleware.Wrap(fileHandler.Preview))
	mux.HandleFunc("POST /api/files/upload", authMiddleware.Wrap(fileHandler.Upload))
	mux.HandleFunc("POST /api/files/mkdir", authMiddleware.Wrap(fileHandler.Mkdir))
	mux.HandleFunc("POST /api/files/rename", authMiddleware.Wrap(fileHandler.Rename))
	mux.HandleFunc("DELETE /api/files", authMiddleware.Wrap(fileHandler.Delete))

	// Disk
	mux.HandleFunc("GET /api/disk", authMiddleware.Wrap(diskHandler.Usage))

	// Serve embedded SPA
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		log.Fatal(err)
	}
	fileServer := http.FileServer(http.FS(staticFS))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Try to serve static file first
		path := r.URL.Path
		if path == "/" {
			path = "/index.html"
		}
		_, err := fs.Stat(staticFS, path[1:])
		if err != nil {
			// SPA fallback: serve index.html for client-side routing
			r.URL.Path = "/"
		}
		fileServer.ServeHTTP(w, r)
	})

	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type"},
		AllowCredentials: true,
	})

	handler := c.Handler(mux)

	log.Printf("CloudDrive starting on :%s (storage: %s)", port, storageRoot)
	log.Fatal(http.ListenAndServe(":"+port, handler))
}
