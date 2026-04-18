# Iceberg Benchmark Scenarios

## 1. IcebergRead (`iceberg_read.go`) — catalog-read

**纯读操作**，按权重分布并发执行。

| 操作 | REST API |
|------|----------|
| NS_GET | `GET /v1/namespaces/{ns}` |
| NS_HEAD | `HEAD /v1/namespaces/{ns}` |
| NS_LIST | `GET /v1/namespaces?parent={parent}` |
| TABLE_GET | `GET /v1/namespaces/{ns}/tables/{table}` |
| TABLE_HEAD | `HEAD /v1/namespaces/{ns}/tables/{table}` |
| TABLE_LIST | `GET /v1/namespaces/{ns}/tables` |
| VIEW_GET | `GET /v1/namespaces/{ns}/views/{view}` |
| VIEW_HEAD | `HEAD /v1/namespaces/{ns}/views/{view}` |
| VIEW_LIST | `GET /v1/namespaces/{ns}/views` |

## 2. IcebergCommits (`iceberg_commits.go`) — catalog-commits

**写/更新操作**，并发提交属性变更（SetProperties）。

| 操作 | REST API |
|------|----------|
| TABLE_UPDATE | `POST /v1/namespaces/{ns}/tables/{table}` (SetProperties) |
| VIEW_UPDATE | `POST /v1/namespaces/{ns}/views/{view}` (SetProperties) |

## 3. IcebergMixed (`iceberg_mixed.go`) — catalog-mixed

**读写混合操作**，按权重分布并发执行所有读+写操作。

| 操作 | REST API |
|------|----------|
| NS_GET | `GET /v1/namespaces/{ns}` |
| NS_HEAD | `HEAD /v1/namespaces/{ns}` |
| NS_LIST | `GET /v1/namespaces?parent={parent}` |
| NS_UPDATE | `POST /v1/namespaces/{ns}/properties` (SetProperties) |
| TABLE_GET | `GET /v1/namespaces/{ns}/tables/{table}` |
| TABLE_HEAD | `HEAD /v1/namespaces/{ns}/tables/{table}` |
| TABLE_LIST | `GET /v1/namespaces/{ns}/tables` |
| TABLE_UPDATE | `POST /v1/namespaces/{ns}/tables/{table}` (SetProperties) |
| VIEW_GET | `GET /v1/namespaces/{ns}/views/{view}` |
| VIEW_HEAD | `HEAD /v1/namespaces/{ns}/views/{view}` |
| VIEW_LIST | `GET /v1/namespaces/{ns}/views` |
| VIEW_UPDATE | `POST /v1/namespaces/{ns}/views/{view}` (SetProperties) |

## 4. Iceberg (`iceberg_sustained.go`) — sustained

**数据写入+元数据提交**，持续工作负载。

| 操作 | REST API |
|------|----------|
| UPLOAD | S3 `PutObject` — 上传 parquet 数据文件 |
| COMMIT | `POST /v1/namespaces/{ns}/tables/{table}` (AppendFiles commit) |
| TABLE_GET (SimulateRead) | `GET /v1/namespaces/{ns}/tables/{table}` — 读表元数据 |