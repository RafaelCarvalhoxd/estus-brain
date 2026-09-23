-- Telegram left the app: an external agent (Hermes, OpenClaw) now reaches
-- the owner on its own channels. Its pairing, schedule and sent-log go too.
drop table if exists telegram_sent;
drop table if exists telegram_settings;
