package opnsense

import (
	"reflect"
	"strings"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/connector"
)

func TestBuildInterfaceTableAttributes(t *testing.T) {
	data := []byte(`{"rows":[
		{"device":"igb0","ipaddr":"203.0.113.5","ipv6":"2001:db8::1","status":"up","media":"1000baseT","enabled":true,"type":"static","gateway":"WAN_GW"},
		{"device":"igb1","ipaddr":"10.0.0.1","status":"up","media":"1000baseT","enabled":false,"type":"static"}
	]}`)
	_, entities := buildInterfaceTable(data)
	if len(entities) != 2 {
		t.Fatalf("entities = %+v, want 2", entities)
	}
	want := []map[string]any{
		{"enabled": true, "ipv4": "203.0.113.5", "ipv6": "2001:db8::1", "type": "static", "gateway": "WAN_GW"},
		{"enabled": false, "ipv4": "10.0.0.1", "ipv6": "", "type": "static"},
	}
	for i, w := range want {
		if !reflect.DeepEqual(entities[i].Attributes, w) {
			t.Errorf("entities[%d].Attributes = %+v, want %+v", i, entities[i].Attributes, w)
		}
	}
}

func TestBuildInterfaceTableIdentity(t *testing.T) {
	data := []byte(`{"rows":[
		{"identifier":"wan","device":"igb0","ipaddr":"203.0.113.5","status":"up","enabled":true,"type":"static"},
		{"identifier":"lan","ipaddr":"10.0.0.1","status":"up","enabled":true,"type":"static"}
	]}`)
	_, entities := buildInterfaceTable(data)
	if len(entities) != 2 {
		t.Fatalf("entities = %+v, want 2", entities)
	}
	if entities[0].Name != "igb0" || entities[0].ExternalID != "igb0" {
		t.Errorf("device interface identity = name %q, id %q; want both %q", entities[0].Name, entities[0].ExternalID, "igb0")
	}
	if entities[1].ExternalID == "" || !strings.HasPrefix(entities[1].ExternalID, "fallback:interface:") {
		t.Errorf("interface without device ExternalID = %q, want a fallback identity", entities[1].ExternalID)
	}

	changed := []byte(`{"rows":[
		{"identifier":"lan","ipaddr":"10.0.0.8","status":"down","enabled":false,"type":"static"}
	]}`)
	_, changedEntities := buildInterfaceTable(changed)
	if len(changedEntities) != 1 || changedEntities[0].ExternalID != entities[1].ExternalID {
		t.Errorf("fallback identity changed with interface state: before %q, after %+v", entities[1].ExternalID, changedEntities)
	}
}

func TestBuildRuleTableIdentity(t *testing.T) {
	data := []byte(`{"rows":[
		{"uuid":"f4cba8a1-0c93-4cb2-9c5c-821331233db9","description":"Allow SSH","action":"pass","protocol":"tcp","ipprotocol":"inet","source_net":"any","source_port":"","destination_net":"any","destination_port":"22","interface":"wan","direction":"in","enabled":"1","log":"1"},
		{"description":"Duplicate","action":"block","protocol":"udp","ipprotocol":"inet","source_net":"10.0.0.0/8","source_port":"53","destination_net":"any","destination_port":"","interface":"lan","direction":"in","enabled":"1","log":"0"},
		{"description":"Duplicate","action":"pass","protocol":"udp","ipprotocol":"inet6","source_net":"10.0.0.0/8","source_port":"53","destination_net":"any","destination_port":"","interface":"lan","direction":"in","enabled":"1","log":"0"},
		{"description":"","action":"pass","protocol":"tcp","ipprotocol":"inet","source_net":"192.0.2.0/24","source_port":"","destination_net":"any","destination_port":"443","interface":"wan","direction":"in","enabled":"1","log":"0"}
	]}`)
	_, entities := buildRuleTable(data)
	if len(entities) != 4 {
		t.Fatalf("entities = %+v, want 4", entities)
	}
	if entities[0].Name != "Allow SSH" || entities[0].ExternalID != "f4cba8a1-0c93-4cb2-9c5c-821331233db9" {
		t.Errorf("UUID rule identity = name %q, id %q", entities[0].Name, entities[0].ExternalID)
	}
	if entities[1].ExternalID == entities[2].ExternalID || entities[1].ExternalID == "" || entities[2].ExternalID == "" {
		t.Errorf("same-description rules need distinct fallback identities: %q and %q", entities[1].ExternalID, entities[2].ExternalID)
	}
	if entities[3].Name != "" || entities[3].ExternalID == "" {
		t.Errorf("nameless rule identity = name %q, id %q; want preserved empty name and non-empty id", entities[3].Name, entities[3].ExternalID)
	}

	changedAndReordered := []byte(`{"rows":[
		{"description":"renamed","action":"pass","protocol":"tcp","ipprotocol":"inet","source_net":"192.0.2.0/24","source_port":"","destination_net":"any","destination_port":"443","interface":"wan","direction":"in","enabled":"0","log":"1"},
		{"description":"Duplicate","action":"pass","protocol":"udp","ipprotocol":"inet6","source_net":"10.0.0.0/8","source_port":"53","destination_net":"any","destination_port":"","interface":"lan","direction":"in","enabled":"0","log":"1"},
		{"description":"Duplicate","action":"block","protocol":"udp","ipprotocol":"inet","source_net":"10.0.0.0/8","source_port":"53","destination_net":"any","destination_port":"","interface":"lan","direction":"in","enabled":"0","log":"1"},
		{"uuid":"f4cba8a1-0c93-4cb2-9c5c-821331233db9","description":"Renamed SSH","action":"pass","protocol":"tcp","ipprotocol":"inet","source_net":"any","source_port":"","destination_net":"any","destination_port":"22","interface":"wan","direction":"in","enabled":"0","log":"0"}
	]}`)
	_, changedEntities := buildRuleTable(changedAndReordered)
	if len(changedEntities) != 4 {
		t.Fatalf("changed entities = %+v, want 4", changedEntities)
	}
	for _, pair := range [][2]int{{3, 0}, {2, 1}, {1, 2}} {
		before, after := entities[pair[0]], changedEntities[pair[1]]
		if after.ExternalID != before.ExternalID {
			t.Errorf("fallback id for rule %q = %q, want stable id %q", after.Name, after.ExternalID, before.ExternalID)
		}
	}
	if changedEntities[3].ExternalID != entities[0].ExternalID {
		t.Errorf("UUID changed when display/state fields changed: before %q, after %q", entities[0].ExternalID, changedEntities[3].ExternalID)
	}
}

