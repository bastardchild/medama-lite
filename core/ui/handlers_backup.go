package ui

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/medama-io/medama/util/logger"
)

// handleBackupDownload packages SQLite metadata and DuckDB analytics databases
// into a single compressed ZIP archive and streams it to the user.
func (s *Server) handleBackupDownload(w http.ResponseWriter, r *http.Request) {
	log := logger.Get()

	tempDir, err := os.MkdirTemp("", "medama-backup-*")
	if err != nil {
		log.Error().Err(err).Msg("ui: failed to create temp dir for backup")
		s.renderError(w, r, http.StatusInternalServerError, "Failed to create backup workspace.")
		return
	}
	defer os.RemoveAll(tempDir)

	// 1. Snapshot SQLite database.
	sqliteBackupFile := filepath.Join(tempDir, "me_meta.db")
	_, err = s.appDB.ExecContext(r.Context(), "VACUUM INTO ?", sqliteBackupFile)
	if err != nil {
		log.Warn().Err(err).Msg("ui: VACUUM INTO failed, falling back to WAL checkpoint and file copy")
		_, _ = s.appDB.ExecContext(r.Context(), "PRAGMA wal_checkpoint(FULL);")
		if copyErr := copyFile(s.appDBPath, sqliteBackupFile); copyErr != nil {
			log.Error().Err(copyErr).Msg("ui: failed to copy sqlite db")
			s.renderError(w, r, http.StatusInternalServerError, "Failed to export application database.")
			return
		}
	}

	// 2. Snapshot DuckDB database.
	duckBackupFile := filepath.Join(tempDir, "me_analytics.db")
	_, _ = s.analytics.ExecContext(r.Context(), "CHECKPOINT;")
	if copyErr := copyFile(s.analyticsDBPath, duckBackupFile); copyErr != nil {
		log.Error().Err(copyErr).Msg("ui: failed to copy duckdb")
		s.renderError(w, r, http.StatusInternalServerError, "Failed to export analytics database.")
		return
	}

	// 3. Prepare ZIP response.
	timestamp := time.Now().UTC().Format("2006-01-02-150405")
	filename := fmt.Sprintf("medama-backup-%s.zip", timestamp)

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))

	zipWriter := zip.NewWriter(w)
	defer zipWriter.Close()

	if err := addFileToZip(zipWriter, "me_meta.db", sqliteBackupFile); err != nil {
		log.Error().Err(err).Msg("ui: failed to add sqlite db to backup zip")
		return
	}

	if err := addFileToZip(zipWriter, "me_analytics.db", duckBackupFile); err != nil {
		log.Error().Err(err).Msg("ui: failed to add duckdb to backup zip")
		return
	}

	// 4. Add manifest with metadata.
	manifest := map[string]any{
		"app":          "MedamaLite",
		"version":      s.version,
		"created_at":   time.Now().UTC().Format(time.RFC3339),
		"app_db":       "me_meta.db",
		"analytics_db": "me_analytics.db",
	}
	manifestBytes, _ := json.MarshalIndent(manifest, "", "  ")
	manifestWriter, err := zipWriter.Create("manifest.json")
	if err == nil {
		_, _ = manifestWriter.Write(manifestBytes)
	}

	log.Info().Str("filename", filename).Msg("ui: database backup created and downloaded")
}

