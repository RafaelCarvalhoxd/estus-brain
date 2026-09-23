-- Day of the month a reminder repeats on (1-31). A month without that day
-- (31 in April) uses its last day. Null means it doesn't repeat monthly.
alter table reminders add column repeat_month_day smallint null check (repeat_month_day between 1 and 31);
