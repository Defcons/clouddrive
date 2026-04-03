package main

import (
	"clouddrive/handlers"
	"clouddrive/middleware"
	"clouddrive/services"
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/cors"
)

//go:embed static/*
var staticFiles embed.FS

func main() {
	// Check for --hash-password utility
	if len(os.Args) > 1 && os.Args[1] == "--hash-password" {
		if len(os.Args) < 3 {
			fmt.Println("Usage: clouddrive --hash-password <password>")
			os.Exit(1)
		}
		hash, err := services.HashPassword(os.Args[2])
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(hash)
		os.Exit(0)
	}

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

	// User store setup
	usersFile := os.Getenv("USERS_FILE")
	if usersFile == "" {
		usersFile = filepath.Join(storageRoot, "users.json")
	}

	// Backward compatibility: migrate from env vars if users.json doesn't exist
	username := os.Getenv("CLOUDDRIVE_USER")
	password := os.Getenv("CLOUDDRIVE_PASS")
	if username != "" && password != "" {
		if err := services.InitFromEnv(usersFile, username, password); err != nil {
			log.Printf("Warning: failed to migrate env vars to users.json: %v", err)
		}
	}

	userStore, err := services.NewUserStore(usersFile)
	if err != nil {
		log.Fatalf("Failed to load user store: %v", err)
	}

	permStore := services.NewPermissionStore(storageRoot)
	auditLog := services.NewAuditLogger(storageRoot)

	// Rate limiter: 5 attempts per 2 minutes, 5 minute lockout
	loginLimiter := middleware.NewRateLimiter(5, 2*time.Minute, 5*time.Minute)

	csrfMiddleware := middleware.NewCSRFMiddleware()

	authHandler := handlers.NewAuthHandler(userStore, jwtSecret, loginLimiter, auditLog)
	authMiddleware := middleware.NewAuthMiddleware(jwtSecret, userStore)
	fileHandler := handlers.NewFileHandler(storageRoot, permStore, auditLog)
	diskHandler := handlers.NewDiskHandler(storageRoot)
	shareHandler := handlers.NewShareHandler(storageRoot, auditLog)
	versionHandler := handlers.NewVersionHandler()
	permHandler := handlers.NewPermissionsHandler(permStore, auditLog)
	auditHandler := handlers.NewAuditHandler(auditLog)

	mux := http.NewServeMux()

	// Helper: auth + CSRF protection for state-changing endpoints
	protectedWrite := func(handler http.HandlerFunc) http.HandlerFunc {
		return authMiddleware.Wrap(csrfMiddleware.Protect(handler))
	}

	// Auth
	mux.HandleFunc("POST /api/auth/login", loginLimiter.WrapLogin(authHandler.Login))
	mux.HandleFunc("GET /api/auth/check", authMiddleware.Wrap(authHandler.Check))
	mux.HandleFunc("POST /api/auth/change-password", protectedWrite(authHandler.ChangePassword))

	// CSRF token endpoint
	mux.HandleFunc("GET /api/csrf", authMiddleware.Wrap(csrfMiddleware.GetToken))

	// Files (protected — reads use auth only, writes use auth + CSRF)
	mux.HandleFunc("GET /api/files", authMiddleware.Wrap(fileHandler.List))
	mux.HandleFunc("GET /api/files/download", authMiddleware.Wrap(fileHandler.Download))
	mux.HandleFunc("GET /api/files/preview", authMiddleware.Wrap(fileHandler.Preview))
	mux.HandleFunc("POST /api/files/upload", protectedWrite(fileHandler.Upload))
	mux.HandleFunc("POST /api/files/mkdir", protectedWrite(fileHandler.Mkdir))
	mux.HandleFunc("POST /api/files/rename", protectedWrite(fileHandler.Rename))
	mux.HandleFunc("DELETE /api/files", protectedWrite(fileHandler.Delete))

	// Permissions (protected)
	mux.HandleFunc("POST /api/files/permissions", protectedWrite(permHandler.SetPrivate))
	mux.HandleFunc("DELETE /api/files/permissions", protectedWrite(permHandler.RemovePrivate))
	mux.HandleFunc("GET /api/files/permissions", authMiddleware.Wrap(permHandler.GetPermission))

	// Shares (management — protected)
	mux.HandleFunc("POST /api/shares", protectedWrite(shareHandler.Create))
	mux.HandleFunc("GET /api/shares", authMiddleware.Wrap(shareHandler.List))
	mux.HandleFunc("POST /api/shares/revoke", protectedWrite(shareHandler.Revoke))

	// Audit log (admin only)
	mux.HandleFunc("GET /api/audit", authMiddleware.Wrap(auditHandler.GetLogs))

	// Version (public — for update notifier polling)
	mux.HandleFunc("GET /api/version", versionHandler.Info)

	// Share download (public — no auth, GET + POST for password form)
	mux.HandleFunc("/share/", shareHandler.Download)

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

	handler := middleware.SecureHeaders(c.Handler(mux))

	log.Printf("CloudDrive starting on :%s (storage: %s)", port, storageRoot)
	log.Fatal(http.ListenAndServe(":"+port, handler))
}
