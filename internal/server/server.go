package server

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"hy2panel/internal/config"
	"hy2panel/internal/racknerd"
	"hy2panel/internal/store"
	panelweb "hy2panel/web"
)

type Server struct {
	store            *store.Store
	auth             *authenticator
	applier          Applier
	applyMu          sync.Mutex
	bbr              BBRApplier
	bbrMu            sync.Mutex
	speedtest        *SpeedTester
	rackNerdClient   rackNerdInfoClient
	rackNerdMu       sync.Mutex
	rackNerdCache    RackNerdView
	rackNerdCachedAt time.Time
	rackNerdTTL      time.Duration
	nodeMetaClient   geoIPLookup
	nodeMetaMu       sync.Mutex
	nodeMetaCache    NodeMeta
	nodeMetaHost     string
	nodeMetaCachedAt time.Time
	nodeMetaTTL      time.Duration
	update           *UpdateManager
}

type stateResponse struct {
	Config    config.Config     `json:"config"`
	Links     map[string]string `json:"links"`
	Status    Status            `json:"status"`
	RackNerd  RackNerdView      `json:"racknerd"`
	SpeedTest SpeedTestState    `json:"speedtest"`
	NodeMeta  NodeMeta          `json:"node_meta"`
	Update    UpdateView        `json:"update"`
}

type updateRequest struct {
	PublicHost string           `json:"public_host"`
	Reality    config.Reality   `json:"reality"`
	Hysteria2  config.Hysteria2 `json:"hysteria2"`
}

func New(store *store.Store, applier Applier) *Server {
	return &Server{
		store:          store,
		auth:           newAuthenticator(),
		applier:        applier,
		bbr:            FileBBRApplier{Dir: filepath.Dir(store.Path())},
		speedtest:      NewSpeedTester(),
		rackNerdClient: &racknerd.Client{},
		nodeMetaClient: &geoIPClient{},
		update:         NewUpdateManager(filepath.Dir(store.Path())),
	}
}

func (s *Server) Serve(ctx context.Context) error {
	cfg, err := s.store.Load()
	if err != nil {
		return err
	}
	prefix, err := netip.ParsePrefix(cfg.Panel.AdminCIDR)
	if err != nil {
		return err
	}
	server := &http.Server{
		Addr:              cfg.Panel.Listen,
		Handler:           s.routes(prefix),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       90 * time.Second,
		MaxHeaderBytes:    32 << 10,
		TLSConfig:         &tls.Config{MinVersion: tls.VersionTLS12},
	}
	go func() {
		<-ctx.Done()
		shutdownContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownContext)
	}()
	err = server.ListenAndServeTLS(cfg.Panel.CertFile, cfg.Panel.KeyFile)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (s *Server) routes(prefix netip.Prefix) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("POST /api/login", s.login)
	mux.Handle("POST /api/logout", s.requireAuth(http.HandlerFunc(s.logout)))
	mux.Handle("GET /api/state", s.requireAuth(http.HandlerFunc(s.state)))
	mux.Handle("POST /api/config", s.requireAuth(http.HandlerFunc(s.updateConfig)))
	mux.Handle("POST /api/settings/racknerd", s.requireAuth(http.HandlerFunc(s.updateRackNerd)))
	mux.Handle("POST /api/settings/recovery", s.requireAuth(http.HandlerFunc(s.updateRecovery)))
	mux.Handle("GET /api/logs", s.requireAuth(http.HandlerFunc(s.logs)))
	mux.Handle("POST /api/diagnostics", s.requireAuth(http.HandlerFunc(s.diagnostics)))
	mux.Handle("POST /api/update/check", s.requireAuth(http.HandlerFunc(s.checkUpdate)))
	mux.Handle("POST /api/update/apply", s.requireAuth(http.HandlerFunc(s.applyUpdate)))
	mux.Handle("POST /api/bbr", s.requireAuth(http.HandlerFunc(s.applyBBR)))
	mux.Handle("POST /api/speedtest", s.requireAuth(http.HandlerFunc(s.startSpeedTest)))
	mux.Handle("GET /", http.FileServer(http.FS(panelweb.Dist())))
	return securityHeaders(whitelist(prefix, mux))
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	if !validMutation(r) {
		writeError(w, http.StatusForbidden, "请求来源无效")
		return
	}
	var input struct {
		Password string `json:"password"`
	}
	if err := decodeJSON(w, r, &input); err != nil || len(input.Password) > 128 {
		writeError(w, http.StatusBadRequest, "登录请求无效")
		return
	}
	cfg, err := s.store.Load()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "读取配置失败")
		return
	}
	token, err := s.auth.login(input.Password, cfg.AdminPasswordHash)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}
	http.SetCookie(w, sessionCookie(token, time.Now().Add(sessionLifetime), int(sessionLifetime.Seconds())))
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if !validMutation(r) {
		writeError(w, http.StatusForbidden, "请求来源无效")
		return
	}
	if cookie, err := r.Cookie("__Host-hy2panel"); err == nil {
		s.auth.logout(cookie.Value)
	}
	http.SetCookie(w, sessionCookie("", time.Unix(1, 0), -1))
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) state(w http.ResponseWriter, r *http.Request) {
	cfg, err := s.store.Load()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "读取配置失败")
		return
	}
	writeJSON(w, http.StatusOK, s.makeState(r.Context(), cfg))
}

