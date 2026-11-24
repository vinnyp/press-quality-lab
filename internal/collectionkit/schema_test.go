package collectionkit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/vinnyp/press-quality-lab/internal/judge"
)

const kit = "../../docs/specs/conformance-collection-1.0"
const sealed = "../../docs/specs/judge-1.0"

func read(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
func decode(t *testing.T, b []byte, v any) {
	t.Helper()
	if err := json.Unmarshal(b, v); err != nil {
		t.Fatal(err)
	}
}

// enumSites checks references at their use sites as well as definitions.
func enumSites(t *testing.T, root any, v any, path string, found map[string][]any, refs map[string]bool) {
	t.Helper()
	switch node := v.(type) {
	case map[string]any:
		if ref, ok := node["$ref"].(string); ok && strings.HasPrefix(ref, "#/") {
			if refs[ref] {
				t.Fatalf("cyclic reference %s", ref)
			}
			target := root
			for _, part := range strings.Split(ref[2:], "/") {
				switch node := target.(type) {
				case map[string]any:
					var ok bool
					target, ok = node[strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")]
					if !ok {
						t.Fatalf("missing reference %s", ref)
					}
				case []any:
					index, err := strconv.Atoi(part)
					if err != nil || index < 0 || index >= len(node) {
						t.Fatalf("invalid reference %s", ref)
					}
					target = node[index]
				default:
					t.Fatalf("invalid reference %s", ref)
				}
			}
			next := map[string]bool{}
			for k, v := range refs {
				next[k] = v
			}
			next[ref] = true
			enumSites(t, root, target, path, found, next)
		}
		if values, ok := node["enum"].([]any); ok {
			found[path] = values
		}
		if value, ok := node["const"]; ok {
			found[path] = append(found[path], value)
		}
		for key, child := range node {
			enumSites(t, root, child, path+"/"+key, found, refs)
		}
	case []any:
		for i, child := range node {
			enumSites(t, root, child, fmt.Sprintf("%s/%d", path, i), found, refs)
		}
	}
}

func TestJudgeSchemaCopies(t *testing.T) {
	for _, name := range []string{"judge-input.schema.json", "judge-output.schema.json"} {
		t.Run(strings.ReplaceAll(name, ".", "_"), func(t *testing.T) {
			if !bytes.Equal(read(t, filepath.Join(kit, "schema", name)), read(t, filepath.Join(sealed, "schema", name))) {
				t.Fatal("schema differs from sealed judge")
			}
		})
	}
}
func TestSchemaEnums(t *testing.T) {
	type policy struct{ oracle, site, owned string }
	policies := map[string]policy{
		"baseline.schema.json/properties/preimages/additionalProperties/allOf/0/if/properties/probe":                                                                              {owned: "[\"discovery\"]"},
		"baseline.schema.json/properties/preimages/additionalProperties/allOf/1/if/properties/probe":                                                                              {owned: "[\"bare\"]"},
		"baseline.schema.json/properties/preimages/additionalProperties/allOf/2/if/properties/probe":                                                                              {owned: "[\"byte_bound\"]"},
		"baseline.schema.json/properties/preimages/additionalProperties/allOf/3/if/properties/probe":                                                                              {owned: "[\"exit_codes\"]"},
		"baseline.schema.json/properties/preimages/additionalProperties/allOf/4/if/properties/probe":                                                                              {owned: "[\"planted_secret\"]"},
		"baseline.schema.json/properties/preimages/additionalProperties/allOf/4/then/properties/parameters/anyOf/1/properties/kind":                                               {owned: "[\"credential\",\"seeded_state\"]"},
		"baseline.schema.json/properties/preimages/additionalProperties/allOf/5/if/properties/probe":                                                                              {owned: "[\"store_unchanged\"]"},
		"baseline.schema.json/properties/preimages/additionalProperties/allOf/6/if/properties/probe":                                                                              {owned: "[null]"},
		"behaviour.schema.json/properties/behaviour_version":                                                                                                                      {owned: "[\"1.0\"]"},
		"behaviour.schema.json/properties/invocations/items/properties/unconfined/oneOf/0/properties/end/oneOf/0/properties/kind":                                                 {owned: "[\"exit\"]"},
		"behaviour.schema.json/properties/invocations/items/properties/unconfined/oneOf/0/properties/end/oneOf/1/properties/kind":                                                 {owned: "[\"signal\"]"},
		"behaviour.schema.json/properties/invocations/items/properties/unconfined/oneOf/0/properties/end/oneOf/1/if/properties/signal":                                            {owned: "[\"SIGKILL\"]"},
		"behaviour.schema.json/properties/invocations/items/properties/unconfined/oneOf/0/properties/end/oneOf/1/then/properties/by":                                              {owned: "[\"confinement\",\"budget\",\"memory\",\"code_signing\",\"before_marker\"]"},
		"behaviour.schema.json/properties/invocations/items/properties/unconfined/oneOf/0/properties/home/properties/changes/items/properties/kind":                               {owned: "[\"file\",\"directory\",\"link\",\"special\"]"},
		"behaviour.schema.json/properties/invocations/items/properties/unconfined/oneOf/0/properties/home/properties/changes/items/properties/change":                             {owned: "[\"created\",\"modified\",\"removed\"]"},
		"behaviour.schema.json/properties/invocations/items/properties/unconfined/oneOf/0/properties/home/properties/changes/items/if/properties/kind":                            {owned: "[\"file\"]"},
		"behaviour.schema.json/properties/invocations/items/properties/unconfined/oneOf/0/properties/home/properties/changes/items/if/properties/change":                          {owned: "[\"created\",\"modified\"]"},
		"behaviour.schema.json/properties/invocations/items/properties/unconfined/oneOf/0/properties/planted/oneOf/1/properties/control_end/oneOf/0/oneOf/0/properties/kind":      {owned: "[\"exit\"]"},
		"behaviour.schema.json/properties/invocations/items/properties/unconfined/oneOf/0/properties/planted/oneOf/1/properties/control_end/oneOf/0/oneOf/1/properties/kind":      {owned: "[\"signal\"]"},
		"behaviour.schema.json/properties/invocations/items/properties/unconfined/oneOf/0/properties/planted/oneOf/1/properties/control_end/oneOf/0/oneOf/1/if/properties/signal": {owned: "[\"SIGKILL\"]"},
		"behaviour.schema.json/properties/invocations/items/properties/unconfined/oneOf/0/properties/planted/oneOf/1/properties/control_end/oneOf/0/oneOf/1/then/properties/by":   {owned: "[\"confinement\",\"budget\",\"memory\",\"code_signing\",\"before_marker\"]"},
		"behaviour.schema.json/properties/invocations/items/properties/unconfined/oneOf/0/properties/leak_scan/properties/leaks/items/properties/encoding":                        {owned: "[\"plain\",\"base64\",\"base64-nopad\",\"base64url\",\"base64url-nopad\",\"hex-lower\",\"hex-upper\",\"percent\"]"},
		"behaviour.schema.json/properties/invocations/items/properties/confined/items/properties/end/oneOf/0/properties/kind":                                                     {owned: "[\"exit\"]"},
		"behaviour.schema.json/properties/invocations/items/properties/confined/items/properties/end/oneOf/1/properties/kind":                                                     {owned: "[\"signal\"]"},
		"behaviour.schema.json/properties/invocations/items/properties/confined/items/properties/end/oneOf/1/if/properties/signal":                                                {owned: "[\"SIGKILL\"]"},
		"behaviour.schema.json/properties/invocations/items/properties/confined/items/properties/end/oneOf/1/then/properties/by":                                                  {owned: "[\"confinement\",\"budget\",\"memory\",\"code_signing\",\"before_marker\"]"},
		"behaviour.schema.json/properties/invocations/items/properties/confined/items/properties/home/properties/changes/items/properties/kind":                                   {owned: "[\"file\",\"directory\",\"link\",\"special\"]"},
		"behaviour.schema.json/properties/invocations/items/properties/confined/items/properties/home/properties/changes/items/properties/change":                                 {owned: "[\"created\",\"modified\",\"removed\"]"},
		"behaviour.schema.json/properties/invocations/items/properties/confined/items/properties/home/properties/changes/items/if/properties/kind":                                {owned: "[\"file\"]"},
		"behaviour.schema.json/properties/invocations/items/properties/confined/items/properties/home/properties/changes/items/if/properties/change":                              {owned: "[\"created\",\"modified\"]"},
		"behaviour.schema.json/properties/invocations/items/properties/confined/items/properties/planted/oneOf/1/properties/control_end/oneOf/0/oneOf/0/properties/kind":          {owned: "[\"exit\"]"},
		"behaviour.schema.json/properties/invocations/items/properties/confined/items/properties/planted/oneOf/1/properties/control_end/oneOf/0/oneOf/1/properties/kind":          {owned: "[\"signal\"]"},
		"behaviour.schema.json/properties/invocations/items/properties/confined/items/properties/planted/oneOf/1/properties/control_end/oneOf/0/oneOf/1/if/properties/signal":     {owned: "[\"SIGKILL\"]"},
		"behaviour.schema.json/properties/invocations/items/properties/confined/items/properties/planted/oneOf/1/properties/control_end/oneOf/0/oneOf/1/then/properties/by":       {owned: "[\"confinement\",\"budget\",\"memory\",\"code_signing\",\"before_marker\"]"},
		"behaviour.schema.json/properties/invocations/items/properties/confined/items/properties/leak_scan/properties/leaks/items/properties/encoding":                            {owned: "[\"plain\",\"base64\",\"base64-nopad\",\"base64url\",\"base64url-nopad\",\"hex-lower\",\"hex-upper\",\"percent\"]"},
		"behaviour.schema.json/$defs/run/properties/end/oneOf/0/properties/kind":                                                                                                  {owned: "[\"exit\"]"},
		"behaviour.schema.json/$defs/run/properties/end/oneOf/1/properties/kind":                                                                                                  {owned: "[\"signal\"]"},
		"behaviour.schema.json/$defs/run/properties/end/oneOf/1/if/properties/signal":                                                                                             {owned: "[\"SIGKILL\"]"},
		"behaviour.schema.json/$defs/run/properties/end/oneOf/1/then/properties/by":                                                                                               {owned: "[\"confinement\",\"budget\",\"memory\",\"code_signing\",\"before_marker\"]"},
		"behaviour.schema.json/$defs/run/properties/home/properties/changes/items/properties/kind":                                                                                {owned: "[\"file\",\"directory\",\"link\",\"special\"]"},
		"behaviour.schema.json/$defs/run/properties/home/properties/changes/items/properties/change":                                                                              {owned: "[\"created\",\"modified\",\"removed\"]"},
		"behaviour.schema.json/$defs/run/properties/home/properties/changes/items/if/properties/kind":                                                                             {owned: "[\"file\"]"},
		"behaviour.schema.json/$defs/run/properties/home/properties/changes/items/if/properties/change":                                                                           {owned: "[\"created\",\"modified\"]"},
		"behaviour.schema.json/$defs/run/properties/planted/oneOf/1/properties/control_end/oneOf/0/oneOf/0/properties/kind":                                                       {owned: "[\"exit\"]"},
		"behaviour.schema.json/$defs/run/properties/planted/oneOf/1/properties/control_end/oneOf/0/oneOf/1/properties/kind":                                                       {owned: "[\"signal\"]"},
		"behaviour.schema.json/$defs/run/properties/planted/oneOf/1/properties/control_end/oneOf/0/oneOf/1/if/properties/signal":                                                  {owned: "[\"SIGKILL\"]"},
		"behaviour.schema.json/$defs/run/properties/planted/oneOf/1/properties/control_end/oneOf/0/oneOf/1/then/properties/by":                                                    {owned: "[\"confinement\",\"budget\",\"memory\",\"code_signing\",\"before_marker\"]"},
		"behaviour.schema.json/$defs/run/properties/leak_scan/properties/leaks/items/properties/encoding":                                                                         {owned: "[\"plain\",\"base64\",\"base64-nopad\",\"base64url\",\"base64url-nopad\",\"hex-lower\",\"hex-upper\",\"percent\"]"},
		"behaviour.schema.json/$defs/end/oneOf/0/properties/kind":                                                                                                                 {owned: "[\"exit\"]"},
		"behaviour.schema.json/$defs/end/oneOf/1/properties/kind":                                                                                                                 {owned: "[\"signal\"]"},
		"behaviour.schema.json/$defs/end/oneOf/1/if/properties/signal":                                                                                                            {owned: "[\"SIGKILL\"]"},
		"behaviour.schema.json/$defs/end/oneOf/1/then/properties/by":                                                                                                              {owned: "[\"confinement\",\"budget\",\"memory\",\"code_signing\",\"before_marker\"]"},
		"behaviour.schema.json/if/properties/violating":                                                                                                                           {owned: "[true]"},
		"behaviour.schema.json/then/properties/invocations/items/properties/unconfined/properties/end/oneOf/0/properties/kind":                                                    {owned: "[\"exit\"]"},
		"behaviour.schema.json/then/properties/invocations/items/properties/unconfined/properties/end/oneOf/1/properties/kind":                                                    {owned: "[\"signal\"]"},
		"behaviour.schema.json/then/properties/invocations/items/properties/unconfined/properties/end/oneOf/1/if/properties/signal":                                               {owned: "[\"SIGKILL\"]"},
		"behaviour.schema.json/then/properties/invocations/items/properties/unconfined/properties/end/oneOf/1/then/properties/by":                                                 {owned: "[\"confinement\",\"budget\",\"memory\",\"code_signing\",\"before_marker\"]"},
		"behaviour.schema.json/then/properties/invocations/items/properties/unconfined/properties/home/properties/changes/items/properties/kind":                                  {owned: "[\"file\",\"directory\",\"link\",\"special\"]"},
		"behaviour.schema.json/then/properties/invocations/items/properties/unconfined/properties/home/properties/changes/items/properties/change":                                {owned: "[\"created\",\"modified\",\"removed\"]"},
		"behaviour.schema.json/then/properties/invocations/items/properties/unconfined/properties/home/properties/changes/items/if/properties/kind":                               {owned: "[\"file\"]"},
		"behaviour.schema.json/then/properties/invocations/items/properties/unconfined/properties/home/properties/changes/items/if/properties/change":                             {owned: "[\"created\",\"modified\"]"},
		"behaviour.schema.json/then/properties/invocations/items/properties/unconfined/properties/planted/oneOf/1/properties/control_end/oneOf/0/oneOf/0/properties/kind":         {owned: "[\"exit\"]"},
		"behaviour.schema.json/then/properties/invocations/items/properties/unconfined/properties/planted/oneOf/1/properties/control_end/oneOf/0/oneOf/1/properties/kind":         {owned: "[\"signal\"]"},
		"behaviour.schema.json/then/properties/invocations/items/properties/unconfined/properties/planted/oneOf/1/properties/control_end/oneOf/0/oneOf/1/if/properties/signal":    {owned: "[\"SIGKILL\"]"},
		"behaviour.schema.json/then/properties/invocations/items/properties/unconfined/properties/planted/oneOf/1/properties/control_end/oneOf/0/oneOf/1/then/properties/by":      {owned: "[\"confinement\",\"budget\",\"memory\",\"code_signing\",\"before_marker\"]"},
		"behaviour.schema.json/then/properties/invocations/items/properties/unconfined/properties/leak_scan/properties/leaks/items/properties/encoding":                           {owned: "[\"plain\",\"base64\",\"base64-nopad\",\"base64url\",\"base64url-nopad\",\"hex-lower\",\"hex-upper\",\"percent\"]"},
		"case.schema.json/properties/expect/oneOf/0/properties/exit_code":                                                                                                         {owned: "[0,1,2,3,4,5,64,70]"},
		"case.schema.json/properties/expect/oneOf/1/properties/exit_code":                                                                                                         {owned: "[0,1,2,3,4,5,64,70]"},
		"case.schema.json/properties/input/properties/verifier":                                                                                                                   {owned: "[\"real\",\"scripted\"]"},
		"case.schema.json/properties/input/properties/host/oneOf/0/properties/confinement/oneOf/0/properties/applied":                                                             {owned: "[true]"},
		"case.schema.json/properties/input/properties/host/oneOf/0/properties/confinement/oneOf/1/properties/applied":                                                             {owned: "[false]"},
		"case.schema.json/properties/input/properties/host/oneOf/0/properties/self_test":                                                                                          {owned: "[\"passed\",\"failed\"]"},
		"case.schema.json/properties/input/properties/host/oneOf/0/properties/known_fast":                                                                                         {owned: "[\"within\",\"over\"]"},
		"case.schema.json/properties/input/properties/host/oneOf/0/properties/read_calibration":                                                                                   {owned: "[\"moved\",\"not_moved\"]"},
		"case.schema.json/properties/input/properties/host/oneOf/0/properties/store_control":                                                                                      {owned: "[\"detected\",\"not_detected\"]"},
		"case.schema.json/properties/input/properties/host/oneOf/0/properties/read_control":                                                                                       {owned: "[\"killed\",\"not_killed\"]"},
		"case.schema.json/properties/input/properties/host/oneOf/0/properties/private_copy":                                                                                       {owned: "[\"unchanged\",\"changed\"]"},
		"case.schema.json/properties/input/properties/host/oneOf/0/properties/kill_attribution/items/properties/by":                                                               {owned: "[\"memory\",\"code_signing\",\"other_process\"]"},
		"case.schema.json/properties/input/if/properties/verifier":                                                                                                                {owned: "[\"scripted\"]"},
		"case.schema.json/properties/input/then/properties/host/properties/confinement/oneOf/0/properties/applied":                                                                {owned: "[true]"},
		"case.schema.json/properties/input/then/properties/host/properties/confinement/oneOf/1/properties/applied":                                                                {owned: "[false]"},
		"case.schema.json/properties/input/then/properties/host/properties/self_test":                                                                                             {owned: "[\"passed\",\"failed\"]"},
		"case.schema.json/properties/input/then/properties/host/properties/known_fast":                                                                                            {owned: "[\"within\",\"over\"]"},
		"case.schema.json/properties/input/then/properties/host/properties/read_calibration":                                                                                      {owned: "[\"moved\",\"not_moved\"]"},
		"case.schema.json/properties/input/then/properties/host/properties/store_control":                                                                                         {owned: "[\"detected\",\"not_detected\"]"},
		"case.schema.json/properties/input/then/properties/host/properties/read_control":                                                                                          {owned: "[\"killed\",\"not_killed\"]"},
		"case.schema.json/properties/input/then/properties/host/properties/private_copy":                                                                                          {owned: "[\"unchanged\",\"changed\"]"},
		"case.schema.json/properties/input/then/properties/host/properties/kill_attribution/items/properties/by":                                                                  {owned: "[\"memory\",\"code_signing\",\"other_process\"]"},
		"case.schema.json/$defs/host/properties/confinement/oneOf/0/properties/applied":                                                                                           {owned: "[true]"},
		"case.schema.json/$defs/host/properties/confinement/oneOf/1/properties/applied":                                                                                           {owned: "[false]"},
		"case.schema.json/$defs/host/properties/self_test":                                                                                                                        {owned: "[\"passed\",\"failed\"]"},
		"case.schema.json/$defs/host/properties/known_fast":                                                                                                                       {owned: "[\"within\",\"over\"]"},
		"case.schema.json/$defs/host/properties/read_calibration":                                                                                                                 {owned: "[\"moved\",\"not_moved\"]"},
		"case.schema.json/$defs/host/properties/store_control":                                                                                                                    {owned: "[\"detected\",\"not_detected\"]"},
		"case.schema.json/$defs/host/properties/read_control":                                                                                                                     {owned: "[\"killed\",\"not_killed\"]"},
		"case.schema.json/$defs/host/properties/private_copy":                                                                                                                     {owned: "[\"unchanged\",\"changed\"]"},
		"case.schema.json/$defs/host/properties/kill_attribution/items/properties/by":                                                                                             {owned: "[\"memory\",\"code_signing\",\"other_process\"]"},
		"case.schema.json/$defs/judge_expect/properties/exit_code":                                                                                                                {owned: "[0,1,2,3,4,5,64,70]"},
		"case.schema.json/$defs/derivation_expect/properties/exit_code":                                                                                                           {owned: "[0,1,2,3,4,5,64,70]"},
		"case.schema.json/if/properties/kind":                          {owned: "[\"judge-checked\"]"},
		"case.schema.json/then/properties/expect/properties/exit_code": {owned: "[0,1,2,3,4,5,64,70]"},
		"case.schema.json/else/properties/expect/properties/exit_code": {owned: "[0,1,2,3,4,5,64,70]"},

		"baseline.schema.json/properties/baseline/properties/entries/items/properties/dimension":                                 {oracle: "judge-input.schema.json", site: "/properties/required/items/properties/dimension"},
		"baseline.schema.json/properties/baseline/properties/entries/items/properties/kind":                                      {oracle: "judge-input.schema.json", site: "/properties/required/items/properties/kind"},
		"baseline.schema.json/properties/baseline/properties/entries/items/properties/result":                                    {oracle: "judge-input.schema.json", site: "/properties/baseline/oneOf/1/properties/entries/items/properties/result"},
		"baseline.schema.json/properties/baseline/properties/entries/items/properties/tier":                                      {oracle: "judge-input.schema.json", site: "/properties/evidence/items/properties/tier"},
		"baseline.schema.json/properties/baseline_version":                                                                       {owned: "[\"1.0\"]"},
		"baseline.schema.json/properties/preimages/additionalProperties/properties/parameters/anyOf/4/properties/kind":           {owned: "[\"credential\",\"seeded_state\"]"},
		"baseline.schema.json/properties/preimages/additionalProperties/properties/probe":                                        {owned: "[\"discovery\",\"bare\",\"byte_bound\",\"exit_codes\",\"planted_secret\",\"store_unchanged\",null]"},
		"baseline.schema.json/properties/preimages/additionalProperties/properties/scheme":                                       {owned: "[\"pql-check-id-1\"]"},
		"baseline.schema.json/properties/spec_version":                                                                           {owned: "[\"1.0\"]"},
		"case.schema.json/properties/input/properties/fixtures/items/properties/kind":                                            {owned: "[\"link\",\"special_file\",\"oversized_output\",\"invalid_bytes\",\"planted_secret\"]"},
		"case.schema.json/properties/input/properties/request":                                                                   {oracle: "judge-input.schema.json", site: "/properties/request"},
		"case.schema.json/properties/input/properties/scope/items":                                                               {oracle: "judge-output.schema.json", site: "/properties/scope/items"},
		"case.schema.json/properties/kind":                                                                                       {owned: "[\"judge-checked\",\"derivation-only\"]"},
		"record.schema.json/properties/exit_code":                                                                                {owned: "[0,1,2,3,4,5,64,70]"},
		"record.schema.json/properties/next_actions/items/oneOf/0/properties/reason":                                             {oracle: "judge-output.schema.json", site: "/properties/checks/items/properties/reason"},
		"record.schema.json/properties/next_actions/items/oneOf/1/properties/dimension":                                          {oracle: "judge-input.schema.json", site: "/properties/required/items/properties/dimension"},
		"record.schema.json/properties/next_actions/items/oneOf/1/properties/reason":                                             {oracle: "judge-output.schema.json", site: "/properties/dimensions/items/properties/reason"},
		"record.schema.json/properties/record_version":                                                                           {owned: "[\"1.0\"]"},
		"record.schema.json/properties/request":                                                                                  {oracle: "judge-input.schema.json", site: "/properties/request"},
		"record.schema.json/properties/scope/items":                                                                              {oracle: "judge-output.schema.json", site: "/properties/scope/items"},
		"record.schema.json/properties/spec_version":                                                                             {owned: "[\"1.0\"]"},
		"subject-file.schema.json/$defs/check/properties/dimension":                                                              {oracle: "judge-input.schema.json", site: "/properties/required/items/properties/dimension"},
		"subject-file.schema.json/$defs/check/properties/predicate":                                                              {owned: "[\"exit_code\",\"stdout_equals\",\"stdout_json_equals\",\"stdout_contains\",\"stdout_not_contains\",\"stderr_contains\",\"stdout_bytes_at_most\",\"json_path_equals\",\"row_count\",\"stdout_empty\"]"},
		"subject-file.schema.json/$defs/command/properties/class/oneOf/0":                                                        {owned: "[\"safe\",\"destructive\"]"},
		"subject-file.schema.json/$defs/dimension":                                                                               {oracle: "judge-input.schema.json", site: "/properties/required/items/properties/dimension"},
		"subject-file.schema.json/$defs/runbook/properties/steps/items/properties/checks/items/properties/dimension":             {oracle: "judge-input.schema.json", site: "/properties/required/items/properties/dimension"},
		"subject-file.schema.json/$defs/runbook/properties/steps/items/properties/checks/items/properties/predicate":             {owned: "[\"exit_code\",\"stdout_equals\",\"stdout_json_equals\",\"stdout_contains\",\"stdout_not_contains\",\"stderr_contains\",\"stdout_bytes_at_most\",\"json_path_equals\",\"row_count\",\"stdout_empty\"]"},
		"subject-file.schema.json/$defs/step/properties/checks/items/properties/dimension":                                       {oracle: "judge-input.schema.json", site: "/properties/required/items/properties/dimension"},
		"subject-file.schema.json/$defs/step/properties/checks/items/properties/predicate":                                       {owned: "[\"exit_code\",\"stdout_equals\",\"stdout_json_equals\",\"stdout_contains\",\"stdout_not_contains\",\"stderr_contains\",\"stdout_bytes_at_most\",\"json_path_equals\",\"row_count\",\"stdout_empty\"]"},
		"subject-file.schema.json/properties/commands/items/properties/class/oneOf/0":                                            {owned: "[\"safe\",\"destructive\"]"},
		"subject-file.schema.json/properties/facts/properties/bounding/oneOf/0":                                                  {oracle: "judge-input.schema.json", site: "/properties/subject_facts/properties/bounding"},
		"subject-file.schema.json/properties/markers/items/properties/dimension":                                                 {oracle: "judge-input.schema.json", site: "/properties/required/items/properties/dimension"},
		"subject-file.schema.json/properties/runbooks/items/properties/steps/items/properties/checks/items/properties/dimension": {oracle: "judge-input.schema.json", site: "/properties/required/items/properties/dimension"},
		"subject-file.schema.json/properties/runbooks/items/properties/steps/items/properties/checks/items/properties/predicate": {owned: "[\"exit_code\",\"stdout_equals\",\"stdout_json_equals\",\"stdout_contains\",\"stdout_not_contains\",\"stderr_contains\",\"stdout_bytes_at_most\",\"json_path_equals\",\"row_count\",\"stdout_empty\"]"},
		"subject-file.schema.json/properties/secret/oneOf/2/if/properties/kind":                                                  {owned: "[\"seeded_state\"]"},
		"subject-file.schema.json/properties/secret/oneOf/2/properties/kind":                                                     {owned: "[\"credential\",\"seeded_state\"]"},
		"subject-file.schema.json/properties/subject_file_version":                                                               {owned: "[\"1.0\"]"},
	}
	oracles := map[string]map[string][]any{}
	for _, name := range []string{"judge-input.schema.json", "judge-output.schema.json"} {
		var v any
		decode(t, read(t, filepath.Join(sealed, "schema", name)), &v)
		sites := map[string][]any{}
		enumSites(t, v, v, "", sites, nil)
		oracles[name] = sites
	}
	files, err := filepath.Glob(filepath.Join(kit, "schema", "*.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no schemas")
	}
	seen := map[string]bool{}
	for _, file := range files {
		var v any
		decode(t, read(t, file), &v)
		sites := map[string][]any{}
		enumSites(t, v, v, "", sites, nil)
		name := filepath.Base(file)
		for site, values := range sites {
			key := name + site
			seen[key] = true
			rule, ok := policies[key]
			if oracles[name] != nil {
				rule = policy{oracle: name, site: site}
				ok = true
			}
			if !ok {
				t.Errorf("unmapped enum site %s", key)
				continue
			}
			if rule.owned != "" {
				var expected []any
				decode(t, []byte(rule.owned), &expected)
				if !reflect.DeepEqual(values, expected) {
					t.Errorf("%s kit-owned values differ: %v", key, values)
				}
				continue
			}
			expected := oracles[rule.oracle][rule.site]
			if len(expected) == 0 {
				t.Errorf("missing judge oracle %s %s", rule.oracle, rule.site)
			}
			for _, value := range values {
				if !slices.ContainsFunc(expected, func(want any) bool { return reflect.DeepEqual(want, value) }) {
					t.Errorf("%s enum value %v absent from sealed judge site", key, value)
				}
			}
		}
	}
	for key := range policies {
		if !seen[key] {
			t.Errorf("enum policy path no longer resolves: %s", key)
		}
	}
}
func TestExampleRecords(t *testing.T) {
	var subject struct {
		Commands []struct {
			Words []string `json:"words"`
			Bound int      `json:"byte_bound"`
			Codes []int    `json:"exit_codes"`
		} `json:"commands"`
	}
	decode(t, read(t, filepath.Join(kit, "examples", "subject-file.json")), &subject)
	if len(subject.Commands) != 1 || !reflect.DeepEqual(subject.Commands[0].Words, []string{"list"}) || subject.Commands[0].Bound == 0 || len(subject.Commands[0].Codes) == 0 {
		t.Fatal("example must declare both probes on list")
	}

	files, err := filepath.Glob(filepath.Join(kit, "examples", "record-*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no records")
	}
	for _, file := range files {
		t.Run(strings.ReplaceAll(filepath.Base(file), ".", "_"), func(t *testing.T) {
			var record struct {
				Input    json.RawMessage `json:"judge_input"`
				Output   any             `json:"judge_output"`
				ExitCode int             `json:"exit_code"`
				Rejected bool            `json:"judge_rejected"`
			}
			decode(t, read(t, file), &record)
			output, code := judge.Judge(record.Input)
			var got map[string]any
			decode(t, output, &got)
			if record.Rejected != (got["outcome"] == "rejected") {
				t.Error("judge_rejected differs from judge outcome")
			}
			expectedCode := record.ExitCode
			if record.Rejected {
				expectedCode = 2
				if record.ExitCode != 70 {
					t.Errorf("rejected record exit code %d, want 70", record.ExitCode)
				}
			}
			if code != expectedCode {
				t.Errorf("exit code %d, record expects %d", code, expectedCode)
			}
			if !reflect.DeepEqual(got, record.Output) {
				t.Errorf("judge output differs from record")
			}
			var input struct {
				Required []struct {
					ID        string `json:"id"`
					Dimension string `json:"dimension"`
				} `json:"required"`
			}
			decode(t, record.Input, &input)
			ids := map[string]bool{}
			probes := map[string]string{}
			for _, check := range input.Required {
				if ids[check.ID] {
					t.Errorf("duplicate required id %q", check.ID)
				}
				ids[check.ID] = true
				probes[check.Dimension] = check.ID
			}
			var evidence struct {
				Evidence []struct {
					ID       string         `json:"check_id"`
					Expected map[string]any `json:"expected"`
				} `json:"evidence"`
			}
			decode(t, record.Input, &evidence)
			matched := map[string]bool{}
			for _, item := range evidence.Evidence {
				if item.ID == probes["bounding"] && item.Expected["bound"] == float64(subject.Commands[0].Bound) {
					matched["bounding"] = true
				}
				codes, err := json.Marshal(subject.Commands[0].Codes)
				if err != nil {
					t.Fatal(err)
				}
				var expectedCodes any
				decode(t, codes, &expectedCodes)
				if item.ID == probes["execution"] && reflect.DeepEqual(item.Expected["exit_codes"], expectedCodes) {
					matched["execution"] = true
				}
			}
			if !matched["bounding"] || !matched["execution"] {
				t.Error("required probes must have evidence matching list bounds and exit codes")
			}
			if probes["bounding"] == "" || probes["execution"] == "" || probes["bounding"] == probes["execution"] {
				t.Error("byte bound and exit codes require distinct ids")
			}
		})
	}
}
