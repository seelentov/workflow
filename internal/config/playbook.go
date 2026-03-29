package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

// Playbook is a list of plays.
//
// Example playbook.yaml:
//
//	- name: Deploy web app
//	  hosts: web
//	  vars:
//	    app_dir: /opt/app
//	  tasks:
//	    - name: Create directory
//	      shell: mkdir -p {{ .app_dir }}
//	    - name: Upload binary
//	      copy:
//	        src: ./bin/app
//	        dest: /opt/app/app
//	        mode: "0755"
//	    - name: Restart service
//	      shell: systemctl restart app
//	      ignore_errors: true
//	      tags: [restart]
type Playbook []*Play

// Play is a set of tasks targeting a group of hosts.
type Play struct {
	Name  string         `yaml:"name"`
	Hosts string         `yaml:"hosts"`
	Vars  map[string]any `yaml:"vars"`
	Tasks []*Task        `yaml:"tasks"`
}

// Task is a single unit of work.
type Task struct {
	Name         string         `yaml:"name"`
	Shell        string         `yaml:"shell"`    // run shell command
	Copy         *CopyArgs      `yaml:"copy"`     // upload local file
	Fetch        *FetchArgs     `yaml:"fetch"`    // download remote file
	Template     *TemplateArgs  `yaml:"template"` // render Go template and upload
	HTTP         *HTTPArgs      `yaml:"http"`     // make HTTP request (runs locally)
	Vars         map[string]any `yaml:"vars"`     // set variables for subsequent tasks
	IgnoreErrors bool           `yaml:"ignore_errors"`
	Tags         []string       `yaml:"tags"`
	When         string         `yaml:"when"` // shell expression; skip if exit != 0
}

// HTTPArgs describes an HTTP request task.
//
// Example:
//
//	- name: Create user
//	  http:
//	    method: POST
//	    url: "https://api.example.com/users"
//	    headers:
//	      Authorization: "Bearer {{ .token }}"
//	    json:
//	      name: "{{ .user_name }}"
//	      role: admin
//	    expect_status: [201]
//	    register: new_user
//
//	- name: Print created ID
//	  shell: echo "id={{ .new_user.id }}"
type HTTPArgs struct {
	Method          string            `yaml:"method"`           // GET POST PUT PATCH DELETE etc. (default GET)
	URL             string            `yaml:"url"`              // supports {{ }} templates
	Headers         map[string]string `yaml:"headers"`          // request headers
	Body            string            `yaml:"body"`             // raw body string
	JSON            any               `yaml:"json"`             // body serialized as JSON (sets Content-Type automatically)
	Form            map[string]string `yaml:"form"`             // application/x-www-form-urlencoded
	BasicAuth       *BasicAuthArgs    `yaml:"basic_auth"`       // HTTP Basic auth
	Bearer          string            `yaml:"bearer"`           // Authorization: Bearer <token>
	Timeout         string            `yaml:"timeout"`          // e.g. "30s" (default "30s")
	ExpectStatus    []int             `yaml:"expect_status"`    // fail if status not in list (default: any 2xx)
	Register        string            `yaml:"register"`         // save response to this variable name
	FollowRedirects *bool             `yaml:"follow_redirects"` // default true
	VerifySSL       *bool             `yaml:"verify_ssl"`       // default true
}

// BasicAuthArgs holds HTTP Basic auth credentials.
type BasicAuthArgs struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type CopyArgs struct {
	Src  string `yaml:"src"`
	Dest string `yaml:"dest"`
	Mode string `yaml:"mode"` // e.g. "0644"
}

type FetchArgs struct {
	Src  string `yaml:"src"`  // remote path
	Dest string `yaml:"dest"` // local path ({{ .host }} is replaced with host name)
}

type TemplateArgs struct {
	Src  string `yaml:"src"`
	Dest string `yaml:"dest"`
	Mode string `yaml:"mode"`
}

func LoadPlaybook(path string) (Playbook, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var pb Playbook
	if err := yaml.Unmarshal(data, &pb); err != nil {
		return nil, err
	}
	return pb, nil
}
