# syntax=docker/dockerfile:1
FROM public.ecr.aws/docker/library/golang as builder
WORKDIR /src/hypatia
ENV GOPROXY direct
COPY go.mod go.sum ./
RUN go mod download && go mod verify
COPY . .
RUN go build -o /bin/protec /src/hypatia/cmd/protec
RUN go build -o /bin/healthcheck /src/hypatia/cmd/healthcheck

FROM public.ecr.aws/nginx/nginx as stager
COPY --from=builder /bin/protec /bin/protec
COPY --from=builder /bin/healthcheck /bin/healthcheck