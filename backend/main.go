package main

import (
	"clouddrive/handlers"
	"clouddrive/middleware"
	"clouddrive/services"
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/rs/cors"
)

//go:embed static/*
var staticFiles embed.FS

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

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

	// JWT secret is mandatory — fail fast if unset or using default.
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" || jwtSecret == "change-me-in-production" {
		slog.Error("JWT_SECRET must be set to a strong random value (min 32 chars). Refusing to start.")
		os.Exit(1)
	}
	if len(jwtSecret) < 32 {
		slog.Error("JWT_SECRET is too short; use at least 32 random characters. Refusing to start.")
		os.Exit(1)
	}

	usersFile := os.Getenv("USERS_FILE")
	if usersFile == "" {
		usersFile = filepath.Join(storageRoot, "users.json")
	}

	username := os.Getenv("CLOUDDRIVE_USER")
	password := os.Getenv("CLOUDDRIVE_PASS")
	if username != "" && password != "" {
		if password == "change-me" || len(password) < 8 {
			slog.Warn("CLOUDDRIVE_PASS is weak or a placeholder — set a strong, unique admin password")
		}
		if err := services.InitFromEnv(usersFile, username, password); err != nil {
			slog.Warn("failed to migrate env vars to users.json", "err", err)
		}
	}

	userStore, err := services.NewUserStore(usersFile)
	if err != nil {
		slog.Error("failed to load user store", "err", err)
		os.Exit(1)
	}

	permStore := services.NewPermissionStore(storageRoot)
	auditLog := services.NewAuditLogger(storageRoot)
	trashStore := services.NewTrashStore(storageRoot)
	tagStore := services.NewTagStore(storageRoot)
	notifStore := services.NewNotificationStore(storageRoot)
	tierStore := services.NewBackupTierStore(storageRoot)

	trashStore.CleanExpired()

	loginLimiter := middleware.NewRateLimiter(5, 2*time.Minute, 5*time.Minute)
	csrfMiddleware := middleware.NewCSRFMiddleware()
	sharePwLimiter := middleware.NewRateLimiter(10, 5*time.Minute, 15*time.Minute)

	mfaHandler := handlers.NewMfaHandler(userStore, jwtSecret, auditLog)
	authHandler := handlers.NewAuthHandler(userStore, jwtSecret, loginLimiter, auditLog, mfaHandler)
	authMiddleware := middleware.NewAuthMiddleware(jwtSecret, userStore)
	fileHandler := handlers.NewFileHandler(storageRoot, permStore, auditLog, trashStore, tagStore, tierStore)
	diskHandler := handlers.NewDiskHandler(storageRoot)
	shareHandler := handlers.NewShareHandler(storageRoot, permStore, auditLog, sharePwLimiter)
	versionHandler := handlers.NewVersionHandler()
	permHandler := handlers.NewPermissionsHandler(permStore, auditLog)
	auditHandler := handlers.NewAuditHandler(auditLog)
	trashHandler := handlers.NewTrashHandler(trashStore, auditLog)
	notifHandler := handlers.NewNotificationHandler(notifStore)
	tierHandler := handlers.NewBackupTierHandler(tierStore, permStore, auditLog)

	mux := http.NewServeMux()

	protectedWrite := func(handler http.HandlerFunc) http.HandlerFunc {
		return authMiddleware.Wrap(csrfMiddleware.Protect(handler))
	}

	registerAuthRoutes(mux, authHandler, authMiddleware, csrfMiddleware, loginLimiter, protectedWrite)
	registerMfaRoutes(mux, mfaHandler, authHandler, authMiddleware, loginLimiter, protectedWrite)
	registerFileRoutes(mux, fileHandler, authMiddleware, protectedWrite)
	registerPermissionRoutes(mux, permHandler, authMiddleware, protectedWrite)
	registerTrashRoutes(mux, trashHandler, authMiddleware, protectedWrite)
	registerShareRoutes(mux, shareHandler, authMiddleware, protectedWrite)
	registerNotificationRoutes(mux, notifHandler, authMiddleware, protectedWrite)
	registerMiscRoutes(mux, auditHandler, tierHandler, diskHandler, versionHandler, authMiddleware, protectedWrite)

	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		slog.Error("failed to load static files", "err", err)
		os.Exit(1)
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

	// CORS: frontend is same-origin (served by this Go server), so credentials
	// flow works without CORS. CORS is here only for public /share/ endpoints.
	// Do NOT combine AllowCredentials with wildcard origins.
	allowedOrigins := strings.Split(os.Getenv("ALLOWED_ORIGINS"), ",")
	for i := range allowedOrigins {
		allowedOrigins[i] = strings.TrimSpace(allowedOrigins[i])
	}
	if len(allowedOrigins) == 0 || allowedOrigins[0] == "" {
		// Default: no cross-origin access. The frontend is same-origin (served by
		// this server) so it needs no CORS. Set ALLOWED_ORIGINS only if a
		// different origin must call the API.
		allowedOrigins = []string{}
	}
	c := cors.New(cors.Options{
		AllowedOrigins:   allowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type", "X-CSRF-Token"},
		AllowCredentials: false,
	})

	handler := middleware.SecureHeaders(c.Handler(mux))

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	// Graceful shutdown
	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)
		<-sig
		slog.Info("shutting down")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			slog.Warn("graceful shutdown failed", "err", err)
		}
		auditLog.Close()
	}()

	slog.Info("CloudDrive starting", "port", port, "storage", storageRoot)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("listen failed", "err", err)
		os.Exit(1)
	}
}