func TestBuildRuleFallbackIdentityUsesIPProtocolAndPorts(t *testing.T) {
	data := []byte(`{"rows":[
		{"description":"Same label","action":"pass","protocol":"tcp","ipprotocol":"inet","source_net":"any","source_port":"53","destination_net":"any","destination_port":"443","interface":"wan","direction":"in"},
		{"description":"Same label","action":"pass","protocol":"tcp","ipprotocol":"inet","source_net":"any","source_port":"54","destination_net":"any","destination_port":"443","interface":"wan","direction":"in"},
		{"description":"Same label","action":"pass","protocol":"tcp","ipprotocol":"inet6","source_net":"any","source_port":"53","destination_net":"any","destination_port":"443","interface":"wan","direction":"in"}
	]}`)
	_, entities := buildRuleTable(data)
	if len(entities) != 3 {
		t.Fatalf("entities = %+v, want 3", entities)
	}
	for i := range entities {
		for j := 0; j < i; j++ {
			if entities[i].ExternalID == entities[j].ExternalID {
				t.Errorf("rules differing by IP protocol or port share ExternalID %q", entities[i].ExternalID)
			}
		}
	}
}

func TestBuildRuleFallbackIdentityDistinguishesIdenticalRows(t *testing.T) {
	first := []byte(`{"rows":[
		{"description":"First label","action":"pass","protocol":"tcp","ipprotocol":"inet","source_net":"any","source_port":"","destination_net":"any","destination_port":"443","interface":"wan","direction":"in"},
		{"description":"Second label","action":"pass","protocol":"tcp","ipprotocol":"inet","source_net":"any","source_port":"","destination_net":"any","destination_port":"443","interface":"wan","direction":"in"}
	]}`)
	reordered := []byte(`{"rows":[
		{"description":"Second label","action":"pass","protocol":"tcp","ipprotocol":"inet","source_net":"any","source_port":"","destination_net":"any","destination_port":"443","interface":"wan","direction":"in"},
		{"description":"First label","action":"pass","protocol":"tcp","ipprotocol":"inet","source_net":"any","source_port":"","destination_net":"any","destination_port":"443","interface":"wan","direction":"in"}
	]}`)
	_, firstEntities := buildRuleTable(first)
	_, reorderedEntities := buildRuleTable(reordered)
	if len(firstEntities) != 2 || len(reorderedEntities) != 2 {
		t.Fatalf("first entities = %d, reordered entities = %d; want 2 each", len(firstEntities), len(reorderedEntities))
	}
	if firstEntities[0].ExternalID == firstEntities[1].ExternalID || firstEntities[0].ExternalID == "" || firstEntities[1].ExternalID == "" {
		t.Fatalf("identical fallback rows need distinct non-empty IDs: %+v", firstEntities)
	}
	for i := range firstEntities {
		if firstEntities[i].ExternalID != reorderedEntities[i].ExternalID {
			t.Errorf("duplicate fallback ID at occurrence %d changed after reorder: %q -> %q", i, firstEntities[i].ExternalID, reorderedEntities[i].ExternalID)
		}
	}
	if reorderedEntities[0].Name != "Second label" || reorderedEntities[1].Name != "First label" {
		t.Errorf("display names = %q, %q; want upstream order preserved", reorderedEntities[0].Name, reorderedEntities[1].Name)
	}
}

