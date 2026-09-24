-- root:   public.reg043_users
-- take:   12
-- expect: ok
-- found:  dogfood session 1 (T-0311), not a torture schema
-- why:    unknownColumnsBesideCertain swept every signal-less character
--         column beside a certain email column into free_text -- a users
--         table's role and ui_mode, a devices table's os_type, log_level,
--         timezone and a text uuid -- so the copy held 255-character word
--         salad where the application expects 'admin' or 'linux' and it did
--         not boot
-- not-masked: public.reg043_users.role, public.reg043_users.state, public.reg043_users.uuid, public.reg043_users.ui_mode, public.reg043_devices.os_type, public.reg043_devices.log_level, public.reg043_devices.timezone, public.reg043_devices.app_version, public.reg043_devices.hostname, public.reg043_devices.serial, public.reg043_devices.asset_path
-- not-copied: public.reg043_users.tag
--
-- Two tables, the shape dogfood session 1 met on a production Rails schema of
-- 143 tables, reduced to the columns that broke it: each table carries an
-- email column the classifier is certain about, and beside it character
-- columns no name rule and no validator recognises. Before T-0311 every one
-- of those was swept into free_text by the neighbouring-column rule's second
-- arm (ARCHITECTURE.md §4). Now a column whose samples are an enumeration
-- (at least ten non-NULL samples, at most twenty distinct values, each seen
-- at least twice, every value an ASCII token carrying no dictionary name,
-- special-category term or gender term) or are all one identifier shape
-- (uuid, hex digest, semantic version, hostname, path) is spared and copied,
-- and its reason line says which -- `not-masked:` is that half.
--
-- The serial is 32-character hex, an MD5 digest's length. A hex column is
-- spared only at a digest's length (8 to 12 characters, or exactly 32, 40,
-- 64 or 128 where the entropy check does not claim the value; T-0354): the
-- 22-character serial this file first carried is a hex token as far as its
-- samples say, and is swept now as it was before T-0311.
--
-- reg043_users.tag is the half THREAT_MODEL.md T1 cares about: a native-
-- script given name per row, each seen twice, so it is an enumeration by
-- count and not by token shape (the round-4 and round-5 red teams' own
-- Amharic names, testdata/regressions/030 and 035). It must still be swept
-- and masked, and `not-copied:` greps the whole target for every source
-- value of it.
--
-- Twelve users are the whole root, and each has one device, so both tables
-- are sampled in full: an enumeration needs ten samples before the rule will
-- call it one, and a smaller table is swept exactly as before.

CREATE TABLE public.reg043_users (
    id       integer PRIMARY KEY,
    email    text NOT NULL,
    role     varchar(255),
    state    varchar(255),
    uuid     varchar(255),
    ui_mode  varchar(255),
    tag      text
);

CREATE TABLE public.reg043_devices (
    id           integer PRIMARY KEY,
    user_id      integer NOT NULL REFERENCES public.reg043_users (id),
    owner_email  text NOT NULL,
    os_type      text,
    log_level    text,
    timezone     text,
    app_version  text,
    hostname     text,
    serial       text,
    asset_path   text
);

INSERT INTO public.reg043_users (id, email, role, state, uuid, ui_mode, tag) VALUES
    (1,  'user01@realcorp.example', 'admin', 'active',    '0b6f3e2a-9c4d-4e1f-8a7b-5d2c1e0f0001', 'light',  'ኣበበ'),
    (2,  'user02@realcorp.example', 'admin', 'active',    '0b6f3e2a-9c4d-4e1f-8a7b-5d2c1e0f0002', 'light',  'ኣበበ'),
    (3,  'user03@realcorp.example', 'admin', 'active',    '0b6f3e2a-9c4d-4e1f-8a7b-5d2c1e0f0003', 'light',  'ተስፋዬ'),
    (4,  'user04@realcorp.example', 'user',  'active',    '0b6f3e2a-9c4d-4e1f-8a7b-5d2c1e0f0004', 'light',  'ተስፋዬ'),
    (5,  'user05@realcorp.example', 'user',  'active',    '0b6f3e2a-9c4d-4e1f-8a7b-5d2c1e0f0005', 'light',  'ገብረ'),
    (6,  'user06@realcorp.example', 'user',  'active',    '0b6f3e2a-9c4d-4e1f-8a7b-5d2c1e0f0006', 'dark',   'ገብረ'),
    (7,  'user07@realcorp.example', 'user',  'pending',   '0b6f3e2a-9c4d-4e1f-8a7b-5d2c1e0f0007', 'dark',   'ኪዳነ'),
    (8,  'user08@realcorp.example', 'user',  'pending',   '0b6f3e2a-9c4d-4e1f-8a7b-5d2c1e0f0008', 'dark',   'ኪዳነ'),
    (9,  'user09@realcorp.example', 'user',  'pending',   '0b6f3e2a-9c4d-4e1f-8a7b-5d2c1e0f0009', 'dark',   'ሰላም'),
    (10, 'user10@realcorp.example', 'user',  'suspended', '0b6f3e2a-9c4d-4e1f-8a7b-5d2c1e0f0010', 'dark',   'ሰላም'),
    (11, 'user11@realcorp.example', 'guest', 'suspended', '0b6f3e2a-9c4d-4e1f-8a7b-5d2c1e0f0011', 'system', 'ማርታ'),
    (12, 'user12@realcorp.example', 'guest', 'suspended', '0b6f3e2a-9c4d-4e1f-8a7b-5d2c1e0f0012', 'system', 'ማርታ');

