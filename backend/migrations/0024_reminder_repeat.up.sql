-- Weekdays a reminder repeats on (0 = Sunday … 6 = Saturday, like Go's
-- time.Weekday). Empty means it doesn't repeat.
alter table reminders add column repeat_days smallint[] not null default '{}';
