package tasks

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/replacearr/replacearr/internal/applog"
	"github.com/replacearr/replacearr/internal/db"
	"github.com/replacearr/replacearr/internal/domain"
)

type TaskInfo struct {
	Name           string     `json:"name"`
	Label          string     `json:"label,omitempty"`
	Interval       string     `json:"interval,omitempty"`
	LastRunAt      *time.Time `json:"lastRunAt,omitempty"`
	NextRunAt      *time.Time `json:"nextRunAt,omitempty"`
	LastDurationMs *int64     `json:"lastDurationMs,omitempty"`
	Status         string     `json:"status,omitempty"`
}

type BackupFile struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Size      int64     `json:"size"`
	CreatedAt time.Time `json:"createdAt"`
}

type Scheduler struct {
	store     *db.Store
	configDir string
	dbPath    string

	mu    sync.Mutex
	tasks map[string]*taskState
}

type taskState struct {
	info     TaskInfo
	interval time.Duration
	fn       func(ctx context.Context, manual bool) error
	running  bool
}

func New(store *db.Store, configDir, dbPath string) *Scheduler {
	s := &Scheduler{
		store:     store,
		configDir: configDir,
		dbPath:    dbPath,
		tasks:     map[string]*taskState{},
	}
	s.register("backup", "Backup", 24*time.Hour, func(ctx context.Context, _ bool) error {
		return s.runBackup(ctx)
	})
	s.register("historyCleanup", "History Cleanup", 24*time.Hour, func(ctx context.Context, _ bool) error {
		return s.runHistoryCleanup(ctx)
	})
	s.register("recycleBinCleanup", "Recycle Bin Cleanup", 24*time.Hour, s.runRecycleCleanup)
	s.register("healthRefresh", "Health Refresh", 5*time.Minute, func(context.Context, bool) error {
		applog.Info("tasks", "health refresh tick")
		return nil
	})
	return s
}

func (s *Scheduler) register(name, label string, interval time.Duration, fn func(context.Context, bool) error) {
	now := time.Now().UTC()
	next := now.Add(interval)
	s.tasks[name] = &taskState{
		info: TaskInfo{
			Name:     name,
			Label:    label,
			Interval: interval.String(),
			Status:   "idle",
			NextRunAt: &next,
		},
		interval: interval,
		fn:       fn,
	}
}

func (s *Scheduler) Run(ctx context.Context) {
	t := time.NewTicker(30 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.tick(ctx)
		}
	}
}

func (s *Scheduler) tick(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	for _, ts := range s.tasks {
		if ts.running || ts.info.NextRunAt == nil || ts.info.NextRunAt.After(now) {
			continue
		}
		go s.execute(ctx, ts.info.Name, false)
	}
}

func (s *Scheduler) execute(ctx context.Context, name string, manual bool) {
	s.mu.Lock()
	ts := s.tasks[name]
	if ts == nil || ts.running {
		s.mu.Unlock()
		return
	}
	ts.running = true
	ts.info.Status = "running"
	s.mu.Unlock()

	start := time.Now().UTC()
	err := ts.fn(ctx, manual)
	dur := time.Since(start).Milliseconds()

	s.mu.Lock()
	defer s.mu.Unlock()
	ts.running = false
	ts.info.LastRunAt = &start
	ts.info.LastDurationMs = &dur
	next := time.Now().UTC().Add(ts.interval)
	ts.info.NextRunAt = &next
	if err != nil {
		ts.info.Status = "failed"
		applog.Error("tasks", fmt.Sprintf("%s failed: %v", name, err))
	} else {
		ts.info.Status = "idle"
		applog.Info("tasks", fmt.Sprintf("%s completed in %dms", name, dur))
	}
}

func (s *Scheduler) List() []TaskInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]TaskInfo, 0, len(s.tasks))
	for _, ts := range s.tasks {
		out = append(out, ts.info)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func (s *Scheduler) RunNow(name string) error {
	s.mu.Lock()
	ts := s.tasks[name]
	s.mu.Unlock()
	if ts == nil {
		return fmt.Errorf("unknown task %q", name)
	}
	go s.execute(context.Background(), name, true)
	return nil
}

func (s *Scheduler) backupDir() (string, error) {
	cfg, err := s.store.GetAppConfig()
	if err != nil {
		return "", err
	}
	folder := strings.TrimSpace(cfg.Backup.Folder)
	if folder == "" {
		folder = "Backups"
	}
	if !filepath.IsAbs(folder) {
		folder = filepath.Join(s.configDir, folder)
	}
	if err := os.MkdirAll(folder, 0o755); err != nil {
		return "", err
	}
	return folder, nil
}

func (s *Scheduler) runBackup(ctx context.Context) error {
	_, err := s.CreateBackup()
	return err
}

func (s *Scheduler) CreateBackup() (*BackupFile, error) {
	dir, err := s.backupDir()
	if err != nil {
		return nil, err
	}
	name := fmt.Sprintf("replacearr-%s.zip", time.Now().UTC().Format("20060102-150405"))
	path := filepath.Join(dir, name)
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	defer zw.Close()

	addFile := func(src, nameInZip string) error {
		in, err := os.Open(src)
		if err != nil {
			return err
		}
		defer in.Close()
		w, err := zw.Create(nameInZip)
		if err != nil {
			return err
		}
		_, err = io.Copy(w, in)
		return err
	}

	if s.dbPath != "" {
		_ = addFile(s.dbPath, "replacearr.db")
		_ = addFile(s.dbPath+"-wal", "replacearr.db-wal")
		_ = addFile(s.dbPath+"-shm", "replacearr.db-shm")
	}
	_ = filepath.Walk(s.configDir, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if strings.Contains(p, string(filepath.Separator)+"Backups"+string(filepath.Separator)) {
			return nil
		}
		rel, err := filepath.Rel(s.configDir, p)
		if err != nil {
			return nil
		}
		_ = addFile(p, filepath.ToSlash(filepath.Join("config", rel)))
		return nil
	})

	fi, _ := os.Stat(path)
	size := int64(0)
	if fi != nil {
		size = fi.Size()
	}
	now := time.Now().UTC()
	_ = s.enforceRetention()
	return &BackupFile{ID: name, Name: name, Size: size, CreatedAt: now}, nil
}

