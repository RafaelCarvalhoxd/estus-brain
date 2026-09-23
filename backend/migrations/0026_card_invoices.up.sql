-- A card's invoice is a payable bill: one per card and due month, its amount
-- kept equal to the sum of that card's credit purchases competing for the
-- month (BillRepo.SyncInvoices). Paying it records no expense — the
-- purchases already are the expenses.
alter table bills add column invoice_card_id uuid null references credit_cards(id) on delete cascade;
alter table bills add column invoice_month date null;
create unique index bills_invoice_key on bills (invoice_card_id, invoice_month) where invoice_card_id is not null;
