package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"time"
	"go/format"
)

// Define the shape of our incoming and outgoing JSON
type ExecuteRequest struct {
	Code string `json:"code"`
}

type ExecuteResponse struct {
	Output string `json:"output,omitempty"`
	Error  string `json:"error,omitempty"`
}

type FormatRequest struct {
	Code string `json:"code"`
}

type FormatResponse struct {
	FormattedCode string `json:"formattedCode,omitempty"`
	Error         string `json:"error,omitempty"`
}

// Helper function to allow your React app to talk to this server
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

// The core execution route
func executeHandler(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)
	
	// Handle preflight requests from the browser
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Read the JSON sent from React
	var req ExecuteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// 1. Create a unique filename using a timestamp
	fileName := fmt.Sprintf("temp_%d.go", time.Now().UnixNano())

	// 2. Write the user's code to the file
	err := os.WriteFile(fileName, []byte(req.Code), 0644)
	if err != nil {
		json.NewEncoder(w).Encode(ExecuteResponse{Error: "Failed to create file on server."})
		return
	}

	// 3. Defer tells Go to delete this file the moment this function finishes
	defer os.Remove(fileName)

	// 4. Run the Go code
	cmd := exec.Command("go", "run", fileName)
	
	// CombinedOutput captures both standard output (fmt.Println) and standard error (compiler bugs)
	out, err := cmd.CombinedOutput() 

	response := ExecuteResponse{}
	if err != nil {
		// If the code failed to compile or crashed, send the output as an error
		response.Error = string(out) 
	} else {
		// If it succeeded, send the output
		response.Output = string(out)
	}

	// 5. Send results back to React
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// This format the code
func formatCode(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)
	
		// Handle the preflight OPTIONS request from the browser
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Ensure it's a POST request
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	// 3. Decode the incoming JSON from the frontend
	var req FormatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		json.NewEncoder(w).Encode(FormatResponse{Error: "Invalid JSON request"})
		return
	}

	// 4. THE MAGIC: Pass the user's code through Go's official formatter
	// format.Source takes a byte slice and returns the perfectly formatted byte slice
	formatted, err := format.Source([]byte(req.Code))
	
	if err != nil {
		// If the user wrote invalid Go code (missing a bracket, typo, etc.), 
		// format.Source throws an error. We send that back to the frontend.
		json.NewEncoder(w).Encode(FormatResponse{
			Error: err.Error(),
		})
		return
	}

	// 5. Send the beautifully formatted code back to React!
	json.NewEncoder(w).Encode(FormatResponse{
		FormattedCode: string(formatted),
	})
}
func main() {
	// Render assigns a dynamic port, so we check for it
	port := os.Getenv("PORT")
	if port == "" {
		port = "3001"
	}

	http.HandleFunc("/ping", pingHandler)
	http.HandleFunc("/execute", executeHandler)
	http.HandleFunc("/format", formatCode)

	fmt.Printf("Go Executor running on port %s\n", port)
	http.ListenAndServe(":"+port, nil)
}