// ---- Route registration (extracted from main for clarity) ----

func registerAuthRoutes(mux *http.ServeMux, h *handlers.AuthHandler, auth *middleware.AuthMiddleware, csrf *middleware.CSRFMiddleware, limiter *middleware.RateLimiter, protectedWrite func(http.HandlerFunc) http.HandlerFunc) {
	mux.HandleFunc("POST /api/auth/login", limiter.WrapLogin(h.Login))
	mux.HandleFunc("POST /api/auth/logout", h.Logout)
	mux.HandleFunc("GET /api/auth/check", auth.Wrap(h.Check))
	mux.HandleFunc("POST /api/auth/change-password", protectedWrite(h.ChangePassword))
	mux.HandleFunc("GET /api/csrf", auth.Wrap(csrf.GetToken))
}

func registerMfaRoutes(mux *http.ServeMux, h *handlers.MfaHandler, auth *handlers.AuthHandler, am *middleware.AuthMiddleware, limiter *middleware.RateLimiter, protectedWrite func(http.HandlerFunc) http.HandlerFunc) {
	// Status + setup/disable/regenerate: all require an existing session.
	mux.HandleFunc("GET /api/auth/mfa/status", am.Wrap(h.Status))
	mux.HandleFunc("POST /api/auth/mfa/setup", protectedWrite(h.StartSetup))
	mux.HandleFunc("POST /api/auth/mfa/confirm", protectedWrite(h.Confirm))
	mux.HandleFunc("POST /api/auth/mfa/disable", protectedWrite(h.Disable))
	mux.HandleFunc("POST /api/auth/mfa/backup/regenerate", protectedWrite(h.RegenerateBackup))
	// Challenge: no session yet — user has only the mfa_token from Login.
	// Rate limit it the same way we rate limit login.
	mux.HandleFunc("POST /api/auth/mfa/challenge", limiter.WrapLogin(h.Challenge(auth)))
}

