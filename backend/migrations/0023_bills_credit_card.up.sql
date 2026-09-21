-- The card a bill is settled on, when its payment method is crédito — set
-- alongside category and payment_method, and confirmed (not re-asked) at pay
-- time, the same way those two already work.
alter table bills add column credit_card_id uuid null references credit_cards(id);
