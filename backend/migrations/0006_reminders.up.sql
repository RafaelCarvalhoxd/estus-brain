create table reminders (
    id          uuid primary key default gen_random_uuid(),
    title       text not null,
    due_at      timestamptz,
    done        boolean not null default false,
    created_at  timestamptz not null default now()
);
