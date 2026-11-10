-- Optimistic concurrency: every update bumps version, and an update that names
-- the version it read is refused when someone else changed the row since.
ALTER TABLE app.product ADD COLUMN version INTEGER NOT NULL DEFAULT 1;
