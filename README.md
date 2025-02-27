# ES Index Exporter with Prometheus Metrics

## Introduction

This tool checks the existence of Elasticsearch indices and exposes Prometheus metrics. It periodically queries Elasticsearch and updates Prometheus metrics based on whether the indices exist within a specified time range.

## Directory Structure
```
.
├── Dockerfile
├── main.go
├── go.mod
└── README.md
```

## Building the Image

### Prerequisites

- Install [Docker](https://docs.docker.com/get-docker/)

### Steps

1. **Clone the Repository**

   ```sh
   git clone https://github.com/acaiblog/esindex_exporter.git
   cd esindex_exporter
   ```

2. **Build the Docker Image**

   Run the following command in the root directory of the project to build the Docker image:
   ```bash
   docker build -t esindex_exporter .
   ```

3. **Run the Container**

   You can use the following command to run the container. By default, the container listens on port 9184 and connects to a service named elasticsearch (assuming it's on the same Docker network).

   ```bash
   docker run -idt --name esindex_exporter \
   -p 9184:9184 \
   -v /etc/localtime:/etc/localtime \
   --restart always esindex_exporter:v1.0 /app/esindex_exporter \
   --es-uri http://{es_user}:{es_password}@{es_host}:9200 \
   --query-interval 10 \
   --es-index-prefix llmstudio- \
   --listen-port 9184 \
   --start-time "09:00" \
   --end-time "18:00" \
   --timeout "5s"
   ```

   **Note**: Replace `{es_user}`, `{es_password}`, and `{es_host}` with your actual Elasticsearch credentials and host.

### Accessing Prometheus Metrics

After starting, you can access the Prometheus metrics via curl:

```bash
curl http://localhost:9184/metrics
```

### Configuration

Here are some commonly used configuration parameters:

- `--es-uri`: Elasticsearch URI (e.g., `http://elastic:password@elasticsearch:9200`)
- `--es-index-prefix`: Index prefix (e.g., `llmstudio-`)
- `--query-interval`: Query interval in seconds (default is 10)
- `--listen-port`: Listening port (default is 9184)
- `--start-time`: Start time in HH:MM format (default is `09:00`)
- `--end-time`: End time in HH:MM format (default is `18:00`)
- `--timeout`: Timeout duration in seconds (default is `5s`)

### Logic Explanation

- The tool periodically checks if an index exists based on the provided `es-index-prefix`.
- If the current time is within the specified start and end times (`--start-time` and `--end-time`), the tool sets the Prometheus metric to `1` if the index exists and `0` if it does not.
- If the current time is outside the specified time range, the tool sets the Prometheus metric to `1` regardless of the index's existence.

## Summary

- **Dockerfile**: Provides all the steps needed to build and run your application.
- **README.md**: Offers detailed instructions to help users understand how to build and use the Docker image.