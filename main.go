package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const (
	// Configuration constants
	uploadPath = "./uploads"  // Base directory for all uploads
	port       = 8080         // Server port
)

// File represents a file or directory in the system
type File struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	IsDir     bool   `json:"is_dir"`
	Size      int64  `json:"size,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

// ResponseMessage represents API response messages
type ResponseMessage struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func main() {
	// Create upload directory if it doesn't exist
	if err := os.MkdirAll(uploadPath, 0755); err != nil {
		log.Fatalf("Failed to create upload directory: %v", err)
	}

	// Set up routes
	http.HandleFunc("/", handleIndex)
	http.HandleFunc("/api/files", handleAPIFiles)
	http.HandleFunc("/api/upload", handleAPIUpload)
	http.HandleFunc("/api/mkdir", handleAPIMkdir)
	http.HandleFunc("/download/", handleDownload)

	// Start the server
	log.Printf("Server starting on port %d...", port)
	log.Printf("Web interface: http://localhost:%d", port)
	log.Printf("Upload directory: %s", uploadPath)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))
}

// handleIndex serves the main web interface
func handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse the HTML template
	tmpl := template.Must(template.New("index").Parse(indexHTML))
	tmpl.Execute(w, map[string]interface{}{
		"Title": "File Server",
	})
}

// handleAPIFiles lists files in the given directory
func handleAPIFiles(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	dirPath := r.URL.Query().Get("path")
	if dirPath == "" {
		dirPath = "/"
	}

	// Make sure we're not accessing outside the upload directory
	fullPath := filepath.Join(uploadPath, filepath.Clean(dirPath))
	relPath, err := filepath.Rel(uploadPath, fullPath)
	if err != nil || strings.HasPrefix(relPath, "..") {
		sendJSONError(w, "Invalid path", http.StatusBadRequest)
		return
	}

	// Initialize an empty file list
	fileList := []File{}

	// Read directory contents
	files, err := os.ReadDir(fullPath)
	if err != nil {
		// If directory doesn't exist yet, just return an empty list
		if os.IsNotExist(err) {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": true,
				"path":    dirPath,
				"files":   fileList,
			})
			return
		}
		
		sendJSONError(w, "Failed to read directory", http.StatusInternalServerError)
		return
	}

	for _, f := range files {
		info, err := f.Info()
		if err != nil {
			continue
		}

		fileList = append(fileList, File{
			Name:      f.Name(),
			Path:      filepath.Join(dirPath, f.Name()),
			IsDir:     f.IsDir(),
			Size:      info.Size(),
			UpdatedAt: info.ModTime().Format("2006-01-02 15:04:05"),
		})
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"path":    dirPath,
		"files":   fileList,
	})
}

// handleAPIUpload handles file uploads
func handleAPIUpload(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		sendJSONError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse the multipart form, 32 << 20 specifies a maximum upload of 32 MB
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		sendJSONError(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	// Get the path where to save the file
	dirPath := r.FormValue("path")
	if dirPath == "" {
		dirPath = "/"
	}

	// Make sure the target directory exists
	fullDirPath := filepath.Join(uploadPath, filepath.Clean(dirPath))
	if err := os.MkdirAll(fullDirPath, 0755); err != nil {
		sendJSONError(w, "Failed to create directory", http.StatusInternalServerError)
		return
	}

	// Get the uploaded file
	file, handler, err := r.FormFile("file")
	if err != nil {
		sendJSONError(w, "Failed to get file from form", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Create the file on the server
	fullPath := filepath.Join(fullDirPath, handler.Filename)
	dst, err := os.Create(fullPath)
	if err != nil {
		sendJSONError(w, "Failed to create file on server", http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	// Copy the file to the destination
	if _, err := io.Copy(dst, file); err != nil {
		sendJSONError(w, "Failed to save file", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(ResponseMessage{
		Success: true,
		Message: fmt.Sprintf("File uploaded successfully to %s", filepath.Join(dirPath, handler.Filename)),
	})
}

// handleAPIMkdir creates a new directory
func handleAPIMkdir(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		sendJSONError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var reqBody struct {
		Path string `json:"path"`
		Name string `json:"name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		sendJSONError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if reqBody.Name == "" {
		sendJSONError(w, "Directory name is required", http.StatusBadRequest)
		return
	}

	// Make sure we're not accessing outside the upload directory
	fullPath := filepath.Join(uploadPath, filepath.Clean(reqBody.Path), reqBody.Name)
	relPath, err := filepath.Rel(uploadPath, fullPath)
	if err != nil || strings.HasPrefix(relPath, "..") {
		sendJSONError(w, "Invalid path", http.StatusBadRequest)
		return
	}

	if err := os.MkdirAll(fullPath, 0755); err != nil {
		sendJSONError(w, "Failed to create directory", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(ResponseMessage{
		Success: true,
		Message: fmt.Sprintf("Directory '%s' created successfully", reqBody.Name),
	})
}

// handleDownload handles file/directory downloads
func handleDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract the path from the URL
	filePath := strings.TrimPrefix(r.URL.Path, "/download")
	if filePath == "" {
		filePath = "/"
	}

	// Make sure we're not accessing outside the upload directory
	fullPath := filepath.Join(uploadPath, filepath.Clean(filePath))
	relPath, err := filepath.Rel(uploadPath, fullPath)
	if err != nil || strings.HasPrefix(relPath, "..") {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	// Check if the path exists
	fileInfo, err := os.Stat(fullPath)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	// If it's a directory and not requesting the root, redirect to the web interface
	if fileInfo.IsDir() && filePath != "/" {
		http.Redirect(w, r, "/?path="+filePath, http.StatusFound)
		return
	}

	// Serve the file
	http.ServeFile(w, r, fullPath)
}

// sendJSONError sends a JSON formatted error response
func sendJSONError(w http.ResponseWriter, message string, statusCode int) {
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": false,
		"error":   message,
	})
}

