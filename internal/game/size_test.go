package game

import (
	"bytes"
	"reflect"
	"testing"
)

// The typed records take the C layouts' sizes on disk (records.h RECORD_SIZE).
func TestDiskSizes(t *testing.T) {
	for _, c := range []struct {
		name string
		v    any
		want int
	}{
		{"career", Career{}, 74},
		{"character", Character{}, 634},
		{"monster_anim", MonsterAnim{}, 25},
		{"item", Item{}, 107},
		{"spell", Spell{}, 89},
		{"disease", Disease{}, 47},
		{"potion_recipe", PotionRecipe{}, 109},
		{"membership", Membership{}, 13},
		{"faction", Faction{}, 92},
		{"bank_account", BankAccount{}, 13},
		{"location", Location{}, 48},
		{"building", Building{}, 26},
		{"settings", Settings{}, 6},
		{"person", Person{}, 3},
		{"blessing", Blessing{}, 7},
		{"logbook", Logbook{}, 3008},
		{"automap", Automap{}, 10240},
		{"block_model", BlockModel{}, 66},
		{"block_flat", BlockFlat{}, 17},
		{"block_door", BlockDoor{}, 19},
		{"block_section3", BlockSection3{}, 16},
		{"quest_npc", QuestNPC{}, 58},
	} {
		if got := diskSize(reflect.TypeOf(c.v)); got != c.want {
			t.Errorf("%s: %d bytes, want %d", c.name, got, c.want)
		}
	}
}

// A record goes to disk and back unchanged.
func TestRecordRoundTrip(t *testing.T) {
	o := &Record{Type: 3, X: -5, Y: 7, ID: 0x1234, Image: 9, Character: &Character{Gold: 77, Health: -3}}
	o.Character.Equipped[4] = &Record{ID: 0x55}
	o.DataSize = 634
	b := recordToDisk(o)
	if len(b) != 71+634 {
		t.Fatalf("%d bytes", len(b))
	}
	r := recordFromDisk(b)
	if r.X != -5 || r.ID != 0x1234 || r.Character.Gold != 77 || r.Character.Health != -3 ||
		r.Character.Equipped[4].ID != 0x55 || !r.Character.Equipped[4].placeholder {
		t.Fatalf("decoded %+v", r.Character)
	}
	if !bytes.Equal(recordToDisk(r), b) {
		t.Fatal("not the same bytes")
	}
}
