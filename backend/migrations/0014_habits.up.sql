-- Hábitos: things done daily (or on some weekdays), either ticked off
-- ("ler") or counted toward a target ("8 copos de água").
create table habits (
    id         uuid primary key default gen_random_uuid(),
    name       text not null,
    kind       text not null check (kind in ('check', 'count')),
    target     integer not null default 1 check (target between 1 and 1000),
    unit       text not null default '',
    weekdays   smallint not null default 127 check (weekdays between 1 and 127),
    color      text not null,
    archived   boolean not null default false,
    -- The first day the habit counts, in the owner's time zone — streaks never
    -- reach back before it.
    start_day  date not null,
    created_at timestamptz not null default now()
);

-- One row per habit per day it was (at least partly) done; a day with no row
-- is a day with nothing logged.
create table habit_logs (
    habit_id uuid not null references habits(id) on delete cascade,
    day      date not null,
    count    integer not null check (count between 1 and 10000),
    primary key (habit_id, day)
);
