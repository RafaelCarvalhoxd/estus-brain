-- series_ended lets the owner explicitly stop a recurring bill from growing
-- new occurrences, without touching history. The original design ("deleting
-- the last occurrence of a series ends the series") turned out to be
-- unimplementable once Materialize runs on every month view: the very next
-- GET /api/bills?month= for that month would see the series' latest
-- occurrence is now further in the past and recreate the one just deleted.
-- A dedicated, explicit flag replaces it — Materialize simply skips any
-- series whose latest occurrence has this set, and deleting an occurrence
-- carries no special meaning of its own again.
alter table bills add column series_ended boolean not null default false;

-- amount_varies is the series-level twin of amount_estimated (an
-- occurrence-level flag): whether THIS SERIES' amount is expected to change
-- every month, copied forward unchanged by each new occurrence and used to
-- seed that new occurrence's own amount_estimated. Collapsing both into one
-- column made the marker wrong as soon as an occurrence was paid or edited
-- (which only ever confirms that one occurrence, not the series).
alter table bills add column amount_varies boolean not null default false;
