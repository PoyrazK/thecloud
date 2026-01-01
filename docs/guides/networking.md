# Networking Guide

This guide covers networking and port mapping features in Mini AWS.

## Port Mapping

Port mapping allows you to expose container ports to your local machine, similar to the `-p` flag in Docker.

### How to use
When launching an instance, use the `-p` or `--port` flag with the format `hostPort:containerPort`.

```bash
cloud compute launch --name web --image nginx:alpine --port 8080:80
```

### Accessing your service
Once launched, if the status is `RUNNING`, you can access the service via `localhost`.

You can see the access URLs by listing your instances:
```bash
cloud compute list
```
Output:
```
┌──────────┬──────────┬──────────────┬─────────┬────────────────────┐
│    ID    │   NAME   │    IMAGE     │ STATUS  │       ACCESS       │
├──────────┼──────────┼──────────────┼─────────┼────────────────────┤
│ a1b2c3d4 │ web      │ nginx:alpine │ RUNNING │ localhost:8080->80 │
└──────────┴──────────┴──────────────┴─────────┴────────────────────┘
```

### Multiple Ports
You can map multiple ports by separating them with a comma (max 10):
```bash
cloud compute launch --name dual --image my-app --port 8080:80,3000:3000
```

## Internal Networking
*Coming soon: VPCs, Subnets, and Security Groups.*
