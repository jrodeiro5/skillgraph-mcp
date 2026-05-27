FROM golang:1.26 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /bin/skillgraph-mcp .

FROM gcr.io/distroless/base-debian12:nonroot
COPY --from=build /bin/skillgraph-mcp /bin/skillgraph-mcp
COPY --from=build /src/LICENSE /LICENSE
ENTRYPOINT ["/bin/skillgraph-mcp"]
