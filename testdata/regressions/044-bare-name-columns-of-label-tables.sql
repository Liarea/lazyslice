-- root:   public.reg044_users
-- take:   5
-- expect: ok
-- found:  dogfood session 1 (T-0313), not a torture schema
-- why:    the rule pack's bare (^|_)names?(_|$) word masked 38 columns
--         called just `name` -- tags, folders, playlists, widgets, roles,
--         languages, AI models, triggers -- and every Paperclip
--         `*_file_name` column as a person's name, and two of them, under
--         unique indexes, refused the plan
-- not-masked: public.reg044_tags.name, public.reg044_folders.name, public.reg044_folders.logo_file_name, public.reg044_ai_models.name, public.reg044_ai_models.display_name
-- not-copied: public.reg044_users.name, public.reg044_orders.name
--
-- The bare word says a column holds *a* name, not *a person's* name.
-- internal/classify's bare_name rule (rules.yml) now needs corroboration
-- before it masks: a word for people in the table or column name, or at least
-- a fifth of the samples carrying a word from the name dictionary. With
-- neither, and enough samples to ask the dictionary, the column stays at
-- `low` and is copied -- `not-masked:` is that half: a tag list under a
-- unique index, mail folder names, AI model names and their display names.
-- A `*_file_name` column is outside the rule altogether.
--
-- `not-copied:` is the half THREAT_MODEL.md T1 cares about. reg044_users.name
-- is corroborated by its table name alone, and its values are chosen so the
-- dictionary carries none of them: that one must not depend on the samples.
-- reg044_orders.name is a bare name in a table not named for people, holding
-- recipients' names the dictionary does carry, so the samples corroborate it.
-- A name the dictionary cannot carry, in such a column, is T1's stated
-- residual and is not pinned here: a leak is not a behaviour to assert end to
-- end (internal/classify's TestBareNameNeedsCorroboration pins it at the unit
-- level, so a change that closes it is noticed).
--
-- Five users are the whole root and each has two tags, one folder, one AI
-- model and one order, so every child table is sampled in full: the rule asks
-- the dictionary only from three samples up, and a smaller table is masked on
-- its name exactly as before.

CREATE TABLE public.reg044_users (
    id     integer PRIMARY KEY,
    name   text NOT NULL,
    email  text NOT NULL
);

CREATE TABLE public.reg044_tags (
    id       integer PRIMARY KEY,
    user_id  integer NOT NULL REFERENCES public.reg044_users (id),
    name     varchar(255) NOT NULL UNIQUE
);

CREATE TABLE public.reg044_folders (
    id              integer PRIMARY KEY,
    user_id         integer NOT NULL REFERENCES public.reg044_users (id),
    name            text NOT NULL,
    logo_file_name  varchar(255)
);

CREATE TABLE public.reg044_ai_models (
    id            integer PRIMARY KEY,
    user_id       integer NOT NULL REFERENCES public.reg044_users (id),
    name          text NOT NULL,
    display_name  text NOT NULL
);

CREATE TABLE public.reg044_orders (
    id       integer PRIMARY KEY,
    user_id  integer NOT NULL REFERENCES public.reg044_users (id),
    name     text NOT NULL
);

INSERT INTO public.reg044_users (id, name, email) VALUES
    (1, 'xX_zed_Xx', 'zed.one@realcorp.example'),
    (2, 'qwop',      'qwop.two@realcorp.example'),
    (3, 'n00b',      'noob.three@realcorp.example'),
    (4, 'kazzt',     'kazzt.four@realcorp.example'),
    (5, 'bzzt',      'bzzt.five@realcorp.example');

INSERT INTO public.reg044_tags (id, user_id, name) VALUES
    (1, 1, 'urgent'),   (2, 1, 'later'),
    (3, 2, 'ideas'),    (4, 2, 'finance'),
    (5, 3, 'shopping'), (6, 3, 'recipes'),
    (7, 4, 'todo'),     (8, 4, 'archive'),
    (9, 5, 'work'),     (10, 5, 'travel');

INSERT INTO public.reg044_folders (id, user_id, name, logo_file_name) VALUES
    (1, 1, 'Inbox',    'logo.png'),
    (2, 2, 'Sent',     'banner.jpg'),
    (3, 3, 'Drafts',   'icon-512.png'),
    (4, 4, 'Trash',    'header_v2.svg'),
    (5, 5, 'Receipts', 'badge.webp');

INSERT INTO public.reg044_ai_models (id, user_id, name, display_name) VALUES
    (1, 1, 'gpt-4o',        'GPT-4o'),
    (2, 2, 'gpt-4o-mini',   'GPT-4o mini'),
    (3, 3, 'llama-3-70b',   'Llama 3 70B'),
    (4, 4, 'mistral-large', 'Mistral Large'),
    (5, 5, 'whisper-1',     'Whisper');

INSERT INTO public.reg044_orders (id, user_id, name) VALUES
    (1, 1, 'Margaret Hamilton'),
    (2, 2, 'Katherine Johnson'),
    (3, 3, 'Dorothy Vaughan'),
    (4, 4, 'Barbara Liskov'),
    (5, 5, 'Frances Allen');
