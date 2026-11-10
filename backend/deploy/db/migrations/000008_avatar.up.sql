-- When the user last set a profile picture; NULL means none. The image itself
-- lives in the file store under avatars/<user id>.
ALTER TABLE usr.usr_user ADD COLUMN avatar_at TIMESTAMPTZ NULL;
