--
-- odoo/generate.sql
--
-- Odoo is the fixture for a schema where one table is referenced by everything.
-- `res_users` has 330 incoming foreign keys, because every model Odoo defines
-- carries `create_uid` and `write_uid` pointing at it, and it is by that measure
-- the most-connected table in the whole set. A slice rooted there is the
-- worst case for a child walk: almost every table in the database is a depth-1
-- child of the root.
--
-- Two other shapes:
--
--   * translatable fields are `jsonb`. `res_country.name`,
--     `res_partner_category.name` and dozens of others hold
--     `{"en_US": "Belgium"}` rather than text, so a classifier that only reads
--     `text`-typed columns sees no names anywhere in this database.
--   * `res_partner` is both a person and a company, and it is what
--     `res_company` and `res_users` are each hung off. The personal data is in
--     the table with the least personal-sounding name in the set.
--
-- Odoo installs its own seed data (currencies, countries, languages, the admin
-- user) when it initialises a database. `schema.sql` is a schema-only dump, so
-- none of that is here and every row below is generated, including the
-- currencies and countries: the fixture's subject is the shape of the schema,
-- not Odoo's reference data, and shipping Odoo's data would make the fixture
-- ten times the size for nothing.
--

SELECT lazyslice_torture.fill('public.res_currency', 20);
SELECT lazyslice_torture.fill('public.res_country', 50);
SELECT lazyslice_torture.fill('public.res_partner', 400);
SELECT lazyslice_torture.fill('public.res_company', 5);
SELECT lazyslice_torture.fill('public.res_users', 300);
-- res_users_log.create_uid is the only column that connects it to a person and
-- it is nullable, so it is filled explicitly rather than left to the generator.
SELECT lazyslice_torture.fill('public.res_users_log', 300,
    '{"create_uid": "(SELECT p.id FROM public.res_users p ORDER BY p.id OFFSET ((s.i - 1) % 300) LIMIT 1)"}');
SELECT lazyslice_torture.fill('public.res_partner_category', 20);
SELECT lazyslice_torture.fill('public.res_partner_bank', 200);
SELECT lazyslice_torture.fill('public.mail_message', 400);
SELECT lazyslice_torture.fill('public.mail_followers', 200);
SELECT lazyslice_torture.fill('public.ir_attachment', 200);

-- The people, in the table Odoo calls a partner.
UPDATE public.res_partner p
SET name = g.given || ' ' || g.family,
    complete_name = g.given || ' ' || g.family,
    email = lower(g.given) || '.' || lower(g.family) || p.id || '@' || g.domain,
    email_normalized = lower(g.given) || '.' || lower(g.family) || p.id || '@' || g.domain,
    phone = '+32 2 7' || lpad((100000 + p.id)::text, 6, '0'),
    mobile = '+32 47' || lpad((1000000 + p.id)::text, 7, '0'),
    phone_sanitized = '+322 7' || lpad((100000 + p.id)::text, 6, '0'),
    street = p.id || ' Rue du Marché',
    street2 = CASE WHEN p.id % 7 = 0 THEN 'Boîte ' || (p.id % 40) END,
    city = (ARRAY['Bruxelles','Gent','Namur','Leuven'])[1 + (p.id % 4)],
    zip = lpad(((p.id * 7) % 9000 + 1000)::text, 4, '0'),
    vat = 'BE0' || lpad((400000000 + p.id)::text, 9, '0'),
    website = 'https://' || lower(g.family) || p.id || '.' || g.domain,
    comment = 'Preferred contact: ' || lower(g.given) || '.' || lower(g.family) || p.id || '@' || g.domain
              || ' — mobile +32 47' || lpad((1000000 + p.id)::text, 7, '0') || '.',
    is_company = (p.id % 9 = 0),
    active = true
FROM (
    SELECT id,
           (ARRAY['Ans','Bavo','Chloé','Dries','Eva','Faisal','Griet','Hind','Imke','Joris'])[1 + (id % 10)] AS given,
           (ARRAY['Aerts','Bogaert','Claes','De Smet','Everaert','Fassin','Goossens','Hermans','Impens','Janssens'])[1 + ((id / 10) % 10)] AS family,
           (ARRAY['kanaalzicht.test','marktplein.test','veldstraat.test'])[1 + (id % 3)] AS domain
    FROM public.res_partner
) g
WHERE g.id = p.id;

-- The login half. Odoo hashes the password into the same table.
UPDATE public.res_users u
SET login = lower(split_part(p.name, ' ', 1)) || '.' || u.id || '@' || split_part(p.email, '@', 2),
    password = '$pbkdf2-sha512$25000$' || md5('pw' || u.id) || '$' || md5('pw2' || u.id),
    signature = '<p>' || p.name || '<br/>' || p.email || '<br/>' || p.phone || '</p>',
    active = true
FROM public.res_partner p
WHERE p.id = u.partner_id;

-- A bank account number is personal data of a kind no name-based rule catches
-- and no value-based rule should miss.
UPDATE public.res_partner_bank b
SET acc_number = 'BE68 5390 ' || lpad((7000000 + b.id)::text, 7, '0'),
    sanitized_acc_number = 'BE685390' || lpad((7000000 + b.id)::text, 7, '0'),
    acc_holder_name = p.name
FROM public.res_partner p
WHERE p.id = b.partner_id;

-- Odoo's chatter: every note anyone has ever typed about a customer.
--
-- `author_id`, `create_uid` and `write_uid` are all nullable — Odoo writes a
-- message with no author for a system notification — so the generator left them
-- NULL and a slice from res_users reached nothing at all. They are set here.
UPDATE public.mail_message m
SET author_id = p.id,
    create_uid = (SELECT id FROM public.res_users ORDER BY id OFFSET ((r.k - 1) % 300) LIMIT 1),
    write_uid = (SELECT id FROM public.res_users ORDER BY id OFFSET ((r.k - 1) % 300) LIMIT 1),
    subject = 'Re: order for ' || p.name,
    body = '<p>Called ' || p.name || ' on ' || p.phone || '. Confirmed the address: '
           || p.street || ', ' || p.zip || ' ' || p.city || '.</p>',
    email_from = '"' || p.name || '" <' || p.email || '>',
    message_type = 'comment',
    model = 'res.partner',
    res_id = p.id
FROM (
    SELECT id, row_number() OVER (ORDER BY id) AS k FROM public.mail_message
) r
JOIN LATERAL (
    SELECT id, name, email, phone, street, zip, city
    FROM public.res_partner ORDER BY id OFFSET ((r.k - 1) % 400) LIMIT 1
) p ON true
WHERE r.id = m.id;

-- The translatable-name shape: a proper noun inside a jsonb document.
UPDATE public.res_country
SET name = jsonb_build_object('en_US', 'Country ' || id, 'nl_BE', 'Land ' || id);

UPDATE public.res_partner_category
SET name = jsonb_build_object('en_US', 'Segment ' || id, 'nl_BE', 'Segment ' || id);
