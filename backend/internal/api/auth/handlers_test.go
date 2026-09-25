package auth

import (
	"reflect"
	"testing"
)

func TestEmailDomainAllowed(t *testing.T) {
	if !emailDomainAllowed("Alice@Example.COM", []string{"  @example.com  "}) {
		t.Error("emailDomainAllowed() rejected case-insensitive matching domain")
	}
	for _, email := range []string{"", "alice", "alice@", "@example.com", "alice@other.com"} {
		if emailDomainAllowed(email, []string{"example.com"}) {
			t.Errorf("emailDomainAllowed(%q) = true, want false", email)
		}
	}
}

func TestOIDCRoleForGroups(t *testing.T) {
	mapping := map[string]string{"readers": "viewer", "admins": "operator", "invalid": "unknown"}
	for _, tc := range []struct {
		name   string
		groups []string
		want   string
	}{
		{name: "default", want: "user"},
		{name: "viewer", groups: []string{"readers"}, want: "user"},
		{name: "operator takes precedence", groups: []string{"readers", "admins"}, want: "admin"},
		{name: "unknown ignored", groups: []string{"invalid"}, want: "user"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := oidcRoleForGroups(tc.groups, mapping); got != tc.want {
				t.Fatalf("oidcRoleForGroups(%v) = %q, want %q", tc.groups, got, tc.want)
			}
		})
	}
}

func TestOIDCConnectorRolesForGroups(t *testing.T) {
	allConnectorIDs := []string{"c1", "c2", "c3"}

	t.Run("wildcard expands to every connector", func(t *testing.T) {
		mapping := map[string]map[string]string{"ops": {"*": "operator"}}
		got := oidcConnectorRolesForGroups([]string{"ops"}, mapping, allConnectorIDs)
		want := map[string]string{"c1": "operator", "c2": "operator", "c3": "operator"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("oidcConnectorRolesForGroups() = %v, want %v", got, want)
		}
	})

	t.Run("highest role wins across groups", func(t *testing.T) {
		mapping := map[string]map[string]string{
			"viewers":   {"c1": "viewer"},
			"operators": {"c1": "operator"},
		}
		got := oidcConnectorRolesForGroups([]string{"viewers", "operators"}, mapping, allConnectorIDs)
		if got["c1"] != "operator" {
			t.Fatalf("oidcConnectorRolesForGroups()[c1] = %q, want operator", got["c1"])
		}
	})

	t.Run("highest role wins regardless of group order", func(t *testing.T) {
		mapping := map[string]map[string]string{
			"viewers":   {"c1": "viewer"},
			"operators": {"c1": "operator"},
		}
		got := oidcConnectorRolesForGroups([]string{"operators", "viewers"}, mapping, allConnectorIDs)
		if got["c1"] != "operator" {
			t.Fatalf("oidcConnectorRolesForGroups()[c1] = %q, want operator", got["c1"])
		}
	})

	t.Run("group names match case-insensitively", func(t *testing.T) {
		// Viper lowercases config map keys, so the mapping key is always
		// lowercase; the IdP's claimed group name may not be.
		mapping := map[string]map[string]string{"homelab-ops": {"c1": "operator"}}
		got := oidcConnectorRolesForGroups([]string{"Homelab-Ops"}, mapping, allConnectorIDs)
		if got["c1"] != "operator" {
			t.Fatalf("oidcConnectorRolesForGroups() case-insensitive match = %v, want c1=operator", got)
		}
	})

	t.Run("unknown connector ids pass through untouched", func(t *testing.T) {
		// Filtering unknown IDs against the live connector list is the
		// caller's job (see syncOIDCConnectorGrants); this function only maps
		// groups to whatever connector IDs the config names.
		mapping := map[string]map[string]string{"family": {"does-not-exist": "viewer"}}
		got := oidcConnectorRolesForGroups([]string{"family"}, mapping, allConnectorIDs)
		want := map[string]string{"does-not-exist": "viewer"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("oidcConnectorRolesForGroups() = %v, want %v", got, want)
		}
	})

	t.Run("no matching groups yields no grants", func(t *testing.T) {
		mapping := map[string]map[string]string{"ops": {"*": "operator"}}
		got := oidcConnectorRolesForGroups([]string{"other-group"}, mapping, allConnectorIDs)
		if len(got) != 0 {
			t.Fatalf("oidcConnectorRolesForGroups() = %v, want empty", got)
		}
	})

	t.Run("empty mapping yields no grants", func(t *testing.T) {
		got := oidcConnectorRolesForGroups([]string{"ops"}, nil, allConnectorIDs)
		if len(got) != 0 {
			t.Fatalf("oidcConnectorRolesForGroups() = %v, want empty", got)
		}
	})

	t.Run("invalid role value is ignored", func(t *testing.T) {
		mapping := map[string]map[string]string{"ops": {"c1": "not-a-role"}}
		got := oidcConnectorRolesForGroups([]string{"ops"}, mapping, allConnectorIDs)
		if len(got) != 0 {
			t.Fatalf("oidcConnectorRolesForGroups() = %v, want empty (invalid role skipped)", got)
		}
	})
}
