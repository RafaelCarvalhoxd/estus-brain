package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/rafael/estus-vault/backend/internal/domain"
)

type DocumentFolderRepo struct{ db *DB }

func NewDocumentFolderRepo(db *DB) *DocumentFolderRepo { return &DocumentFolderRepo{db: db} }

func scanDocumentFolder(row scanner, f *domain.DocumentFolder) error {
	return row.Scan(&f.ID, &f.ParentID, &f.Name, &f.CreatedAt)
}

// List returns the whole tree in one query — a personal archive is a few
// dozen folders at most, so the frontend can build the breadcrumb and the
// children of any folder without a round trip per level.
func (r *DocumentFolderRepo) List(ctx context.Context) ([]domain.DocumentFolder, error) {
	rows, err := r.db.Pool.Query(ctx, `
		select id, parent_id, name, created_at
		from document_folders
		order by name`)
	if err != nil {
		return nil, fmt.Errorf("list document folders: %w", err)
	}
	defer rows.Close()

	var out []domain.DocumentFolder
	for rows.Next() {
		var f domain.DocumentFolder
		if err := scanDocumentFolder(rows, &f); err != nil {
			return nil, fmt.Errorf("scan document folder: %w", err)
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func (r *DocumentFolderRepo) Get(ctx context.Context, id string) (domain.DocumentFolder, error) {
	row := r.db.Pool.QueryRow(ctx, `select id, parent_id, name, created_at from document_folders where id = $1`, id)
	var f domain.DocumentFolder
	if err := scanDocumentFolder(row, &f); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.DocumentFolder{}, domain.ErrNotFound
		}
		return domain.DocumentFolder{}, fmt.Errorf("get document folder: %w", err)
	}
	return f, nil
}

func (r *DocumentFolderRepo) Create(ctx context.Context, f domain.DocumentFolder) (domain.DocumentFolder, error) {
	row := r.db.Pool.QueryRow(ctx, `
		insert into document_folders (id, parent_id, name)
		values (gen_random_uuid(), $1, $2)
		returning id, parent_id, name, created_at`,
		f.ParentID, f.Name,
	)
	var out domain.DocumentFolder
	if err := scanDocumentFolder(row, &out); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Code {
			case pgUniqueViolation:
				return domain.DocumentFolder{}, fmt.Errorf("%w: a folder named %q already exists here", domain.ErrConflict, f.Name)
			case pgForeignKeyViolation:
				return domain.DocumentFolder{}, fmt.Errorf("%w: parent folder does not exist", domain.ErrValidation)
			}
		}
		return domain.DocumentFolder{}, fmt.Errorf("create document folder: %w", err)
	}
	return out, nil
}

func (r *DocumentFolderRepo) Rename(ctx context.Context, id, name string) (domain.DocumentFolder, error) {
	row := r.db.Pool.QueryRow(ctx, `
		update document_folders set name = $2 where id = $1
		returning id, parent_id, name, created_at`, id, name)
	var out domain.DocumentFolder
	if err := scanDocumentFolder(row, &out); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.DocumentFolder{}, domain.ErrNotFound
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
			return domain.DocumentFolder{}, fmt.Errorf("%w: a folder named %q already exists here", domain.ErrConflict, name)
		}
		return domain.DocumentFolder{}, fmt.Errorf("rename document folder: %w", err)
	}
	return out, nil
}

// Counts reports what a folder still holds, so the service can refuse to
// delete one that isn't empty instead of orphaning files on disk.
func (r *DocumentFolderRepo) Counts(ctx context.Context, id string) (subfolders, documents int, err error) {
	err = r.db.Pool.QueryRow(ctx, `
		select
			(select count(*) from document_folders where parent_id = $1),
			(select count(*) from documents where folder_id = $1)`, id,
	).Scan(&subfolders, &documents)
	if err != nil {
		return 0, 0, fmt.Errorf("count folder contents: %w", err)
	}
	return subfolders, documents, nil
}

func (r *DocumentFolderRepo) Delete(ctx context.Context, id string) error {
	tag, err := r.db.Pool.Exec(ctx, `delete from document_folders where id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete document folder: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

type DocumentRepo struct{ db *DB }

func NewDocumentRepo(db *DB) *DocumentRepo { return &DocumentRepo{db: db} }

func scanDocument(row scanner, d *domain.Document) error {
	return row.Scan(&d.ID, &d.FolderID, &d.Name, &d.StoredName, &d.ContentType, &d.SizeBytes, &d.CreatedAt)
}

const documentColumns = `id, folder_id, name, stored_name, content_type, size_bytes, created_at`

// List returns the documents directly inside one folder; a nil folderID
// means the root.
func (r *DocumentRepo) List(ctx context.Context, folderID *string) ([]domain.Document, error) {
	rows, err := r.db.Pool.Query(ctx, `
		select `+documentColumns+`
		from documents
		where folder_id is not distinct from $1
		order by name`, folderID)
	if err != nil {
		return nil, fmt.Errorf("list documents: %w", err)
	}
	defer rows.Close()

	var out []domain.Document
	for rows.Next() {
		var d domain.Document
		if err := scanDocument(rows, &d); err != nil {
			return nil, fmt.Errorf("scan document: %w", err)
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (r *DocumentRepo) Count(ctx context.Context) (int, error) {
	var n int
	if err := r.db.Pool.QueryRow(ctx, `select count(*) from documents`).Scan(&n); err != nil {
		return 0, fmt.Errorf("count documents: %w", err)
	}
	return n, nil
}

func (r *DocumentRepo) Get(ctx context.Context, id string) (domain.Document, error) {
	row := r.db.Pool.QueryRow(ctx, `select `+documentColumns+` from documents where id = $1`, id)
	var d domain.Document
	if err := scanDocument(row, &d); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Document{}, domain.ErrNotFound
		}
		return domain.Document{}, fmt.Errorf("get document: %w", err)
	}
	return d, nil
}

func (r *DocumentRepo) Create(ctx context.Context, d domain.Document) (domain.Document, error) {
	row := r.db.Pool.QueryRow(ctx, `
		insert into documents (id, folder_id, name, stored_name, content_type, size_bytes)
		values (gen_random_uuid(), $1, $2, $3, $4, $5)
		returning `+documentColumns,
		d.FolderID, d.Name, d.StoredName, d.ContentType, d.SizeBytes,
	)
	var out domain.Document
	if err := scanDocument(row, &out); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgForeignKeyViolation {
			return domain.Document{}, fmt.Errorf("%w: folder does not exist", domain.ErrValidation)
		}
		return domain.Document{}, fmt.Errorf("create document: %w", err)
	}
	return out, nil
}

// Move changes both the folder and the display name in one statement — the
// two things about a stored file that can change after upload.
func (r *DocumentRepo) Move(ctx context.Context, id string, folderID *string, name string) (domain.Document, error) {
	row := r.db.Pool.QueryRow(ctx, `
		update documents set folder_id = $2, name = $3 where id = $1
		returning `+documentColumns, id, folderID, name)
	var out domain.Document
	if err := scanDocument(row, &out); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Document{}, domain.ErrNotFound
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgForeignKeyViolation {
			return domain.Document{}, fmt.Errorf("%w: folder does not exist", domain.ErrValidation)
		}
		return domain.Document{}, fmt.Errorf("move document: %w", err)
	}
	return out, nil
}

func (r *DocumentRepo) Delete(ctx context.Context, id string) error {
	tag, err := r.db.Pool.Exec(ctx, `delete from documents where id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete document: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}
