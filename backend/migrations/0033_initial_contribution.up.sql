-- O valor inicial é dinheiro que já estava investido: conta no investimento,
-- mas não saiu da conta no mês. Só os aportes seguintes baixam o saldo.
alter table investment_contributions add column is_initial boolean not null default false;
update investment_contributions c set is_initial = true
where c.id = (
    select id from investment_contributions f
    where f.investment_id = c.investment_id
    order by f.created_at asc, f.id asc
    limit 1
);
