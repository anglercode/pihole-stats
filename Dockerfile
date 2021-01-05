#
# Dockerfile which builds code using golang:alpine image.
# Generated binary is then packaged into smaller image with Alpine. 
#

# Use the golang image to build the code:
FROM golang:alpine as builder_image

RUN apk update && apk add --no-cache git
WORKDIR /app

COPY . .
RUN go mod download
RUN GOOS=linux go build -o pihole_stats .

# Build the App image using Apline for smaller size:
FROM alpine:latest
RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy binary file from the build stage:
COPY --from=builder_image /app/pihole_stats .
COPY --from=builder_image /app/.env .       

CMD ["./pihole_stats"]
