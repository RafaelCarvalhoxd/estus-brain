-- Documents live in two places on purpose: the bytes go to a directory on
-- the server's disk (DOCUMENTS_DIR), and only the metadata lives here. A
-- personal archive is mostly PDFs and scans; putting them in Postgres would
-- bloat every backup of a database that is otherwise tiny, and the files are
-- already protected by the same login as everything else.
create table document_folders (
    id         uuid primary key default gen_random_uuid(),
    parent_id  uuid references document_folders(id) on delete restrict,
    name       text not null,
    created_at timestamptz not null default now()
);

-- Two folders can share a name only if they sit in different parents, the
-- way a filesystem works. The coalesce gives root-level folders (parent_id
-- null) a stable key to be unique against, since null never equals null.
create unique index document_folders_unique_name
    on document_folders (coalesce(parent_id, '00000000-0000-0000-0000-000000000000'::uuid), lower(name));

create table documents (
    id           uuid primary key default gen_random_uuid(),
    folder_id    uuid references document_folders(id) on delete restrict,
    name         text not null,
    -- File name on disk: a uuid keeps uploads from colliding or escaping
    -- the storage directory, with the readable name kept alongside it so
    -- the folder is still browsable by a human over ssh.
    stored_name  text not null unique,
    content_type text not null,
    size_bytes   bigint not null,
    created_at   timestamptz not null default now()
);

create index documents_folder_idx on documents (folder_id);
