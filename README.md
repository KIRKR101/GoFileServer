# Go File Server

A simple, self-contained file server written in Go. It provides a web interface for browsing, uploading, and creating directories, as well as a JSON API for programmatic access.

## Features

*   **Web Interface:** Easy-to-use UI for managing files.
    *   Browse files and directories.
    *   Navigate up and down directory structures.
    *   Upload files to the current directory.
    *   Create new directories.
    *   Download files.
*   **JSON API:** Programmatic access to all server functionalities.
*   **File Storage:** Serves files from a local `uploads` directory (created automatically or defined as separate location).
*   **Lightweight:** Single binary, no external dependencies needed at runtime besides the Go standard library.

## Prerequisites

*   [Go](https://golang.org/dl/)

## Getting Started

1.  **Clone the repository (or download `main.go`):**
    ```bash
    git clone https://github.com/KIRKR101/GoFileServer.git
    cd GoFileServer
    ```
    Or, simply save the provided Go code as `main.go`.

2.  **Run the server:**
    ```bash
    go run main.go
    ```
    This will compile and run the server. You should see output similar to:
    ```
    Server starting on port 8080...
    Web interface: http://localhost:8080
    Upload directory: ./uploads
    ```

3.  **Access the server:**
    *   **Web Interface:** Open your browser and navigate to `http://localhost:8080`.
    *   **API:** Use `curl` or any HTTP client to interact with the API endpoints (see below).

The `uploads` directory will be created in the same location as the `main.go` file if it doesn't already exist.

## Configuration

The server's configuration is defined by constants at the top of `main.go`:

*   `uploadPath`: The base directory for all uploaded files. Default: `./uploads`
*   `port`: The port on which the server listens. Default: `8080`

To change these, modify the constants in `main.go` and re-run the server.

## Web Interface

The web interface provides a user-friendly way to interact with the file server.

*   **Navigation:** Click on directory names to enter them. Use the "Go Up" button to navigate to the parent directory.
*   **Upload:** Click "Upload File", select a file, and it will be uploaded to the current directory.
*   **Create Directory:** Click "Create Directory", enter a name, and a new directory will be created in the current path.
*   **Download:** Click on a file name to download it.

## API Endpoints

All API responses are in JSON format. Successful operations typically return a `success: true` field, while errors return `success: false` and an `error` message.

The base URL for the API is `http://localhost:8080/api`.

---

### 1. List Files and Directories

*   **Endpoint:** `GET /api/files`
*   **Description:** Lists files and directories within a specified path.
*   **Query Parameters:**
    *   `path` (string, optional): The directory path to list. Defaults to `/` (root of the `uploads` directory).
*   **Example `curl`:**
    ```bash
    # List files in the root directory
    curl "http://localhost:8080/api/files"

    # List files in a specific directory (e.g., /my-folder)
    curl "http://localhost:8080/api/files?path=/my-folder"
    ```
*   **Example Success Response:**
    ```json
    {
        "success": true,
        "path": "/my-folder",
        "files": [
            {
                "name": "another-subfolder",
                "path": "/my-folder/another-subfolder",
                "is_dir": true
            },
            {
                "name": "document.txt",
                "path": "/my-folder/document.txt",
                "is_dir": false,
                "size": 1024,
                "updated_at": "2023-10-27 10:30:00"
            }
        ]
    }
    ```
    If the directory is empty or doesn't exist, `files` will be an empty array.

---

### 2. Upload a File

*   **Endpoint:** `POST /api/upload`
*   **Description:** Uploads a file to a specified path.
*   **Request Type:** `multipart/form-data`
*   **Form Fields:**
    *   `file` (file): The file to upload.
    *   `path` (string, optional): The directory path where the file should be uploaded. Defaults to `/`. If the directory doesn't exist, it will be created.
*   **Example `curl`:**
    ```bash
    # Upload 'localfile.txt' to the root directory
    curl -X POST -F "file=@/path/to/your/localfile.txt" http://localhost:8080/api/upload

    # Upload 'image.jpg' to '/pictures' directory
    curl -X POST -F "file=@/path/to/your/image.jpg" -F "path=/pictures" http://localhost:8080/api/upload
    ```
*   **Example Success Response:**
    ```json
    {
        "success": true,
        "message": "File uploaded successfully to /pictures/image.jpg"
    }
    ```

---

### 3. Create a Directory

*   **Endpoint:** `POST /api/mkdir`
*   **Description:** Creates a new directory at the specified path.
*   **Request Type:** `application/json`
*   **JSON Payload:**
    *   `path` (string): The parent directory path where the new directory will be created.
    *   `name` (string): The name of the new directory to create.
*   **Example `curl`:**
    ```bash
    # Create 'new-documents' directory in the root
    curl -X POST -H "Content-Type: application/json" \
         -d '{"path":"/", "name":"new-documents"}' \
         http://localhost:8080/api/mkdir

    # Create 'reports' subdirectory inside '/projects'
    curl -X POST -H "Content-Type: application/json" \
         -d '{"path":"/projects", "name":"reports"}' \
         http://localhost:8080/api/mkdir
    ```
*   **Example Success Response:**
    ```json
    {
        "success": true,
        "message": "Directory 'reports' created successfully"
    }
    ```

---

### 4. Download a File

*   **Endpoint:** `GET /download/<file_path>`
*   **Description:** Downloads a specific file. If the path points to a directory, it redirects to the web interface showing that directory's content.
*   **Path Parameter:**
    *   `<file_path>`: The full path to the file within the `uploads` directory (e.g., `my-folder/document.txt`).
*   **Example `curl`:**
    ```bash
    # Download 'document.txt' from the root of uploads directory
    curl -O http://localhost:8080/download/document.txt

    # Download 'report.pdf' from '/projects/data' directory
    # (-O saves with the original filename)
    curl -O http://localhost:8080/download/projects/data/report.pdf

    # Download and save as 'local_report.pdf'
    curl http://localhost:8080/download/projects/data/report.pdf -o local_report.pdf
    ```
*   **Response:**
    *   If the file exists, the server responds with the file content and appropriate `Content-Type` and `Content-Disposition` headers.
    *   If the path is a directory, it redirects to `/?path=<directory_path>`.
    *   If the file is not found, it returns a `404 Not Found`.
    *   If the path is invalid, it returns a `400 Bad Request`.

---

## Error Responses

If an API request fails, the server will respond with an appropriate HTTP status code (e.g., 400, 405, 500) and a JSON body like this:

```json
{
    "success": false,
    "error": "Descriptive error message here"
}
```
Example: Invalid path for `/api/files`:
```json
{
    "success": false,
    "error": "Invalid path"
}
```