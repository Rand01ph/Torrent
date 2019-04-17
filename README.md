`spec.template.metadata.annotations`

使用K8S `annotations` 标记需要收集日志的`Pod`

使用 `torrent/module_name` 标记模块名称
使用 `torrent/log_path` 标记log信息，使用`;`分隔多个日志路径，使用`:`分隔日志类型及日志目录及日志文件名。

示例:
```yaml
apiVersion: v1
kind: Pod
metadata:
  name: busybox
  namespace: default
  annotations:
    torrent/module_name: busybox
    torrent/log_path: "nginx:/busybox-data:*.log;pro:/var/log:pro.log"
```

支持使用ConfigMap对Filebeat的input进行配置并进行热更新,其中K8S标准输出部分日志通过该方式支持:
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: filebeat-inputs
  namespace: monitoring
  labels:
    k8s-app: torrent-logging
data:
  kubernetes.yml: |-
    - type: docker
      containers.ids:
      - "*"
      processors:
        - add_kubernetes_metadata:
            in_cluster: true
```

# TODO
Update Pod for log