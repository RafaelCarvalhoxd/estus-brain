-- Investimentos: cada aporte é dinheiro que saiu da conta no dia em que foi
-- feito, então entra nas saídas do mês e baixa o saldo.
create table investments (
    id          uuid primary key default gen_random_uuid(),
    name        text not null,
    kind        text not null,
    -- Rendimento em centésimos de ponto percentual: 1% = 100.
    rate_bp     integer not null default 0 check (rate_bp >= 0),
    rate_period text not null default 'mes' check (rate_period in ('mes', 'ano')),
    created_at  timestamptz not null default now()
);

create table investment_contributions (
    id            uuid primary key default gen_random_uuid(),
    investment_id uuid not null references investments (id) on delete cascade,
    amount_cents  bigint not null check (amount_cents > 0),
    contributed_on date not null,
    created_at    timestamptz not null default now()
);

create index investment_contributions_on_idx on investment_contributions (contributed_on);
create index investment_contributions_investment_idx on investment_contributions (investment_id);
