drop index if exists bills_due_date_idx;
drop index if exists bills_series_due_idx;
alter table bills add column recurring boolean not null default false;
alter table bills drop column transaction_id;
alter table bills drop column payment_method;
alter table bills drop column amount_estimated;
alter table bills drop column series_id;