func (s *Scheduler) ListBackups() ([]BackupFile, error) {
	dir, err := s.backupDir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	out := []BackupFile{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".zip") {
			continue
		}
		fi, err := e.Info()
		if err != nil {
			continue
		}
		out = append(out, BackupFile{
			ID:        e.Name(),
			Name:      e.Name(),
			Size:      fi.Size(),
			CreatedAt: fi.ModTime().UTC(),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

func (s *Scheduler) BackupPath(id string) (string, error) {
	if id == "" || strings.Contains(id, "..") || strings.ContainsAny(id, `/\`) {
		return "", fmt.Errorf("invalid backup id")
	}
	dir, err := s.backupDir()
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, id)
	if _, err := os.Stat(path); err != nil {
		return "", err
	}
	return path, nil
}

func (s *Scheduler) DeleteBackup(id string) error {
	path, err := s.BackupPath(id)
	if err != nil {
		return err
	}
	return os.Remove(path)
}

func (s *Scheduler) RestoreBackup(id string) error {
	path, err := s.BackupPath(id)
	if err != nil {
		return err
	}
	r, err := zip.OpenReader(path)
	if err != nil {
		return err
	}
	defer r.Close()
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		name := f.Name
		var dest string
		switch {
		case name == "replacearr.db":
			dest = s.dbPath
		case strings.HasPrefix(name, "config/"):
			dest = filepath.Join(s.configDir, strings.TrimPrefix(name, "config/"))
		default:
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.Create(dest)
		if err != nil {
			rc.Close()
			return err
		}
		_, copyErr := io.Copy(out, rc)
		out.Close()
		rc.Close()
		if copyErr != nil {
			return copyErr
		}
	}
	cfg, err := s.store.GetAppConfig()
	if err == nil {
		cfg.Host.PendingRestart = true
		_, _ = s.store.SaveAppConfig(cfg)
	}
	return nil
}

func (s *Scheduler) enforceRetention() error {
	cfg, err := s.store.GetAppConfig()
	if err != nil {
		return err
	}
	days := cfg.Backup.Retention
	if days <= 0 {
		return nil
	}
	cutoff := time.Now().UTC().AddDate(0, 0, -days)
	files, err := s.ListBackups()
	if err != nil {
		return err
	}
	for _, f := range files {
		if f.CreatedAt.Before(cutoff) {
			_ = s.DeleteBackup(f.ID)
		}
	}
	return nil
}

func (s *Scheduler) runHistoryCleanup(ctx context.Context) error {
	cfg, err := s.store.GetAppConfig()
	if err != nil {
		return err
	}
	days := cfg.Logging.HistoryCleanupDays
	if days <= 0 {
		return nil
	}
	return s.store.DeleteHistoryOlderThan(time.Now().UTC().AddDate(0, 0, -days))
}

func (s *Scheduler) runRecycleCleanup(_ context.Context, manual bool) error {
	cfg, err := s.store.GetAppConfig()
	if err != nil {
		return err
	}
	bin := strings.TrimSpace(cfg.MediaManagement.RecycleBin)
	if bin == "" {
		bin = filepath.Join(s.configDir, "RecycleBin")
	}
	entries, err := os.ReadDir(bin)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	days := cfg.MediaManagement.RecycleBinCleanupDays
	if days <= 0 {
		days = 30
	}
	cutoff := time.Now().AddDate(0, 0, -days)
	for _, e := range entries {
		full := filepath.Join(bin, e.Name())
		if manual {
			_ = os.RemoveAll(full)
			continue
		}
		fi, err := e.Info()
		if err != nil {
			continue
		}
		if fi.ModTime().Before(cutoff) {
			_ = os.RemoveAll(full)
		}
	}
	return nil
}

// AdjustIntervals updates backup interval from config.
func (s *Scheduler) AdjustIntervals(cfg domain.AppConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ts, ok := s.tasks["backup"]; ok {
		d := time.Duration(cfg.Backup.Interval) * 24 * time.Hour
		if d <= 0 {
			d = 7 * 24 * time.Hour
		}
		ts.interval = d
		ts.info.Interval = d.String()
	}
}
