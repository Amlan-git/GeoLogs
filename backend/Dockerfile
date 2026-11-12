FROM golang:1.27.4-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /out/geolog ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/geolog /geolog
EXPOSE 8080
ENTRYPOINT ["/geolog"]