// HTML template for the web interface
const indexHTML = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.Title}}</title>
    <style>
        body {
            font-family: Arial, sans-serif;
            max-width: 1200px;
            margin: 0 auto;
            padding: 20px;
            line-height: 1.6;
        }
        h1, h2 {
            color: #333;
        }
        .container {
            margin-top: 20px;
        }
        .path-nav {
            background: #f5f5f5;
            padding: 10px;
            border-radius: 4px;
            margin-bottom: 15px;
        }
        .file-list {
            border: 1px solid #ddd;
            border-radius: 4px;
        }
        .file-item {
            display: flex;
            padding: 12px 15px;
            border-bottom: 1px solid #eee;
            align-items: center;
        }
        .file-item:last-child {
            border-bottom: none;
        }
        .file-item:hover {
            background-color: #f9f9f9;
        }
        .file-item .icon {
            margin-right: 10px;
            font-size: 18px;
            width: 24px;
            text-align: center;
        }
        .file-item .name {
            flex: 1;
        }
        .file-item .meta {
            color: #777;
            font-size: 14px;
            margin-right: 15px;
        }
        .file-item a {
            text-decoration: none;
            color: #333;
        }
        .file-item a:hover {
            text-decoration: underline;
        }
        .actions {
            display: flex;
            gap: 10px;
            margin: 20px 0;
        }
        button, .button {
            background: #4CAF50;
            color: white;
            border: none;
            padding: 8px 16px;
            border-radius: 4px;
            cursor: pointer;
            font-size: 14px;
        }
        button:hover, .button:hover {
            background: #45a049;
        }
        input[type="file"] {
            display: none;
        }
        #pathDisplay {
            margin-right: 10px;
            font-weight: bold;
        }
        .modal {
            display: none;
            position: fixed;
            z-index: 1;
            left: 0;
            top: 0;
            width: 100%;
            height: 100%;
            background-color: rgba(0,0,0,0.4);
        }
        .modal-content {
            background-color: #fefefe;
            margin: 15% auto;
            padding: 20px;
            border: 1px solid #888;
            width: 300px;
            border-radius: 4px;
        }
        .modal-content input[type="text"] {
            width: 100%;
            padding: 8px;
            margin: 10px 0;
            box-sizing: border-box;
            border: 1px solid #ddd;
            border-radius: 4px;
        }
        .modal-actions {
            display: flex;
            justify-content: flex-end;
            gap: 10px;
            margin-top: 15px;
        }
        .cancel-btn {
            background: #f44336;
        }
        .cancel-btn:hover {
            background: #d32f2f;
        }
        .curl-examples {
            background: #f5f5f5;
            padding: 15px;
            border-radius: 4px;
            margin-top: 20px;
            font-family: monospace;
            white-space: pre-wrap;
        }
        .spinner {
            border: 4px solid rgba(0, 0, 0, 0.1);
            width: 20px;
            height: 20px;
            border-radius: 50%;
            border-left-color: #09f;
            animation: spin 1s linear infinite;
            display: none;
            margin-left: 10px;
        }
        @keyframes spin {
            0% { transform: rotate(0deg); }
            100% { transform: rotate(360deg); }
        }
    </style>
