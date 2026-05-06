# Gophermart — Документация проекта

## Обзор

**Gophermart** — REST API сервис системы лояльности (накопление баллов) для интернет-магазина. Пользователи регистрируются, загружают номера заказов, получают баллы через внешнюю систему начисления и могут тратить баллы на оплату заказов.

Проект является дипломной работой курса Яндекс Практикум (Golang).

---

## Архитектура

```
cmd/gophermart/          — точка входа
internal/
├── config/              — конфигурация (флаги + env)
├── db/                  — инициализация БД и миграции
├── model/               — доменные модели (User, Order, Withdrawal)
├── middleware/
│   ├── auth/            — HMAC-SHA256 аутентификация через cookie
│   └── logger/          — HTTP-логирование (slog)
├── pkg/api/             — сгенерированные типы OpenAPI + обработчики
├── repo_pg/             — слой доступа к данным (GORM, PostgreSQL)
├── service/
│   ├── accrual_worker/  — фоновый воркер для опроса системы начисления
│   └── calculate_points/ — зарезервировано
└── accrual/             — HTTP-клиент внешней системы начисления
migrations/              — SQL-миграции (Goose)
api/                     — OpenAPI 3.0 спецификация + Swagger UI
```

**Стек технологий:**

| Компонент | Технология |
|-----------|-----------|
| HTTP-фреймворк | Echo v4 |
| ORM | GORM |
| База данных | PostgreSQL 16 |
| Миграции | Goose v3 |
| Кодогенерация API | oapi-codegen v2 |
| Хеширование паролей | bcrypt |
| Аутентификация | HMAC-SHA256 cookie |
| Логирование | slog (JSON) |
| Конфигурация | caarlos0/env + флаги |
| Тестирование | testify + SQLite in-memory |

---

## Конфигурация

Настройки загружаются из переменных окружения (или `.env` файла) и могут быть переопределены флагами запуска.

| Переменная | Флаг | Значение по умолчанию | Описание |
|-----------|------|----------------------|----------|
| `RUN_ADDRESS` | `-a` | `127.0.0.1:8080` | Адрес HTTP-сервера |
| `DATABASE_URI` | `-d` | `postgres://gophermart:gophermart@localhost:5432/gophermart` | DSN PostgreSQL |
| `ACCRUAL_SYSTEM_ADDRESS` | `-r` | `""` | Базовый URL внешней системы начисления |
| `SECRET_KEY_FOR_JWT` | — | *(встроенный ключ)* | Секрет для HMAC-подписи сессий |
| `WORKER_NUM` | — | `10` | Размер батча воркера начисления |

Запуск с флагами:
```bash
./gophermart -a 0.0.0.0:8080 -d "postgres://..." -r "http://accrual:8081"
```

---

## База данных

### Таблицы

**users**
```sql
id              BIGSERIAL PRIMARY KEY
login           VARCHAR UNIQUE NOT NULL
password_hash   TEXT NOT NULL
current_balance DOUBLE PRECISION DEFAULT 0
withdrawn       DOUBLE PRECISION DEFAULT 0
created_at      TIMESTAMP DEFAULT NOW()
updated_at      TIMESTAMP DEFAULT NOW()
```

**orders**
```sql
number          TEXT PRIMARY KEY
user_id         BIGINT REFERENCES users(id) ON DELETE CASCADE
status          VARCHAR NOT NULL   -- NEW | PROCESSING | INVALID | PROCESSED
accrual         DOUBLE PRECISION   -- NULL до получения от системы начисления
accrual_applied BOOLEAN DEFAULT false
uploaded_at     TIMESTAMP DEFAULT NOW()
```

**withdrawals**
```sql
id              BIGSERIAL PRIMARY KEY
user_id         BIGINT REFERENCES users(id) ON DELETE CASCADE
order_number    TEXT NOT NULL
sum             DOUBLE PRECISION NOT NULL
processed_at    TIMESTAMP DEFAULT NOW()
```

