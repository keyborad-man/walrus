package service

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"walrus/internal/apperr"
	"walrus/internal/db"
	"walrus/internal/dst"
	"walrus/internal/runtime"
)

type BackupService struct {
	backupsRoot string
	store       *db.Store
	fs          *dst.FS
	driver      runtime.Driver
	onImported  func()
}

func NewBackupService(backupsRoot string, store *db.Store, fs *dst.FS, driver runtime.Driver) *BackupService {
	return &BackupService{
		backupsRoot: backupsRoot,
		store:       store,
		fs:          fs,
		driver:      driver,
	}
}

func (s *BackupService) SetOnImportedHook(fn func()) {
	s.onImported = fn
}

func (s *BackupService) CreateBackup(ctx context.Context, clusterID, trigger, note, actor string) (*db.BackupRecord, error) {
	clusterDir, err := s.fs.ClusterDir(clusterID)
	if err != nil {
		return nil, err
	}
	if !s.fs.ClusterExists(clusterID) {
		return nil, db.ErrNotFound
	}

	// If Master is running, trigger a save before archiving.
	if st := s.driver.GetShardStatus(clusterID, "master"); st.State == runtime.StateRunning {
		_ = s.driver.SendCommand(clusterID, "master", "c_save()")
		select {
		case <-time.After(1200 * time.Millisecond):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	destDir := filepath.Join(s.backupsRoot, clusterID)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return nil, err
	}

	ts := time.Now().Format("20060102-150405")
	filename := fmt.Sprintf("%s_%s_%s.zip", clusterID, ts, trigger)
	zipPath := filepath.Join(destDir, filename)

	size, err := zipDirectory(clusterDir, zipPath)
	if err != nil {
		_ = os.Remove(zipPath)
		return nil, err
	}

	rec, err := s.store.CreateBackupRecord(&db.BackupRecord{
		ClusterID: clusterID,
		Filename:  filename,
		SizeBytes: size,
		Trigger:   trigger,
		Note:      strings.TrimSpace(note),
		CreatedBy: actor,
	})
	if err != nil {
		_ = os.Remove(zipPath)
		return nil, err
	}

	if trigger == "scheduled" {
		if st, err := s.store.GetSystemSettings(false); err == nil && st.BackupMaxRetain > 0 {
			_ = s.EnforceRetention(clusterID, st.BackupMaxRetain)
		}
	}

	return rec, nil
}

// EnforceRetention trims oldest backups beyond maxRetain for a cluster.
func (s *BackupService) EnforceRetention(clusterID string, maxRetain int) error {
	if maxRetain <= 0 {
		return nil
	}
	list, err := s.store.ListBackups(clusterID)
	if err != nil || len(list) <= maxRetain {
		return err
	}
	for _, old := range list[maxRetain:] {
		_ = s.DeleteBackup(clusterID, old.ID)
	}
	return nil
}

// ImportClusterZip extracts a local DST client/server save ZIP, auto-detects the cluster root,
// remaps ports into the configured free port pool, and registers the cluster.
func (s *BackupService) ImportClusterZip(clusterID, clusterName, tokenOverride string, r io.ReaderAt, size int64) error {
	clusterID = strings.TrimSpace(clusterID)
	if err := dst.ValidateClusterID(clusterID); err != nil {
		return err
	}
	if s.fs.ClusterExists(clusterID) {
		return apperr.Newf(apperr.CodeClusterAlreadyExists, map[string]any{"cluster_id": clusterID}, "cluster %q already exists", clusterID)
	}

	zr, err := zip.NewReader(r, size)
	if err != nil {
		return apperr.WrapDetail(apperr.CodeInvalidZipArchive, err, "invalid zip archive: %v", err)
	}

	tmpDir, err := os.MkdirTemp("", "dst-import-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	if err := unzipReaderSafely(zr, tmpDir); err != nil {
		return err
	}

	clusterRoot := findImportedClusterRoot(tmpDir)
	if clusterRoot == "" {
		return apperr.New(apperr.CodeImportClusterRootMissing, "could not find cluster.ini or Master/ directory inside the uploaded ZIP")
	}

	destDir, err := s.fs.ClusterDir(clusterID)
	if err != nil {
		return err
	}
	if err := copyDirSimple(clusterRoot, destDir); err != nil {
		_ = os.RemoveAll(destDir)
		return err
	}

	// Allocate conflict-free ports from the pool and normalize shard INI files.
	st, err := s.store.GetSystemSettings(false)
	if err != nil {
		_ = os.RemoveAll(destDir)
		return err
	}
	ports, err := s.fs.AllocatePorts(st.PortPoolStart, st.PortPoolEnd)
	if err != nil {
		_ = os.RemoveAll(destDir)
		return err
	}

	hasCaves := dirExists(filepath.Join(destDir, "Caves"))
	ci, err := s.fs.ReadClusterINI(clusterID)
	if err != nil {
		_ = os.RemoveAll(destDir)
		return err
	}
	if strings.TrimSpace(clusterName) != "" {
		ci.ClusterName = strings.TrimSpace(clusterName)
	} else if strings.TrimSpace(ci.ClusterName) == "" {
		ci.ClusterName = clusterID
	}
	ci.ShardEnabled = hasCaves
	ci.BindIP = "127.0.0.1"
	ci.MasterIP = "127.0.0.1"
	ci.MasterPort = ports.ClusterMasterPort
	if ci.ClusterKey == "" {
		ci.ClusterKey = dst.RandomClusterKey()
	}
	if err := s.fs.WriteClusterINI(clusterID, ci); err != nil {
		_ = os.RemoveAll(destDir)
		return err
	}

	masterINI, _ := s.fs.ReadServerINI(clusterID, "master")
	masterINI.IsMaster = true
	masterINI.Name = "Master"
	masterINI.ServerPort = ports.MasterServerPort
	masterINI.AuthenticationPort = ports.MasterAuthPort
	masterINI.MasterServerPort = ports.MasterSteamPort
	if err := s.fs.WriteServerINI(clusterID, "master", masterINI); err != nil {
		_ = os.RemoveAll(destDir)
		return err
	}

	if hasCaves {
		cavesINI, _ := s.fs.ReadServerINI(clusterID, "caves")
		cavesINI.IsMaster = false
		cavesINI.Name = "Caves"
		cavesINI.ServerPort = ports.CavesServerPort
		cavesINI.AuthenticationPort = ports.CavesAuthPort
		cavesINI.MasterServerPort = ports.CavesSteamPort
		if err := s.fs.WriteServerINI(clusterID, "caves", cavesINI); err != nil {
			_ = os.RemoveAll(destDir)
			return err
		}
	}

	if strings.TrimSpace(tokenOverride) != "" {
		_ = s.fs.WriteClusterToken(clusterID, tokenOverride)
	}

	meta := &db.ClusterMeta{
		ClusterID:      clusterID,
		DisplayName:    ci.ClusterName,
		Description:    ci.ClusterDescription,
		CavesEnabled:   hasCaves,
		ModSyncEnabled: true,
		AutoRestart:    true,
	}
	if err := s.store.UpsertClusterMeta(meta); err != nil {
		return err
	}
	if s.onImported != nil {
		s.onImported()
	}
	return nil
}

func findImportedClusterRoot(dir string) string {
	isClusterRoot := func(p string) bool {
		if _, err := os.Stat(filepath.Join(p, "cluster.ini")); err == nil {
			return true
		}
		if dirExists(filepath.Join(p, "Master")) {
			return true
		}
		return false
	}
	if isClusterRoot(dir) {
		return dir
	}
	// Walk up to 2 levels deep to handle nested wrappers like Cluster_1/ or MyDediServer/Cluster_1/.
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), "__MACOSX") {
			continue
		}
		sub := filepath.Join(dir, e.Name())
		if isClusterRoot(sub) {
			return sub
		}
		subEntries, _ := os.ReadDir(sub)
		for _, e2 := range subEntries {
			if !e2.IsDir() || strings.HasPrefix(e2.Name(), "__MACOSX") {
				continue
			}
			sub2 := filepath.Join(sub, e2.Name())
			if isClusterRoot(sub2) {
				return sub2
			}
		}
	}
	return ""
}

func dirExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

func unzipReaderSafely(r *zip.Reader, destDir string) error {
	cleanDest := filepath.Clean(destDir) + string(os.PathSeparator)
	for _, f := range r.File {
		target := filepath.Join(destDir, filepath.FromSlash(f.Name))
		if !strings.HasPrefix(filepath.Clean(target)+string(os.PathSeparator), cleanDest) && filepath.Clean(target) != filepath.Clean(destDir) {
			return apperr.Newf(apperr.CodeIllegalZipPath, map[string]any{"path": f.Name}, "illegal file path in zip: %s", f.Name)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, f.Mode()&0o644|0o600)
		if err != nil {
			_ = rc.Close()
			return err
		}
		_, copyErr := io.Copy(out, rc)
		_ = rc.Close()
		_ = out.Close()
		if copyErr != nil {
			return copyErr
		}
	}
	return nil
}

func (s *BackupService) ListBackups(clusterID string) ([]db.BackupRecord, error) {
	if err := dst.ValidateClusterID(clusterID); err != nil {
		return nil, err
	}
	list, err := s.store.ListBackups(clusterID)
	if err != nil {
		return nil, err
	}
	if list == nil {
		return []db.BackupRecord{}, nil
	}
	return list, nil
}

func (s *BackupService) BackupFilePath(clusterID string, backupID int64) (string, *db.BackupRecord, error) {
	rec, err := s.store.GetBackup(backupID)
	if err != nil {
		return "", nil, err
	}
	if rec.ClusterID != clusterID {
		return "", nil, db.ErrNotFound
	}
	cleanName := filepath.Base(rec.Filename)
	full := filepath.Join(s.backupsRoot, clusterID, cleanName)
	if _, err := os.Stat(full); err != nil {
		return "", nil, apperr.Wrap(apperr.CodeBackupFileMissing, err, nil, fmt.Sprintf("backup archive missing on disk: %v", err))
	}
	return full, rec, nil
}

