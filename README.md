使用k8s标签 `torrent` 区分是否需要收集日志的`Pod`

使用 `MODULE_NAME` 标记模块名称

使用 `LOG_PATH` 标记log信息，使用`;`分隔多个日志路径，使用`:`分隔日志类型及路径。
```yaml
- name: MODULE_NAME
  value: "busybox"
- name: LOG_PATH
  value: "nginx:/tmp/*.log;pro:/var/log/*.log"
```
      
