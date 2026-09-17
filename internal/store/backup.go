package store

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const keepBackups = 14

var backupName = regexp.MustCompile(`^[0-9]{8}-[0-9]{6}(-pre-restore)?\.db$`)

type BackupInfo struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
	At   string `json:"at"`
}

func (s *Store) backupsDir() string {
	return filepath.Join(filepath.Dir(s.path), "backups")
}

func ValidBackupName(name string) bool {
	return backupName.MatchString(filepath.Base(name))
}

func (s *Store) Backup() (BackupInfo, error) {
	if s.path == "" {
		return BackupInfo{}, fmt.Errorf("no database path")
	}
	dir := s.backupsDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return BackupInfo{}, err
	}
	name := time.Now().UTC().Format("20060102-150405") + ".db"
	dest := filepath.Join(dir, name)
	if _, err := s.db.Exec(`VACUUM INTO ?`, dest); err != nil {
		return BackupInfo{}, err
	}
	_ = os.Chmod(dest, 0o600)
	if err := pruneBackups(dir); err != nil {
		return BackupInfo{}, err
	}
	st, err := os.Stat(dest)
	if err != nil {
		return BackupInfo{}, err
	}
	return BackupInfo{Name: name, Size: st.Size(), At: st.ModTime().UTC().Format(time.RFC3339)}, nil
}

func (s *Store) ListBackups() ([]BackupInfo, error) {
	return listBackups(s.backupsDir())
}

func (s *Store) StageRestore(name string) error {
	name = filepath.Base(strings.TrimSpace(name))
	if !backupName.MatchString(name) {
		return fmt.Errorf("invalid backup name")
	}
	src := filepath.Join(s.backupsDir(), name)
	st, err := os.Stat(src)
	if err != nil || st.IsDir() {
		return fmt.Errorf("backup not found")
	}
	next := s.path + ".next"
	if err := copyFile(src, next); err != nil {
		return err
	}
	return os.Chmod(next, 0o600)
}

func applyStagedRestore(path string) error {
	next := path + ".next"
	st, err := os.Stat(next)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if st.IsDir() {
		return fmt.Errorf("staged restore is a directory")
	}
	dir := filepath.Join(filepath.Dir(path), "backups")
	if _, err := os.Stat(path); err == nil {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return err
		}
		dest := filepath.Join(dir, time.Now().UTC().Format("20060102-150405")+"-pre-restore.db")
		if err := copyFile(path, dest); err != nil {
			return err
		}
		_ = os.Chmod(dest, 0o600)
	}
	_ = os.Remove(path + "-wal")
	_ = os.Remove(path + "-shm")
	if err := os.Rename(next, path); err != nil {
		return err
	}
	_ = os.Remove(path + "-wal")
	_ = os.Remove(path + "-shm")
	return nil
}

func listBackups(dir string) ([]BackupInfo, error) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []BackupInfo{}, nil
		}
		return nil, err
	}
	var out []BackupInfo
	for _, e := range ents {
		if e.IsDir() || !backupName.MatchString(e.Name()) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		out = append(out, BackupInfo{Name: e.Name(), Size: info.Size(), At: info.ModTime().UTC().Format(time.RFC3339)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name > out[j].Name })
	if out == nil {
		out = []BackupInfo{}
	}
	return out, nil
}

func pruneBackups(dir string) error {
	list, err := listBackups(dir)
	if err != nil {
		return err
	}
	for i := keepBackups; i < len(list); i++ {
		_ = os.Remove(filepath.Join(dir, list[i].Name))
	}
	return nil
}

func copyFile(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}
