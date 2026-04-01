package main

import (
	"context" // 🚨 ADDED: Required for the timeout kill switch
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/gorilla/websocket"
)

// Define the shape of our WebSocket messages
type WSMessage struct {
	Type string `json:"type"` // "start" (from React), "input" (from React), "output" (from Go), "error" (from Go), "exit" (from Go)
	Data string `json:"data"` // The actual code, typed input, or terminal output
}

type FormatRequest struct {
	Code string `json:"code"`
}

type FormatResponse struct {
	FormattedCode string `json:"formattedCode,omitempty"`
	Error         string `json:"error,omitempty"`
}

// Helper function for HTTP routes
func enableCors(w *http.ResponseWriter) {
	(*w).Header().Set("Access-Control-Allow-Origin", "*")
	(*w).Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
	(*w).Header().Set("Access-Control-Allow-Headers", "Content-Type")
}

// The "Wake Up" route
func pingHandler(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)
	w.Write([]byte("Server is awake and ready!"))
}

// The WebSocket Upgrader
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for the sandbox
	},
}

// The interactive execution route
func wsExecuteHandler(w http.ResponseWriter, r *http.Request) {
	// 1. Upgrade the standard HTTP request to a persistent WebSocket tunnel
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println("WebSocket Upgrade Error:", err)
		return
	}
	defer conn.Close()

	// 2. Wait for the React frontend to send the "start" message with the code
	var startMsg WSMessage
	if err := conn.ReadJSON(&startMsg); err != nil || startMsg.Type != "start" {
		conn.WriteJSON(WSMessage{Type: "error", Data: "Failed to receive start command."})
		return
	}

	// 3. Create a unique file for this session
	fileName := fmt.Sprintf("temp_%d.go", time.Now().UnixNano())
	os.WriteFile(fileName, []byte(startMsg.Data), 0644)
	defer os.Remove(fileName)

	// 🚨 4. THE BACKEND FIX: Create a 60-second timeout context
	// 🚨 THE FIX: Increased to 60 seconds to allow for fmt.Scan and human typing
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Second)
	defer cancel()

	// 🚨 5. THE BACKEND FIX: Attach the context to the command
	cmd := exec.CommandContext(ctx, "go", "run", fileName)

	// 6. Create "Pipes" directly into the running program's brain
	stdinPipe, _ := cmd.StdinPipe()
	stdoutPipe, _ := cmd.StdoutPipe()
	stderrPipe, _ := cmd.StderrPipe()

	// 7. Start the program (do not wait for it to finish yet!)
	if err := cmd.Start(); err != nil {
		conn.WriteJSON(WSMessage{Type: "error", Data: "Failed to start compiler: " + err.Error()})
		return
	}

	// 8. STREAM OUTPUT: Continuously read terminal output and send it to React
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := stdoutPipe.Read(buf)
			if n > 0 {
				conn.WriteJSON(WSMessage{Type: "output", Data: string(buf[:n])})
			}
			if err != nil {
				break
			}
		}
	}()

	// 9. STREAM ERRORS: Continuously read compiler errors and send to React
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := stderrPipe.Read(buf)
			if n > 0 {
				conn.WriteJSON(WSMessage{Type: "error", Data: string(buf[:n])})
			}
			if err != nil {
				break
			}
		}
	}()

	// 10. HANDLE INPUT: Listen for the user typing in React, and inject it into the Go program
	go func() {
		for {
			var inMsg WSMessage
			if err := conn.ReadJSON(&inMsg); err != nil {
				// If the user closes the browser tab, kill the Go program immediately
				if cmd.Process != nil {
					cmd.Process.Kill()
				}
				break
			}
			if inMsg.Type == "input" {
				// Inject the text, adding a newline so fmt.Scan knows they pressed Enter
				stdinPipe.Write([]byte(inMsg.Data + "\n"))
			}
		}
	}()

	// 🚨 11. WAIT & EVALUATE: Check why the program finished
	err = cmd.Wait()

	// Did it die because our 5-second context killed it?
	if ctx.Err() == context.DeadlineExceeded {
		conn.WriteJSON(WSMessage{Type: "error", Data: "\n\n[SYSTEM] 🛑 Execution terminated: Max execution time (50 seconds) reached.\nIf its not infinite loop, kindly take your code and run it on your personal pc\n"})
	} else if err != nil {
		// Did it die from a normal compiler error or panic?
		conn.WriteJSON(WSMessage{Type: "exit", Data: "\n[Process exited with an error]"})
	} else {
		// Did it finish naturally?
		conn.WriteJSON(WSMessage{Type: "exit", Data: "\n[Process finished successfully]"})
	}
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "3001"
	}

	http.HandleFunc("/ping", pingHandler)
	http.HandleFunc("/execute", wsExecuteHandler)

	fmt.Printf("Go Engine running on port %s\n", port)
	http.ListenAndServe(":"+port, nil)
}
