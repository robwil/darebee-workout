# build stage
FROM golang:1.10-alpine AS build-env
#RUN apk add --no-cache git
RUN mkdir -p /go/src/github.com/robwil/darebee-workout
WORKDIR /go/src/github.com/robwil/darebee-workout
COPY . .
RUN go build -o darebee-workout

# final stage
FROM alpine:3.7
WORKDIR /app
RUN apk add --no-cache ca-certificates apache2-utils
COPY --from=build-env /go/src/github.com/robwil/darebee-workout/darebee-workout /app/
COPY .gcloud_darebee.json /app/
EXPOSE 5000
ENV GOOGLE_APPLICATION_CREDENTIALS=".gcloud_darebee.json"
ENTRYPOINT ./darebee-workout