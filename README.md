# filebeat-throttle-plugin

This plugins allows to throttle beat events.

Throttle processor retrieves configuration from separate component called `policy manager`.


```
┌──────┐                    ┌──────┐                     ┌──────┐
│Node 1│                    │Node 2│                     │Node 3│
├──────┴─────────────┐      ├──────┴─────────────┐       ├──────┴─────────────┐
│ ┌──────┐  ┌──────┐ │      │ ┌──────┐  ┌──────┐ │       │ ┌──────┐  ┌──────┐ │
│ │      │  │      │ │      │ │      │  │      │ │       │ │      │  │      │ │
│ │      │  │      │ │      │ │      │  │      │ │       │ │      │  │      │ │
│ └──────┘  └──────┘ │      │ └──────┘  └──────┘ │       │ └──────┘  └──────┘ │
│ ┌──────┐  ┌──────┐ │      │ ┌──────┐  ┌──────┐ │       │ ┌──────┐  ┌──────┐ │
│ │      │  │      │ │      │ │      │  │      │ │       │ │      │  │      │ │
│ │      │  │      │ │      │ │      │  │      │ │       │ │      │  │      │ │
│ └──────┘  └──────┘ │      │ └──────┘  └──────┘ │       │ └──────┘  └──────┘ │
│ ┌──────┐  ┌──────┐ │      │ ┌──────┐  ┌──────┐ │       │ ┌──────┐  ┌──────┐ │
│ │      │  │      │ │      │ │      │  │      │ │       │ │      │  │      │ │
│ │      │  │      │ │      │ │      │  │      │ │       │ │      │  │      │ │
│ └──────┘  └──────┘ │      │ └──────┘  └──────┘ │       │ └──────┘  └──────┘ │
│                    │      │                    │       │                    │
│                    │      │                    │       │                    │
│ ┌────────────────┐ │      │ ┌────────────────┐ │       │ ┌────────────────┐ │
│ │    filebeat    │ │      │ │    filebeat    │ │       │ │    filebeat    │ │
│ └────────────────┘ │      │ └────────────────┘ │       │ └────────────────┘ │
│          │         │      │          │         │       │          │         │
└──────────┼─────────┘      └──────────┼─────────┘       └──────────┼─────────┘
           │                           │                            │
           └───────────────┐           │            ┌───────────────┘
                           │           │            │
                           │           │            │
                           ▼           ▼            ▼
                         ┌─────────────────────────────┐
                         │                             │
                         │       Policy Manager        │
                         │                             │
                         └─────────────────────────────┘
                              HTTP /policy endpoint
```

## Configuration

To enable throttling you have to add `throttle` processor to configuration:

```
- throttle:
    prometheus_port: 9090
    metric_name: proccessed_records
    metric_labels:
        - from: kubernetes_container_name
            to: container_name
        - from: "labels.app"
            to: app
    policy_host: "http://policymanager.local:8080/policy"
    policy_update_interval: 1s
    bucket_size: 1
    buckets: 1000
```

 - `prometheus_port` - prometheus metrics handler to listen on
 - `metric_name` - name of counter metric with number of processed/throttled events
 - `metric_labels` - additional fields that will be converted to metric labels
 - `policy_host` - policy manager host
 - `policy_update_interval` - how often processor refresh policies
 - `buckets` - number of buckets
 - `bucket_size` - bucket duration (in seconds)

## Policy Manager

Policy manager exposes configuration by `/policy` endpoint in following format:
```yaml
---
limits:
  - value: 500
    conditions:
      kubernetes_container_name: "simple-generator"
  - value: 5000
    conditions:
      kubernetes_namespace: "bx"
```

`value` указывает на максимальное разрешенное кол-во сообщений в интервал времени, указанный в `bucket_size`.  
В `conditions` можно использовать любые поля из событий. Все условия работают по принципу `equals`, т.е. по полному совпадению.


## Throttling algorithm

Для реализации троттлинга используется имплементация `token bucket` алгоритма. В простейшем случае мы можем хранить только один текущий бакет и считать лимиты по ней. Однако, такой алгоритм плохо работает в случае, когда filebeat некоторое время не работает (плановое обновление, падение и т.д.): в этом случае все накопившиеся сообщения будут попадать в один бакет, что может привести к игнорированию тех сообщений, которые не должны были быть проигнорированы при обычной работе. Чтобы избежать таких проблем, нам нужно хранить информацию о N последних бакетах. 

Представим, что processor имеет следующие настройки:
```
bucket_size: 1 // 1 секунда
buckets: 5
```

и имеем одно правило с `limit: 10`

Тогда в момент времени T для этого правила будут существовать такие бакеты:
```
T - 5        T - 4         T - 3         T - 2         T - 1          T
  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐
  │   0/10   │  │   0/10   │  │   0/10   │  │   0/10   │  │   0/10   │
  └──────────┘  └──────────┘  └──────────┘  └──────────┘  └──────────┘
```

При добавление события со временем между `T-3` и `T-2` ситуация изменится на такую:
```
T - 5        T - 4         T - 3         T - 2         T - 1          T
  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐
  │   0/10   │  │   0/10   │  │   1/10   │  │   0/10   │  │   0/10   │
  └──────────┘  └──────────┘  └──────────┘  └──────────┘  └──────────┘
```

При переполнении какого-то бакета, все события попадающие в этот временной интервал будут проигнорированы.

В момент времени `T + 1` бакеты будут такие:
```
T - 4        T - 3         T - 2         T - 1           T          T + 1
  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐
  │   0/10   │  │   1/10   │  │   0/10   │  │   0/10   │  │   0/10   │
  └──────────┘  └──────────┘  └──────────┘  └──────────┘  └──────────┘
```

Т.е. все бакеты сдвинулись на одну влево. Все события со временем старше, чем `T-4` будут проигнорированы.


## Сборка

Filebeat не предоставляет механизма для динамического подключения плагинов, поэтому нужно пересобирать бинарник filebeat'a с нашим плагином.  
Для этого в пакет `main` filebeat'a подкладывается файл `beats/register_ratelimit_plugin.go`, который импортирует пакет плагина.

## Генератор логов

В `generator` лежит простой генератор логов, который удобно использовать для локального тестирования.  
```
➜ go get -u gitlab.ozon.ru/sre/filebeat-ratelimit-plugin/generator
➜ generator -h
Usage of generator:
  -id id
        id field value (default "generator")
  -rate float
        records per second (default 10)
Usage of generator:
  -id id
        id field value (default "generator")
  -rate float
        records per second (default 10)
➜ generator -id foo
{"ts":"2019-01-09T18:59:56.109522+03:00", "message": "1", "id":"foo"}
{"ts":"2019-01-09T18:59:56.207788+03:00", "message": "2", "id":"foo"}
{"ts":"2019-01-09T18:59:56.310223+03:00", "message": "3", "id":"foo"}
{"ts":"2019-01-09T18:59:56.409879+03:00", "message": "4", "id":"foo"}
{"ts":"2019-01-09T18:59:56.509572+03:00", "message": "5", "id":"foo"}
{"ts":"2019-01-09T18:59:56.608653+03:00", "message": "6", "id":"foo"}
{"ts":"2019-01-09T18:59:56.708547+03:00", "message": "7", "id":"foo"}
{"ts":"2019-01-09T18:59:56.809872+03:00", "message": "8", "id":"foo"}
^C
```

С помощью ключа `-rate` можно управлять кол-вом сообщением в секунду, которое будет выдавать генератор.
