package service

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"walrus/internal/apperr"
	"walrus/internal/dst"
)

const maxViewFileSize = 2 * 1024 * 1024 // 2 MB cap for browser viewing/editing

var editableExts = map[string]bool{
	".lua":  true,
	".ini":  true,
	".txt":  true,
	".log":  true,
	".json": true,
	".cfg":  true,
	".md":   true,
	".yaml": true,
	".yml":  true,
}

var protectedRelPaths = map[string]bool{
	"":                  true,
	".":                 true,
	"cluster.ini":       true,
	"Master":            true,
	"Caves":             true,
	"Master/server.ini": true,
	"Caves/server.ini":  true,
	"Master/save":       true,
	"Caves/save":        true,
}

type FileEntry struct {
	Name      string    `json:"name"`
	RelPath   string    `json:"rel_path"`
	IsDir     bool      `json:"is_dir"`
	SizeBytes int64     `json:"size_bytes"`
	ModTime   time.Time `json:"mod_time"`
	Editable  bool      `json:"editable"`
	Protected bool      `json:"protected"`
}

type FileContent struct {
	RelPath   string `json:"rel_path"`
	SizeBytes int64  `json:"size_bytes"`
	Editable  bool   `json:"editable"`
	Truncated bool   `json:"truncated"`
	IsBinary  bool   `json:"is_binary"`
	Content   string `json:"content"`
}

type FileService struct {
	fs *dst.FS
}

func NewFileService(fs *dst.FS) *FileService {
	return &FileService{fs: fs}
}

// resolveSafePath jails relPath strictly within clusterID's canonical directory, blocking traversal and symlink escapes.
func (s *FileService) resolveSafePath(clusterID, relPath string) (absPath string, cleanRel string, err error) {
	cDir, err := s.fs.ClusterDir(clusterID)
	if err != nil {
		return "", "", err
	}
	realRoot, err := filepath.EvalSymlinks(cDir)
	if err != nil {
		return "", "", apperr.Wrap(apperr.CodeClusterDirUnavailable, err, nil, "cluster directory unavailable: "+err.Error())
	}
	realRoot = filepath.Clean(realRoot)

	cleaned := filepath.Clean("/" + strings.ReplaceAll(strings.TrimSpace(relPath), "\\", "/"))
	cleaned = strings.TrimPrefix(cleaned, string(filepath.Separator))
	cleaned = strings.TrimPrefix(cleaned, "/")
	if cleaned == "." {
		cleaned = ""
	}

	candidate := filepath.Join(realRoot, filepath.FromSlash(cleaned))
	rel, err := filepath.Rel(realRoot, candidate)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", "", apperr.New(apperr.CodePathAccessDenied, "access denied: path escapes cluster directory")
	}

	// If path exists, resolve symlinks and re-verify it stays inside realRoot.
	if _, statErr := os.Lstat(candidate); statErr == nil {
		realTarget, symErr := filepath.EvalSymlinks(candidate)
		if symErr != nil {
			return "", "", apperr.New(apperr.CodePathAccessDenied, "access denied: unreadable symlink")
		}
		relReal, relErr := filepath.Rel(realRoot, filepath.Clean(realTarget))
		if relErr != nil || relReal == ".." || strings.HasPrefix(relReal, ".."+string(filepath.Separator)) {
			return "", "", apperr.New(apperr.CodePathAccessDenied, "access denied: symlink escapes cluster directory")
		}
		candidate = realTarget
	} else {
		// Parent directory must still resolve inside realRoot.
		parentReal, pErr := filepath.EvalSymlinks(filepath.Dir(candidate))
		if pErr == nil {
			relParent, rErr := filepath.Rel(realRoot, filepath.Clean(parentReal))
			if rErr != nil || relParent == ".." || strings.HasPrefix(relParent, ".."+string(filepath.Separator)) {
				return "", "", apperr.New(apperr.CodePathAccessDenied, "access denied: parent escapes cluster directory")
			}
		}
	}

	normRel := filepath.ToSlash(rel)
	if normRel == "." {
		normRel = ""
	}
	return candidate, normRel, nil
}

