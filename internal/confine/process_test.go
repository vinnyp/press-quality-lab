package confine

import "testing"

func TestGroupEnded(t *testing.T) {
	for _, tc := range []struct {
		name    string
		members []groupMember
		ended   bool
	}{
		{"empty", nil, true},
		{"zombie leader", []groupMember{{pid: 101, zombie: true}}, true},
		{"all zombies", []groupMember{{pid: 101, zombie: true}, {pid: 102, zombie: true}}, true},
		{"live leader", []groupMember{{pid: 101}}, false},
		{"live descendant", []groupMember{{pid: 101, zombie: true}, {pid: 102}}, false},
		{"live before zombie", []groupMember{{pid: 102}, {pid: 101, zombie: true}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := groupEnded(tc.members); got != tc.ended {
				t.Fatalf("groupEnded(%+v)=%v, want %v", tc.members, got, tc.ended)
			}
		})
	}
}