func TestBuildRuleFallbackIdentityIncludesOtherMatchFields(t *testing.T) {
	first := []byte(`{"rows":[
		{"description":"Same name","action":"pass","protocol":"tcp","source_net":"any","destination_net":"any","tagged":"prod","tcpflags":"ACK","sequence":"10"},
		{"description":"Same name","action":"pass","protocol":"tcp","source_net":"any","destination_net":"any","tagged":"dev","tcpflags":"SYN","sequence":"20"}
	]}`)
	reordered := []byte(`{"rows":[
		{"description":"Renamed","action":"pass","protocol":"tcp","source_net":"any","destination_net":"any","tagged":"dev","tcpflags":"SYN","sequence":"30"},
		{"description":"Renamed","action":"pass","protocol":"tcp","source_net":"any","destination_net":"any","tagged":"prod","tcpflags":"ACK","sequence":"40"}
	]}`)
	_, before := buildRuleTable(first)
	_, after := buildRuleTable(reordered)
	if len(before) != 2 || len(after) != 2 {
		t.Fatalf("entity counts = %d and %d, want 2 each", len(before), len(after))
	}
	if before[0].ExternalID == before[1].ExternalID {
		t.Fatal("rules with different tags and TCP flags share an identity")
	}
	if before[0].ExternalID != after[1].ExternalID || before[1].ExternalID != after[0].ExternalID {
		t.Fatal("fallback identities changed after reorder, sequence and description edits")
	}
}

func TestBuildRuleTableAttributes(t *testing.T) {
	data := []byte(`{"rows":[
		{"description":"Allow SSH","action":"pass","protocol":"tcp","source_net":"any","destination_net":"any","destination_port":"22","interface":"wan","direction":"in","enabled":"1","log":"1"},
		{"description":"Block DNS","action":"block","protocol":"udp","source_net":"10.0.0.0/8","destination_net":"any","enabled":"0","disabled_reason":"maintenance"}
	]}`)
	_, entities := buildRuleTable(data)
	if len(entities) != 2 {
		t.Fatalf("entities = %+v, want 2", entities)
	}

	sshAttrs := entities[0].Attributes
	if sshAttrs["enabled"] != true || sshAttrs["action"] != "pass" || sshAttrs["protocol"] != "tcp" ||
		sshAttrs["source"] != "any" || sshAttrs["destination"] != "any" || sshAttrs["destination_port"] != "22" ||
		sshAttrs["interface"] != "wan" || sshAttrs["direction"] != "in" || sshAttrs["log"] != true {
		t.Errorf("Allow SSH attributes = %+v", sshAttrs)
	}
	if _, ok := sshAttrs["disabled_reason"]; ok {
		t.Errorf("Allow SSH attributes should omit disabled_reason when absent: %+v", sshAttrs)
	}

	dnsAttrs := entities[1].Attributes
	if dnsAttrs["enabled"] != false || dnsAttrs["action"] != "block" || dnsAttrs["disabled_reason"] != "maintenance" || dnsAttrs["log"] != false {
		t.Errorf("Block DNS attributes = %+v", dnsAttrs)
	}
	if _, ok := dnsAttrs["interface"]; ok {
		t.Errorf("Block DNS attributes should omit interface when absent: %+v", dnsAttrs)
	}
}

// TestAttributeCatalogCoversEmittedKeys ensures every attribute key this
// connector emits for interface/rule entities is declared in its catalog
// with a matching type.
func TestAttributeCatalogCoversEmittedKeys(t *testing.T) {
	catalog := attributeCatalog
	emitted := map[string]map[string]string{
		"interface": {"enabled": "boolean", "type": "string", "ipv4": "string", "ipv6": "string", "gateway": "string"},
		"rule": {
			"enabled": "boolean", "action": "string", "interface": "string", "direction": "string",
			"protocol": "string", "source": "string", "destination": "string", "destination_port": "string",
			"log": "boolean", "disabled_reason": "string",
		},
	}
	for kind, keys := range emitted {
		specs, ok := catalog[kind]
		if !ok {
			t.Fatalf("catalog missing entity kind %q", kind)
		}
		byName := make(map[string]connector.AttributeSpec, len(specs))
		for _, s := range specs {
			byName[s.Name] = s
		}
		for key, typ := range keys {
			spec, ok := byName[key]
			if !ok {
				t.Errorf("catalog[%q] missing emitted attribute %q", kind, key)
				continue
			}
			if spec.Type != typ {
				t.Errorf("catalog[%q][%q].Type = %q, want %q", kind, key, spec.Type, typ)
			}
		}
	}
}
