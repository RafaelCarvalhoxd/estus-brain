-- Note categories are deliberately separate from the finance `categories`
-- table: "Trabalho"/"Pessoal"/"Ideias" has nothing to do with "Alimentação"/
-- "Transporte", and coupling the two would mean a delete in one domain
-- accidentally threatening data in the other.
create table note_categories (
    id         uuid primary key default gen_random_uuid(),
    name       text not null unique,
    color      text not null,
    created_at timestamptz not null default now()
);

-- Nullable on purpose: a note with no category is "Geral" — a virtual
-- bucket the frontend renders for null, not a row that has to exist and be
-- protected from deletion.
alter table notes add column category_id uuid references note_categories(id);
