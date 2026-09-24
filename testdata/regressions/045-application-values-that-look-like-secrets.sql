-- root:   public.reg045_accounts
-- take:   8
-- expect: ok
-- found:  dogfood session 1 (T-0315), not a torture schema
-- why:    the entropy validator read file names, MD5 and SHA-256 digests,
--         Rails single-table-inheritance class names, formatter class paths,
--         component names and environment variable names as secrets, and
--         decided two columns on one and four samples, so each was masked to
--         the fixed $lazyslice$invalid; an STI `type` column holding that
--         literal raises on every row the application loads
-- not-masked: public.reg045_exports.type, public.reg045_exports.klass, public.reg045_exports.formatter, public.reg045_exports.component_name, public.reg045_exports.report_file, public.reg045_exports.screenshot, public.reg045_exports.logo, public.reg045_exports.release_file, public.reg045_exports.webcam_shot, public.reg045_exports.file_md5, public.reg045_exports.upload_sha256, public.reg045_exports.env_var, public.reg045_sync_states.cursor_ref
-- not-copied: public.reg045_integrations.client_secret, public.reg045_integrations.access_token, public.reg045_integrations.opaque_ref, public.reg045_integrations.password_digest, public.reg045_uploads.upload_file
--
-- Three child tables of one root, the shapes dogfood session 1 met on a
-- production Rails schema, each reported "N/N samples look like secrets" and
-- masked as `credential` before T-0315. textsig.LooksSecret now refuses four
-- value shapes before it measures entropy -- a name ending in a known file
-- extension, a hex digest of exactly 32, 40 or 64 characters (bare, under an
-- algorithm prefix, or as colon-separated byte pairs), a `::` or `.`
-- namespaced identifier, and an environment variable's name -- and
-- internal/classify does not ask it about a column named `type`, `klass` or
-- `component_name` at all, which is what spares the un-namespaced class
-- names here. reg045_sync_states holds four random-looking values under a
-- neutral name: the entropy check no longer decides a column on fewer than
-- five samples, so it is copied. internal/verify's second net carries the
-- same floor and the same three names, so it does not refuse what the
-- classifier left alone. `not-masked:` is that half.
--
-- reg045_integrations is the half THREAT_MODEL.md T1 cares about: the real
-- secrets the same run caught correctly must stay caught. client_secret and
-- password_digest are caught on their names and their values; access_token
-- holds 64-character hex, which is now a digest's shape, and is caught on its
-- name alone -- the residual T-0315 states is a digest-length hex token under
-- a neutral name. opaque_ref is a 32-character base62 token under a name no
-- rule matches, caught on eight samples by the entropy check alone.
-- reg045_uploads is the same half for a file name: mastodon's
-- `photo-<username>-<n>.jpg`, every file named after its owner, and the
-- name dictionary holds half of the owners. A file name whose stem carries a
-- dictionary word is still read by the entropy check, and once a fifth of a
-- column's samples are that, every file name in it is again, so the column
-- is masked as it was before T-0315 rather than copied with the other half's
-- names in it. It has a table of its own, so no neighbouring credential
-- column is what masks it. `not-copied:` greps the whole target for every
-- source value of all five.
--
-- The root carries the one email column, which the harness's own leak check
-- needs to find in the source; it is the only column of its table, so the
-- neighbouring-column rule's sweep (043) reaches nothing, and no other table
-- here has a certain column for it to start from.

CREATE TABLE public.reg045_accounts (
    id             integer PRIMARY KEY,
    billing_email  text NOT NULL
);

CREATE TABLE public.reg045_exports (
    id              integer PRIMARY KEY,
    account_id      integer NOT NULL REFERENCES public.reg045_accounts (id),
    type            varchar(255) NOT NULL,
    klass           text,
    formatter       text,
    component_name  text,
    report_file     text,
    screenshot      text,
    logo            text,
    release_file    text,
    webcam_shot     text,
    file_md5        varchar(32),
    upload_sha256   text,
    env_var         text
);

