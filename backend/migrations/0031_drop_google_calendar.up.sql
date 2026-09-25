-- The Google Calendar sync is gone. Events pulled from Google stay as
-- ordinary agenda events; only the link to Google and the token go.
drop table if exists google_oauth_tokens;
alter table events drop column if exists google_event_id;
