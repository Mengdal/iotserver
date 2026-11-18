FROM alpine:3.20

# docker-cli 用于轮转后重启 broker，让它重新打开新的日志文件。
RUN apk add --no-cache logrotate docker-cli

COPY logrotate.conf /etc/logrotate.conf
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