CREATE TABLE public.reg045_integrations (
    id               integer PRIMARY KEY,
    account_id       integer NOT NULL REFERENCES public.reg045_accounts (id),
    client_secret    text,
    access_token     text,
    opaque_ref       text,
    password_digest  varchar(255)
);

CREATE TABLE public.reg045_uploads (
    id           integer PRIMARY KEY,
    account_id   integer NOT NULL REFERENCES public.reg045_accounts (id),
    upload_file  text
);

CREATE TABLE public.reg045_sync_states (
    id          integer PRIMARY KEY,
    account_id  integer NOT NULL REFERENCES public.reg045_accounts (id),
    cursor_ref  text
);

INSERT INTO public.reg045_accounts (id, billing_email) VALUES
    (1, 'billing01@realcorp.example'),
    (2, 'billing02@realcorp.example'),
    (3, 'billing03@realcorp.example'),
    (4, 'billing04@realcorp.example'),
    (5, 'billing05@realcorp.example'),
    (6, 'billing06@realcorp.example'),
    (7, 'billing07@realcorp.example'),
    (8, 'billing08@realcorp.example');

INSERT INTO public.reg045_exports (id, account_id, type, klass, formatter, component_name, report_file, screenshot, logo, release_file, webcam_shot, file_md5, upload_sha256, env_var) VALUES
    (1, 1, 'WebhookDeliveryAttempt', 'Reports::Formatters::XlsxFormatter', 'Reports::Formatters::CsvFormatter', 'DashboardWidget3', 'export-2024-03-01T101511Z.csv', 'Screenshot_2024-03-01_8xcwpa.png', 'logo_c4ca4238a0b92382.png', 'player-4.1.1-BvVT2-x64.exe', 'webcam_20240301_101512_cam1.JPG', 'f0f650f52453df0c46514137d3e051de', 'sha256:eb4ef66604bddbfe1af5c4f11771678bd67814a6ce7619fcdd385775aa9c88f5', 'SENTRY_DSN_EU2_PROD'),
    (2, 2, 'Exports::PdfExport', 'Notifications::SlackDelivery2', 'com.example.reports.XlsxFormatter', 'ScheduleCalendar2', 'export-2024-03-02T101512Z.csv', 'Screenshot_2024-03-02_ZkQhNr.png', 'logo_c81e728d9d4c2f63.png', 'player-4.2.2-mdyCz-x64.exe', 'webcam_20240302_101512_cam2.JPG', 'd88eb2ebb4d153905066bbd27fb8f156', 'sha256:df3f950262e51498ae1de02f1f2a9e1409400fa2f5d58da47d3f71ebac30eb63', 'OAUTH2_CLIENT_ID_PROD'),
    (3, 3, 'Exports::CsvExport', 'Reports::Formatters::CsvFormatter', 'Logger::JsonFormatter2', 'PlaylistEditorV2', 'export-2024-03-03T101513Z.csv', 'Screenshot_2024-03-03_pIZd9w.png', 'logo_eccbc87e4b5ce2fe.png', 'player-4.3.0-94Bmh-x64.exe', 'webcam_20240303_101512_cam0.JPG', '5f1aad9b93e268836fdc5ba96fbc58d8', 'sha256:34fe024e5deecdd5171b8eef5b9e3f2a47c8c3f4c7e9095cb65431b603c14fce', 'AWS_S3_BUCKET_V2'),
    (4, 4, 'ScheduledReportExport', 'Reports::Formatters::XlsxFormatter', 'Reports::Formatters::CsvFormatter', 'DashboardWidget3', 'export-2024-03-04T101514Z.csv', 'Screenshot_2024-03-04_tBDxZR.png', 'logo_a87ff679a2f3e71d.png', 'player-4.4.1-KmOoC-x64.exe', 'webcam_20240304_101512_cam1.JPG', '4c80cb1315f81d2ab55b2609414aec4e', 'sha256:99443781ef0a0ef7b9fba5f5b3f5d8cc081146253111d5cc2c7eb7b0ad67dd15', 'SENTRY_DSN_EU2_PROD'),
    (5, 5, 'WebhookDeliveryAttempt', 'Notifications::SlackDelivery2', 'com.example.reports.XlsxFormatter', 'ScheduleCalendar2', 'export-2024-03-05T101515Z.csv', 'Screenshot_2024-03-05_tylyEh.png', 'logo_e4da3b7fbbce2345.png', 'player-4.5.2-ds7Ar-x64.exe', 'webcam_20240305_101512_cam2.JPG', '098ec330b8bbc022818ce78fa71df54f', 'sha256:ec4d5ea0956f2bcaa84988394a22eb93e246a84aa451014be10a3fc82a0bcf95', 'OAUTH2_CLIENT_ID_PROD'),
    (6, 6, 'Exports::PdfExport', 'Reports::Formatters::CsvFormatter', 'Logger::JsonFormatter2', 'PlaylistEditorV2', 'export-2024-03-06T101516Z.csv', 'Screenshot_2024-03-06_PkwB6I.png', 'logo_1679091c5a880faf.png', 'player-4.6.0-e0FrW-x64.exe', 'webcam_20240306_101512_cam0.JPG', 'a114124eb4e6263d0d08559c1ed30a18', 'sha256:fbfbbb8e6946c517fd0ea2a21712e21a42a7563419678158413bcefc3348a2d2', 'AWS_S3_BUCKET_V2'),
    (7, 7, 'Exports::CsvExport', 'Reports::Formatters::XlsxFormatter', 'Reports::Formatters::CsvFormatter', 'DashboardWidget3', 'export-2024-03-07T101517Z.csv', 'Screenshot_2024-03-07_Y1Glda.png', 'logo_8f14e45fceea167a.png', 'player-4.7.1-EYhsG-x64.exe', 'webcam_20240307_101512_cam1.JPG', 'b32c340f43ebb3414826bfa2d43f6f0f', 'sha256:b1bbd880576f4a7c0fc32ecc5ddb5a29f6683eb50b9147b7a45bccd19a8d3f3b', 'SENTRY_DSN_EU2_PROD'),
    (8, 8, 'ScheduledReportExport', 'Notifications::SlackDelivery2', 'com.example.reports.XlsxFormatter', 'ScheduleCalendar2', 'export-2024-03-08T101518Z.csv', 'Screenshot_2024-03-08_j6Ajj8.png', 'logo_c9f0f895fb98ab91.png', 'player-4.8.2-47kUL-x64.exe', 'webcam_20240308_101512_cam2.JPG', '50585837425cfc01835b2be8e98b2bfb', 'sha256:94c76f7321aab5f4972588d5ebf9333d1fe6d171dacb6a2d7b65bb2b08736aa6', 'OAUTH2_CLIENT_ID_PROD');

