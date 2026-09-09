--
-- calcom/generate.sql
--
-- Cal.com is the quoting fixture. Prisma names its tables and columns in the
-- application's own case — "EventType", "Booking"."startTime",
-- "Attendee"."email" — so every identifier in every statement lazyslice
-- generates has to be quoted or the run fails at the first SELECT.
-- testdata/nasty.sql trap 9 is one table shaped like this on purpose; this is a
-- hundred of them, written by somebody who was not thinking about us.
--
-- It is also the schema with the most enum types (46), and the one where the
-- personal data is spread across the widest set of tables: a booking's
-- attendees are people who never had an account, with their own names, email
-- addresses, phone numbers and timezones, and they are two foreign keys away
-- from the root.
--

SELECT lazyslice_torture.fill('public.users', 300);
SELECT lazyslice_torture.fill('public."UserPassword"', 300);
SELECT lazyslice_torture.fill('public."Team"', 60);

-- "Membership" carries a BEFORE INSERT trigger, update_membership_custom_role,
-- that rewrites "customRoleId" to the literal 'owner_role', 'admin_role' or
-- 'member_role' according to the row's role — and "customRoleId" has a foreign
-- key to "Role". So those three rows have to exist before a single membership
-- can be inserted, whatever the insert says. The generator has no way to know
-- that; this is what "written by hand instead" looks like.
INSERT INTO public."Role" (id, name, "updatedAt", description)
VALUES ('owner_role',  'Owner',  TIMESTAMP '2025-01-01 00:00:00', 'Full access'),
       ('admin_role',  'Admin',  TIMESTAMP '2025-01-01 00:00:00', 'Manage the team'),
       ('member_role', 'Member', TIMESTAMP '2025-01-01 00:00:00', 'Ordinary member');

SELECT lazyslice_torture.fill('public."Membership"', 300);
SELECT lazyslice_torture.fill('public."Schedule"', 300);
SELECT lazyslice_torture.fill('public."Availability"', 300);
SELECT lazyslice_torture.fill('public."EventType"', 200);
SELECT lazyslice_torture.fill('public."Booking"', 400);
SELECT lazyslice_torture.fill('public."Attendee"', 500);
SELECT lazyslice_torture.fill('public."Credential"', 200);
SELECT lazyslice_torture.fill('public."SecondaryEmail"', 150);
SELECT lazyslice_torture.fill('public."VerifiedNumber"', 150);

-- The account holders.
UPDATE public.users u
SET name = g.given || ' ' || g.family,
    username = lower(g.given) || '-' || lower(g.family) || u.id,
    email = lower(g.given) || '.' || lower(g.family) || u.id || '@' || g.domain,
    bio = 'Booking page for ' || g.given || ' ' || g.family || '. Reach me on ' || g.phone || '.',
    "timeZone" = (ARRAY['Europe/Berlin','America/Chicago','Asia/Singapore','Pacific/Auckland'])[1 + (u.id % 4)],
    "completedOnboarding" = (u.id % 5 <> 0),
    "twoFactorSecret" = CASE WHEN u.id % 6 = 0 THEN md5('2fa' || u.id) END,
    created = TIMESTAMP '2025-01-01 00:00:00' + (u.id || ' hours')::interval
FROM (
    SELECT id,
           (ARRAY['Ada','Bilal','Coco','Dilek','Eero','Fatou','Genji','Hilde','Ilyas','Jun'])[1 + (id % 10)] AS given,
           (ARRAY['Achebe','Baumann','Cabrales','Deniz','Engberg','Fournier','Gonzalez','Haugen','Ilves','Jung'])[1 + ((id / 10) % 10)] AS family,
           (ARRAY['lantern-studio.test','ridgeway.test','fernbank.test'])[1 + (id % 3)] AS domain,
           '+49 30 9' || lpad((100000 + id)::text, 6, '0') AS phone
    FROM public.users
) g
WHERE g.id = u.id;

