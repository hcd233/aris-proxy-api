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

## Updating api.env

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

Not all configs trigger an automatic reload. If the app doesn't pick up the change, check if pods need a rolling restart:

```bash
ssh ubuntu@api.lvlvko.top 'kubectl rollout restart deployment -n aris-proxy-api'
```

## Safety rules

1. Always verify the change after applying (`grep` for api.env, `kubectl get configmap` for ConfigMap).
2. Never commit or log the raw server IP address.
3. If you need to check current config before modifying, read it first with `cat` or `kubectl get configmap`.
4. For multi-line or complex config values, echo with single quotes around the value to avoid shell expansion.
