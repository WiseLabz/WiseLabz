package doclink

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

const docID = "11111111-1111-4111-8111-111111111111"
const entityID = "22222222-2222-4222-8222-222222222222"

func TestResolveLinksAndExclusions(t *testing.T) {
	content := "[[Guide#install]] [[node:42|[node]]] \\[[Guide]]\n" +
		"`[[Guide]]`\n\n```md\n[[Guide]]\n```\n" +
		"![[Guide]]\n" +
		"<!-- wl:gen key=\"generated\" h=\"abcdef123456\" -->\n[[Guide]]\n<!-- /wl:gen -->\n"
	got, warnings, err := Resolve(content, func(name string) ([]Target, error) {
		switch name {
		case "Guide":
			return []Target{{Type: "doc", ID: docID, Label: "Guide"}}, nil
		case "node:42":
			return []Target{{Type: "entity", ID: entityID, Label: "Node 42"}}, nil
		default:
			t.Fatalf("unexpected lookup %q", name)
			return nil, nil
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	want := "[Guide](/docs/" + docID + ") [\\[node\\]](/entities/" + entityID + ") \\[[Guide]]\n" +
		"`[[Guide]]`\n\n```md\n[[Guide]]\n```\n" +
		"![[Guide]]\n" +
		"<!-- wl:gen key=\"generated\" h=\"abcdef123456\" -->\n[[Guide]]\n<!-- /wl:gen -->\n"
	if got != want {
		t.Fatalf("Resolve() = %q, want %q", got, want)
	}
	if len(warnings) != 0 {
		t.Fatalf("warnings = %v", warnings)
	}
}

func TestResolveUnresolvedAndAmbiguousPreserveText(t *testing.T) {
	input := "[[missing]] [[multiple]]"
	got, warnings, err := Resolve(input, func(name string) ([]Target, error) {
		if name == "multiple" {
			return []Target{{Type: "doc", ID: docID}, {Type: "doc", ID: entityID}}, nil
		}
		return nil, nil
	})
	if err != nil || got != input || len(warnings) != 2 || !strings.Contains(warnings[0], "unresolved") || !strings.Contains(warnings[1], "ambiguous") {
		t.Fatalf("Resolve() = %q, %v, %v", got, warnings, err)
	}
}

func TestResolveLookupErrorKeepsOriginal(t *testing.T) {
	wantErr := errors.New("lookup failed")
	input := "[[good]] [[broken]]"
	got, warnings, err := Resolve(input, func(name string) ([]Target, error) {
		if name == "broken" {
			return nil, wantErr
		}
		return []Target{{Type: "doc", ID: docID, Label: "Good"}}, nil
	})
	if got != input || warnings != nil || !errors.Is(err, wantErr) {
		t.Fatalf("Resolve() = %q, %v, %v", got, warnings, err)
	}
}

func TestExtractValidDistinctLinksOutsideExcludedSections(t *testing.T) {
	content := "[doc](/docs/" + docID + ") [same](/docs/" + docID + "#part) " +
		"[entity](</entities/" + entityID + ">) [bad](/docs/not-a-uuid) " +
		"![image](/docs/33333333-3333-4333-8333-333333333333) " +
		"\\[escaped](/docs/44444444-4444-4444-8444-444444444444) " +
		"[label with \\[brackets\\]](/docs/55555555-5555-4555-8555-555555555555)\n" +
		"`[code](/docs/33333333-3333-4333-8333-333333333333)`\n" +
		"~~~\n[code](/entities/44444444-4444-4444-8444-444444444444)\n~~~\n" +
		"<!-- wl:gen key=\"g\" h=\"abcdefabcdef\" -->\n[gen](/docs/55555555-5555-4555-8555-555555555555)\n<!-- /wl:gen -->"
	want := []Target{
		{Type: "doc", ID: docID},
		{Type: "entity", ID: entityID},
		{Type: "doc", ID: "55555555-5555-4555-8555-555555555555"},
	}
	if got := Extract(content); !reflect.DeepEqual(got, want) {
		t.Fatalf("Extract() = %#v, want %#v", got, want)
	}
}

func TestRewriteOutsideCodePreservesCodeAndNewlines(t *testing.T) {
	input := "a `b`\n~~~\nc\n~~~\nd\n"
	got := RewriteOutsideCode(input, func(s string) string { return strings.ToUpper(s) })
	want := "A `b`\n~~~\nc\n~~~\nD\n"
	if got != want {
		t.Fatalf("RewriteOutsideCode() = %q, want %q", got, want)
	}
}

func TestResolveUsesExactInlineAndFenceDelimiters(t *testing.T) {
	input := "``\n[[hidden]] ` tick `` [[shown]]\n``\n" +
		"````md\n[[hidden]]\n```\n[[still hidden]]\n````\n" +
		"[[outside]]\n"
	var lookups []string
	got, _, err := Resolve(input, func(name string) ([]Target, error) {
		lookups = append(lookups, name)
		return []Target{{Type: "doc", ID: docID, Label: name}}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	want := "``\n[[hidden]] ` tick `` [shown](/docs/" + docID + ")\n``\n" +
		"````md\n[[hidden]]\n```\n[[still hidden]]\n````\n" +
		"[outside](/docs/" + docID + ")\n"
	if got != want || !reflect.DeepEqual(lookups, []string{"shown", "outside"}) {
		t.Fatalf("Resolve() = %q, lookups %v", got, lookups)
	}
}

func TestResolveSkipsMultilineCodeSpans(t *testing.T) {
	input := "[[before]] `code begins\n[[inside]]\nand ends` [[after]]"
	got, warnings, err := Resolve(input, func(name string) ([]Target, error) {
		return []Target{{Type: "doc", ID: docID, Label: name}}, nil
	})
	if err != nil || len(warnings) != 0 {
		t.Fatalf("Resolve() warnings=%v err=%v", warnings, err)
	}
	want := "[before](/docs/" + docID + ") `code begins\n[[inside]]\nand ends` [after](/docs/" + docID + ")"
	if got != want {
		t.Fatalf("Resolve() = %q, want %q", got, want)
	}
}

func TestExtractMarkdownCodeLabelsReferencesAndGeneratedLinks(t *testing.T) {
	input := "[`Guide`](/docs/" + docID + ") [Use `docker`](/docs/" + docID + ")\n\n" +
		"[reference][target]\n\n[target]: /entities/" + entityID + "\n\n" +
		"    [code](/docs/33333333-3333-4333-8333-333333333333)\n\n" +
		"> ~~~\n> [code](/docs/33333333-3333-4333-8333-333333333333)\n> ~~~\n" +
		"<!-- wl:gen key=\"g\" h=\"abcdefabcdef\" -->\n[gen](/docs/44444444-4444-4444-8444-444444444444)\n<!-- /wl:gen -->"
	want := []Target{{Type: "doc", ID: docID}, {Type: "entity", ID: entityID}, {Type: "doc", ID: "44444444-4444-4444-8444-444444444444"}}
	if got := Extract(input); !reflect.DeepEqual(got, want) {
		t.Fatalf("Extract = %#v, want %#v", got, want)
	}
}

func TestResolveSkipsNestedAndIndentedCode(t *testing.T) {
	input := "> ~~~\n> [[hidden]]\n> ~~~\n\n    [[hidden]]\n\n- example:\n\n      [[hidden]]\n\n`[[hidden]]\\`\n\n[[shown]]"
	lookups := []string{}
	_, _, err := Resolve(input, func(name string) ([]Target, error) {
		lookups = append(lookups, name)
		return []Target{{Type: "doc", ID: docID, Label: "Guide"}}, nil
	})
	if err != nil || !reflect.DeepEqual(lookups, []string{"shown"}) {
		t.Fatalf("lookups=%v error=%v", lookups, err)
	}
}

func TestResolveLargeMalformedInput(t *testing.T) {
	input := strings.Repeat("[", 100000)
	got, warnings, err := Resolve(input, func(string) ([]Target, error) { t.Fatal("malformed input looked up a link"); return nil, nil })
	if err != nil || got != input || len(warnings) != 0 {
		t.Fatalf("malformed input changed or returned error: %v", err)
	}
}
