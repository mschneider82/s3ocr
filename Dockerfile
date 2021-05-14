FROM golang:alpine as builder
RUN mkdir -p /s3ocr
ADD . /s3ocr
WORKDIR /s3ocr
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o s3ocr .

FROM jbarlow83/ocrmypdf
RUN mkdir -p /home/docker
COPY --from=builder /s3ocr/s3ocr /home/docker
RUN chmod +x /home/docker/s3ocr
ENTRYPOINT [ "/home/docker/s3ocr" ]