</head>
<body>
    <h1>File Server</h1>
    
    <div class="container">
        <div class="path-nav">
            Current Path: <span id="pathDisplay">/</span>
            <button onclick="navigateToParent()">Go Up</button>
        </div>
        
        <div class="actions">
            <button onclick="document.getElementById('fileInput').click()">Upload File</button>
            <input type="file" id="fileInput" onchange="uploadFile()">
            <button onclick="openMkdirModal()">Create Directory</button>
            <div class="spinner" id="spinner"></div>
        </div>
        
        <div class="file-list" id="fileList">
            <!-- Files will be populated here -->
            <div class="file-item">Loading...</div>
        </div>
        
        <div id="mkdirModal" class="modal">
            <div class="modal-content">
                <h3>Create New Directory</h3>
                <input type="text" id="dirName" placeholder="Directory Name">
                <div class="modal-actions">
                    <button class="cancel-btn" onclick="closeMkdirModal()">Cancel</button>
                    <button onclick="createDirectory()">Create</button>
                </div>
            </div>
        </div>
        
        <h2>API Usage</h2>
        <div class="curl-examples">
# List files in root directory
curl http://localhost:8080/api/files

# List files in a specific directory
curl http://localhost:8080/api/files?path=/my-dir

# Upload a file to root directory
curl -X POST -F "file=@/path/to/local/file.txt" http://localhost:8080/api/upload

# Upload a file to a specific directory
curl -X POST -F "file=@/path/to/local/file.txt" -F "path=/my-dir" http://localhost:8080/api/upload

# Create a directory
curl -X POST -H "Content-Type: application/json" -d '{"path":"/", "name":"new-dir"}' http://localhost:8080/api/mkdir