func (s *BackupService) RestoreBackup(ctx context.Context, clusterID string, backupID int64, actor string) error {
	zipPath, rec, err := s.BackupFilePath(clusterID, backupID)
	if err != nil {
		return err
	}
	clusterDir, err := s.fs.ClusterDir(clusterID)
	if err != nil {
		return err
	}

	// Stop shards cleanly first.
	_ = s.driver.StopShard(ctx, clusterID, "caves")
	_ = s.driver.StopShard(ctx, clusterID, "master")

	// Create automatic pre-restore safety snapshot.
	_, _ = s.CreateBackup(ctx, clusterID, "pre_restore", "Auto safety snapshot before restoring "+rec.Filename, actor)

	// Extract to a staging sibling dir first, then swap atomically.
	stagingDir := clusterDir + ".restore_tmp"
	_ = os.RemoveAll(stagingDir)
	if err := os.MkdirAll(stagingDir, 0o755); err != nil {
		return err
	}
	if err := unzipSafely(zipPath, stagingDir); err != nil {
		_ = os.RemoveAll(stagingDir)
		return err
	}

	oldDir := clusterDir + ".restore_old"
	_ = os.RemoveAll(oldDir)
	if err := os.Rename(clusterDir, oldDir); err != nil {
		_ = os.RemoveAll(stagingDir)
		return err
	}
	if err := os.Rename(stagingDir, clusterDir); err != nil {
		_ = os.Rename(oldDir, clusterDir)
		return err
	}
	_ = os.RemoveAll(oldDir)
	return nil
}

func (s *BackupService) DeleteBackup(clusterID string, backupID int64) error {
	rec, err := s.store.GetBackup(backupID)
	if err != nil {
		return err
	}
	if rec.ClusterID != clusterID {
		return db.ErrNotFound
	}
	full := filepath.Join(s.backupsRoot, clusterID, filepath.Base(rec.Filename))
	_ = os.Remove(full)
	return s.store.DeleteBackupRecord(backupID)
}

func zipDirectory(srcDir, destZip string) (int64, error) {
	out, err := os.Create(destZip)
	if err != nil {
		return 0, err
	}
	defer out.Close()

	zw := zip.NewWriter(out)
	err = filepath.Walk(srcDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == srcDir {
			return nil
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if info.IsDir() {
			_, err = zw.Create(rel + "/")
			return err
		}
		hdr, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		hdr.Name = rel
		hdr.Method = zip.Deflate
		w, err := zw.CreateHeader(hdr)
		if err != nil {
			return err
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(w, f)
		return err
	})
	if err != nil {
		_ = zw.Close()
		return 0, err
	}
	if err := zw.Close(); err != nil {
		return 0, err
	}
	fi, err := out.Stat()
	if err != nil {
		return 0, err
	}
	return fi.Size(), nil
}

func unzipSafely(zipPath, destDir string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	cleanDest := filepath.Clean(destDir) + string(os.PathSeparator)

	for _, f := range r.File {
		target := filepath.Join(destDir, filepath.FromSlash(f.Name))
		if !strings.HasPrefix(filepath.Clean(target)+string(os.PathSeparator), cleanDest) && filepath.Clean(target) != filepath.Clean(destDir) {
			return apperr.Newf(apperr.CodeIllegalZipPath, map[string]any{"path": f.Name}, "illegal file path in zip: %s", f.Name)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, f.Mode()&0o644|0o600)
		if err != nil {
			_ = rc.Close()
			return err
		}
		_, copyErr := io.Copy(out, rc)
		_ = rc.Close()
		_ = out.Close()
		if copyErr != nil {
			return copyErr
		}
	}
	return nil
}
