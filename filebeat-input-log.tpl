- type: log
  enabled: true
  paths:
    - {{ .logPath }}/{{ .logFiles }}
  fields:
    module_name: {{ .moduleName }}
    log_type: {{ .logType }}