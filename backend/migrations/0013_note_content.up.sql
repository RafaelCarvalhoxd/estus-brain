-- Notas get a real editor. The document itself (headings, lists, checklists,
-- code, tables, images) is the editor's JSON, kept as-is; `body` stays as its
-- plain-text copy, which is what search, the overview and the local AI read.
-- Notes from before have no content: the editor opens their body as text.
alter table notes add column content jsonb;
