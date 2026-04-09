# Workflow Examples

This directory contains example playbooks and inventories demonstrating various use cases for the Workflow tool.

## Examples Overview

| Example | Description | Key Features Demonstrated |
|---------|-------------|---------------------------|
| [web-deploy](./web-deploy/) | Deploy a web application | Basic shell tasks, copy, host groups, vars |
| [database-backup](./database-backup/) | PostgreSQL backup automation | Host vars, fetch task, ignore_errors |
| [monitoring-setup](./monitoring-setup/) | Install monitoring agents | Multiple plays, different host groups |
| [http-api-tasks](./http-api-tasks/) | HTTP API interactions | HTTP task (GET/POST/PUT/DELETE), auth, JSON, forms, register |
| [file-sync](./file-sync/) | File synchronization | Copy and fetch tasks, file permissions |
| [system-maintenance](./system-maintenance/) | Routine server maintenance | Shell commands, ignore_errors, sequential tasks |
| [multi-play-deploy](./multi-play-deploy/) | Multi-tier deployment | Multiple plays targeting different groups, ordered deployment |
| [tagged-tasks](./tagged-tasks/) | Selective task execution | Task tags, `--tags` filtering |
| [conditional-tasks](./conditional-tasks/) | Conditional execution | `when` conditionals based on shell expressions |
| [template-deploy](./template-deploy/) | Template-based configs | Template task, Go template rendering, host-specific vars |

## Quick Start

### Run an example

```bash
cd examples/web-deploy
workflow run playbook.yaml -i inventory.yaml
```

### Run with dry-run (preview tasks without executing)

```bash
workflow run playbook.yaml -i inventory.yaml --dry-run
```

### Run only specific tagged tasks

```bash
cd examples/tagged-tasks
workflow run playbook.yaml -i inventory.yaml --tags deploy
```

### Run on specific hosts only

```bash
workflow run playbook.yaml -i inventory.yaml -l web1,web2
```

### Run with verbose output

```bash
workflow run playbook.yaml -i inventory.yaml -v
```
