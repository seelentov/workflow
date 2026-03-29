package config

import (
	"testing"
)

func TestLoadPlaybook_Success(t *testing.T) {
	content := `
- name: Test play
  hosts: web
  vars:
    app_dir: /opt/app
  tasks:
    - name: Run command
      shell: echo hello
      tags: [deploy]
      ignore_errors: true
    - name: Upload file
      copy:
        src: ./bin/app
        dest: /opt/app/app
        mode: "0755"
    - name: Download file
      fetch:
        src: /remote/log
        dest: ./logs/{{ .host }}.log
    - name: Render template
      template:
        src: nginx.conf.tmpl
        dest: /etc/nginx/nginx.conf
        mode: "0644"
`
	f := writeTempFile(t, content)
	pb, err := LoadPlaybook(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pb) != 1 {
		t.Fatalf("want 1 play, got %d", len(pb))
	}
	play := pb[0]
	if play.Name != "Test play" {
		t.Errorf("want name 'Test play', got %s", play.Name)
	}
	if play.Hosts != "web" {
		t.Errorf("want hosts 'web', got %s", play.Hosts)
	}
	if play.Vars["app_dir"] != "/opt/app" {
		t.Errorf("want var app_dir=/opt/app, got %v", play.Vars["app_dir"])
	}
	if len(play.Tasks) != 4 {
		t.Fatalf("want 4 tasks, got %d", len(play.Tasks))
	}

	shell := play.Tasks[0]
	if shell.Shell != "echo hello" {
		t.Errorf("want shell 'echo hello', got %s", shell.Shell)
	}
	if len(shell.Tags) != 1 || shell.Tags[0] != "deploy" {
		t.Errorf("want tags [deploy], got %v", shell.Tags)
	}
	if !shell.IgnoreErrors {
		t.Error("want ignore_errors=true")
	}

	cp := play.Tasks[1]
	if cp.Copy == nil {
		t.Fatal("expected copy task")
	}
	if cp.Copy.Src != "./bin/app" {
		t.Errorf("want src=./bin/app, got %s", cp.Copy.Src)
	}
	if cp.Copy.Mode != "0755" {
		t.Errorf("want mode 0755, got %s", cp.Copy.Mode)
	}

	fetch := play.Tasks[2]
	if fetch.Fetch == nil {
		t.Fatal("expected fetch task")
	}
	if fetch.Fetch.Src != "/remote/log" {
		t.Errorf("want fetch.src=/remote/log, got %s", fetch.Fetch.Src)
	}

	tmpl := play.Tasks[3]
	if tmpl.Template == nil {
		t.Fatal("expected template task")
	}
	if tmpl.Template.Src != "nginx.conf.tmpl" {
		t.Errorf("want template.src=nginx.conf.tmpl, got %s", tmpl.Template.Src)
	}
}

func TestLoadPlaybook_MultiplePlays(t *testing.T) {
	content := `
- name: Play 1
  hosts: web
  tasks:
    - shell: echo 1
- name: Play 2
  hosts: db
  tasks:
    - shell: echo 2
`
	f := writeTempFile(t, content)
	pb, err := LoadPlaybook(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pb) != 2 {
		t.Fatalf("want 2 plays, got %d", len(pb))
	}
	if pb[0].Name != "Play 1" || pb[1].Name != "Play 2" {
		t.Error("play names mismatch")
	}
}

func TestLoadPlaybook_HTTP(t *testing.T) {
	content := `
- name: HTTP play
  hosts: all
  tasks:
    - name: Make request
      http:
        method: POST
        url: "https://example.com/api"
        json:
          key: value
        expect_status: [200, 201]
        register: response
        timeout: "10s"
        bearer: "mytoken"
        follow_redirects: false
        verify_ssl: false
`
	f := writeTempFile(t, content)
	pb, err := LoadPlaybook(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	task := pb[0].Tasks[0]
	if task.HTTP == nil {
		t.Fatal("expected http task")
	}
	if task.HTTP.Method != "POST" {
		t.Errorf("want method POST, got %s", task.HTTP.Method)
	}
	if task.HTTP.URL != "https://example.com/api" {
		t.Errorf("want url https://example.com/api, got %s", task.HTTP.URL)
	}
	if task.HTTP.Register != "response" {
		t.Errorf("want register=response, got %s", task.HTTP.Register)
	}
	if task.HTTP.Timeout != "10s" {
		t.Errorf("want timeout=10s, got %s", task.HTTP.Timeout)
	}
	if task.HTTP.Bearer != "mytoken" {
		t.Errorf("want bearer=mytoken, got %s", task.HTTP.Bearer)
	}
	if len(task.HTTP.ExpectStatus) != 2 {
		t.Errorf("want 2 expect_status codes, got %d", len(task.HTTP.ExpectStatus))
	}
	if task.HTTP.FollowRedirects == nil || *task.HTTP.FollowRedirects != false {
		t.Error("want follow_redirects=false")
	}
	if task.HTTP.VerifySSL == nil || *task.HTTP.VerifySSL != false {
		t.Error("want verify_ssl=false")
	}
}

func TestLoadPlaybook_HTTPBasicAuth(t *testing.T) {
	content := `
- name: Auth play
  hosts: all
  tasks:
    - name: Authenticated request
      http:
        url: "https://example.com"
        basic_auth:
          username: admin
          password: secret
`
	f := writeTempFile(t, content)
	pb, err := LoadPlaybook(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	http := pb[0].Tasks[0].HTTP
	if http.BasicAuth == nil {
		t.Fatal("expected basic_auth")
	}
	if http.BasicAuth.Username != "admin" || http.BasicAuth.Password != "secret" {
		t.Errorf("basic_auth mismatch: %+v", http.BasicAuth)
	}
}

func TestLoadPlaybook_VarsTask(t *testing.T) {
	content := `
- name: Vars play
  hosts: all
  tasks:
    - vars:
        deploy_path: /opt/myapp
        version: "1.2.3"
    - when: test -f /etc/flag
      shell: echo conditional
`
	f := writeTempFile(t, content)
	pb, err := LoadPlaybook(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	varsTask := pb[0].Tasks[0]
	if varsTask.Vars == nil {
		t.Fatal("expected vars task")
	}
	if varsTask.Vars["deploy_path"] != "/opt/myapp" {
		t.Errorf("want deploy_path=/opt/myapp, got %v", varsTask.Vars["deploy_path"])
	}

	whenTask := pb[0].Tasks[1]
	if whenTask.When != "test -f /etc/flag" {
		t.Errorf("want when='test -f /etc/flag', got %s", whenTask.When)
	}
}

func TestLoadPlaybook_FileNotFound(t *testing.T) {
	_, err := LoadPlaybook("/nonexistent/playbook.yaml")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestLoadPlaybook_InvalidYAML(t *testing.T) {
	f := writeTempFile(t, ":::invalid yaml:::")
	_, err := LoadPlaybook(f)
	if err == nil {
		t.Fatal("expected error for invalid YAML, got nil")
	}
}
