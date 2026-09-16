-- The assistant: its conversations (whatever engine answered them) and the
-- owner's engine settings. API keys are stored encrypted with the vault key.
create table assistant_settings (
    id          smallint primary key default 1 check (id = 1),
    provider    text not null default 'none',
    models      jsonb not null default '{}'::jsonb,
    secrets     jsonb not null default '{}'::jsonb,
    ollama_url  text not null default 'http://127.0.0.1:11434',
    updated_at  timestamptz not null default now()
);
insert into assistant_settings (id) values (1);

create table assistant_conversations (
    id         uuid primary key default gen_random_uuid(),
    title      text not null default '',
    module     text not null default '',
    -- The engine the conversation last used, and that engine's own session
    -- id when it keeps one (Claude Code and Codex resume by it).
    provider   text not null default '',
    session_id text not null default '',
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);
create index assistant_conversations_updated_idx on assistant_conversations (updated_at desc);

create table assistant_messages (
    id              uuid primary key default gen_random_uuid(),
    conversation_id uuid not null references assistant_conversations(id) on delete cascade,
    role            text not null check (role in ('user', 'assistant')),
    content         text not null default '',
    -- Tool calls made while answering, and cards a ready-made flow rendered.
    data            jsonb not null default '{}'::jsonb,
    provider        text not null default '',
    created_at      timestamptz not null default now()
);
create index assistant_messages_conversation_idx on assistant_messages (conversation_id, created_at);
