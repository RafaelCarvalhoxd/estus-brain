-- Bills payable/receivable: scheduled obligations tracked independently of
-- the transactions ledger. Status (pendente/atrasado/pago/recebido) is
-- derived in application code from due_date and paid_at, never stored.

create table bills (
    id            uuid primary key default gen_random_uuid(),
    description   text not null,
    amount_cents  bigint not null check (amount_cents > 0),
    due_date      date not null,
    direction     text not null check (direction in ('pagar', 'receber')),
    category_id   uuid references categories(id),
    paid_at       timestamptz,
    recurring     boolean not null default false,
    created_at    timestamptz not null default now()
);

create index bills_direction_paid_at_idx on bills (direction, paid_at);