func registerFileRoutes(mux *http.ServeMux, h *handlers.FileHandler, auth *middleware.AuthMiddleware, protectedWrite func(http.HandlerFunc) http.HandlerFunc) {
	mux.HandleFunc("GET /api/files", auth.Wrap(h.List))
	mux.HandleFunc("GET /api/files/download", auth.Wrap(h.Download))
	mux.HandleFunc("GET /api/files/preview", auth.Wrap(h.Preview))
	mux.HandleFunc("GET /api/files/search", auth.Wrap(h.Search))
	mux.HandleFunc("GET /api/files/recent", auth.Wrap(h.Recent))
	mux.HandleFunc("GET /api/files/tags", auth.Wrap(h.GetTags))
	mux.HandleFunc("POST /api/files/upload", protectedWrite(h.Upload))
	mux.HandleFunc("POST /api/files/mkdir", protectedWrite(h.Mkdir))
	mux.HandleFunc("POST /api/files/rename", protectedWrite(h.Rename))
	mux.HandleFunc("POST /api/files/move", protectedWrite(h.Move))
	mux.HandleFunc("POST /api/files/copy", protectedWrite(h.Copy))
	mux.HandleFunc("POST /api/files/extract", protectedWrite(h.Extract))
	mux.HandleFunc("POST /api/files/compress", protectedWrite(h.Compress))
	mux.HandleFunc("POST /api/files/tags", protectedWrite(h.SetTags))
	mux.HandleFunc("DELETE /api/files", protectedWrite(h.Delete))
}

func registerPermissionRoutes(mux *http.ServeMux, h *handlers.PermissionsHandler, auth *middleware.AuthMiddleware, protectedWrite func(http.HandlerFunc) http.HandlerFunc) {
	mux.HandleFunc("POST /api/files/permissions", protectedWrite(h.SetPrivate))
	mux.HandleFunc("DELETE /api/files/permissions", protectedWrite(h.RemovePrivate))
	mux.HandleFunc("GET /api/files/permissions", auth.Wrap(h.GetPermission))
}

func registerTrashRoutes(mux *http.ServeMux, h *handlers.TrashHandler, auth *middleware.AuthMiddleware, protectedWrite func(http.HandlerFunc) http.HandlerFunc) {
	mux.HandleFunc("GET /api/trash", auth.Wrap(h.List))
	mux.HandleFunc("POST /api/trash/restore", protectedWrite(h.Restore))
	mux.HandleFunc("DELETE /api/trash", protectedWrite(h.Delete))
	mux.HandleFunc("DELETE /api/trash/empty", protectedWrite(h.Empty))
}

func registerShareRoutes(mux *http.ServeMux, h *handlers.ShareHandler, auth *middleware.AuthMiddleware, protectedWrite func(http.HandlerFunc) http.HandlerFunc) {
	mux.HandleFunc("POST /api/shares", protectedWrite(h.Create))
	mux.HandleFunc("GET /api/shares", auth.Wrap(h.List))
	mux.HandleFunc("POST /api/shares/revoke", protectedWrite(h.Revoke))

	// Public share endpoints (no auth)
	mux.HandleFunc("/share/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && strings.Contains(r.URL.Path, "/upload") {
			h.Upload(w, r)
			return
		}
		h.Download(w, r)
	})
}

func registerNotificationRoutes(mux *http.ServeMux, h *handlers.NotificationHandler, auth *middleware.AuthMiddleware, protectedWrite func(http.HandlerFunc) http.HandlerFunc) {
	mux.HandleFunc("GET /api/notifications", auth.Wrap(h.GetAll))
	mux.HandleFunc("GET /api/notifications/unread", auth.Wrap(h.GetUnreadCount))
	mux.HandleFunc("POST /api/notifications/read", protectedWrite(h.MarkRead))
}

func registerMiscRoutes(mux *http.ServeMux, audit *handlers.AuditHandler, tier *handlers.BackupTierHandler, disk *handlers.DiskHandler, version *handlers.VersionHandler, auth *middleware.AuthMiddleware, protectedWrite func(http.HandlerFunc) http.HandlerFunc) {
	mux.HandleFunc("GET /api/audit", auth.Wrap(audit.GetLogs))
	mux.HandleFunc("GET /api/files/backup-tier", auth.Wrap(tier.Get))
	mux.HandleFunc("GET /api/backup-tiers", auth.Wrap(tier.List))
	mux.HandleFunc("POST /api/files/backup-tier", protectedWrite(tier.Set))
	mux.HandleFunc("GET /api/disk", auth.Wrap(disk.Usage))
	mux.HandleFunc("GET /api/version", version.Info)
}
