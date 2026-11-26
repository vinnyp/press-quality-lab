package observe

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestControlProjection(t *testing.T) {
	// QT4: every control shape defined in U23 projects independently.
	for _, row := range []struct {
		name string
		c    Controls
		want string
	}{
		{"Passing", Controls{ConfinementApplied: true, SelfTestPassed: true, SystemNamed: true, KnownFastWithin: true, ReadCalibrationMoved: true, StoreControlDetected: true, ReadControlKilled: true, PrivateCopyUnchanged: true}, `{"confinement":{"applied":true,"sandbox_exec_exit":null},"self_test":"passed","known_fast":"within","read_calibration":"moved","store_control":"detected","read_control":"killed","private_copy":"unchanged"}`},
		{"Failing", Controls{}, `{"confinement":{"applied":false,"sandbox_exec_exit":null},"self_test":"failed","known_fast":"over","read_calibration":"not_moved","store_control":"not_detected","read_control":"not_killed","private_copy":"changed"}`},
	} {
		t.Run(row.name, func(t *testing.T) {
			data, err := json.Marshal(ProjectControls(row.c))
			if err != nil {
				t.Fatal(err)
			}
			var got, want any
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(row.want), &want); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("got %s want %s", data, row.want)
			}
		})
	}
	exit := 65
	data, err := json.Marshal(ProjectControls(Controls{SandboxExecExit: &exit})["confinement"])
	if err != nil || string(data) != `{"applied":false,"sandbox_exec_exit":65}` {
		t.Fatalf("%s %v", data, err)
	}
}
func TestMarkerProjection(t *testing.T) {
	// QT5: both protocol fields survive the projection in both directions.
	for _, row := range []struct {
		m    Markers
		want string
	}{
		{Markers{}, `{"confinement":false,"could_not_start":false}`},
		{Markers{Confinement: true}, `{"confinement":true,"could_not_start":false}`},
		{Markers{CouldNotStart: true}, `{"confinement":false,"could_not_start":true}`},
		{Markers{Confinement: true, CouldNotStart: true}, `{"confinement":true,"could_not_start":true}`},
	} {
		data, err := json.Marshal(ProjectMarkers(row.m))
		if err != nil || string(data) != row.want {
			t.Fatalf("%s %v want %s", data, err, row.want)
		}
	}
}

func TestIndependentControlProjection(t *testing.T) {
	// QT4: each control affects only its own projected field.
	base := Controls{ConfinementApplied: true, SelfTestPassed: true, SystemNamed: true, KnownFastWithin: true, ReadCalibrationMoved: true, StoreControlDetected: true, ReadControlKilled: true, PrivateCopyUnchanged: true}
	for _, row := range []struct {
		name   string
		change func(*Controls)
		field  string
		want   any
	}{
		{"Confinement", func(c *Controls) { c.ConfinementApplied = false }, "confinement", map[string]any{"applied": false, "sandbox_exec_exit": nil}},
		{"SelfTest", func(c *Controls) { c.SelfTestPassed = false }, "self_test", "failed"},
		{"KnownFast", func(c *Controls) { c.KnownFastWithin = false }, "known_fast", "over"},
		{"Calibration", func(c *Controls) { c.ReadCalibrationMoved = false }, "read_calibration", "not_moved"},
		{"Store", func(c *Controls) { c.StoreControlDetected = false }, "store_control", "not_detected"},
		{"Read", func(c *Controls) { c.ReadControlKilled = false }, "read_control", "not_killed"},
		{"Copy", func(c *Controls) { c.PrivateCopyUnchanged = false }, "private_copy", "changed"},
	} {
		t.Run(row.name, func(t *testing.T) {
			// The literal expected object is independent of ProjectControls.
			want := map[string]any{"confinement": map[string]any{"applied": true, "sandbox_exec_exit": nil}, "self_test": "passed", "known_fast": "within", "read_calibration": "moved", "store_control": "detected", "read_control": "killed", "private_copy": "unchanged"}
			want[row.field] = row.want
			c := base
			row.change(&c)
			b, err := json.Marshal(ProjectControls(c))
			if err != nil {
				t.Fatal(err)
			}
			var got map[string]any
			if err := json.Unmarshal(b, &got); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("got %v want %v", got, want)
			}
		})
	}
}
