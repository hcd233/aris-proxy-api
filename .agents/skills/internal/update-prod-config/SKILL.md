---
name: update-prod-config
description: Update production configuration for aris-proxy-api. Use when the user asks to update api.env, K8s ConfigMap, or any production configuration on the remote server. Also triggers for related actions like "add config", "modify env", "update cron settings", "change production variables". The production host resolves via api.lvlvko.top, NOT a raw IP.
---

# update-prod-config

SSH to the production server and update configuration. Never use raw IPs — always resolve via `api.lvlvko.top`.

## SSH Connection

```bash
ssh ubuntu@api.lvlvko.top
```

## Config locations

| Config | Path | Update method |
|--------|------|---------------|
| api.env | `/home/ubuntu/code/aris-proxy-api/env/api.env` | Direct file edit (`echo >>` or `sed -i`) |
| K8s ConfigMap | namespace `aris-proxy-api`, name `aris-proxy-api-config` | `kubectl patch configmap` |

> **重要**：Deployment 通过 `envFrom` 引用 ConfigMap（而非直接读取 api.env），因此 **api.env 和 ConfigMap 必须同步更新**，否则重启后配置不生效。具体流程：先修改 api.env 作为源文件记录，然后 patch ConfigMap，最后滚动重启。

## Updating api.env

`api.env` 是配置文件源码，修改后必须同步到 ConfigMap。

Append new key=value:

```bash
ssh ubuntu@api.lvlvko.top 'echo "NEW_KEY=value" >> /home/ubuntu/code/aris-proxy-api/env/api.env'
```

Modify an existing value (sed in-place):

```bash
ssh ubuntu@api.lvlvko.top "sed -i 's/^EXISTING_KEY=.*/EXISTING_KEY=new-value/' /home/ubuntu/code/aris-proxy-api/env/api.env"
```

Verify:

```bash
ssh ubuntu@api.lvlvko.top 'grep "NEW_KEY\|EXISTING_KEY" /home/ubuntu/code/aris-proxy-api/env/api.env'
```

## Updating K8s ConfigMap

Add a new key:

```bash
ssh ubuntu@api.lvlvko.top 'kubectl patch configmap aris-proxy-api-config -n aris-proxy-api --type merge -p "{\"data\":{\"KEY\":\"value\"}}"'
```

Verify:

```bash
ssh ubuntu@api.lvlvko.top 'kubectl get configmap aris-proxy-api-config -n aris-proxy-api -o jsonpath="{.data}"'
```

## After updating

K8s 不会自动监听到 ConfigMap 变更后热加载环境变量，必须滚动重启使新配置生效：

```bash
ssh ubuntu@api.lvlvko.top 'kubectl rollout restart deployment -n aris-proxy-api'
```

等待 rollout 完成：

```bash
ssh ubuntu@api.lvlvko.top 'kubectl rollout status deployment -n aris-proxy-api --timeout=120s'
```

## Safety rules

1. Always verify the change after applying (`grep` for api.env, `kubectl get configmap` for ConfigMap).
2. Never commit or log the raw server IP address.
3. If you need to check current config before modifying, read it first with `cat` or `kubectl get configmap`.
4. For multi-line or complex config values, echo with single quotes around the value to avoid shell expansion.
