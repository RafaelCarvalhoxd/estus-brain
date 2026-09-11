-- Notas: freeform personal notes, pinned ones surfaced first.

create table notes (
    id         uuid primary key default gen_random_uuid(),
    title      text not null default '',
    body       text not null default '',
    pinned     boolean not null default false,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create index notes_pinned_updated_idx on notes (pinned desc, updated_at desc);
