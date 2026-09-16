-- Training and diet share a shape: a plan made of named blocks (a workout,
-- a meal), each scheduled on some days of the week and made of ordered
-- lines (exercises, foods).
--
-- Days of the week are a 7-bit mask, bit 0 = Sunday … bit 6 = Saturday —
-- the same numbering as JavaScript's Date.getDay(). A block with no day at
-- all would never show up anywhere, so at least one bit is required.

create table workouts (
    id         uuid primary key default gen_random_uuid(),
    name       text not null,
    focus      text not null default '',
    weekdays   smallint not null check (weekdays between 1 and 127),
    notes      text not null default '',
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

-- Exercises are owned by their workout and rewritten whole on every edit,
-- so position is just the order they were listed in.
create table workout_exercises (
    id           uuid primary key default gen_random_uuid(),
    workout_id   uuid not null references workouts(id) on delete cascade,
    position     int not null,
    name         text not null,
    sets         int not null check (sets between 1 and 50),
    reps         text not null default '',
    weight       text not null default '',
    rest_seconds int not null default 0 check (rest_seconds between 0 and 3600),
    notes        text not null default ''
);

create index workout_exercises_workout_idx on workout_exercises (workout_id, position);

create table meals (
    id          uuid primary key default gen_random_uuid(),
    name        text not null,
    -- "HH:MM", local time. Text rather than time: it's a label on a plan,
    -- compared only against the clock of the day it's shown on.
    time_of_day text not null check (time_of_day ~ '^([01][0-9]|2[0-3]):[0-5][0-9]$'),
    weekdays    smallint not null check (weekdays between 1 and 127),
    notes       text not null default '',
    created_at  timestamptz not null default now(),
    updated_at  timestamptz not null default now()
);

-- Macros are per item as served (the quantity already applied), so a meal's
-- totals are a plain sum.
create table meal_items (
    id        uuid primary key default gen_random_uuid(),
    meal_id   uuid not null references meals(id) on delete cascade,
    position  int not null,
    food      text not null,
    quantity  text not null default '',
    kcal      double precision not null default 0 check (kcal >= 0),
    protein_g double precision not null default 0 check (protein_g >= 0),
    carbs_g   double precision not null default 0 check (carbs_g >= 0),
    fat_g     double precision not null default 0 check (fat_g >= 0)
);

create index meal_items_meal_idx on meal_items (meal_id, position);

-- One row, always: the daily targets the diet screen measures against.
-- Zero means "no target set" for that macro.
create table diet_targets (
    id         smallint primary key default 1 check (id = 1),
    kcal       double precision not null default 0 check (kcal >= 0),
    protein_g  double precision not null default 0 check (protein_g >= 0),
    carbs_g    double precision not null default 0 check (carbs_g >= 0),
    fat_g      double precision not null default 0 check (fat_g >= 0),
    updated_at timestamptz not null default now()
);

insert into diet_targets (id) values (1);
