package npm

import (
	"encoding/json"
	"testing"
)

func TestFlexBoolAcceptsOnlyBooleanAndLegacyIntegerValues(t *testing.T) {
	for input, want := range map[string]bool{"true": true, "false": false, "0": false, "1": true, " \t1\n": true, " false ": false} {
		var got flexBool
		if err := json.Unmarshal([]byte(input), &got); err != nil || bool(got) != want {
			t.Errorf("UnmarshalJSON(%q) = %v, %v; want %v", input, got, err, want)
		}
	}
	for _, input := range []string{"2", "-1", "1.0", `"true"`, "null", "{}"} {
		var got flexBool
		if err := json.Unmarshal([]byte(input), &got); err == nil {
			t.Errorf("UnmarshalJSON(%q) unexpectedly succeeded", input)
		}
	}
}
