-- root:   public.reg036_members
-- take:   20
-- expect: exit 12 plan.refused.unique_domain
-- found:  the T-0253 review round, high finding
-- why:    fkPairs used to bound its walk to cref's *direct* foreign-key
--         partners only, in both directions; that closed the round-5
--         red-team leak (035) but reopened the same leak one hop further
--         out, wherever a raised partner is itself the child end of a
--         further validated foreign key -- the further parent was reached
--         by nothing and crossed into the target verbatim

CREATE TABLE public.reg036_name_root (
    slug_root varchar(12) PRIMARY KEY
);

INSERT INTO public.reg036_name_root (slug_root) VALUES
    ('ኣበበ ኪዳነ'),
    ('ተስፋዬ ኪዳነ'),
    ('ገብረ ኣበበ'),
    ('ኪዳነ ተስፋዬ'),
    ('ኣበበ ገብረ');

CREATE TABLE public.reg036_name_mid (
    slug_mid varchar(12) PRIMARY KEY REFERENCES public.reg036_name_root(slug_root)
);

INSERT INTO public.reg036_name_mid (slug_mid) VALUES
    ('ኣበበ ኪዳነ'),
    ('ተስፋዬ ኪዳነ'),
    ('ገብረ ኣበበ'),
    ('ኪዳነ ተስፋዬ'),
    ('ኣበበ ገብረ');

CREATE TABLE public.reg036_members (
    id        integer PRIMARY KEY,
    email     text NOT NULL,
    slug_leaf varchar(12) NOT NULL REFERENCES public.reg036_name_mid(slug_mid)
);

INSERT INTO public.reg036_members (id, email, slug_leaf)
SELECT i,
       'member' || lpad(i::text, 2, '0') || '@realcorp.example',
       (ARRAY['ኣበበ ኪዳነ', 'ተስፋዬ ኪዳነ', 'ገብረ ኣበበ', 'ኪዳነ ተስፋዬ', 'ኣበበ ገብረ'])[1 + ((i - 1) % 5)]
FROM generate_series(1, 20) AS g(i);
