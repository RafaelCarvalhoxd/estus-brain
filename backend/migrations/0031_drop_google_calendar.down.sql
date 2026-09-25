-- Restores the schema empty; the links to Google and the token are gone.
alter table events add column google_event_id text unique;
create table google_oauth_tokens (
    id            uuid primary key default gen_random_uuid(),
    access_token  text not null,
    refresh_token text not null,
    expiry        timestamptz not null,
    created_at    timestamptz not null default now()
);
