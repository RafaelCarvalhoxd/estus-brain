-- A card purchase used to count in the month its invoice is due. Now it
-- counts in the month it was made (competence_month), and invoice_month
-- keeps the invoice it is billed on — what the card invoice bill sums.
-- Installment n of a purchase counts n-1 months after the purchase.
alter table transactions add column invoice_month date null;

update transactions set invoice_month = competence_month
where payment_method = 'credito' and credit_card_id is not null;

update transactions
set competence_month = (date_trunc('month', purchase_date) + make_interval(months => greatest(installment_number, 1) - 1))::date
where payment_method = 'credito' and credit_card_id is not null;

create index transactions_invoice_idx on transactions (credit_card_id, invoice_month) where invoice_month is not null;
