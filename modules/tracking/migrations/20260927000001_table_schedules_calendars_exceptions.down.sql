-- Reverses 20260927000001. The tables are new in TRACK-018 and nothing outside them references
-- their rows, so dropping them restores the schema exactly as TRACK-007 left it. Order is the
-- reverse of creation: the schedules point at the calendars.

DROP TABLE IF EXISTS tracking.route_exceptions;
DROP TABLE IF EXISTS tracking.route_schedules;
DROP TABLE IF EXISTS tracking.calendar_dates;
DROP TABLE IF EXISTS tracking.calendars;
