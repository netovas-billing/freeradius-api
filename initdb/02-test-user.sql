-- User test untuk verifikasi instalasi.
-- HAPUS / ganti sebelum production.
INSERT INTO radcheck (username, attribute, op, value)
VALUES ('bob', 'Cleartext-Password', ':=', 'hello');

INSERT INTO radreply (username, attribute, op, value)
VALUES ('bob', 'Reply-Message', ':=', 'Hello from SQL');
