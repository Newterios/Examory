CREATE TABLE courses (
 id uuid PRIMARY KEY, title text NOT NULL,
 status text NOT NULL CHECK (status IN ('draft','published','archived')),
 revision integer NOT NULL CHECK (revision>0), modules jsonb NOT NULL DEFAULT '[]'
);
CREATE TABLE course_members (
 course_id uuid NOT NULL REFERENCES courses(id), kind text NOT NULL CHECK(kind IN ('teacher','group')),
 member_id uuid NOT NULL, PRIMARY KEY(course_id,kind,member_id)
);
CREATE INDEX course_members_reverse ON course_members(kind,member_id,course_id);
CREATE TABLE material_completions (
 course_id uuid NOT NULL REFERENCES courses(id), material_id uuid NOT NULL, student_id uuid NOT NULL,
 PRIMARY KEY(course_id,material_id,student_id)
);
