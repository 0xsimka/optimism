package main

// Let's make sure we have dependencies only on the standard library.
import (
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

var (
	appRoot       string
	configsDir    string
	buildDir      string
	buildCmd      string
	port          int
	lastBuildHash string
	buildMutex    sync.Mutex
)

func init() {
	flag.StringVar(&appRoot, "app-root", "/app", "Root directory for the application")
	flag.StringVar(&configsDir, "configs-dir", "chainconfig/configs", "Directory for config files (relative to build-dir)")
	flag.StringVar(&buildDir, "build-dir", "op-program", "Directory where the build command will be executed (relative to app-root)")
	flag.StringVar(&buildCmd, "build-cmd", "just build", "Build command to execute")
	flag.IntVar(&port, "port", 8080, "Port to listen on")
}

func main() {
	flag.Parse()

	// Create a file server for the proofs directory
	proofsDir := filepath.Join(appRoot, buildDir, "bin")
	proofsFileServer := http.FileServer(http.Dir(proofsDir))

	// Set up routes
	http.HandleFunc("/", handleUpload)
	http.Handle("/proofs/", http.StripPrefix("/proofs/", proofsFileServer))

	log.Printf("Starting server on :%d with:", port)
	log.Printf("  app-root: %s", appRoot)
	log.Printf("  configs-dir: %s", filepath.Join(appRoot, buildDir, configsDir))
	log.Printf("  build-dir: %s", filepath.Join(appRoot, buildDir))
	log.Printf("  build-cmd: %s", buildCmd)
	log.Printf("  proofs-dir: %s", proofsDir)

	if err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	log.Printf("Received upload request from %s", r.RemoteAddr)

	// Create configs directory if it doesn't exist
	fullConfigsDir := filepath.Join(appRoot, buildDir, configsDir)
	if err := os.MkdirAll(fullConfigsDir, 0755); err != nil {
		http.Error(w, fmt.Sprintf("Failed to create config directory: %v", err), http.StatusInternalServerError)
		return
	}

	// Parse the multipart form
	if err := r.ParseMultipartForm(1 << 30); err != nil { // 1GB max memory
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	// Process uploaded files
	files := r.MultipartForm.File["files[]"]

	log.Printf("Processing %d files:", len(files))
	for _, fileHeader := range files {
		log.Printf("  - %s (size: %d bytes)", fileHeader.Filename, fileHeader.Size)
	}

	// Calculate hash of all files
	hasher := sha256.New()
	for _, fileHeader := range files {
		file, err := fileHeader.Open()
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to open file: %v", err), http.StatusInternalServerError)
			return
		}
		if _, err := io.Copy(hasher, file); err != nil {
			file.Close()
			http.Error(w, fmt.Sprintf("Failed to hash file: %v", err), http.StatusInternalServerError)
			return
		}
		file.Close()
	}
	currentHash := hex.EncodeToString(hasher.Sum(nil))
	log.Printf("Files hash: %s", currentHash)

	buildMutex.Lock()
	defer buildMutex.Unlock()

	// Check if we need to rebuild
	if currentHash == lastBuildHash {
		log.Printf("Hash matches last build, skipping")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Files unchanged, skipping build")
		return
	}

	log.Printf("Hash differs from last build (%s), proceeding with build", lastBuildHash)

	// Save the files
	for _, fileHeader := range files {
		file, err := fileHeader.Open()
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to open file: %v", err), http.StatusInternalServerError)
			return
		}
		defer file.Close()

		destPath := filepath.Join(fullConfigsDir, normalizeFilename(fileHeader.Filename))
		dst, err := os.Create(destPath)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to create destination file: %v", err), http.StatusInternalServerError)
			return
		}
		defer dst.Close()

		if _, err := io.Copy(dst, file); err != nil {
			http.Error(w, fmt.Sprintf("Failed to save file: %v", err), http.StatusInternalServerError)
			return
		}
		log.Printf("Saved file: %s", destPath)
	}

	// Execute the build script
	log.Printf("Starting build...")
	cmdParts := strings.Fields(buildCmd)
	cmd := exec.Command(cmdParts[0], cmdParts[1:]...)
	cmd.Dir = filepath.Join(appRoot, buildDir)
	out, err := cmd.CombinedOutput()
	log.Printf("Build output:\n%s", out)
	if err != nil {
		log.Printf("Build failed: %v", err)
		http.Error(w, fmt.Sprintf("Build failed: %v\nOutput: %s", err, out), http.StatusInternalServerError)
		return
	}
	log.Printf("Build completed successfully")

	// Update the last successful build hash
	lastBuildHash = currentHash

	// Redirect to /proofs/
	http.Redirect(w, r, "/proofs/", http.StatusSeeOther)
}

func normalizeFilename(filename string) string {
	// Get just the filename without directories
	filename = filepath.Base(filename)

	// Check if filename matches PREFIX-NUMBER.json pattern
	if parts := strings.Split(filename, "-"); len(parts) == 2 {
		if numStr := strings.TrimSuffix(parts[1], ".json"); numStr != parts[1] {
			// Check if the number part is actually numeric
			if _, err := strconv.Atoi(numStr); err == nil {
				// It matches the pattern and has a valid number, reorder to NUMBER-PREFIX.json
				return numStr + "-" + parts[0] + ".json"
			}
		}
	}

	return filename
}
