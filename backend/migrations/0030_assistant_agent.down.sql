-- Restores the columns empty; the dropped keys and sessions are gone.
alter table assistant_conversations add column session_id text not null default '';
alter table assistant_settings
    add column models     jsonb not null default '{}'::jsonb,
    add column ollama_url text  not null default 'http://127.0.0.1:11434';
alter table assistant_settings drop column agent_url, drop column agent_model;
