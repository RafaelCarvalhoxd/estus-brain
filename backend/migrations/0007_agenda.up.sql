-- Agenda: local events with optional two-way sync to Google Calendar.

create table events (
    id              uuid primary key default gen_random_uuid(),
    title           text not null,
    location        text not null default '',
    notes           text not null default '',
    starts_at       timestamptz not null,
    ends_at         timestamptz not null,
    google_event_id text unique,
    created_at      timestamptz not null default now()
);

create index events_starts_at_idx on events (starts_at);

-- Single row in practice: one connected Google account per install.
create table google_oauth_tokens (
    id            uuid primary key default gen_random_uuid(),
    access_token  text not null,
    refresh_token text not null,
    expiry        timestamptz not null,
    created_at    timestamptz not null default now()
);
