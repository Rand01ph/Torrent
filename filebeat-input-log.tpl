- type: log
  enabled: true
  harvester_buffer_size: 10485760
  paths:
    - {{ .logPath }}/{{ .logFiles }}
  fields:
    module_name: {{ .moduleName }}
    log_type: {{ .logType }}
  tail_files: true
  max_bytes: 1048576
  close_inactive: 1h
  close_renamed: true
  close_removed: true
  close_eof: false
  close_timeout: 3600
