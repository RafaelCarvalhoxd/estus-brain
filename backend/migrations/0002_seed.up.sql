-- Seed data matching the categories and sample month used in the dashboard
-- design, so a fresh local environment renders something meaningful
-- immediately instead of an empty state.

insert into categories (name, nature, color) values
    ('Moradia',      'essencial',    '#2a78d6'),
    ('Alimentação',  'essencial',    '#eb6834'),
    ('Transporte',   'essencial',    '#1baf7a'),
    ('Assinaturas',  'variavel',     '#c98d00'),
    ('Lazer',        'variavel',     '#e2588e'),
    ('Saúde',        'essencial',    '#008300'),
    ('Compras',      'variavel',     '#4a3aa7');

insert into credit_cards (name, closing_day, due_day) values
    ('Cartão principal', 25, 15);
