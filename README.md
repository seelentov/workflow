# Workflow

Инструмент автоматизации задач на удалённых серверах. Вы описываете задачи в YAML-плейбуках, указываете список серверов в инвентори — Workflow выполняет всё по SSH.

---

## Содержание

- [Инвентори](#инвентори)
- [Плейбук](#плейбук)
- [Типы задач](#типы-задач)
  - [shell](#shell)
  - [copy](#copy)
  - [fetch](#fetch)
  - [template](#template)
  - [http](#http)
  - [vars](#vars)
- [Переменные и шаблоны](#переменные-и-шаблоны)
- [Теги](#теги)
- [Флаги запуска](#флаги-запуска)
- [Модель выполнения](#модель-выполнения)

---

## Инвентори

Файл описывает серверы и группы. По умолчанию читается `inventory.yaml`.

```yaml
hosts:
  - name: web1
    host: 192.168.1.10
    port: 22           # по умолчанию 22
    user: ubuntu
    key_file: ~/.ssh/id_rsa

  - name: web2
    host: 192.168.1.11
    user: ubuntu
    password: secret   # пароль вместо ключа

  - name: db1
    host: 192.168.1.20
    user: root
    key_file: $HOME/.ssh/deploy_key
    vars:              # переменные, доступные только этому хосту
      pg_port: 5432

groups:
  web:
    - web1
    - web2
  db:
    - db1
```

**Аутентификация** — на выбор:
- `key_file` — путь к приватному ключу (`~/` и `$VAR` раскрываются)
- `password` — пароль
- если ничего не указано — пробуются `~/.ssh/id_rsa`, `~/.ssh/id_ed25519`, `~/.ssh/id_ecdsa`

---

## Плейбук

Плейбук — список **plays**. Каждый play нацелен на группу хостов и содержит список задач.

```yaml
- name: Название play
  hosts: web          # имя группы, хоста или "all"
  vars:               # переменные play
    app_dir: /opt/app
    app_port: 8080

  tasks:
    - name: Создать директорию
      shell: mkdir -p {{ .app_dir }}

    - name: Перезапустить сервис
      shell: systemctl restart app
      ignore_errors: true
      tags: [restart]
```

**Поля play:**

| Поле | Описание |
|------|----------|
| `name` | Название (отображается в выводе) |
| `hosts` | Цель: имя хоста, группы или `all` |
| `vars` | Переменные, доступные во всех задачах play |
| `tasks` | Список задач |

**Общие поля задачи:**

| Поле | Описание |
|------|----------|
| `name` | Название задачи |
| `ignore_errors` | `true` — продолжить выполнение даже при ошибке |
| `tags` | Список тегов для выборочного запуска |
| `when` | Shell-команда: задача пропускается если exit code ≠ 0 |

---

## Типы задач

### shell

Выполняет команду на удалённом сервере по SSH.

```yaml
- name: Установить пакеты
  shell: apt-get install -y nginx curl

- name: Перезапустить если запущен
  shell: systemctl is-active app && systemctl restart app
  ignore_errors: true

- name: Создать директорию
  shell: mkdir -p {{ .app_dir }}
  when: test ! -d {{ .app_dir }}   # пропустить если директория уже есть
```

Поведение в выводе:
- команда вернула 0 и stdout пуст → `ok`
- команда вернула 0 и есть stdout → `changed`
- exit code ≠ 0 → `FAILED`

---

### copy

Копирует локальный файл на удалённый сервер по SFTP. Родительские директории создаются автоматически.

```yaml
- name: Загрузить бинарник
  copy:
    src: ./bin/app
    dest: /opt/app/app
    mode: "0755"

- name: Загрузить конфиг
  copy:
    src: ./config/nginx.conf
    dest: /etc/nginx/nginx.conf
    mode: "0644"        # по умолчанию 0644
```

| Поле | Описание |
|------|----------|
| `src` | Локальный путь к файлу |
| `dest` | Путь на удалённом сервере |
| `mode` | Права доступа в восьмеричном формате (опционально) |

---

### fetch

Скачивает файл с удалённого сервера на локальную машину.

```yaml
- name: Забрать логи
  fetch:
    src: /var/log/app.log
    dest: ./logs/{{ .host }}-app.log
```

В `dest` доступна переменная `{{ .host }}` — имя хоста из инвентори, что удобно при одновременном скачивании с нескольких серверов.

---

### template

Рендерит локальный файл как Go-шаблон и загружает результат на сервер. Синтаксис шаблонов — стандартный `text/template`: `{{ .var }}`, `{{ if }}`, `{{ range }}` и т.д.

```yaml
- name: Загрузить конфиг из шаблона
  template:
    src: ./templates/app.conf.tmpl
    dest: /etc/app/app.conf
    mode: "0644"
```

```
# templates/app.conf.tmpl
listen = 0.0.0.0:{{ .app_port }}
host   = {{ .hostname }}
env    = production
```

---

### http

Выполняет HTTP-запрос **с управляющей машины** (не через SSH). Переменные хоста при этом доступны — через `{{ .hostname }}` можно обращаться к каждому серверу.

#### GET

```yaml
- name: Проверить health endpoint
  http:
    method: GET
    url: "http://{{ .hostname }}:{{ .app_port }}/health"
    timeout: 5s
    expect_status: [200]
    register: health
```

#### POST с JSON-телом

```yaml
- name: Создать пользователя
  http:
    method: POST
    url: "https://api.example.com/users"
    bearer: "{{ .token }}"
    json:
      name: deploy-bot
      role: ci
      host: "{{ .host }}"
    expect_status: [201]
    register: created_user
```

#### Все поля

| Поле | Описание |
|------|----------|
| `method` | HTTP-метод: GET POST PUT PATCH DELETE HEAD (по умолчанию GET) |
| `url` | URL запроса, поддерживает шаблоны |
| `headers` | Произвольные заголовки |
| `json` | Тело запроса как объект, сериализуется в JSON; Content-Type выставляется автоматически |
| `body` | Тело запроса как сырая строка |
| `form` | Тело как form-encoded (`application/x-www-form-urlencoded`) |
| `bearer` | Токен для заголовка `Authorization: Bearer ...` |
| `basic_auth` | HTTP Basic auth: поля `username` и `password` |
| `expect_status` | Список допустимых статус-кодов (по умолчанию любой 2xx) |
| `timeout` | Таймаут запроса, например `10s` (по умолчанию `30s`) |
| `follow_redirects` | Следовать редиректам (по умолчанию `true`) |
| `verify_ssl` | Проверять SSL-сертификат (по умолчанию `true`; `false` для self-signed) |
| `register` | Имя переменной для сохранения ответа |

#### register — сохранение ответа

Ответ сохраняется как объект с полями:

| Поле | Описание |
|------|----------|
| `status` | HTTP-статус код |
| `body` | Тело ответа как строка |
| `ok` | `true` если статус попал в `expect_status` |
| `headers` | Заголовки ответа |
| `json` | Тело ответа, распарсенное как JSON |

Если тело ответа — JSON-объект, его поля верхнего уровня автоматически выносятся на тот же уровень для удобного доступа:

```yaml
- name: Получить пользователя
  http:
    method: GET
    url: "https://api.example.com/users/1"
    register: user

- name: Использовать поле из ответа
  shell: echo "id={{ .user.id }} name={{ .user.name }}"
```

---

### vars

Устанавливает переменные для последующих задач внутри play. SSH-соединение не требуется.

```yaml
- name: Задать переменные окружения
  vars:
    deploy_time: "2024-01-01"
    app_version: "2.1.0"

- name: Использовать их дальше
  shell: echo "Deploying {{ .app_version }}"
```

---

## Переменные и шаблоны

Переменные доступны во всех строковых полях задачи через синтаксис Go-шаблонов `{{ .name }}`.

**Приоритет** (от низшего к высшему): переменные play → переменные хоста из инвентори → переменные задачи `vars`.

**Встроенные переменные:**

| Переменная | Значение |
|------------|----------|
| `{{ .host }}` | Имя хоста из инвентори (`name`) |
| `{{ .hostname }}` | IP-адрес или DNS-имя (`host`) |
| `{{ .user }}` | Пользователь SSH |

**Пример с условием `when`:**

```yaml
- name: Запустить только если файла нет
  shell: echo "first run" > /opt/app/.initialized
  when: test ! -f /opt/app/.initialized
```

---

## Теги

Теги позволяют запускать только часть задач плейбука.

```yaml
tasks:
  - name: Обновить пакеты
    shell: apt-get update -y
    # без тега — запускается всегда

  - name: Перезапустить nginx
    shell: systemctl restart nginx
    tags: [restart, nginx]

  - name: Проверить здоровье
    http:
      method: GET
      url: "http://{{ .hostname }}/health"
    tags: [health]
```

```bash
# Запустить только задачи с тегом restart
workflow run -i inventory.yaml --tags restart playbook.yaml

# Несколько тегов (задачи с любым из них)
workflow run -i inventory.yaml --tags restart,health playbook.yaml
```

---

## Флаги запуска

```
workflow run [флаги] playbook.yaml
```

| Флаг | Краткий | По умолчанию | Описание |
|------|---------|--------------|----------|
| `--inventory` | `-i` | `inventory.yaml` | Путь к файлу инвентори |
| `--limit` | `-l` | — | Ограничить выполнение хостами или группой (через запятую) |
| `--tags` | — | — | Запустить только задачи с указанными тегами (через запятую) |
| `--dry-run` | — | `false` | Показать задачи без выполнения |
| `--parallel` | `-p` | `true` | Параллельное выполнение по хостам |
| `--verbose` | `-v` | `false` | Показывать полный вывод команд |

**Примеры:**

```bash
# Запустить плейбук
workflow run -i inventory.yaml playbook.yaml

# Только на одном хосте
workflow run -i inventory.yaml -l web1 playbook.yaml

# Только на группе, только тег restart, с выводом
workflow run -i inventory.yaml -l web --tags restart --verbose playbook.yaml

# Посмотреть план без выполнения
workflow run -i inventory.yaml --dry-run playbook.yaml

# Последовательное выполнение (без параллелизма)
workflow run -i inventory.yaml --parallel=false playbook.yaml
```

---

## Модель выполнения

Задачи внутри play выполняются **последовательно** — следующая задача начинается только после того, как текущая завершилась на всех хостах.

На каждой задаче хосты обрабатываются **параллельно** (если не передан `--parallel=false`).

```
task 1 → [web1, web2, web3] параллельно → ждём всех → task 2 → ...
```

Это соответствует стандартной linear-стратегии Ansible.

**Задачи типа `http`** выполняются на управляющей машине без SSH. Если в play 3 хоста — будет выполнено 3 запроса параллельно (по одному в контексте каждого хоста), что удобно для health-check'ов: `http://{{ .hostname }}:8080/health`.
