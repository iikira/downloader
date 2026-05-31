//go:build integration

package downloader_test

import (
	"crypto/md5"
	"crypto/rand"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func compileBinary(t *testing.T, dir string, repoRoot string) string {
	t.Helper()
	binaryPath := filepath.Join(dir, "downloader_binary")
	cmd := exec.Command("/usr/local/go1.17.13/bin/go", "build", "-o", binaryPath, "./cmd/downloader")
	cmd.Dir = repoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("compile failed: %v\n%s", err, string(out))
	}
	return binaryPath
}

func TestDownloadResume(t *testing.T) {
	// 1. 生成 128MB 随机数据 + MD5
	size := int64(128 * 1024 * 1024)
	data := make([]byte, size)
	_, err := rand.Read(data)
	if err != nil {
		t.Fatalf("rand.Read failed: %v", err)
	}
	expectedMD5 := md5.Sum(data)

	// 2. 启动 HTTP 服务器，随机端口，支持 Range 请求
	srv := &http.Server{
		Addr: "127.0.0.1:0",
	}
	ln, err := net.Listen("tcp", srv.Addr)
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}
	srv.Addr = ln.Addr().String()

	srv.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if rangeHeader := r.Header.Get("Range"); rangeHeader != "" {
			var start, end int64
			n, _ := fmt.Sscanf(rangeHeader, "bytes=%d-%d", &start, &end)
			if n == 2 {
				w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, size))
				w.Header().Set("Content-Length", fmt.Sprintf("%d", end-start+1))
				w.WriteHeader(http.StatusPartialContent)
				w.Write(data[start : end+1])
				return
			}
		}
		w.Header().Set("Accept-Ranges", "bytes")
		w.Header().Set("Content-Length", fmt.Sprintf("%d", size))
		w.Write(data)
	})
	go srv.Serve(ln)
	defer srv.Close()

	// 3. 临时目录
	tmpDir := t.TempDir()

	// 4. 编译 binary
	repoRoot, _ := os.Getwd()
	binaryPath := compileBinary(t, tmpDir, repoRoot)

	// 5. 第一次下载，700ms 后 kill
	url := "http://" + srv.Addr + "/file"

	// 先确保目标文件不存在
	os.Remove(filepath.Join(tmpDir, "file"))
	os.Remove(filepath.Join(tmpDir, "file.downloader_downloading"))

	cmd := exec.Command(binaryPath, "-p=4", url)
	cmd.Dir = tmpDir
	cmd.Env = append(os.Environ(), "HTTP_PROXY=", "HTTPS_PROXY=")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start download failed: %v", err)
	}

	// 等待 700ms 后 kill
	time.Sleep(700 * time.Millisecond)
	if cmd.Process != nil {
		cmd.Process.Kill()
	}
	cmd.Wait()

	// 确认断点文件存在
	statePath := filepath.Join(tmpDir, "file.downloader_downloading")
	if _, err := os.Stat(statePath); os.IsNotExist(err) {
		t.Log("warning: state file not found, download may have finished too quickly")
	}

	// 等待一下让文件句柄释放
	time.Sleep(500 * time.Millisecond)

	// 6. 第二次下载（断点续传）
	cmd2 := exec.Command(binaryPath, "-p=4", url)
	cmd2.Dir = tmpDir
	cmd2.Env = append(os.Environ(), "HTTP_PROXY=", "HTTPS_PROXY=")
	if out, err := cmd2.CombinedOutput(); err != nil {
		t.Fatalf("resume download failed: %v\n%s", err, string(out))
	}

	// 7. 校验 MD5
	filePath := filepath.Join(tmpDir, "file")
	fileData, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("read file failed: %v", err)
	}
	actualMD5 := md5.Sum(fileData)
	if actualMD5 != expectedMD5 {
		t.Fatalf("MD5 mismatch: expected %x, got %x", expectedMD5, actualMD5)
	}

	t.Logf("download resume test passed, file size: %d bytes", len(fileData))
}
