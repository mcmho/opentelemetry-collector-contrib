# Loki Exporter

| Status                   |           |
| ------------------------ |-----------|
| Stability                | [beta]    |
| Supported pipeline types | logs      |
| Distributions            | [contrib] |

Exports data via HTTP to [Loki](https://grafana.com/docs/loki/latest/).

## Getting Started

The following settings are required:

- `endpoint` (no default): The target URL to send Loki log streams to (e.g.: `http://loki:3100/loki/api/v1/push`).
  
- `labels.{attributes/resource}` (no default): Either a map of attributes or resource names to valid Loki label names 
  (must match "^[a-zA-Z_][a-zA-Z0-9_]*$") allowed to be added as labels to Loki log streams. 
  Attributes are log record attributes that describe the log message itself. Resource attributes are attributes that 
  belong to the infrastructure that create the log (container_name, cluster_name, etc.). At least one attribute from
  attribute or resource is required 
  Logs that do not have at least one of these attributes will be dropped. 
  This is a safety net to help prevent accidentally adding dynamic labels that may significantly increase cardinality, 
  thus having a performance impact on your Loki instance. See the 
  [Loki label best practices](https://grafana.com/docs/loki/latest/best-practices/) page for 
  additional details on the types of labels you may want to associate with log streams.

- `labels.record` (no default): A map of record attributes to valid Loki label names (must match 
  "^[a-zA-Z_][a-zA-Z0-9_]*$") allowed to be added as labels to Loki log streams.
  Record attributes can be: `traceID`, `spanID`, `severity`, `severityN`. These attributes will be added as log labels 
  and will be removed from the log body.

The following settings can be optionally configured:

- `tenant`: composed of the properties `tenant.source` and `tenant.value`.
- `tenant.source`: one of "static", "context", or "attribute". 
- `tenant.value`: the semantics depend on the tenant source. See the "Tenant information" section.

- `tls`:
  - `insecure` (default = false): When set to true disables verifying the server's certificate chain and host name. The
  connection is still encrypted but server identity is not verified.
  - `ca_file` (no default) Path to the CA cert to verify the server being connected to. Should only be used if `insecure` 
  is set to false.
  - `cert_file` (no default) Path to the TLS cert to use for client connections when TLS client auth is required. 
  Should only be used if `insecure` is set to false.
  - `key_file` (no default) Path to the TLS key to use for TLS required connections. Should only be used if `insecure` is
  set to false.


- `timeout` (default = 30s): HTTP request time limit. For details see https://golang.org/pkg/net/http/#Client
- `read_buffer_size` (default = 0): ReadBufferSize for HTTP client.
- `write_buffer_size` (default = 512 * 1024): WriteBufferSize for HTTP client.


- `headers` (no default): Name/value pairs added to the HTTP request headers.

- `format` Deprecated without replacement. If you rely on this, let us know by opening an issue before v0.59.0 and we'll 
assist you in finding a solution. The current default is `body` but the `json` encoder will be used after v0.59.0. To be
ready for future versions, set this to `json` explicitly.

Example:

```yaml
loki:
  endpoint: http://loki:3100/loki/api/v1/push
  tenant_id: "example"
  labels:
    resource:
      # Allowing 'container.name' attribute and transform it to 'container_name', which is a valid Loki label name.
      container.name: "container_name"
      # Allowing 'k8s.cluster.name' attribute and transform it to 'k8s_cluster_name', which is a valid Loki label name.
      k8s.cluster.name: "k8s_cluster_name"
    attributes:
      # Allowing 'severity' attribute and not providing a mapping, since the attribute name is a valid Loki label name.
      severity: ""
      http.status_code: "http_status_code" 
    record:
      # Adds 'traceID' as a log label, seen as 'traceid' in Loki.
      traceID: "traceid"

  headers:
    "X-Custom-Header": "loki_rocks"
```

The full list of settings exposed for this exporter are documented [here](./config.go) with detailed sample
configurations [here](./testdata/config.yaml).

## Tenant information

This processor is able to acquire the tenant ID based on different sources. At this moment, there are three possible sources:

- static
- context
- attribute

Each one has a strategy for obtaining the tenant ID, as follows:

- when "static" is set, the tenant is the literal value from the "tenant.value" property. 
- when "context" is set, the tenant is looked up from the request metadata, such as HTTP headers, using the "value" as the
key (likely the header name).
- when "attribute" is set, the tenant is looked up from the resource attributes in the batch: the first value found among
the resource attributes is used. If you intend to have multiple tenants per HTTP request, make sure to use a processor
that groups tenants in batches, such as the `groupbyattrs` processor.

The value that is determined to be the tenant is then sent as the value for the HTTP header `X-Scope-OrgID`. When a tenant
is not provided, or a tenant cannot be determined, the logs are still sent to Loki but without the HTTP header.

## Advanced Configuration

Several helper files are leveraged to provide additional capabilities automatically:

- [HTTP settings](https://github.com/open-telemetry/opentelemetry-collector/blob/main/config/confighttp/README.md)
- [Queuing and retry settings](https://github.com/open-telemetry/opentelemetry-collector/blob/main/exporter/exporterhelper/README.md)

[beta]:https://github.com/open-telemetry/opentelemetry-collector#beta
[contrib]:https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol-contrib