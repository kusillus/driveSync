package sqlite

import (
	"context"
	"database/sql"
	"time"

	"driveSync/internal/domain"

	_ "modernc.org/sqlite"
)

type SqliteMetadataRepository struct {
	db *sql.DB
}

// NewSqliteRepository opens/creates the SQLite database and runs migrations
func NewSqliteRepository(dbPath string) (*SqliteMetadataRepository, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	// Ping database to ensure connection works
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}

	repo := &SqliteMetadataRepository{db: db}
	if err := repo.migrate(); err != nil {
		db.Close()
		return nil, err
	}

	return repo, nil
}

// Close closes the database connection
func (r *SqliteMetadataRepository) Close() error {
	return r.db.Close()
}

func (r *SqliteMetadataRepository) migrate() error {
	query := `
	CREATE TABLE IF NOT EXISTS file_metadata (
		path TEXT PRIMARY KEY,
		size INTEGER NOT NULL,
		mtime DATETIME NOT NULL,
		drive_id TEXT NOT NULL DEFAULT '',
		status TEXT NOT NULL,
		last_uploaded_at DATETIME,
		last_synced_at DATETIME
	);`
	_, err := r.db.Exec(query)
	return err
}

func (r *SqliteMetadataRepository) Save(ctx context.Context, meta *domain.FileMetadata) error {
	query := `
	INSERT INTO file_metadata (path, size, mtime, drive_id, status, last_uploaded_at, last_synced_at)
	VALUES (?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(path) DO UPDATE SET
		size = excluded.size,
		mtime = excluded.mtime,
		drive_id = excluded.drive_id,
		status = excluded.status,
		last_uploaded_at = excluded.last_uploaded_at,
		last_synced_at = excluded.last_synced_at;
	`
	// Handle nil time values gracefully for SQLite
	var lastUploaded, lastSynced interface{}
	if !meta.LastUploadedAt.IsZero() {
		lastUploaded = meta.LastUploadedAt
	}
	if !meta.LastSyncedAt.IsZero() {
		lastSynced = meta.LastSyncedAt
	}

	_, err := r.db.ExecContext(ctx, query,
		meta.Path,
		meta.Size,
		meta.MTime,
		meta.DriveID,
		string(meta.Status),
		lastUploaded,
		lastSynced,
	)
	return err
}

func (r *SqliteMetadataRepository) FindByPath(ctx context.Context, path string) (*domain.FileMetadata, error) {
	query := `
	SELECT path, size, mtime, drive_id, status, last_uploaded_at, last_synced_at
	FROM file_metadata
	WHERE path = ?;
	`
	var meta domain.FileMetadata
	var statusStr string
	var lastUploadedNull, lastSyncedNull sql.NullTime

	err := r.db.QueryRowContext(ctx, query, path).Scan(
		&meta.Path,
		&meta.Size,
		&meta.MTime,
		&meta.DriveID,
		&statusStr,
		&lastUploadedNull,
		&lastSyncedNull,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	meta.Status = domain.FileStatus(statusStr)
	if lastUploadedNull.Valid {
		meta.LastUploadedAt = lastUploadedNull.Time
	}
	if lastSyncedNull.Valid {
		meta.LastSyncedAt = lastSyncedNull.Time
	}

	return &meta, nil
}

func (r *SqliteMetadataRepository) UpdateStatus(ctx context.Context, path string, status domain.FileStatus, driveID string) error {
	var query string
	var err error

	now := time.Now()

	if status == domain.StatusSynced {
		query = `
		UPDATE file_metadata
		SET status = ?, drive_id = ?, last_uploaded_at = ?, last_synced_at = ?
		WHERE path = ?;
		`
		_, err = r.db.ExecContext(ctx, query, string(status), driveID, now, now, path)
	} else {
		query = `
		UPDATE file_metadata
		SET status = ?, drive_id = ?, last_synced_at = ?
		WHERE path = ?;
		`
		_, err = r.db.ExecContext(ctx, query, string(status), driveID, now, path)
	}

	return err
}

func (r *SqliteMetadataRepository) ListPending(ctx context.Context) ([]*domain.FileMetadata, error) {
	query := `
	SELECT path, size, mtime, drive_id, status, last_uploaded_at, last_synced_at
	FROM file_metadata
	WHERE status = 'pending';
	`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*domain.FileMetadata
	for rows.Next() {
		var meta domain.FileMetadata
		var statusStr string
		var lastUploadedNull, lastSyncedNull sql.NullTime

		err := rows.Scan(
			&meta.Path,
			&meta.Size,
			&meta.MTime,
			&meta.DriveID,
			&statusStr,
			&lastUploadedNull,
			&lastSyncedNull,
		)
		if err != nil {
			return nil, err
		}

		meta.Status = domain.FileStatus(statusStr)
		if lastUploadedNull.Valid {
			meta.LastUploadedAt = lastUploadedNull.Time
		}
		if lastSyncedNull.Valid {
			meta.LastSyncedAt = lastSyncedNull.Time
		}
		list = append(list, &meta)
	}

	return list, nil
}

func (r *SqliteMetadataRepository) ListAll(ctx context.Context) ([]*domain.FileMetadata, error) {
	query := `
	SELECT path, size, mtime, drive_id, status, last_uploaded_at, last_synced_at
	FROM file_metadata;
	`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*domain.FileMetadata
	for rows.Next() {
		var meta domain.FileMetadata
		var statusStr string
		var lastUploadedNull, lastSyncedNull sql.NullTime

		err := rows.Scan(
			&meta.Path,
			&meta.Size,
			&meta.MTime,
			&meta.DriveID,
			&statusStr,
			&lastUploadedNull,
			&lastSyncedNull,
		)
		if err != nil {
			return nil, err
		}

		meta.Status = domain.FileStatus(statusStr)
		if lastUploadedNull.Valid {
			meta.LastUploadedAt = lastUploadedNull.Time
		}
		if lastSyncedNull.Valid {
			meta.LastSyncedAt = lastSyncedNull.Time
		}
		list = append(list, &meta)
	}

	return list, nil
}