# Download a file
curl -O http://localhost:8080/download/my-dir/file.txt
        </div>
    </div>
    
    <script>
        let currentPath = '/';
        
        // Load files when the page loads
        window.onload = function() {
            loadFiles(currentPath);
        };
        
        // Function to load files from the current path
        function loadFiles(path) {
            currentPath = path;
            document.getElementById('pathDisplay').textContent = currentPath;
            
            fetch('/api/files?path=' + encodeURIComponent(path))
                .then(function(response) { return response.json(); })
                .then(function(data) {
                                            if (data.success) {
                        const fileList = document.getElementById('fileList');
                        fileList.innerHTML = '';
                        
                        // Check if files array exists and handle if it's null or missing
                        if (!data.files || data.files.length === 0) {
                            fileList.innerHTML = '<div class="file-item">No files found</div>';
                            return;
                        }
                        
                        // Sort: directories first, then files
                        data.files.sort(function(a, b) {
                            if (a.is_dir && !b.is_dir) return -1;
                            if (!a.is_dir && b.is_dir) return 1;
                            return a.name.localeCompare(b.name);
                        });
                        
                        data.files.forEach(function(file) {
                            const fileItem = document.createElement('div');
                            fileItem.className = 'file-item';
                            
                            const isDir = file.is_dir;
                            const icon = document.createElement('div');
                            icon.className = 'icon';
                            icon.innerHTML = isDir ? '📁' : '📄';
                            
                            const name = document.createElement('div');
                            name.className = 'name';
                            
                            const link = document.createElement('a');
                            link.textContent = file.name;
                            
                            if (isDir) {
                                link.href = 'javascript:void(0)';
                                link.onclick = () => loadFiles(file.path);
                            } else {
                                link.href = '/download' + file.path;
                                link.setAttribute('download', '');
                            }
                            
                            name.appendChild(link);
                            
                            const meta = document.createElement('div');
                            meta.className = 'meta';
                            if (!isDir) {
                                meta.textContent = formatFileSize(file.size);
                            }
                            
                            fileItem.appendChild(icon);
                            fileItem.appendChild(name);
                            fileItem.appendChild(meta);
                            
                            fileList.appendChild(fileItem);
                        });
                    } else {
                        alert('Error loading files: ' + data.error);
                    }
                })
                .catch(function(error) {
                    console.error('Error:', error);
                    alert('Failed to load files. See console for details.');
                });
        }
        
        // Function to navigate to parent directory
        function navigateToParent() {
            if (currentPath === '/') return;
            
            const parts = currentPath.split('/').filter(Boolean);
            parts.pop();
            const parentPath = parts.length ? '/' + parts.join('/') : '/';
            loadFiles(parentPath);
        }
        
        // Function to upload a file
        function uploadFile() {
            const fileInput = document.getElementById('fileInput');
            if (!fileInput.files.length) return;
            
            const formData = new FormData();
            formData.append('file', fileInput.files[0]);
            formData.append('path', currentPath);
            
            // Show spinner
            document.getElementById('spinner').style.display = 'inline-block';
            
            fetch('/api/upload', {
                method: 'POST',
                body: formData
            })
            .then(response => response.json())
            .then(data => {
                // Hide spinner
                document.getElementById('spinner').style.display = 'none';
                
                if (data.success) {
                    alert(data.message);
                    loadFiles(currentPath); // Reload files
                } else {
                    alert('Error: ' + data.error);
                }
                
                // Reset file input
                fileInput.value = '';
            })
            .catch(error => {
                // Hide spinner
                document.getElementById('spinner').style.display = 'none';
                console.error('Error:', error);
                alert('Upload failed. See console for details.');
            });
        }
        
        // Modal functions
        function openMkdirModal() {
            document.getElementById('mkdirModal').style.display = 'block';
            document.getElementById('dirName').focus();
        }
        
        function closeMkdirModal() {
            document.getElementById('mkdirModal').style.display = 'none';
            document.getElementById('dirName').value = '';
        }
        
        // Function to create a directory
        function createDirectory() {
            const dirName = document.getElementById('dirName').value.trim();
            if (!dirName) {
                alert('Please enter a directory name');
                return;
            }
            
            fetch('/api/mkdir', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify({
                    path: currentPath,
                    name: dirName
                })
            })
            .then(response => response.json())
            .then(data => {
                if (data.success) {
                    closeMkdirModal();
                    loadFiles(currentPath); // Reload files
                } else {
                    alert('Error: ' + data.error);
                }
            })
            .catch(error => {
                console.error('Error:', error);
                alert('Failed to create directory. See console for details.');
            });
        }
        
        // Utility function to format file size
        function formatFileSize(bytes) {
            if (bytes === 0) return '0 Bytes';
            
            const k = 1024;
            const sizes = ['Bytes', 'KB', 'MB', 'GB', 'TB'];
            const i = Math.floor(Math.log(bytes) / Math.log(k));
            
            return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
        }
        
        // Handle Enter key in the directory name input
        document.getElementById('dirName').addEventListener('keyup', function(event) {
            if (event.key === 'Enter' || event.keyCode === 13) {
                createDirectory();
            }
        });
    </script>
</body>
</html>
`