### Миграции

Выполняются автоматически при старте через Goose. Файлы в `migrations/`.

---

## API

Базовый URL: `/api`

Аутентификация: cookie `session` (устанавливается при логине/регистрации). Также принимается заголовок `Authorization: Bearer <token>`.

### POST /api/user/register

Регистрация нового пользователя.

**Тело запроса:**
```json
{ "login": "user1", "password": "secret" }
```

**Ответы:**
- `200` — успешная регистрация, cookie установлена
- `400` — пустой логин или пароль
- `409` — логин уже занят
- `500` — внутренняя ошибка

---

### POST /api/user/login

Аутентификация пользователя.

**Тело запроса:**
```json
{ "login": "user1", "password": "secret" }
```

**Ответы:**
- `200` — успех, cookie установлена
- `400` — пустые данные
- `401` — неверный логин или пароль
- `500` — внутренняя ошибка

---

### POST /api/user/orders

Загрузка номера заказа для начисления баллов.

**Тело запроса:** `Content-Type: text/plain`
```
79927398713
```

**Валидация:**
- Непустая строка
- Только цифры
- Алгоритм Луна

**Ответы:**
- `202` — принят, поставлен в очередь
- `200` — уже загружен этим пользователем
- `400` — пустой номер
- `401` — не аутентифицирован
- `409` — номер загружен другим пользователем
- `422` — неверный формат (не прошёл Луна)
- `500` — внутренняя ошибка

---

### GET /api/user/orders

Список всех заказов пользователя (сортировка: новые первые).

**Ответ 200:**
```json
[
  {
    "number": "79927398713",
    "status": "PROCESSED",
    "accrual": 500.0,
    "uploaded_at": "2024-01-01T12:00:00Z"
  }
]
```

Поле `accrual` присутствует только для статуса `PROCESSED`.

**Статусы заказа:**
- `NEW` — принят, ожидает обработки
- `PROCESSING` — обрабатывается системой начисления
- `INVALID` — не принят (баллы не начисляются)
- `PROCESSED` — обработан, баллы начислены

**Ответы:** `200`, `204` (нет заказов), `401`, `500`

---

### GET /api/user/balance

Текущий баланс пользователя.

**Ответ 200:**
```json
{
  "current": 500.0,
  "withdrawn": 200.0
}
```

**Ответы:** `200`, `401`, `500`

---

### POST /api/user/balance/withdraw

Списание баллов на оплату заказа.

**Тело запроса:**
```json
{ "order": "79927398713", "sum": 100.0 }
```

**Ответы:**
- `200` — успешно списано
- `401` — не аутентифицирован
- `402` — недостаточно баллов
- `422` — неверный номер заказа
- `500` — внутренняя ошибка

---

### GET /api/user/withdrawals

История списаний пользователя (сортировка: новые первые).

**Ответ 200:**
```json
[
  {
    "order": "79927398713",
    "sum": 100.0,
    "processed_at": "2024-01-01T12:00:00Z"
  }
]
```

**Ответы:** `200`, `204` (нет списаний), `401`, `500`

---

## Аутентификация

Cookie-сессия на основе HMAC-SHA256:

```
Cookie: session=<userID>:<HMAC-подпись>
```

- Алгоритм: HMAC-SHA256 с ключом `SECRET_KEY_FOR_JWT`
- Сравнение подписи: константное время (`hmac.Equal`)
- Срок жизни: 30 дней
- Флаги: `HttpOnly`, `SameSite=Strict`
- Публичные маршруты (без проверки): `/api/user/register`, `/api/user/login`

Функции (пакет `internal/middleware/auth`):
- `SetSessionCookie(ctx, userID)` — установить сессию
- `GetUserID(ctx)` — получить ID пользователя из контекста
- `Middleware(db)` — Echo middleware для проверки

---

## Фоновый воркер начисления

