package schema

import (
	"strings"
	"testing"
)

func TestBuildDDLTable(t *testing.T) {
	tbl := TableInfo{
		Schema: "public", Name: "users", Type: TableTypeRegular,
		Columns: []ColumnInfo{
			{Name: "id", DataType: "integer", IsNullable: false, HasDefault: true, DefaultExpr: "nextval('users_id_seq'::regclass)", Position: 1, IsPK: true},
			{Name: "email", DataType: "character varying(255)", IsNullable: false, Position: 2},
			{Name: "status", DataType: "text", IsNullable: false, HasDefault: true, DefaultExpr: "'active'::text", Position: 3},
		},
		Indexes: []IndexInfo{
			{Name: "users_pkey", Method: "btree", IsUnique: true, IsPrimary: true, Columns: []string{"id"}, Definition: "CREATE UNIQUE INDEX users_pkey ON public.users USING btree (id)"},
			{Name: "users_email_idx", Method: "btree", IsUnique: true, IsPrimary: false, Columns: []string{"email"}, Definition: "CREATE UNIQUE INDEX users_email_idx ON public.users USING btree (email)"},
		},
		UniqueConstraints: []Constraint{{Name: "users_email_key", Definition: "UNIQUE (email)"}},
		CheckConstraints:  []Constraint{{Name: "users_status_chk", Definition: "CHECK (status <> ''::text)"}},
	}
	ddl := BuildDDL(tbl)
	for _, want := range []string{
		"CREATE TABLE public.users (",
		"id integer NOT NULL DEFAULT nextval('users_id_seq'::regclass)",
		"email character varying(255) NOT NULL",
		"status text NOT NULL DEFAULT 'active'::text",
		"PRIMARY KEY (id)",
		"CONSTRAINT users_email_key UNIQUE (email)",
		"CONSTRAINT users_status_chk CHECK (status <> ''::text)",
		"CREATE UNIQUE INDEX users_email_idx",
	} {
		if !strings.Contains(ddl, want) {
			t.Errorf("DDL missing %q\n---\n%s", want, ddl)
		}
	}
	if strings.Contains(ddl, "CREATE UNIQUE INDEX users_pkey") {
		t.Errorf("pkey should not be emitted as a separate CREATE INDEX\n%s", ddl)
	}
}

func TestBuildDDLView(t *testing.T) {
	v := TableInfo{
		Schema: "public", Name: "active_orders", Type: TableTypeView,
		ViewDefinition: "SELECT id FROM orders WHERE status = 'active';",
	}
	ddl := BuildDDL(v)
	if !strings.HasPrefix(ddl, "CREATE VIEW public.active_orders AS") {
		t.Errorf("unexpected view DDL: %s", ddl)
	}
}
