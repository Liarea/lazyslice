-- root:         public.reg025_tickets
-- take:         6
-- expect:       ok
-- found:        the 2026-09-15 red team round 3, attack:1:r3 (the A2 residual
--               and its NEW VARIANT)
-- phone-region: GB
-- not-copied:   public.reg025_tickets.contact, public.reg025_tickets.kontaktnr
-- why:          a real, correctly formatted UK phone number -- dictated in
--               words in `contact`, written plainly with spaces and brackets
--               in `kontaktnr` -- crossed into the target verbatim under exit
--               0, both columns reporting "no name or value signal": every
--               phone validator on the row path parses under
--               textsig.PhoneRegionHint ("ZZ") only, which admits nothing
--               that is not already international, and no obfuscation is
--               needed at all
--
-- What happened, and to what. docs/reviews/2026-09-15-redteam/round3-still-
-- leaking.json's own A2 entry: "plus four four, seven seven oh oh, nine one
-- one, nine one one" parses fine under "ZZ" because it spells the
-- international form, but "oh seven nine one one one two three four five
-- six" spells the identical number's NATIONAL form, and there is no "+" for
-- libphonenumber to read a country code out of, so ValidPhone(candidate)
-- returns false for every candidate textsig.Candidates offers it -- the
-- kontaktnr variant (index 3 of the same file) removes the dictation
-- entirely: "07911 123456" and "020 7946 0958" are copied under exactly the
-- same green tick, in a column name (an abbreviation in another language)
-- rules.yml's phone pattern does not match.
--
-- The fix is --phone-region REGION (T-0221): internal/classify's row path
-- now parses a second candidate reading under the configured region --
-- textsig.ValidPhoneRegion, which PhoneRegionHint alone can never reach --
-- on the same strong, decides-the-column-outright footing the international
-- entry already has, and internal/verify's second net asks the identical
-- question over the loaded target. Both `contact` (spelled) and `kontaktnr`
-- (plain) are masked once GB is named, whatever the column is called: the
-- fix works from the value, not from a widened name pattern. `note` carries
-- no personal data and is unmasked deliberately, as a control -- if it were
-- masked too the fix would have gone too wide.

CREATE TABLE public.reg025_tickets (
    id        integer PRIMARY KEY,
    contact   text NOT NULL,
    kontaktnr text NOT NULL,
    note      text NOT NULL
);

-- Gives assertTortureNoLiteralSurvives -- the leak check every `expect: ok`
-- file gets automatically -- something to find without relying on
-- scan_test.go's phone detector, which needs a leading "+" or a column named
-- phone/mobile/msisdn/fax/telephone and so cannot see either column above:
-- neither `contact` nor `kontaktnr` matches that pattern, and neither value
-- carries a "+".
CREATE TABLE public.reg025_contacts (
    id         integer PRIMARY KEY,
    ticket_id  integer NOT NULL REFERENCES public.reg025_tickets(id),
    email      text NOT NULL
);

INSERT INTO public.reg025_tickets (id, contact, kontaktnr, note) VALUES
    (1, 'oh seven nine one one one two three four five six', '07911 123456', 'ticket opened by phone'),
    (2, 'oh two oh seven nine four six oh nine five eight', '020 7946 0958', 'callback requested'),
    (3, 'oh one six one four nine six oh one two three', '(0161) 496 0123', 'left a voicemail'),
    (4, 'oh seven nine one one one two three four five six', '020 7946 0958', 'duplicate of ticket 1'),
    (5, 'oh two oh seven nine four six oh nine five eight', '07911 123456', 'escalated'),
    (6, 'oh one six one four nine six oh one two three', '(0161) 496 0123', 'closed, no reply');

INSERT INTO public.reg025_contacts (id, ticket_id, email) VALUES
    (1, 1, 'grace.hopper1@realcorp.example'),
    (2, 2, 'grace.hopper2@realcorp.example'),
    (3, 3, 'grace.hopper3@realcorp.example'),
    (4, 4, 'grace.hopper4@realcorp.example'),
    (5, 5, 'grace.hopper5@realcorp.example'),
    (6, 6, 'grace.hopper6@realcorp.example');
