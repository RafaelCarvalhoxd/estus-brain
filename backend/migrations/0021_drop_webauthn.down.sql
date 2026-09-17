create table webauthn_credentials (
    id                uuid primary key default gen_random_uuid(),
    credential_id     bytea not null unique,
    public_key        bytea not null,
    attestation_type  text not null default '',
    transports        text[] not null default '{}',
    sign_count        bigint not null default 0,
    created_at        timestamptz not null default now()
);