Воркер запускается при наличии `ACCRUAL_SYSTEM_ADDRESS`.

**Цикл работы:**
1. Выбрать батч заказов в статусе `NEW` или `PROCESSING` (лимит: `WORKER_NUM`)
2. Для каждого заказа запросить статус у внешней системы `GET /api/orders/{number}`
3. Обновить статус заказа
4. При `PROCESSED` — атомарно начислить баллы пользователю (с блокировкой строки)
5. Пометить заказ `accrual_applied = true` (защита от двойного начисления)

**Обработка ошибок:**
- `429 Too Many Requests` — экспоненциальный backoff (500ms → 5s)
- `204 No Content` — заказ ещё не зарегистрирован, оставить статус `NEW`
- Прочие ошибки — логировать, продолжить

**Атомарность:** Используются GORM-транзакции с пессимистичной блокировкой (`SELECT FOR UPDATE`).

---

## Репозитории

Пакет `internal/repo_pg/`:

**BaseRepository[T]** — универсальный CRUD:
- `Create`, `GetByID`, `Update`, `UpdateFields`, `Delete`, `GetAll`
- `FirstWhere(query, args)`, `FindWhere(query, args)`

**UserRepository** — наследует Base:
- `GetByLogin(ctx, login)` — поиск по логину

**OrderRepository** — наследует Base:
- `GetByUserID(ctx, userID)` — заказы пользователя (DESC)
- `GetProcessingBatch(ctx, limit)` — NEW/PROCESSING заказы для воркера

**WithdrawalRepository** — наследует Base:
- `GetByUserID(ctx, userID)` — списания пользователя (DESC)

---

## Запуск локально

**Зависимости:** Go 1.25+, Docker

```bash
# 1. Поднять PostgreSQL
docker-compose up -d

# 2. Скопировать конфигурацию
cp .env .env.local
# Отредактировать при необходимости

# 3. Запустить сервер
go run ./cmd/gophermart/

# 4. (Опционально) запустить систему начисления
./cmd/accrual/accrual_linux_amd64 -a 127.0.0.1:8081
```

Swagger UI доступен по адресу: `http://127.0.0.1:8080/swagger/`

---

## Тестирование

```bash
# Все тесты
go test ./...

# С покрытием
go test ./... -coverprofile=cover/coverage.out
go tool cover -html=cover/coverage.out
```

Тесты используют SQLite in-memory вместо реального PostgreSQL (пакет `glebarez/sqlite`).

Ключевые тест-сценарии:
- Регистрация и логин
- Загрузка заказа (дубли, гонки)
- Баланс и списание
- Валидация по алгоритму Луна
- Проверка cookie-подписи
- Атомарность начисления воркером
- Обработка 429 (rate limit)

---

## CI/CD

GitHub Actions (`.github/workflows/`):

- **gophermart.yml** — сборка + автотесты Яндекс Практикума
- **statictest.yml** — статический анализ

Окружение CI: Go 1.26, PostgreSQL сервис-контейнер, запуск `gophermarttest`.

---

## Структура файлов проекта

```
.
├── api/
│   ├── embed.go
│   ├── oapi-codegen.yaml
│   └── openapi.yaml
├── cmd/
│   └── gophermart/
│       └── main.go
├── internal/
│   ├── accrual/
│   │   └── client.go
│   ├── config/
│   │   └── settings.go
│   ├── db/
│   │   └── init.go
│   ├── middleware/
│   │   ├── auth/auth_middleware.go
│   │   └── logger/log_middleware.go
│   ├── model/
│   │   └── user.go
│   ├── pkg/api/
│   │   ├── api.gen.go
│   │   └── impl.go
│   ├── repo_pg/
│   │   ├── base_repository.go
│   │   ├── points.go
│   │   └── user_repo.go
│   └── service/
│       └── accrual_worker/worker.go
├── migrations/
│   └── 20260405172435_init.sql
├── docker-compose.yml
├── .env
└── go.mod
```
