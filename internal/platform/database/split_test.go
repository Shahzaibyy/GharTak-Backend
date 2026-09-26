package database

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSplitSQL(t *testing.T) {
	tests := []struct {
		name  string
		sql   string
		want  int
		first string
	}{
		{name: "two statements", sql: "SELECT 1;\nSELECT 2;", want: 2, first: "SELECT 1"},
		{name: "string semicolon", sql: "SELECT 'a;b';\nSELECT 2;", want: 2, first: "SELECT 'a;b'"},
		{name: "comment semicolon", sql: "SELECT 1; -- tail; still comment\nSELECT 2;", want: 2, first: "SELECT 1"},
		{
			name:  "dollar quote",
			sql:   "CREATE FUNCTION f() AS $$\nBEGIN\nRETURN 1;\nEND;\n$$;",
			want:  1,
			first: "CREATE FUNCTION f() AS $$\nBEGIN\nRETURN 1;\nEND;\n$$",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := splitSQL(tt.sql)
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != tt.want {
				t.Fatalf("len = %d, stmts = %#v", len(got), got)
			}
			if got[0] != tt.first {
				t.Fatalf("first = %q", got[0])
			}
		})
	}
}

func TestSplitInitMigration(t *testing.T) {
	body := readMigration(t, "0001_init.up.sql")
	stmts, err := splitSQL(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(stmts) < 20 {
		t.Fatalf("len = %d", len(stmts))
	}
	joined := strings.Join(stmts, "\n")
	if !strings.Contains(joined, "RETURN NEW") {
		t.Fatal("function body was split")
	}
	for _, stmt := range stmts {
		if strings.Contains(stmt, "CREATE FUNCTION set_updated_at") && !strings.Contains(stmt, "RETURN NEW") {
			t.Fatal("updated_at function lost its body")
		}
	}
}

func TestSplitUnterminated(t *testing.T) {
	if _, err := splitSQL("SELECT 'abc"); err == nil {
		t.Fatal("expected error")
	}
}

func readMigration(t *testing.T, name string) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	path := filepath.Join(filepath.Dir(file), "..", "..", "..", "migrations", name)
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}