// handleBackupRestore handles uploading and restoring a previous backup ZIP archive.
func (s *Server) handleBackupRestore(w http.ResponseWriter, r *http.Request) {
	log := logger.Get()

	if s.auth.IsDemoMode {
		s.renderFeedback(w, "#backup-feedback", false, "Database restore is disabled while demo mode is on.")
		return
	}

	// Limit uploaded backup size to 500 MB.
	r.Body = http.MaxBytesReader(w, r.Body, 500<<20)
	if err := r.ParseMultipartForm(500 << 20); err != nil {
		s.renderFeedback(w, "#backup-feedback", false, "The uploaded file is too large or could not be processed.")
		return
	}

	file, header, err := r.FormFile("backup_file")
	if err != nil {
		s.renderFeedback(w, "#backup-feedback", false, "Please choose a valid backup file (.zip).")
		return
	}
	defer file.Close()

	tempDir, err := os.MkdirTemp("", "medama-restore-*")
	if err != nil {
		s.renderFeedback(w, "#backup-feedback", false, "Failed to initialize restore environment.")
		return
	}
	defer os.RemoveAll(tempDir)

	uploadedPath := filepath.Join(tempDir, header.Filename)
	out, err := os.Create(uploadedPath)
	if err != nil {
		s.renderFeedback(w, "#backup-feedback", false, "Failed to store uploaded backup temporarily.")
		return
	}
	if _, err := io.Copy(out, file); err != nil {
		_ = out.Close()
		s.renderFeedback(w, "#backup-feedback", false, "Failed to write uploaded file.")
		return
	}
	_ = out.Close()

	// Extract or discover databases.
	var sqlitePath, duckdbPath string

	if strings.HasSuffix(strings.ToLower(header.Filename), ".zip") {
		sqlitePath, duckdbPath, err = extractBackupZip(uploadedPath, tempDir)
		if err != nil {
			log.Error().Err(err).Msg("ui: failed to unpack backup zip")
			s.renderFeedback(w, "#backup-feedback", false, "Invalid or corrupted backup ZIP archive.")
			return
		}
	} else if strings.HasSuffix(strings.ToLower(header.Filename), ".db") {
		// Single DB upload detection
		if isSQLiteFile(uploadedPath) {
			sqlitePath = uploadedPath
		} else {
			duckdbPath = uploadedPath
		}
	} else {
		s.renderFeedback(w, "#backup-feedback", false, "Unsupported file format. Please upload a .zip or .db file.")
		return
	}

	if sqlitePath == "" && duckdbPath == "" {
		s.renderFeedback(w, "#backup-feedback", false, "No valid MedamaLite database found inside the uploaded backup.")
		return
	}

	// 1. Restore SQLite if present.
	if sqlitePath != "" {
		if !isSQLiteFile(sqlitePath) {
			s.renderFeedback(w, "#backup-feedback", false, "Extracted SQLite file does not have a valid header.")
			return
		}

		if err := s.restoreSQLite(r.Context(), sqlitePath); err != nil {
			log.Error().Err(err).Msg("ui: failed to restore SQLite database")
			s.renderFeedback(w, "#backup-feedback", false, "Failed to restore application metadata.")
			return
		}
	}

	// 2. Restore DuckDB if present.
	if duckdbPath != "" {
		if err := s.restoreDuckDB(r.Context(), duckdbPath); err != nil {
			log.Error().Err(err).Msg("ui: failed to restore DuckDB database")
			s.renderFeedback(w, "#backup-feedback", false, "Failed to restore analytics database.")
			return
		}
	}

	log.Info().Msg("ui: database restore completed successfully")
	s.renderFeedback(w, "#backup-feedback", true, "Database successfully restored from backup! Please refresh the page to see changes.")
}

// restoreSQLite attaches the uploaded database and atomically copies all application tables.
func (s *Server) restoreSQLite(ctx context.Context, sourceFile string) error {
	log := logger.Get()
	cleanPath := filepath.ToSlash(sourceFile)

	attachSQL := fmt.Sprintf("ATTACH DATABASE '%s' AS uploaded_backup;", cleanPath)
	if _, err := s.appDB.ExecContext(ctx, attachSQL); err != nil {
		return err
	}
	defer func() {
		_, _ = s.appDB.ExecContext(context.Background(), "DETACH DATABASE uploaded_backup;")
	}()

	// Query available tables in uploaded database.
	rows, err := s.appDB.QueryContext(ctx, "SELECT name FROM uploaded_backup.sqlite_master WHERE type='table';")
	if err != nil {
		return err
	}
	tables := make(map[string]bool)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err == nil {
			tables[name] = true
		}
	}
	_ = rows.Close()

	tx, err := s.appDB.Beginx()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	// Temporarily disable foreign keys during bulk table swap.
	if _, err := tx.Exec("PRAGMA foreign_keys = OFF;"); err != nil {
		return err
	}

	if tables["tenant_settings"] {
		if _, err := tx.Exec("DELETE FROM main.tenant_settings; INSERT INTO main.tenant_settings SELECT * FROM uploaded_backup.tenant_settings;"); err != nil {
			log.Warn().Err(err).Msg("ui: failed copying tenant_settings table")
		}
	}

	if tables["users"] {
		if _, err := tx.Exec("DELETE FROM main.users; INSERT INTO main.users SELECT * FROM uploaded_backup.users;"); err != nil {
			log.Warn().Err(err).Msg("ui: failed copying users table")
		}
	}

	if tables["websites"] {
		if _, err := tx.Exec("DELETE FROM main.websites; INSERT INTO main.websites SELECT * FROM uploaded_backup.websites;"); err != nil {
			log.Warn().Err(err).Msg("ui: failed copying websites table")
		}
	}

	if _, err := tx.Exec("PRAGMA foreign_keys = ON;"); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	// Reload website hostnames cache so the ingestion engine stays in sync.
	_ = s.svc.ReloadHostnames(ctx)

	return nil
}

