-- Whether a category is money going out (despesa) or coming in (receita).
-- The dashboard counts spending only from despesa categories and income
-- only from receita ones. Recebimentos is the one income category so far.
alter table categories add column kind text not null default 'despesa' check (kind in ('despesa', 'receita'));
update categories set kind = 'receita' where name = 'Recebimentos';
