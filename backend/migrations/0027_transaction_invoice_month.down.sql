drop index transactions_invoice_idx;
update transactions set competence_month = invoice_month where invoice_month is not null;
alter table transactions drop column invoice_month;