// restoreDuckDB attaches the uploaded DuckDB database and atomically copies views and events.
func (s *Server) restoreDuckDB(ctx context.Context, sourceFile string) error {
	log := logger.Get()
	cleanPath := filepath.ToSlash(sourceFile)

	attachSQL := fmt.Sprintf("ATTACH '%s' AS uploaded_duck (READ_ONLY);", cleanPath)
	if _, err := s.analytics.ExecContext(ctx, attachSQL); err != nil {
		return err
	}
	defer func() {
		_, _ = s.analytics.ExecContext(context.Background(), "DETACH uploaded_duck;")
	}()

	// Check if views table exists.
	var countViews int
	_ = s.analytics.QueryRowContext(ctx, "SELECT count(*) FROM information_schema.tables WHERE table_schema='uploaded_duck' AND table_name='views';").Scan(&countViews)
	if countViews > 0 {
		tx, err := s.analytics.Beginx()
		if err == nil {
			_, _ = tx.Exec("DELETE FROM main.views;")
			_, _ = tx.Exec("INSERT INTO main.views SELECT * FROM uploaded_duck.views;")
			if err := tx.Commit(); err != nil {
				log.Warn().Err(err).Msg("ui: failed committing duckdb views table")
			}
		}
	}

	// Check if events table exists.
	var countEvents int
	_ = s.analytics.QueryRowContext(ctx, "SELECT count(*) FROM information_schema.tables WHERE table_schema='uploaded_duck' AND table_name='events';").Scan(&countEvents)
	if countEvents > 0 {
		tx, err := s.analytics.Beginx()
		if err == nil {
			_, _ = tx.Exec("DELETE FROM main.events;")
			_, _ = tx.Exec("INSERT INTO main.events SELECT * FROM uploaded_duck.events;")
			if err := tx.Commit(); err != nil {
				log.Warn().Err(err).Msg("ui: failed committing duckdb events table")
			}
		}
	}

	_, _ = s.analytics.ExecContext(ctx, "CHECKPOINT;")

	return nil
}

// Helper utilities for file extraction and zip creation.

func copyFile(src, dst string) error {
	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destination.Close()

	_, err = io.Copy(destination, source)
	return err
}

func addFileToZip(zw *zip.Writer, entryName, filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return err
	}

	header, err := zip.FileInfoHeader(stat)
	if err != nil {
		return err
	}
	header.Name = entryName
	header.Method = zip.Deflate

	writer, err := zw.CreateHeader(header)
	if err != nil {
		return err
	}

	_, err = io.Copy(writer, file)
	return err
}

func isSQLiteFile(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	magic := make([]byte, 16)
	n, err := f.Read(magic)
	if err != nil || n < 16 {
		return false
	}

	return string(magic) == "SQLite format 3\x00"
}

func extractBackupZip(zipPath, targetDir string) (sqliteFile, duckdbFile string, err error) {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", "", err
	}
	defer r.Close()

	for _, f := range r.File {
		// Prevent Zip Slip vulnerability.
		cleanName := filepath.Clean(f.Name)
		if strings.Contains(cleanName, "..") {
			continue
		}

		destPath := filepath.Join(targetDir, filepath.Base(cleanName))
		if f.FileInfo().IsDir() {
			continue
		}

		outFile, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return "", "", err
		}

		rc, err := f.Open()
		if err != nil {
			_ = outFile.Close()
			return "", "", err
		}

		_, err = io.Copy(outFile, rc)
		_ = rc.Close()
		_ = outFile.Close()
		if err != nil {
			return "", "", err
		}

		lower := strings.ToLower(filepath.Base(cleanName))
		if lower == "me_meta.db" || isSQLiteFile(destPath) {
			sqliteFile = destPath
		} else if lower == "me_analytics.db" || strings.HasSuffix(lower, ".db") {
			duckdbFile = destPath
		}
	}

	return sqliteFile, duckdbFile, nil
}
