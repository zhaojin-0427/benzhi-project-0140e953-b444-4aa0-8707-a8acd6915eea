# specimen-custody-gate

`specimen-custody-gate` 是面向自然保护地科研团队的样本监管 HTTP 服务。它把野外样本从建批、许可登记、样本与封签登记、离场预检与正式核验、连续保管交接、到站验收、差异整改和复验，一直推进到合规复核与保藏入库凭证签发。所有写操作都要求 `expectedVersion` 和 `idempotencyKey`，从而避免并发覆盖和重复提交。

批次详情会附带按许可编号和材料代码稳定分配的额度占用、剩余额度与结构化缺口预警，以及当前保管人、最后交接地点和在途异常概览。离场就绪预检和差异筛选都是只读操作，不产生事件也不改变批次版本。整改提交会保留单调递增的修订历史，复验必须针对最新修订、填写意见并由不同操作者执行。入库凭证查询同时验证凭证字段摘要、冻结清单摘要、数量与版本，以及 `certificate.issued` 签发事件的校验链和审计结果。

服务只依赖 Go 标准库。数据保存在本地 JSON Lines 事件日志中；日志包含单调序号和连续 SHA-256 校验摘要。每次提交还会同步生成通过临时文件原子替换的投影快照。启动时服务校验完整事件链并重放恢复，遇到损坏会拒绝启动并指出事件序号。

## 构建与测试

要求 Go 1.22 或更高版本。

```text
go build ./cmd/server
go test ./...
```

项目自检会实际启动回环 HTTP 监听，执行含整改分支的完整流程，验证幂等重放、冻结保护、时间线和凭证摘要，然后主动关闭：

```text
go run ./cmd/server -selfcheck -addr=127.0.0.1:19081
```

## 运行

默认监听 `127.0.0.1:19081`，默认数据目录为 `.data/specimen-custody-gate`：

```text
go run ./cmd/server
```

可显式指定回环监听地址和数据目录：

```text
go run ./cmd/server -addr=127.0.0.1:19082 -data=./runtime-data
```

未传入 `-addr` 时，也可通过 `PORT` 指定端口；服务会绑定 `127.0.0.1:<PORT>`。为避免意外暴露监管数据，`-addr` 只接受 `127.0.0.1`。

## API 使用约定

写请求使用 `Content-Type: application/json`，请求体最大 1 MiB，未知 JSON 字段会被拒绝。每个写请求还要提供以下请求头：

- `X-Actor`：实际操作者；
- `X-Role`：`collector`、`custodian`、`receiver` 或 `compliance`；
- `X-Request-ID`：调用方生成的请求标识。

请求体必须包含当前批次版本 `expectedVersion` 和调用方稳定生成的 `idempotencyKey`。相同幂等键与相同载荷返回原结果；相同键用于不同载荷会返回 `409 idempotency_conflict`。版本不匹配返回 `409 version_conflict`，字段或业务核验问题返回带 `issues` 清单的 `422 validation_failed`。

主要路由如下：

- `POST /api/v1/batches`：创建 `draft` 批次；
- `POST /api/v1/batches/{batchID}/permits`：登记采集许可；
- `POST /api/v1/batches/{batchID}/specimens`：登记样本、容器和封签；
- `GET /api/v1/batches/{batchID}/departure-readiness`：只读执行离场就绪预检并返回分类计数；
- `POST /api/v1/batches/{batchID}/departure-verification`：执行离场核验；
- `POST /api/v1/batches/{batchID}/handoffs`：记录已由双方确认的顺序交接；
- `POST /api/v1/batches/{batchID}/arrival-inspections`：登记实收清单并生成差异；
- `POST /api/v1/batches/{batchID}/discrepancies/{discrepancyID}/remediation`：以请求头操作者身份追加整改修订；
- `POST /api/v1/batches/{batchID}/discrepancies/{discrepancyID}/review`：验收员提交最新 `revision`、`approved` 和 `opinion`；
- `POST /api/v1/batches/{batchID}/arrival-reverification`：所有问题关闭后完成到站复验；
- `POST /api/v1/batches/{batchID}/compliance-approval`：冻结清单并签发入库凭证；
- `GET /api/v1/batches/{batchID}`：查询批次详情；
- `GET /api/v1/batches/{batchID}/discrepancies`：查询问题清单，可组合使用单值 `status`、`category`、`specimenId`，并返回全批次 `summary` 和 `matchedCount`；
- `GET /api/v1/batches/{batchID}/timeline`：查询带校验摘要的审计时间线；
- `GET /api/v1/batches/{batchID}/certificate`：查询入库凭证及四组独立深度验证结果。

清单冻结后，许可、样本、交接、验收和整改类修改都会被拒绝。凭证包含最终批次版本、样本数、清单摘要、审核人、签发时间和可独立复算的校验摘要。
