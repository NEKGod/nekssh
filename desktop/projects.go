package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"regexp"
	"strings"

	"github.com/pkg/sftp"
)

type ProjectResult struct {
	Path, Template string
	Files          []string
}

type projectFile struct{ name, content string }

var projectNamePattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]{0,63}$`)

func projectFiles(name, template string) ([]projectFile, error) {
	if !projectNamePattern.MatchString(name) {
		return nil, errors.New("项目名称须以字母开头，使用 1–64 位字母、数字、短横线或下划线")
	}
	var files []projectFile
	switch template {
	case "empty":
		return files, nil
	case "go":
		files = []projectFile{{"go.mod", "module example.com/" + strings.ToLower(name) + "\n\ngo 1.20\n"}, {"main.go", `package main

import (
    "fmt"
    "log"
    "net/http"
    "os"
)

func main() {
    port := os.Getenv("PORT")
    if port == "" { port = "3000" }
    http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        fmt.Fprintln(w, "Hello from NekSSH!")
    })
    log.Printf("http://127.0.0.1:%s", port)
    log.Fatal(http.ListenAndServe("127.0.0.1:"+port, nil))
}
`}, {".gitignore", "/" + name + "\n*.exe\n.env\n"}, {"README.md", "# " + name + "\n\nRun: `go run .`\n\nOpen http://127.0.0.1:3000 (PORT can override the port).\n"}}
	case "node":
		files = []projectFile{{"package.json", "{\n  \"name\": \"" + strings.ToLower(name) + "\",\n  \"version\": \"0.1.0\",\n  \"private\": true,\n  \"type\": \"module\",\n  \"scripts\": {\"start\": \"node server.mjs\"}\n}\n"}, {"server.mjs", `import {createServer} from 'node:http';
const port = Number(process.env.PORT || 3000);
createServer((request, response) => {
  response.writeHead(200, {'Content-Type': 'text/plain; charset=utf-8'});
  response.end('Hello from NekSSH!\n');
}).listen(port, '127.0.0.1', () => console.log('http://127.0.0.1:' + port));
`}, {".gitignore", "node_modules/\n.env\n*.log\n"}, {"README.md", "# " + name + "\n\nRun: `npm start` (no dependency installation needed).\n\nOpen http://127.0.0.1:3000 (PORT can override the port).\n"}}
	case "python":
		files = []projectFile{{"main.py", `from http.server import BaseHTTPRequestHandler, HTTPServer
import os

class Handler(BaseHTTPRequestHandler):
    def do_GET(self):
        self.send_response(200)
        self.send_header("Content-Type", "text/plain; charset=utf-8")
        self.end_headers()
        self.wfile.write(b"Hello from NekSSH!\n")

if __name__ == "__main__":
    port = int(os.environ.get("PORT", "3000"))
    print(f"http://127.0.0.1:{port}", flush=True)
    HTTPServer(("127.0.0.1", port), Handler).serve_forever()
`}, {".gitignore", ".venv/\n__pycache__/\n*.pyc\n.env\n"}, {"README.md", "# " + name + "\n\nRun: `python3 main.py` (Python standard library only).\n\nOpen http://127.0.0.1:3000 (PORT can override the port).\n"}}
	default:
		return nil, errors.New("不支持的项目模板")
	}
	return files, nil
}

func (a *App) projectSFTP(id string) (*sftp.Client, error) {
	session, err := a.getSession(id)
	if err != nil {
		return nil, err
	}
	session.sftpMu.Lock()
	defer session.sftpMu.Unlock()
	if !session.sftpReady {
		return nil, errors.New("SFTP 正在启动，请稍后重试")
	}
	if session.sftp == nil {
		return nil, errors.New("目标服务器不支持 SFTP")
	}
	return session.sftp, nil
}

type projectFS interface {
	RealPath(string) (string, error)
	Mkdir(string) error
	WriteNewFile(string, string) error
}
type projectSFTPFS struct{ *sftp.Client }

func (fs projectSFTPFS) WriteNewFile(name, content string) error {
	file, err := fs.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL)
	if err != nil {
		return err
	}
	_, writeErr := io.WriteString(file, content)
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}

func createProject(fs projectFS, parent, name, template string) (ProjectResult, error) {
	files, err := projectFiles(name, template)
	if err != nil {
		return ProjectResult{}, err
	}
	if strings.TrimSpace(parent) == "" {
		parent = "."
	}
	resolved, err := fs.RealPath(parent)
	if err != nil {
		return ProjectResult{}, err
	}
	root := path.Join(resolved, name)
	// Mkdir must fail for an existing project. Never populate/overwrite it.
	if err = fs.Mkdir(root); err != nil {
		return ProjectResult{}, fmt.Errorf("无法创建 %s（同名目录不会覆盖）: %w", root, err)
	}
	result := ProjectResult{Path: root, Template: template, Files: []string{}}
	for _, file := range files {
		if err = fs.WriteNewFile(path.Join(root, file.name), file.content); err != nil {
			return result, fmt.Errorf("写入 %s 失败；已创建的项目保留在 %s，请检查后处理，不要直接重复创建: %w", file.name, root, err)
		}
		result.Files = append(result.Files, file.name)
	}
	return result, nil
}

func (a *App) CreateProject(id, parent, name, template string) (ProjectResult, error) {
	client, err := a.projectSFTP(id)
	if err != nil {
		return ProjectResult{}, err
	}
	return createProject(projectSFTPFS{client}, parent, name, template)
}
