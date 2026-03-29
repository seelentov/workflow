package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Inventory describes all known hosts and groups.
//
// Example inventory.yaml:
//
//	hosts:
//	  - name: web1
//	    host: 192.168.1.10
//	    user: ubuntu
//	    key_file: ~/.ssh/id_rsa
//	  - name: web2
//	    host: 192.168.1.11
//	    user: root
//	    password: secret
//	groups:
//	  web:
//	    - web1
//	    - web2
type Inventory struct {
	Hosts  []*HostDef            `yaml:"hosts"`
	Groups map[string][]string   `yaml:"groups"`

	byName map[string]*HostDef
}

// HostDef is a single server entry in the inventory.
type HostDef struct {
	Name     string         `yaml:"name"`
	Host     string         `yaml:"host"`
	Port     int            `yaml:"port"`
	User     string         `yaml:"user"`
	Password string         `yaml:"password"`
	KeyFile  string         `yaml:"key_file"`
	Vars     map[string]any `yaml:"vars"`
}

func LoadInventory(path string) (*Inventory, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var inv Inventory
	if err := yaml.Unmarshal(data, &inv); err != nil {
		return nil, err
	}
	inv.byName = make(map[string]*HostDef, len(inv.Hosts))
	for _, h := range inv.Hosts {
		if h.Name == "" {
			h.Name = h.Host
		}
		if h.Port == 0 {
			h.Port = 22
		}
		if h.User == "" {
			h.User = "root"
		}
		inv.byName[h.Name] = h
	}
	return &inv, nil
}

// Resolve returns the list of hosts matching the target pattern.
// target can be "all", a group name, a host name, or a comma-separated mix.
func (inv *Inventory) Resolve(target string) ([]*HostDef, error) {
	if target == "" || target == "all" {
		return inv.Hosts, nil
	}

	seen := map[string]bool{}
	var result []*HostDef

	for _, part := range strings.Split(target, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// direct host name
		if h, ok := inv.byName[part]; ok {
			if !seen[h.Name] {
				seen[h.Name] = true
				result = append(result, h)
			}
			continue
		}

		// group name
		if names, ok := inv.Groups[part]; ok {
			for _, name := range names {
				h, ok := inv.byName[name]
				if !ok {
					return nil, fmt.Errorf("group %q references unknown host %q", part, name)
				}
				if !seen[h.Name] {
					seen[h.Name] = true
					result = append(result, h)
				}
			}
			continue
		}

		return nil, fmt.Errorf("unknown host or group: %q", part)
	}

	return result, nil
}