func (s *Server) updateConfig(w http.ResponseWriter, r *http.Request) {
	if !validMutation(r) {
		writeError(w, http.StatusForbidden, "请求来源无效")
		return
	}
	var input updateRequest
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "配置请求无效")
		return
	}
	s.applyMu.Lock()
	defer s.applyMu.Unlock()
	current, err := s.store.Load()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "读取配置失败")
		return
	}
	next := *current
	next.Panel.PublicHost = strings.TrimSpace(input.PublicHost)
	next.Reality = input.Reality
	next.Hysteria2 = input.Hysteria2
	next.Hysteria2.Domain = current.Hysteria2.Domain
	next.Hysteria2.CertFile = current.Hysteria2.CertFile
	next.Hysteria2.KeyFile = current.Hysteria2.KeyFile
	next.AdminPasswordHash = current.AdminPasswordHash
	if err := next.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	singBoxJSON, err := next.SingBoxJSON()
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if s.applier == nil {
		writeError(w, http.StatusInternalServerError, "配置应用器不可用")
		return
	}
	applyContext, cancelApply := context.WithTimeout(context.Background(), 50*time.Second)
	defer cancelApply()
	if err := s.applier.Apply(applyContext, singBoxJSON); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	if err := s.store.Save(&next); err != nil {
		writeError(w, http.StatusInternalServerError, "节点已应用，但面板配置保存失败")
		return
	}
	writeJSON(w, http.StatusOK, s.makeState(r.Context(), &next))
}

func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("__Host-hy2panel")
		if err != nil || !s.auth.valid(cookie.Value) {
			writeError(w, http.StatusUnauthorized, "请先登录")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func whitelist(prefix netip.Prefix, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			host = r.RemoteAddr
		}
		addr, err := netip.ParseAddr(host)
		if err == nil {
			addr = addr.Unmap()
		}
		if err != nil || !prefix.Contains(addr) {
			writeError(w, http.StatusForbidden, "当前管理 IP 不在白名单内")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; style-src 'self' 'unsafe-inline'; script-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Strict-Transport-Security", "max-age=15552000")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Cache-Control", "no-store")
		}
		next.ServeHTTP(w, r)
	})
}

func validMutation(r *http.Request) bool {
	if r.Header.Get("X-Requested-With") != "fetch" {
		return false
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	return err == nil && parsed.Host == r.Host && (parsed.Scheme == "https" || parsed.Scheme == "http")
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("请求只能包含一个 JSON 对象")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
