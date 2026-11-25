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

func TestExitDetail(t *testing.T) {
	for _, detail := range []uint32{0x10000, 0x20000, 0x40000, 0} {
		got, err := exitDetail(0, 0x70000, int64(detail|9))
		if err != nil || got != detail {
			t.Fatalf("exit detail from data: got %#x error %v want %#x", got, err, detail)
		}
	}
	got, err := exitDetail(0x4000, 0, 3)
	if got != 0 || err == nil || err.Error() != "no such process" {
		t.Fatalf("EV_ERROR not decoded: %#x %v", got, err)
	}
}
