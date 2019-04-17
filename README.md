# torrent



# Overview

`torrent` is a kubernetes log tool. 

With `torrent` you can collect logs from kubernetes pods and send them to your centralized log system such as elasticsearch, kafka, logstash and etc.

 `torrent` can collect not only container stdout but also log file that inside kubernetes containers.



![Torrent](./docs/filebeat/Torrent.png)

# Feature

- Support both stdout and container log files

- Watching and re-reading config files

- You could add tags(module_name) on the logs collected, and later filter by tags in log management.

- Multiple log path support(log_path), support one container have many log dir and wildcard match log name.

  

# Quick Start

```bash
kubectl apply -f https://raw.githubusercontent.com/Rand01ph/Torrent/master/torrent.yaml
```



# Configurations



### Container log files config

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



### Container stdout config

同样也可以使用ConfigMap对Filebeat的input进行配置并支持热更新,其中K8S容器stdout部分日志通过该方式支持:

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



### Logstash config for log files

```bash
input {
  kafka {
    bootstrap_servers => "broker:9092"
    topics => ["topics"]
    codec => json
  }
}

filter {
    grok {
        match => {
           source => "/(?<logname>[^/]+)\.log$"
        }
    }
    date {
        match => ["timestamp", "yyyy-MM-dd'T'HH:mm:ss.SSSSSSZZ"]
    }
}

output {
#    stdout { codec => rubydebug }
    file {
        path => "/opt/log_bak/%{+yyyy}/%{+MM}/%{+dd}/%{[fields][module_name]}/%{[fields][log_type]}-%{logname}.log"
        codec => line {
            format => "%{message}"
        }
        flush_interval => 2
    }
}
```





# TODO

- [ ] Update pod for log change
- [ ] README
- [ ] push torrent image to DockerHub



# Contribute

Feel free to open issues and pull requests. Any feedback is highly appreciated!