package native

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/replacearr/replacearr/internal/domain"
)

var tagLabelPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

func (s *Server) handleGetConfig(w http.ResponseWriter, _ *http.Request) {
	cfg, err := s.store.GetAppConfig()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, s.presentConfig(cfg))
}

func (s *Server) handlePutConfig(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	cfg, err := s.store.GetAppConfig()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	prevBind := cfg.Host.BindAddress
	prevPort := cfg.Host.Port
	prevURLBase := cfg.Host.URLBase
	if err := json.Unmarshal(body, &cfg); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if os.Getenv("REPLACEARR_HOST") != "" {
		cfg.Host.BindAddress = prevBind
	}
	if os.Getenv("REPLACEARR_PORT") != "" {
		cfg.Host.Port = prevPort
	}
	if os.Getenv("REPLACEARR_URL_BASE") != "" {
		cfg.Host.URLBase = prevURLBase
	}
	saved, err := s.store.SaveAppConfig(cfg)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, s.presentConfig(saved))
}

func (s *Server) presentConfig(cfg domain.AppConfig) domain.AppConfig {
	storedBind := cfg.Host.BindAddress
	storedPort := cfg.Host.Port
	storedURLBase := cfg.Host.URLBase

	cfg.Host.BindAddressEnvLocked = false
	cfg.Host.PortEnvLocked = false
	cfg.Host.URLBaseEnvLocked = false
	cfg.Host.PendingRestart = false

	if os.Getenv("REPLACEARR_HOST") != "" {
		cfg.Host.BindAddress = s.cfg.Host
		cfg.Host.BindAddressEnvLocked = true
	} else if storedBind != s.cfg.Host {
		cfg.Host.PendingRestart = true
	}

	if os.Getenv("REPLACEARR_PORT") != "" {
		cfg.Host.Port = s.cfg.Port
		cfg.Host.PortEnvLocked = true
	} else if storedPort != s.cfg.Port {
		cfg.Host.PendingRestart = true
	}

	if os.Getenv("REPLACEARR_URL_BASE") != "" {
		cfg.Host.URLBase = s.cfg.URLBase
		cfg.Host.URLBaseEnvLocked = true
	} else if storedURLBase != s.cfg.URLBase {
		cfg.Host.PendingRestart = true
	}

	if !cfg.Host.BindAddressEnvLocked && cfg.Host.BindAddress != s.cfg.Host {
		cfg.Host.PendingRestart = true
	}
	if !cfg.Host.PortEnvLocked && cfg.Host.Port != s.cfg.Port {
		cfg.Host.PendingRestart = true
	}
	if !cfg.Host.URLBaseEnvLocked && cfg.Host.URLBase != s.cfg.URLBase {
		cfg.Host.PendingRestart = true
	}

	cfg.Security.APIKey = s.authSvc.GetMasterAPIKey()
	cfg.Security.AuthenticationMethod = "forms"
	return cfg
}

func (s *Server) handleRotateAPIKey(w http.ResponseWriter, _ *http.Request) {
	raw, err := s.authSvc.RotateMasterAPIKey()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"apiKey": raw})
}

func (s *Server) handleListQualities(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, domain.BuiltinQualityDefinitions())
}

func (s *Server) handleCreateProfile(w http.ResponseWriter, r *http.Request) {
	var qp domain.QualityProfile
	if err := json.NewDecoder(r.Body).Decode(&qp); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if strings.TrimSpace(qp.Name) == "" {
		writeErr(w, http.StatusBadRequest, errors.New("name is required"))
		return
	}
	if len(qp.Items) == 0 {
		defs := domain.BuiltinQualityDefinitions()
		qp.Items = make([]domain.QualityItem, 0, len(defs))
		for _, def := range defs {
			qp.Items = append(qp.Items, domain.QualityItem{
				ID:         def.ID,
				Name:       def.Name,
				Source:     def.Source,
				Resolution: def.Resolution,
				Allowed:    true,
			})
		}
	}
	created, err := s.store.CreateQualityProfile(qp)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) handleUpdateProfile(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	var qp domain.QualityProfile
	if err := json.NewDecoder(r.Body).Decode(&qp); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	qp.ID = id
	updated, err := s.store.UpdateQualityProfile(qp)
	if errors.Is(err, sql.ErrNoRows) {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) handleDeleteProfile(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err := s.store.DeleteQualityProfile(id); err != nil {
		if strings.Contains(err.Error(), "in use") {
			writeErr(w, http.StatusConflict, err)
			return
		}
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListTags(w http.ResponseWriter, _ *http.Request) {
	items, err := s.store.ListTags()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleCreateTag(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Label string `json:"label"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	label := strings.ToLower(strings.TrimSpace(body.Label))
	if !tagLabelPattern.MatchString(label) {
		writeErr(w, http.StatusBadRequest, errors.New("invalid label"))
		return
	}
	created, err := s.store.CreateTag(label)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) handleGetTag(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	details, err := s.store.TagDetails(id)
	if errors.Is(err, sql.ErrNoRows) {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, details)
}

func (s *Server) handleDeleteTag(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err := s.store.DeleteTag(id); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleDeleteRootFolder(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	folders, err := s.store.ListRootFolders("")
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	var folder *domain.RootFolder
	for i := range folders {
		if folders[i].ID == id {
			folder = &folders[i]
			break
		}
	}
	if folder == nil {
		writeErr(w, http.StatusNotFound, sql.ErrNoRows)
		return
	}
	series, err := s.store.ListSeries()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	for _, sr := range series {
		if sr.RootFolderPath == folder.Path {
			writeErr(w, http.StatusConflict, errors.New("root folder is in use"))
			return
		}
	}
	movies, err := s.store.ListMovies()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	for _, mv := range movies {
		if mv.RootFolderPath == folder.Path {
			writeErr(w, http.StatusConflict, errors.New("root folder is in use"))
			return
		}
	}
	if err := s.store.DeleteRootFolder(id); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleUpdateIndexer(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	var idx domain.Indexer
	if err := json.NewDecoder(r.Body).Decode(&idx); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	idx.ID = id
	if idx.APIKey == "" {
		existing, err := s.store.GetIndexerByID(id)
		if errors.Is(err, sql.ErrNoRows) {
			writeErr(w, http.StatusNotFound, err)
			return
		}
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
		idx.APIKey = existing.APIKey
	}
	if idx.Fields == nil {
		idx.Fields = []domain.ConfigField{}
	}
	if idx.Tags == nil {
		idx.Tags = []int64{}
	}
	updated, err := s.store.UpdateIndexer(idx)
	if errors.Is(err, sql.ErrNoRows) {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) handleUpdateDownloadClient(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	var dc domain.DownloadClient
	if err := json.NewDecoder(r.Body).Decode(&dc); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	dc.ID = id
	updated, err := s.store.UpdateDownloadClient(dc)
	if errors.Is(err, sql.ErrNoRows) {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) handleTestDownloadClient(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Host   string `json:"host"`
		Port   int    `json:"port"`
		UseSsl bool   `json:"useSsl"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	host := strings.TrimSpace(req.Host)
	if host == "" || req.Port < 1 || req.Port > 65535 {
		writeErr(w, http.StatusBadRequest, errors.New("host and a port between 1 and 65535 are required"))
		return
	}
	_ = req.UseSsl
	start := time.Now()
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, strconv.Itoa(req.Port)), 3*time.Second)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":        false,
			"message":   err.Error(),
			"latencyMs": latency,
		})
		return
	}
	_ = conn.Close()
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":        true,
		"message":   "connected",
		"latencyMs": latency,
	})
}
