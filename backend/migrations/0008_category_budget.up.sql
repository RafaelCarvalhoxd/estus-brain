-- A monthly spending target per category, shown against the actual total in
-- the category pie chart. Null means "no budget set" — not the same as a
-- budget of zero, so it has to be nullable rather than defaulting to 0.
alter table categories add column monthly_budget_cents bigint check (monthly_budget_cents > 0);
