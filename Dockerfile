FROM alpine
COPY ./app /app
COPY ./filebeat-input-log.tpl /filebeat-input-log.tpl
ENTRYPOINT /app
