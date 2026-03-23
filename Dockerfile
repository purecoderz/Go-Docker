# Start from the official Go image
FROM golang:alpine

# Set the working directory inside the container
WORKDIR /app

# 🚨 THE FIX: Copy the module files FIRST
COPY go.mod go.sum ./

# 🚨 THE FIX: Download all dependencies (like Gorilla WebSockets)
RUN go mod download

# Now copy the rest of your code (main.go)
COPY . .

# Build the application
RUN go build -o server main.go

# Expose the port Render uses
EXPOSE 3001

# Run the executable
CMD ["./server"]