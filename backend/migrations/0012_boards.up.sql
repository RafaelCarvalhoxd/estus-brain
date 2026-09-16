-- Quadros: whiteboards drawn in the Excalidraw editor. The scene (elements,
-- a few view settings and pasted images) is stored as-is in jsonb — the
-- backend never needs to look inside it, only to keep it and hand it back.
-- The preview is an SVG the browser renders on save, so the list of boards
-- can show thumbnails without loading every scene.
create table boards (
    id         uuid primary key default gen_random_uuid(),
    name       text not null,
    scene      jsonb not null default '{}'::jsonb,
    preview    text not null default '',
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create index boards_updated_idx on boards (updated_at desc);