INSERT INTO public.reg043_devices (id, user_id, owner_email, os_type, log_level, timezone, app_version, hostname, serial, asset_path) VALUES
    (1,  1,  'owner01@realcorp.example', 'linux',   'debug', 'Europe/London',    '2.1.0',  'api.alpha.prod.internal',   '3f9a2c7e1b4d8a60c5e79d02b7e4a101', '/assets/devices/alpha.png'),
    (2,  2,  'owner02@realcorp.example', 'linux',   'debug', 'Europe/London',    '2.2.0',  'api.bravo.prod.internal',   '3f9a2c7e1b4d8a60c5e79d02b7e4a102', '/assets/devices/bravo.png'),
    (3,  3,  'owner03@realcorp.example', 'linux',   'debug', 'Europe/London',    '2.3.0',  'api.charlie.prod.internal', '3f9a2c7e1b4d8a60c5e79d02b7e4a103', '/assets/devices/charlie.png'),
    (4,  4,  'owner04@realcorp.example', 'linux',   'info',  'Europe/London',    '2.4.0',  'api.delta.prod.internal',   '3f9a2c7e1b4d8a60c5e79d02b7e4a104', '/assets/devices/delta.png'),
    (5,  5,  'owner05@realcorp.example', 'windows', 'info',  'Europe/London',    '2.5.0',  'api.echo.prod.internal',    '3f9a2c7e1b4d8a60c5e79d02b7e4a105', '/assets/devices/echo.png'),
    (6,  6,  'owner06@realcorp.example', 'windows', 'info',  'Europe/London',    '2.6.0',  'api.foxtrot.prod.internal', '3f9a2c7e1b4d8a60c5e79d02b7e4a106', '/assets/devices/foxtrot.png'),
    (7,  7,  'owner07@realcorp.example', 'windows', 'info',  'America/New_York', '2.7.0',  'api.golf.prod.internal',    '3f9a2c7e1b4d8a60c5e79d02b7e4a107', '/assets/devices/golf.png'),
    (8,  8,  'owner08@realcorp.example', 'windows', 'info',  'America/New_York', '2.8.0',  'api.hotel.prod.internal',   '3f9a2c7e1b4d8a60c5e79d02b7e4a108', '/assets/devices/hotel.png'),
    (9,  9,  'owner09@realcorp.example', 'macos',   'warn',  'America/New_York', '2.9.0',  'api.india.prod.internal',   '3f9a2c7e1b4d8a60c5e79d02b7e4a109', '/assets/devices/india.png'),
    (10, 10, 'owner10@realcorp.example', 'macos',   'warn',  'America/New_York', '2.10.0', 'api.juliet.prod.internal',  '3f9a2c7e1b4d8a60c5e79d02b7e4a110', '/assets/devices/juliet.png'),
    (11, 11, 'owner11@realcorp.example', 'android', 'error', 'Europe/Berlin',    '2.11.0', 'api.kilo.prod.internal',    '3f9a2c7e1b4d8a60c5e79d02b7e4a111', '/assets/devices/kilo.png'),
    (12, 12, 'owner12@realcorp.example', 'android', 'error', 'Europe/Berlin',    '2.12.0', 'api.lima.prod.internal',    '3f9a2c7e1b4d8a60c5e79d02b7e4a112', '/assets/devices/lima.png');
