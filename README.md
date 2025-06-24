# Activator

A simple HTTP reverse proxy for preview environments in Kubernetes. It automatically scales up deployments on demand and proxies requests to the appropriate service.

## Features
- On-demand scaling of preview deployments
- Reverse proxy to Kubernetes services
- Readiness cache for efficient routing

## Prerequisites
- Go 1.18+
- Access to a Kubernetes cluster (in-cluster configuration)

## Setup
1. Build the binary:
   ```sh
   make build
   ```
2. Deploy to your Kubernetes cluster as a Pod or Deployment.

## Usage
The service listens on port 8080 and proxies requests to preview services:

```
GET /preview/<env>/<svc>/<path>
```

- `<env>`: Preview environment name
- `<svc>`: Service name
- `<path>`: Path to forward to the service

Example:
```
GET /preview/dev/myservice/api/healthz
```

## License
MIT 