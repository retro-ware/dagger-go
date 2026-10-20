package game

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// TestQuestSaveRoundTrip decodes every QBN file, resolves its ops' records as a started
// quest does, and checks that the save form reads back the same.
func TestQuestSaveRoundTrip(t *testing.T) {
	dir := os.Getenv("DAGGER_DATA")
	if dir == "" {
		t.Skip("DAGGER_DATA not set")
	}
	files, _ := filepath.Glob(filepath.Join(dir, "ARENA2", "*.QBN"))
	if len(files) == 0 {
		t.Skip("no QBN files")
	}
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		q := decodeQuest(raw, false)
		for i := range q.Ops {
			for a := range q.Ops[i].Args {
				arg := &q.Ops[i].Args[a]
				if a < int(q.Ops[i].ArgCount) && arg.Ref != qbnNoRecord && arg.Value != -1 && arg.Value != -2 {
					arg.Rec = QbnRef{int8(arg.Ref >> 8), int16(arg.Ref & 255)}
				}
			}
		}
		q2 := decodeQuest(encodeQuest(q), true)
		name := strings.TrimSuffix(filepath.Base(f), ".QBN")
		for _, c := range []struct {
			what string
			a, b any
		}{
			{"items", q.Items, q2.Items}, {"persons", q.Persons, q2.Persons}, {"places", q.Places, q2.Places},
			{"timers", q.Timers, q2.Timers}, {"foes", q.Foes, q2.Foes}, {"states", q.States, q2.States},
			{"textvars", q.TextVars, q2.TextVars},
		} {
			if !reflect.DeepEqual(c.a, c.b) {
				t.Errorf("%s: %s differ", name, c.what)
			}
		}
		for i := range q.Ops {
			for a := 0; a < int(q.Ops[i].ArgCount) && a < 5; a++ {
				if q.Ops[i].Args[a].Rec != q2.Ops[i].Args[a].Rec {
					t.Errorf("%s: op %d arg %d: %v != %v", name, i, a, q.Ops[i].Args[a].Rec, q2.Ops[i].Args[a].Rec)
				}
			}
		}
	}
}
