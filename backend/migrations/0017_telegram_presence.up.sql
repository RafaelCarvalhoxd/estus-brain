-- When the bot last answered. Written by the poller on every successful poll
-- so that, after the laptop sleeps or the process dies, the "Estus conectado"
-- notice can say how long the owner was talking to nobody.
alter table telegram_settings add column last_online_at timestamptz;
