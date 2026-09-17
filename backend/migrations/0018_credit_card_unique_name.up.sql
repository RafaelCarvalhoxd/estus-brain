-- Cards used to exist only from the seed, so duplicates were impossible in
-- practice. Now that they can be created from the app, two cards sharing a
-- name make the assistant pick one at random when the owner says "lancei no
-- Nubank" — and the purchase silently takes the wrong billing cycle, with no
-- recomputation to undo it. categories has carried this same constraint
-- since 0001_init, for the same reason.
alter table credit_cards add constraint credit_cards_name_key unique (name);
