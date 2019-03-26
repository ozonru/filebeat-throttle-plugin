[![Build Status](https://travis-ci.org/ozonru/filebeat-throttle-plugin.svg?branch=master)](https://travis-ci.org/ozonru/filebeat-throttle-plugin)
[![Go Report Card](https://goreportcard.com/badge/github.com/ozonru/filebeat-throttle-plugin)](https://goreportcard.com/report/github.com/ozonru/filebeat-throttle-plugin)

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

`value` specifies maximum number of events that will be passed in interval `bucket_size`.
In `conditions` section you use any fields from your events. All conditions works as `equal`.


## Throttling algorithm

We use `token bucket` algorithm for throttling. In the simplest way we can use only single bucket for events limit. But in real life beats can be down (maintance, some failures, etc): in this case all events (new and old ones) use tokens from same bucket and some events can be skipped because of overflow. To avoid such situations we need to keep N last bucket and use event timestamp to choose bucket.

Imagine that processor has following configuration:
```
bucket_size: 1 // 1 second
buckets: 5
```

And rule with `limit: 10`

Then in the time moment T we'll have buckets:
```
T - 5        T - 4         T - 3         T - 2         T - 1          T
  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐
  │   0/10   │  │   0/10   │  │   0/10   │  │   0/10   │  │   0/10   │
  └──────────┘  └──────────┘  └──────────┘  └──────────┘  └──────────┘
```

After adding event in interval between `T-3` and `T-2`:
```
T - 5        T - 4         T - 3         T - 2         T - 1          T
  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐
  │   0/10   │  │   0/10   │  │   1/10   │  │   0/10   │  │   0/10   │
  └──────────┘  └──────────┘  └──────────┘  └──────────┘  └──────────┘
```

If one one of buckets overflows all new events will be ignored.

At `T + 1` all buckets shift left:
```
T - 4        T - 3         T - 2         T - 1           T          T + 1
  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐
  │   0/10   │  │   1/10   │  │   0/10   │  │   0/10   │  │   0/10   │
  └──────────┘  └──────────┘  └──────────┘  └──────────┘  └──────────┘
```

All events with timestamp earlier than `T-4` will be ignored.


## Building

There are two options how you can build and use throttle processor:
 - build beat binary with throttle processor inside
 - use it as separate plugin
 
### Compile-in

You have to copy `register/compile/plugin.go` to the `main` package of required beat.
```
cp $GOPATH/github.com/ozonru/filebeat-throttle-plugin/register/compile/plugin.go $GOPATH/github.com/elastic/beats/filebeat
go build github.com/elastic/beats/filebeat
```

### Plugin

You can build plugin both for linux and MacOS:

```
make linux_plugin
make darwin_plugin
```

To use plugin make sure that you have the same Go version as beat binary was built.

## Log generator (for development)

In `generator` you can find simple log generator that can be used for local development & testing).  
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
