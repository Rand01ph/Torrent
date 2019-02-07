- type: log
  enabled: true
  paths:
    - {{ .logPath }}/*.log
  fields:
    module_name: {{ .moduleName }}