UPDATE public."UserPassword"
SET hash = '$2a$12$' || md5('pw' || "userId") || substr(md5('pw2' || "userId"), 1, 21);

UPDATE public."SecondaryEmail" e
SET email = 'alt.' || u.email,
    "emailVerified" = CASE WHEN e.id % 3 = 0 THEN TIMESTAMP '2025-04-01 00:00:00' END
FROM public.users u
WHERE u.id = e."userId";

UPDATE public."VerifiedNumber" v
SET "phoneNumber" = '+49 30 9' || lpad((900000 + v.id)::text, 6, '0');

-- An OAuth credential's `key` is the access token and the refresh token, in
-- jsonb, in a column called `key`. Nothing about the name says secret.
UPDATE public."Credential"
SET type = (ARRAY['google_calendar','office365_calendar','zoom_video','stripe_payment'])[1 + (id % 4)],
    key = jsonb_build_object(
        'access_token', 'ya29.' || md5('at' || id) || md5('at2' || id),
        'refresh_token', '1//0' || md5('rt' || id),
        'expiry_date', 1893456000000::bigint,
        'scope', 'https://www.googleapis.com/auth/calendar');

-- The bookings, and the people on the other end of them. An attendee is
-- somebody who never signed up: their name, address, phone number and timezone
-- exist only here.
-- "Booking"."userId" and "EventType"."userId" are nullable, so the generator
-- left them NULL and a slice from public.users reached no booking at all. They
-- are set here: a booking with no user is a real shape (a team booking), so a
-- tenth of them keep it.
UPDATE public."Booking" b
SET "userId" = CASE WHEN b.id % 10 <> 0 THEN u.id END,
    title = 'Intro call between ' || u.name || ' and guest ' || b.id,
    description = 'Requested through the public page. Guest phone +33 1 7' || lpad((200000 + b.id)::text, 6, '0') || '.',
    "startTime" = TIMESTAMP '2025-09-01 09:00:00' + (b.id || ' hours')::interval,
    "endTime"   = TIMESTAMP '2025-09-01 09:30:00' + (b.id || ' hours')::interval,
    location = (ARRAY['integrations:daily','inPerson','phone'])[1 + (b.id % 3)],
    uid = md5('booking' || b.id)
FROM public.users u
WHERE u.id = ((b.id - 1) % 300) + 1;

-- "Attendee"."bookingId" is nullable too, so it is set here for the same reason
-- "Booking"."userId" is: without it the attendees — the people who never signed
-- up, whose names, addresses and phone numbers exist nowhere else — are two
-- foreign keys from the root and reachable through neither.
UPDATE public."Attendee" a
SET "bookingId" = (SELECT b.id FROM public."Booking" b ORDER BY b.id OFFSET ((g.id - 1) % 400) LIMIT 1),
    name = g.given || ' ' || g.family,
    email = lower(g.given) || '.' || lower(g.family) || a.id || '@' || g.domain,
    "phoneNumber" = '+33 1 7' || lpad((300000 + a.id)::text, 6, '0'),
    "timeZone" = (ARRAY['Europe/Paris','America/Sao_Paulo','Africa/Lagos'])[1 + (a.id % 3)]
FROM (
    SELECT id,
           (ARRAY['Kofi','Lenka','Mirela','Nils','Oona','Pekka','Quim','Rosita','Sanaa','Tuur'])[1 + (id % 10)] AS given,
           (ARRAY['Kimani','Larsson','Mendonca','Nowacki','Olesen','Pires','Quesada','Riis','Salmi','Tremblay'])[1 + ((id / 10) % 10)] AS family,
           (ARRAY['guestmail.test','rivierapost.test','tramline.test'])[1 + (id % 3)] AS domain
    FROM public."Attendee"
) g
WHERE g.id = a.id;

UPDATE public."EventType" e
SET "userId" = u.id,
    title = 'Consultation with ' || u.name,
    slug = 'consult-' || e.id,
    description = 'Booked by clients of ' || u.email
FROM public.users u
WHERE u.id = ((e.id - 1) % 300) + 1;