INSERT INTO public.reg045_integrations (id, account_id, client_secret, access_token, opaque_ref, password_digest) VALUES
    (1, 1, 'qvIptSoGsYd0DoYfY0feG6zFjFbPybyx8AUQ0al9', '65dcf16ea3dfa49069628089eb4a75483070f5584b2a21ee64912b5f621f12da', 'WtVDQ3OolUfYaCJQ1GLTowLS9Vkww3jL', '$2a$12$zStzLBVezGyz4wdX6QU6Z0fjAov2EifGoK9Why1lyKpyYDpBWlfeT'),
    (2, 2, 'wtuYYvjlWm9hExlrRMWHReTKSTg6RsIX6P69Tshl', 'b9d7f2826c798e990d30dd291fddb436a01327c293788ce57195e60c7efb82b2', 'b49py3fPDjVBCrTV6Lc2pwHZcyIa3aEn', '$2a$12$haLKWg6db8eGOshgy1tL0WFDOPuQpRZQmoK2uWOVbN5xoofMGQGeQ'),
    (3, 3, 'OpZx31hu54zZsxtyjpzkbiK07SWUGfdBUjAnh2Zg', '823c72b0b895c3d404b6af5e9cc204a80a2ec3e9f0d004ec3ed90dfcd8c1ccd3', 'VC554XvbuzxvsKSgZIYGsJWtzhjHrDfH', '$2a$12$etzvLsdYsBQxj44jEggxhct4UpkY4fWpflmeVs5bALWxKUIadrTnA'),
    (4, 4, 'hwlouDtzP3SarnEBtUy0XJnNgIFXjMxEQ5BUT1iF', '9a6e73d028d018f96d67f1a8114c64d52bf92ee50dd28090cba7524ddf94d4be', 'IIfIzSwUPDXIP9dJ0P8wPabIUve6qsHK', '$2a$12$dpyrLpS8R9TFcuo7ezFBOgl9HjJwQ5zLxEss5rKRSJ8r5FkSq2DHr'),
    (5, 5, 'djGZ1hKtwy8ZwbYPBFpW9PGX3xztdOGB27InAj4l', 'acebc6f8e4c668167b1f91dfa5af2d4f1eaadcaa0fffb9ef3df0ed3cad610b20', 'xzMlcIxyUEoC4V0QdrPt2YXq7sGnMkCZ', '$2a$12$scjkLZv6khANknYEPQYjwXrNKGeqk1ZsNPKCQRYuOYJrdw44v733I'),
    (6, 6, 'aS0UjblvILzipFdNiTasVYVQdbgKnfa1s5sWSp54', 'f52c0ac289512b149af9fae5e43aaade11d6e5ef7d1a44337eb1b65dce43413c', 'bncoeNVDLwdfLYMaHsP3kijbxsaTRqg8', '$2a$12$pl9CqnCgXrPpo7oGJPqZxTXkR0TFMvffaLdgpi0o4iwNkJdLSx65Q'),
    (7, 7, 'XUF5iHGZVXKtQxb2a4Q7kVZb0Pbv2CF86oZdiaYm', '62fcd3f03d26b381a192d6c92fd52357102e9d420bae655d7da33ddb74eb5610', '4NArcwaG8SN8uKtxNgeHm7cUIJeOYkLa', '$2a$12$6ZyJw0Flf4peXHlvnhOpIfOzAaZmt6Rgp9f4fwxbYbiy2EXSRJE5q'),
    (8, 8, 'NPFhpkrqyDvIP5kAcL1lMeNo1HZ6INTGKDZRhVlF', 'f989fe5b65a687a469523b675276e998adf9bf9dcb7c6c99a65917b22dbb2ec9', 'LrSVditBv0OU98L2SJ88UpXF5mzKKDCo', '$2a$12$fkoDXu3jwFJPDgm22yZz2ejGFfCdBdIxsGTkYPcftXlsB2eQmFeap');

INSERT INTO public.reg045_uploads (id, account_id, upload_file) VALUES
    (1, 1, 'photo-dmytro_egorov1-11.jpg'),
    (2, 2, 'photo-grace_hopper2-12.jpg'),
    (3, 3, 'photo-dmytro_egorov3-13.jpg'),
    (4, 4, 'photo-grace_hopper4-14.jpg'),
    (5, 5, 'photo-dmytro_egorov5-15.jpg'),
    (6, 6, 'photo-grace_hopper6-16.jpg'),
    (7, 7, 'photo-dmytro_egorov7-17.jpg'),
    (8, 8, 'photo-grace_hopper8-18.jpg');

INSERT INTO public.reg045_sync_states (id, account_id, cursor_ref) VALUES
    (1, 1, 'LQJelybw7iK1aVYCs2P89C7W'),
    (2, 2, 'BAc0Uk6zbhu3Xk7KhYFGuLPI'),
    (3, 3, 'ntb9Eyg4ghaGyQY7UZIe2JJk'),
    (4, 4, 'XM7dbmIC4FRSqzitKj4v5luv');
