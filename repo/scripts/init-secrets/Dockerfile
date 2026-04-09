FROM alpine:3.19
RUN apk add --no-cache openssl
COPY init-secrets.sh /scripts/init-secrets.sh
RUN chmod +x /scripts/init-secrets.sh
ENTRYPOINT ["/scripts/init-secrets.sh"]
