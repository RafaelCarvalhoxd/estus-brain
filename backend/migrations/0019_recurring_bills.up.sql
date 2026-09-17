-- A recurring bill becomes a series of occurrences linked by series_id.
-- There is no template row: the mould for next month is the latest
-- occurrence, so correcting this month's amount is what defines the next.
alter table bills add column series_id uuid;
alter table bills add column amount_estimated boolean not null default false;
alter table bills add column payment_method text
    check (payment_method is null or payment_method in ('debito', 'credito', 'pix'));

-- The expense a settled bill created. Without it, undoing a payment would
-- have to guess which transaction to remove from description and date.
alter table bills add column transaction_id uuid references transactions(id);

-- recurring was decorative: nothing ever read it to generate anything.
-- series_id is not null replaces it. No data to convert — the table has been
-- empty since the owner cleared it to start using the app for real.
alter table bills drop column recurring;

create index bills_series_due_idx on bills (series_id, due_date) where series_id is not null;
create index bills_due_date_idx on bills (due_date);
