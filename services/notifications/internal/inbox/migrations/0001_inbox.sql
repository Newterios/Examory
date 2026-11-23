CREATE TABLE notification_owners (
 target_type text NOT NULL, target_id text NOT NULL, course_id text NOT NULL, student_id text NOT NULL,
 PRIMARY KEY (target_type,target_id)
);
CREATE TABLE notifications (
 event_id uuid PRIMARY KEY, kind text NOT NULL CHECK(kind IN ('deadline','grade','review')),
 course_id text NOT NULL, group_id text, target_type text NOT NULL, target_id text NOT NULL,
 occurred_at timestamptz NOT NULL, due_at timestamptz, decision text CHECK(decision IN ('confirmed','dismissed'))
);
CREATE INDEX notification_target ON notifications(target_type,target_id,event_id DESC);
CREATE INDEX notification_deadline ON notifications(group_id,event_id DESC) WHERE kind='deadline';
CREATE INDEX notification_owners_student ON notification_owners(student_id,course_id,target_type,target_id);
