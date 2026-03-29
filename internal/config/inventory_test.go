package config

import (
	"os"
	"testing"
)

// writeTempFile creates a temporary file with the given content and registers cleanup.
func writeTempFile(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp("", "workflow-test-*.yaml")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	t.Cleanup(func() { os.Remove(f.Name()) })
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	f.Close()
	return f.Name()
}

// makeInventory creates a test inventory with two hosts in group "web".
func makeInventory() *Inventory {
	h1 := &HostDef{Name: "web1", Host: "192.168.1.10", Port: 22, User: "root"}
	h2 := &HostDef{Name: "web2", Host: "192.168.1.11", Port: 22, User: "root"}
	return &Inventory{
		Hosts:  []*HostDef{h1, h2},
		Groups: map[string][]string{"web": {"web1", "web2"}},
		byName: map[string]*HostDef{"web1": h1, "web2": h2},
	}
}

func TestLoadInventory_Success(t *testing.T) {
	content := `
hosts:
  - name: web1
    host: 192.168.1.10
    user: ubuntu
    port: 2222
  - name: web2
    host: 192.168.1.11
groups:
  web:
    - web1
    - web2
`
	f := writeTempFile(t, content)
	inv, err := LoadInventory(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(inv.Hosts) != 2 {
		t.Fatalf("want 2 hosts, got %d", len(inv.Hosts))
	}
	if inv.Hosts[0].Port != 2222 {
		t.Errorf("want port 2222, got %d", inv.Hosts[0].Port)
	}
	if inv.Hosts[1].Port != 22 {
		t.Errorf("want default port 22, got %d", inv.Hosts[1].Port)
	}
	if inv.Hosts[1].User != "root" {
		t.Errorf("want default user root, got %s", inv.Hosts[1].User)
	}
	if len(inv.Groups["web"]) != 2 {
		t.Errorf("want 2 hosts in group web, got %d", len(inv.Groups["web"]))
	}
}

func TestLoadInventory_Defaults(t *testing.T) {
	content := `
hosts:
  - host: 10.0.0.1
`
	f := writeTempFile(t, content)
	inv, err := LoadInventory(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	h := inv.Hosts[0]
	if h.Name != "10.0.0.1" {
		t.Errorf("want name=host when name is empty, got %s", h.Name)
	}
	if h.Port != 22 {
		t.Errorf("want default port 22, got %d", h.Port)
	}
	if h.User != "root" {
		t.Errorf("want default user root, got %s", h.User)
	}
}

func TestLoadInventory_ByNameIndex(t *testing.T) {
	content := `
hosts:
  - name: srv1
    host: 10.0.0.1
`
	f := writeTempFile(t, content)
	inv, err := LoadInventory(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	hosts, err := inv.Resolve("srv1")
	if err != nil {
		t.Fatalf("resolve by name: %v", err)
	}
	if len(hosts) != 1 || hosts[0].Name != "srv1" {
		t.Errorf("expected host srv1 via byName index")
	}
}

func TestLoadInventory_FileNotFound(t *testing.T) {
	_, err := LoadInventory("/nonexistent/path/inventory.yaml")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestLoadInventory_InvalidYAML(t *testing.T) {
	f := writeTempFile(t, "{unclosed")
	_, err := LoadInventory(f)
	if err == nil {
		t.Fatal("expected error for invalid YAML, got nil")
	}
}

func TestLoadInventory_WithVars(t *testing.T) {
	content := `
hosts:
  - name: app1
    host: 10.0.0.1
    vars:
      env: production
      port: 8080
`
	f := writeTempFile(t, content)
	inv, err := LoadInventory(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	h := inv.Hosts[0]
	if h.Vars["env"] != "production" {
		t.Errorf("want vars.env=production, got %v", h.Vars["env"])
	}
}

func TestResolve_All(t *testing.T) {
	inv := makeInventory()
	hosts, err := inv.Resolve("all")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hosts) != 2 {
		t.Errorf("want 2 hosts, got %d", len(hosts))
	}
}

func TestResolve_Empty(t *testing.T) {
	inv := makeInventory()
	hosts, err := inv.Resolve("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hosts) != 2 {
		t.Errorf("want 2 hosts for empty target, got %d", len(hosts))
	}
}

func TestResolve_ByName(t *testing.T) {
	inv := makeInventory()
	hosts, err := inv.Resolve("web1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hosts) != 1 || hosts[0].Name != "web1" {
		t.Errorf("expected single host web1, got %v", hosts)
	}
}

func TestResolve_ByGroup(t *testing.T) {
	inv := makeInventory()
	hosts, err := inv.Resolve("web")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hosts) != 2 {
		t.Errorf("want 2 hosts in group web, got %d", len(hosts))
	}
}

func TestResolve_CSV(t *testing.T) {
	inv := makeInventory()
	hosts, err := inv.Resolve("web1, web2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hosts) != 2 {
		t.Errorf("want 2 hosts from CSV, got %d", len(hosts))
	}
}

func TestResolve_Deduplication(t *testing.T) {
	inv := makeInventory()
	// web1 appears directly and also via group "web"
	hosts, err := inv.Resolve("web1, web")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hosts) != 2 {
		t.Errorf("want 2 unique hosts after dedup, got %d", len(hosts))
	}
}

func TestResolve_UnknownHost(t *testing.T) {
	inv := makeInventory()
	_, err := inv.Resolve("nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown host")
	}
}

func TestResolve_GroupWithUnknownMember(t *testing.T) {
	h1 := &HostDef{Name: "web1", Host: "1.1.1.1", Port: 22, User: "root"}
	inv := &Inventory{
		Hosts:  []*HostDef{h1},
		Groups: map[string][]string{"bad": {"web1", "ghost"}},
		byName: map[string]*HostDef{"web1": h1},
	}
	_, err := inv.Resolve("bad")
	if err == nil {
		t.Fatal("expected error for group referencing unknown host")
	}
}
