# Start with the official Go image
FROM golang:1.22-alpine

# Set our working directory inside the container
WORKDIR /app

# Copy our main.go file into the container
COPY main.go .

# Compile our server into a binary program
RUN go build -o server main.go

# Expose the port
EXPOSE 3001

# Run our compiled server
CMD ["./server"]