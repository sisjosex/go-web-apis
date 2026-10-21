-- TRACK-067 D2: a supervisor sees only the riders of the groups it is given (riders.group_label: a
-- class, a shift, an area); without groups it sees the whole organization. admin and viewer never
-- carry groups.
ALTER TABLE tracking.organization_members
    ADD COLUMN groups TEXT[],
    ADD CONSTRAINT chk_organization_member_groups CHECK (groups IS NULL OR role = 'supervisor');

COMMENT ON COLUMN tracking.organization_members.groups IS
'The rider groups (riders.group_label) a supervisor is limited to; NULL = the whole organization, only a supervisor carries them (TRACK-067 D2)';
