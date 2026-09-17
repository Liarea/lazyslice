-- tools/egress/fixture.sql — the schema tools/egress/run.sh loads into the
-- source container. Everything in it is invented: no real person's name,
-- email or phone number appears here. The phone numbers use the NANP 555
-- range reserved for fiction (Alliance for Telecommunications Industry
-- Solutions, ATIS-0300055).
--
-- A customers table (email, full_name, phone — the three columns the
-- classifier's name rules and validators are built to catch) with an orders
-- child, so the run also proves a foreign key is followed.

CREATE TABLE public.customers (
    id serial PRIMARY KEY,
    email text NOT NULL,
    full_name text NOT NULL,
    phone text NOT NULL
);

CREATE TABLE public.orders (
    id serial PRIMARY KEY,
    customer_id integer NOT NULL REFERENCES public.customers (id),
    item text NOT NULL,
    amount numeric(10, 2) NOT NULL
);

INSERT INTO public.customers (email, full_name, phone) VALUES
    ('alice.tester@example.test', 'Alice Testerson', '+15555550101'),
    ('bob.tester@example.test', 'Bob Testerman', '+15555550102'),
    ('carol.tester@example.test', 'Carol Testworth', '+15555550103');

INSERT INTO public.orders (customer_id, item, amount) VALUES
    (1, 'Widget', 9.99),
    (1, 'Gadget', 19.99),
    (2, 'Gizmo', 29.99),
    (3, 'Doohickey', 4.50);
