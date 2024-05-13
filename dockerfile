# syntax=docker/dockerfile:1
FROM public.ecr.aws/docker/library/golang as builder
WORKDIR /src/hypatia
ENV GOPROXY direct
COPY go.mod go.sum ./
RUN go mod download && go mod verify
COPY *.go .
COPY cmd cmd
RUN go build -o /bin/protec /src/hypatia/cmd/protec
RUN go build -o /bin/healthcheck /src/hypatia/cmd/healthcheck
RUN go build -o /bin/hypatia /src/hypatia/cmd/hypatia


FROM public.ecr.aws/nginx/nginx as stager
RUN apt update -y
RUN apt install -y supervisor
COPY --from=builder /bin/protec /bin/protec
COPY --from=builder /bin/healthcheck /bin/healthcheck
COPY --from=builder /bin/hypatia /bin/hypatia
COPY default.conf /etc/nginx/conf.d/default.conf
COPY supervisord.conf /etc/supervisor/conf.d/supervisord.conf
RUN touch local.status
RUN touch remote.status
CMD ["/usr/bin/supervisord"]