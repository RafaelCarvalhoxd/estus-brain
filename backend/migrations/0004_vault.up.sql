-- Vault de senhas: entries are encrypted at rest (AES-256-GCM, key held by
-- the server process, never in Postgres) and gated behind a WebAuthn
-- platform authenticator (Touch ID / Face ID) for reveal. There is exactly
-- one vault owner, so webauthn_credentials has no user_id column.

create table vault_entries (
    id                   uuid primary key default gen_random_uuid(),
    title                text not null,
    username             text not null default '',
    url                  text not null default '',
    notes                text not null default '',
    password_ciphertext  bytea not null,
    password_nonce       bytea not null,
    created_at           timestamptz not null default now(),
    updated_at           timestamptz not null default now()
);

create table webauthn_credentials (
    id                uuid primary key default gen_random_uuid(),
    credential_id     bytea not null unique,
    public_key        bytea not null,
    attestation_type  text not null default '',
    transports        text[] not null default '{}',
    sign_count        bigint not null default 0,
    created_at        timestamptz not null default now()
);
