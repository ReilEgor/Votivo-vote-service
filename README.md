# Vote Service

Go-мікросервіс системи Votivo: приймає й зберігає голоси, не дає проголосувати вдруге, дозволяє змінити або скасувати голос там, де це дозволено опитуванням, і повідомляє Result Service про кожну зміну. Закриває історії KAN-41 і KAN-43.

## Як працює

| Що | Як |
|---|---|
| Користувач | JWT перевіряє API Gateway (NGINX + Keycloak) і передає `sub` у заголовку `X-User-ID`. Сервіс доступний лише через gateway |
| Правила опитування | Статус, анонімність, дозвіл змінювати голос, питання та варіанти сервіс бере зі своєї копії опитування в таблиці `polls`. Її наповнюють події Poll Service: `poll.started` (повний знімок опитування) і `poll.ended` (`closed` або `blocked`), див. нижче. Голосувати можна лише в опитуванні, про початок якого подія вже дійшла |
| Повторне голосування | `UNIQUE (poll_id, user_id)` у `participations`: гарантія працює й за паралельних запитів |
| Анонімне опитування | Участь (`participations`) і бюлетень (`votes`) окремо; в анонімному `votes.user_id = NULL`, тож зв'язку «користувач → варіант» немає. Змінити чи скасувати такий голос не можна |
| Події | `vote.cast` / `vote.revoked` пишуться в `outbox_messages` у тій самій транзакції, що й голос; relay публікує їх у topic-exchange `vote.events.exchange` з routing key `vote.cast` / `vote.revoked`. `message_id` AMQP-повідомлення — UUID події (`outbox_messages.event_id`), за ним споживач відкидає дублікати. Зміна голосу — це `revoked` зі старими відповідями і `cast` з новими. У подіях немає користувача |
| Ідентифікатори | Опитування, питання й варіанти — UUID, їх призначає Poll Service |
| Ліміти | до 50 питань, до 20 варіантів на питання |

Доставка подій — щонайменше один раз, тож Result Service має бути ідемпотентним. Повідомлення, для якого в exchange немає жодної прив'язаної черги, брокер повертає (`mandatory`), і воно лишається в outbox до наступної спроби: подія не губиться, поки споживач ще не створив чергу. Формат подій — [`pkg/contracts`](pkg/contracts/vote.go).

## API

Усі шляхи під `/api/v1`, заголовок `X-User-ID` обов'язковий. `/health` і `/version` — без префікса.

| Метод | Шлях | Дія | Успіх |
|---|---|---|---|
| `POST` | `/polls/{poll_id}/vote` | перший голос | `201` |
| `PUT` | `/polls/{poll_id}/vote` | змінити голос | `200` |
| `DELETE` | `/polls/{poll_id}/vote` | скасувати голос | `200` |
| `GET` | `/polls/{poll_id}/vote` | чи голосував; для неанонімного ще й відповіді | `200` |

Тіло `POST`/`PUT`: `{ "answers": [ { "question_id": "<uuid>", "option_ids": ["<uuid>"] } ] }`. Потрібна відповідь на кожне питання.

| Код | Коли |
|---|---|
| `400` | тіло не розбирається, `poll_id`, `question_id` чи `option_ids` не UUID |
| `401` | немає `X-User-ID` |
| `403` | опитування не дозволяє змінити/скасувати голос (або воно анонімне) |
| `404` | опитування не знайдено (подія `poll.started` про нього ще не дійшла) або голосу немає |
| `409` | уже голосував; опитування не активне |
| `422` | відповіді не відповідають опитуванню |

## Запуск

Спільне оточення (Postgres, RabbitMQ, усі сервіси) описане в [`../docker-compose.yml`](../docker-compose.yml):

```sh
cd ..
docker compose up -d --build --wait   # сервіс на http://127.0.0.1:18083
```

## Події

Vote Service і публікує події, і споживає їх. Усі повідомлення мають AMQP `message_id` — UUID події; доставка щонайменше один раз.

| Напрям | Exchange (topic) | Routing key | Черга | Зміст |
|---|---|---|---|---|
| споживає | `poll.events.exchange` | `poll.started` | `vote-service.poll-events` | знімок опитування: `poll_id`, `title`, `anonymous`, `allow_vote_change`, `questions[{id, text, multiple, options[{id, text}]}]` |
| споживає | `poll.events.exchange` | `poll.ended` | `vote-service.poll-events` | `poll_id`, `reason` (`closed` / `blocked`) |
| публікує | `vote.events.exchange` | `vote.cast`, `vote.revoked` | — | `poll_id`, `answers[{question_id, option_ids}]`, `request_id` |

Обробка ідемпотентна: повторний `poll.started` оновлює знімок, але не відкриває знову опитування, яке вже завершилось. Повідомлення, яке не розбирається (не JSON, невідомий ключ чи `reason`), без повторів іде в `votivo.dlx` і лежить у черзі `vote-service.poll-events.dlq`; збій бази повертає повідомлення в чергу. Сервіс сам оголошує exchange, чергу, прив'язки та dead-letter, тож порядок старту сервісів не важливий.

## Змінні середовища

| Змінна | За замовчуванням | Опис |
|---|---|---|
| `APP_HTTP_PORT` | `8080` | порт HTTP |
| `DB_URL` | — (обов'язкова) | рядок підключення до PostgreSQL |
| `RABBITMQ_URL` | — (обов'язкова) | адреса RabbitMQ |
| `OUTBOX_INTERVAL` | `1s` | як часто relay шукає неопубліковані події |

## Розробка

`make help` показує цілі: `fmt`, `lint`, `arch` (go-arch-lint), `test`, `build`, `generate` (wire), `setup` (інструменти).

Шари: `domain` (модель та інтерфейси) ← `usecase` ← `transport/http` (REST) і `transport/amqp` (події Poll Service); `repository/postgres` та `infrastructure` реалізують інтерфейси; `cmd/vote` збирає все через wire.
