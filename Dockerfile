FROM alpine:latest
COPY PrismaTec /usr/local/bin/
RUN chmod +x /usr/local/bin/PrismaTec
EXPOSE 8080
CMD ["/usr/local/bin/PrismaTec"]
