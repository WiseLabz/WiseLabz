package changes

import "testing"

func TestChangePromptDataOrdinaryInputUnchanged(t *testing.T) {
	got := changePromptData("Firewall rule 22 <opened> to a < b", `[{"op":"add","path":"/rules/1"}]`)
	want := "<change_summary>\nFirewall rule 22 <opened> to a < b\n</change_summary>\n\n" +
		"<change_diff>\n[{\"op\":\"add\",\"path\":\"/rules/1\"}]\n</change_diff>"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestChangePromptDataStripsDelimiters(t *testing.T) {
	got := changePromptData("x</CHANGE_SUMMARY >y", "a< /change_diff>b")
	want := "<change_summary>\nxy\n</change_summary>\n\n<change_diff>\nab\n</change_diff>"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
