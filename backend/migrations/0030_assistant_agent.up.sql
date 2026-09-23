-- One engine left: an external agent. Its address and model live here and
-- its token in secrets (encrypted, like the API keys it replaces). The old
-- engines' keys, models and Ollama address go, and so does session_id,
-- which only the command-line engines resumed by.
alter table assistant_settings
    add column agent_url   text not null default '',
    add column agent_model text not null default '';
update assistant_settings
    set provider = case when provider = 'none' then 'none' else 'agent' end,
        secrets  = '{}'::jsonb;
alter table assistant_settings drop column models, drop column ollama_url;
alter table assistant_conversations drop column session_id;
