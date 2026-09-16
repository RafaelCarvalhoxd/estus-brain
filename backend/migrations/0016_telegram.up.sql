-- Telegram: one bot, one owner chat, and what the bot sends on its own.
-- The token is encrypted with the vault key, like the assistant's API keys.
create table telegram_settings (
    id                    smallint primary key default 1 check (id = 1),
    token_secret          text not null default '',
    bot_username          text not null default '',
    chat_id               bigint,
    owner_name            text not null default '',
    pairing_code          text not null default '',
    pairing_expires_at    timestamptz,
    -- The last update_id handled + 1, so a restart doesn't replay messages.
    update_offset         bigint not null default 0,
    conversation_id       uuid references assistant_conversations(id) on delete set null,
    morning_enabled       boolean not null default true,
    morning_time          text not null default '07:00',
    evening_enabled       boolean not null default true,
    evening_time          text not null default '21:00',
    reminders_enabled     boolean not null default true,
    events_enabled        boolean not null default true,
    events_minutes_before integer not null default 30 check (events_minutes_before between 1 and 1440),
    last_morning_on       date,
    last_evening_on       date,
    updated_at            timestamptz not null default now()
);
insert into telegram_settings (id) values (1);

-- Alerts already sent. ref carries the item's time, so a rescheduled
-- reminder or event alerts again.
create table telegram_sent (
    kind    text not null,
    ref     text not null,
    sent_at timestamptz not null default now(),
    primary key (kind, ref)
);
