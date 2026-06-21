package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/otaviosoaresp/dbtui/internal/schema"
)

func sampleSchemaTable() schema.TableInfo {
	return schema.TableInfo{
		Schema: "public", Name: "users", Type: schema.TableTypeRegular,
		Columns: []schema.ColumnInfo{
			{Name: "id", DataType: "integer", IsNullable: false, IsPK: true, Position: 1},
			{Name: "email", DataType: "varchar(255)", IsNullable: false, Position: 2},
		},
		Indexes:          []schema.IndexInfo{{Name: "users_pkey", Method: "btree", IsUnique: true, IsPrimary: true, Columns: []string{"id"}}},
		CheckConstraints: []schema.Constraint{{Name: "c1", Definition: "CHECK (id > 0)"}},
	}
}

func TestTableInfoTabCycle(t *testing.T) {
	ti := TableInfo{}
	ti.SetSize(120, 40)
	ti.Show(sampleSchemaTable(), nil)
	if !ti.Visible() {
		t.Fatal("expected visible")
	}
	start := ti.activeTab
	ti = ti.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	if ti.activeTab == start {
		t.Error("l should advance tab")
	}
	ti = ti.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")})
	if ti.activeTab != start {
		t.Error("h should return to first tab")
	}
}

func TestTableInfoRendersDDL(t *testing.T) {
	ti := TableInfo{}
	ti.SetSize(120, 40)
	ti.Show(sampleSchemaTable(), nil)
	ti.activeTab = tabDDL
	out := ti.View()
	if out == "" {
		t.Fatal("empty DDL view")
	}
}

func TestTableInfoHide(t *testing.T) {
	ti := TableInfo{}
	ti.SetSize(120, 40)
	ti.Show(sampleSchemaTable(), nil)
	ti.Hide()
	if ti.Visible() {
		t.Error("should be hidden")
	}
}
