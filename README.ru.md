# remnawave-node-go

[![Go Version](https://img.shields.io/github/go-mod/go-version/hteppl/remnawave-node-go)](https://go.dev/)
[![Docker Hub](https://img.shields.io/docker/v/hteppl/remnawave-node-go?label=Docker%20Hub)](https://hub.docker.com/r/hteppl/remnawave-node-go)

[English](README.md) | [Русский](README.ru.md)

Неофициальная community Go-реализация [Remnawave node](https://github.com/remnawave/node)
для [xray-core](https://github.com/XTLS/Xray-core). Управляет жизненным циклом xray-core, пользователями, статистикой
трафика и блокировкой IP через REST API.

> **⚠️ Внимание:** Это неофициальная community-реализация remnawave-node. Проект не связан с официальной командой
> Remnawave. Используйте на свой страх и риск.

## Сравнение с официальной нодой

|                  | [remnawave/node](https://github.com/remnawave/node) | remnawave-node-go                |
|------------------|-----------------------------------------------------|----------------------------------|
| **Язык**         | TypeScript (NestJS)                                 | Go                               |
| **Среда**        | Node.js / Bun                                       | Нативный бинарник                |
| **RAM (idle)**   | ~109 MB                                             | ~70 MB                           |
| **Docker-образ** | ~490 MB                                             | ~79 MB                           |
| **xray-core**    | Бинарник + геоданные в образе                       | Скомпилирован как Go-библиотека* |
| **Архитектура**  | Микросервис (NestJS + supervisord)                  | Один статический бинарник        |
| **Статус**       | Официальный                                         | Неофициальный (community)        |

> *\* xray-core компилируется непосредственно в бинарник как Go-библиотека. Только
геоданные (`geoip.dat`, `geosite.dat`) скачиваются при первом запуске и кэшируются в Docker volume для последующих
запусков.*

## Конфигурация

| Переменная           | Обязательная | По умолчанию | Описание                                                          |
|----------------------|--------------|--------------|-------------------------------------------------------------------|
| `SECRET_KEY`         | Да           | —            | Base64-закодированный payload, сгенерированный панелью Remnawave. |
| `NODE_PORT`          | Нет          | `2222`       | Порт основного HTTPS-сервера (mTLS + JWT авторизация)             |
| `INTERNAL_REST_PORT` | Нет          | `61001`      | Порт внутреннего HTTP-сервера (только localhost)                  |
| `LOG_LEVEL`          | Нет          | `info`       | Уровень логирования: `debug`, `info`, `warn`, `error`             |

### Пример `.env`

```env
SECRET_KEY=<your-base64-encoded-secret-key>
NODE_PORT=2222
INTERNAL_REST_PORT=61001
LOG_LEVEL=info
```

## Docker

### Быстрый старт

```bash
docker run -d \
  --name remnawave-node-go \
  --network host \
  --restart unless-stopped \
  -e SECRET_KEY=<your-secret-key> \
  -v xray-geodata:/usr/local/share/xray \
  hteppl/remnawave-node-go:latest
```

### Docker Compose

Создайте файл `.env` (см. пример выше), затем:

```bash
docker compose up -d
```

`docker-compose.yml`:

```yaml
services:
  remnawave-node-go:
    image: hteppl/remnawave-node-go:latest
    container_name: remnawave-node-go
    restart: unless-stopped
    env_file:
      - .env
    volumes:
      - xray-geodata:/usr/local/share/xray
    networks:
      - remnawave-node-go-network

networks:
  remnawave-node-go-network:
    name: remnawave-node-go-network
    driver: bridge

volumes:
  xray-geodata:
```

### Логи

```bash
# Следить за логами
docker logs -f remnawave-node-go

# Последние 100 строк
docker logs --tail 100 remnawave-node-go

# С временными метками
docker logs -t remnawave-node-go
```

Для более подробных логов установите `LOG_LEVEL=debug` в файле `.env` и перезапустите контейнер.

### Внутренний API

Текущую конфигурацию xray можно получить через внутренний REST API:

```bash
# Вывести конфигурацию в stdout
curl -s http://127.0.0.1:61001/internal/get-config | jq

# Сохранить конфигурацию в файл
curl -s http://127.0.0.1:61001/internal/get-config | jq > config.json
```

### Управление

```bash
# Остановить
docker compose down

# Перезапустить
docker compose restart

# Обновить до последней версии
docker compose pull && docker compose up -d
```

## Благодарности

- [Remnawave](https://github.com/remnawave) — за оригинальную панель и экосистему нод
- [W-Nana/remnawave-node-go](https://github.com/W-Nana/remnawave-node-go) — за оригинальную кодовую базу этого проекта

## Лицензия

Проект распространяется под лицензией [AGPL-3.0](LICENSE).