func isEditableFile(name string, size int64) bool {
	if size > maxViewFileSize {
		return false
	}
	ext := strings.ToLower(filepath.Ext(name))
	return editableExts[ext]
}

func (s *FileService) ListDir(clusterID, relPath string) ([]FileEntry, string, error) {
	abs, normRel, err := s.resolveSafePath(clusterID, relPath)
	if err != nil {
		return nil, "", err
	}
	entries, err := os.ReadDir(abs)
	if err != nil {
		return nil, "", err
	}

	out := make([]FileEntry, 0, len(entries))
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		childRel := e.Name()
		if normRel != "" {
			childRel = normRel + "/" + e.Name()
		}
		isDir := e.IsDir()
		out = append(out, FileEntry{
			Name:      e.Name(),
			RelPath:   childRel,
			IsDir:     isDir,
			SizeBytes: info.Size(),
			ModTime:   info.ModTime().UTC(),
			Editable:  !isDir && isEditableFile(e.Name(), info.Size()),
			Protected: protectedRelPaths[childRel],
		})
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].IsDir != out[j].IsDir {
			return out[i].IsDir
		}
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out, normRel, nil
}

func (s *FileService) ReadFile(clusterID, relPath string) (*FileContent, error) {
	abs, normRel, err := s.resolveSafePath(clusterID, relPath)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return nil, apperr.New(apperr.CodeCannotReadDir, "cannot read a directory as a file")
	}

	f, err := os.Open(abs)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	truncated := info.Size() > maxViewFileSize
	readLen := info.Size()
	if readLen > maxViewFileSize {
		readLen = maxViewFileSize
	}
	buf := make([]byte, readLen)
	n, _ := f.Read(buf)
	buf = buf[:n]

	isBin := bytes.IndexByte(buf, 0) >= 0
	content := ""
	if !isBin {
		content = string(buf)
	}

	return &FileContent{
		RelPath:   normRel,
		SizeBytes: info.Size(),
		Editable:  !isBin && !truncated && isEditableFile(info.Name(), info.Size()),
		Truncated: truncated,
		IsBinary:  isBin,
		Content:   content,
	}, nil
}

func (s *FileService) WriteFile(clusterID, relPath, content string) error {
	abs, normRel, err := s.resolveSafePath(clusterID, relPath)
	if err != nil {
		return err
	}
	if normRel == "" {
		return apperr.New(apperr.CodeCannotOverwriteRoot, "cannot overwrite cluster root")
	}
	if !isEditableFile(filepath.Base(abs), int64(len(content))) {
		return apperr.New(apperr.CodeFileNotEditable, "file type is not editable or exceeds 2MB size limit")
	}
	if info, err := os.Lstat(abs); err == nil {
		if info.IsDir() {
			return apperr.New(apperr.CodeCannotOverwriteDir, "cannot overwrite a directory")
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return apperr.New(apperr.CodeCannotWriteSymlink, "refusing to write through symlink")
		}
	}
	return dst.WriteFileAtomic(abs, []byte(content), 0o644)
}

func (s *FileService) DeleteFile(clusterID, relPath string) error {
	abs, normRel, err := s.resolveSafePath(clusterID, relPath)
	if err != nil {
		return err
	}
	if protectedRelPaths[normRel] {
		return apperr.Newf(apperr.CodeProtectedPathDelete, map[string]any{"path": normRel}, "protected path %q cannot be deleted", normRel)
	}
	info, err := os.Lstat(abs)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return apperr.New(apperr.CodeDirDeleteDisabled, "directory deletion is disabled in the file browser for safety")
	}
	return os.Remove(abs)
}

func (s *FileService) DownloadPath(clusterID, relPath string) (string, string, error) {
	abs, normRel, err := s.resolveSafePath(clusterID, relPath)
	if err != nil {
		return "", "", err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", "", err
	}
	if info.IsDir() {
		return "", "", apperr.New(apperr.CodeCannotDownloadDir, "cannot download a directory directly")
	}
	return abs, filepath.Base(normRel), nil
}
