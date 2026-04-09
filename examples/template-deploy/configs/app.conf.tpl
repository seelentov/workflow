[server]
host = 0.0.0.0
port = {{ .server_port }}
name = {{ .server_name }}

[database]
host = {{ .db_host }}
port = {{ .db_port }}
max_connections = 100
pool_timeout = 30

[logging]
level = info
output = /var/log/{{ .server_name }}/app.log
format = json
