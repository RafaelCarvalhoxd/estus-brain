-- Estus Vault schema.
--
-- Money is stored as bigint cents everywhere: numeric/decimal round-trips
-- through the Go pgx driver as a string type that's easy to mis-scan, and
-- float is a non-starter for a ledger. Application code is the single
-- source of truth for currency formatting.

create extension if not exists "pgcrypto";

create table categories (
    id          uuid primary key default gen_random_uuid(),
    name        text not null unique,
    nature      text not null check (nature in ('essencial', 'variavel', 'investimento')),
    color       text not null, -- hex, matches the dashboard's category palette
    created_at  timestamptz not null default now()
);

create table credit_cards (
    id          uuid primary key default gen_random_uuid(),
    name        text not null,
    closing_day smallint not null check (closing_day between 1 and 28),
    due_day     smallint not null check (due_day between 1 and 28),
    created_at  timestamptz not null default now()
);

create table transactions (
    id                   uuid primary key default gen_random_uuid(),
    description          text not null,
    amount_cents         bigint not null check (amount_cents > 0),
    category_id          uuid not null references categories(id),
    payment_method       text not null check (payment_method in ('debito', 'credito', 'pix')),
    purchase_date        date not null,
    credit_card_id       uuid references credit_cards(id),
    -- first day of the month this expense counts against in the dashboard;
    -- computed once at write time by domain.CompetenceMonth so every query
    -- that aggregates by month is a plain group-by with no date math.
    competence_month     date not null,
    installment_group_id uuid,
    installment_number   smallint not null default 0,
    installment_total    smallint not null default 1,
    is_recurring         boolean not null default false,
    created_at           timestamptz not null default now(),
    check ((payment_method = 'credito') = (credit_card_id is not null)),
    check (date_trunc('month', competence_month) = competence_month)
);

create index transactions_competence_month_idx on transactions (competence_month);
create index transactions_category_idx on transactions (category_id);
create index transactions_installment_group_idx on transactions (installment_group_id) where installment_group_id is not null;
