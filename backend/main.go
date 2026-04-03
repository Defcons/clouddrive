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
	"strings"
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

	// Services
	permStore := services.NewPermissionStore(storageRoot)
	auditLog := services.NewAuditLogger(storageRoot)
	trashStore := services.NewTrashStore(storageRoot)
	tagStore := services.NewTagStore(storageRoot)
	notifStore := services.NewNotificationStore(storageRoot)

	// Clean expired trash items on startup
	trashStore.CleanExpired()

	// Rate limiter: 5 attempts per 2 minutes, 5 minute lockout
	loginLimiter := middleware.NewRateLimiter(5, 2*time.Minute, 5*time.Minute)
	csrfMiddleware := middleware.NewCSRFMiddleware()

	// Handlers
	authHandler := handlers.NewAuthHandler(userStore, jwtSecret, loginLimiter, auditLog)
	authMiddleware := middleware.NewAuthMiddleware(jwtSecret, userStore)
	fileHandler := handlers.NewFileHandler(storageRoot, permStore, auditLog, trashStore, tagStore)
	diskHandler := handlers.NewDiskHandler(storageRoot)
	shareHandler := handlers.NewShareHandler(storageRoot, auditLog)
	versionHandler := handlers.NewVersionHandler()
	permHandler := handlers.NewPermissionsHandler(permStore, auditLog)
	auditHandler := handlers.NewAuditHandler(auditLog)
	trashHandler := handlers.NewTrashHandler(trashStore, auditLog)
	notifHandler := handlers.NewNotificationHandler(notifStore)

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
	mux.HandleFunc("GET /api/files/search", authMiddleware.Wrap(fileHandler.Search))
	mux.HandleFunc("GET /api/files/recent", authMiddleware.Wrap(fileHandler.Recent))
	mux.HandleFunc("GET /api/files/tags", authMiddleware.Wrap(fileHandler.GetTags))
	mux.HandleFunc("POST /api/files/upload", protectedWrite(fileHandler.Upload))
	mux.HandleFunc("POST /api/files/mkdir", protectedWrite(fileHandler.Mkdir))
	mux.HandleFunc("POST /api/files/rename", protectedWrite(fileHandler.Rename))
	mux.HandleFunc("POST /api/files/move", protectedWrite(fileHandler.Move))
	mux.HandleFunc("POST /api/files/copy", protectedWrite(fileHandler.Copy))
	mux.HandleFunc("POST /api/files/extract", protectedWrite(fileHandler.Extract))
	mux.HandleFunc("POST /api/files/compress", protectedWrite(fileHandler.Compress))
	mux.HandleFunc("POST /api/files/tags", protectedWrite(fileHandler.SetTags))
	mux.HandleFunc("DELETE /api/files", protectedWrite(fileHandler.Delete))

	// Permissions (protected)
	mux.HandleFunc("POST /api/files/permissions", protectedWrite(permHandler.SetPrivate))
	mux.HandleFunc("DELETE /api/files/permissions", protectedWrite(permHandler.RemovePrivate))
	mux.HandleFunc("GET /api/files/permissions", authMiddleware.Wrap(permHandler.GetPermission))

	// Trash
	mux.HandleFunc("GET /api/trash", authMiddleware.Wrap(trashHandler.List))
	mux.HandleFunc("POST /api/trash/restore", protectedWrite(trashHandler.Restore))
	mux.HandleFunc("DELETE /api/trash", protectedWrite(trashHandler.Delete))
	mux.HandleFunc("DELETE /api/trash/empty", protectedWrite(trashHandler.Empty))

	// Shares (management — protected)
	mux.HandleFunc("POST /api/shares", protectedWrite(shareHandler.Create))
	mux.HandleFunc("GET /api/shares", authMiddleware.Wrap(shareHandler.List))
	mux.HandleFunc("POST /api/shares/revoke", protectedWrite(shareHandler.Revoke))

	// Notifications
	mux.HandleFunc("GET /api/notifications", authMiddleware.Wrap(notifHandler.GetAll))
	mux.HandleFunc("GET /api/notifications/unread", authMiddleware.Wrap(notifHandler.GetUnreadCount))
	mux.HandleFunc("POST /api/notifications/read", protectedWrite(notifHandler.MarkRead))

	// Audit log (admin only)
	mux.HandleFunc("GET /api/audit", authMiddleware.Wrap(auditHandler.GetLogs))

	// Version (public — for update notifier polling)
	mux.HandleFunc("GET /api/version", versionHandler.Info)

	// Share (public — no auth)
	mux.HandleFunc("/share/", func(w http.ResponseWriter, r *http.Request) {
		// Route upload requests to the upload handler
		if r.Method == "POST" && strings.Contains(r.URL.Path, "/upload") {
			shareHandler.Upload(w, r)
			return
		}
		shareHandler.Download(w, r)
	})

	// Disk
	mux.HandleFunc("GET /api/disk", authMiddleware.Wrap(diskHandler.Usage))

	// Serve embedded SPA
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		log.Fatal(err)
	}
	fileServer := http.FileServer(http.FS(staticFS))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if path == "/" {
			path = "/index.html"
		}
		_, err := fs.Stat(staticFS, path[1:])
		if err != nil {
			r.URL.Path = "/"
		}
		fileServer.ServeHTTP(w, r)
	})

	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type", "X-CSRF-Token"},
		AllowCredentials: true,
	})

	handler := middleware.SecureHeaders(c.Handler(mux))

	log.Printf("CloudDrive starting on :%s (storage: %s)", port, storageRoot)
	log.Fatal(http.ListenAndServe(":"+port, handler))
}
