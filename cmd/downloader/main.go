package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/iikira/iikira-go-utils/pcsverbose"
	"github.com/iikira/iikira-go-utils/requester"
	"github.com/iikira/iikira-go-utils/requester/downloader"
	"github.com/iikira/iikira-go-utils/requester/transfer"
	"github.com/iikira/iikira-go-utils/utils/converter"
	"github.com/olekukonko/tablewriter"
)

const (
	//StrDownloadInitError 初始化下载发生错误
	StrDownloadInitError = "初始化下载发生错误"
)

var (
	parallel       int
	cacheSize      int
	test           bool
	isPrintStatus  bool
	downloadSuffix = ".downloader_downloading"
	Version        = "1.0.0-dev"
	showVersion    = false
)

func init() {
	flag.IntVar(&parallel, "p", 5, "download max parallel")
	flag.IntVar(&cacheSize, "c", 30000, "download cache size")
	flag.BoolVar(&pcsverbose.IsVerbose, "verbose", false, "verbose")
	flag.BoolVar(&test, "test", false, "test download")
	flag.BoolVar(&isPrintStatus, "status", false, "print status")
	flag.StringVar(&requester.UserAgent, "ua", "", "User-Agent")
	flag.BoolVar(&showVersion, "version", false, "print version")

	flag.Parse()
}

// getFileNameAndSize 通过 GET 请求获取文件名和远端文件大小
// 与 GetFileName 不同, 使用 GET 而非 HEAD, 并同时返回 Content-Length
func getFileNameAndSize(uri string) (filename string, remoteSize int64, err error) {
	resp, err := requester.NewHTTPClient().Req(http.MethodGet, uri, nil, nil)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close() // 不读Body, 马上关闭连接

	if resp.StatusCode >= 400 {
		return "", 0, fmt.Errorf("获取文件信息失败: %s", resp.Status)
	}

	remoteSize = resp.ContentLength

	filename = path.Base(uri)
	if _, params, parseErr := mime.ParseMediaType(resp.Header.Get("Content-Disposition")); parseErr == nil {
		if name, ueErr := url.QueryUnescape(params["filename"]); ueErr == nil && name != "" {
			filename = name
		}
	}
	return filename, remoteSize, nil
}

// diagnoseFile 仅诊断: 对比远端文件大小与本地已存在文件, 不影响下载行为
func diagnoseFile(id int, savePath string, remoteSize int64) {
	// 本地文件大小受 WriteAt 按偏移写入影响, 未完成的下载也可能已达到远端大小, 大小一致不代表下载完整
	if _, err := os.Stat(savePath + downloadSuffix); err == nil {
		fmt.Printf("[%d] 诊断: 存在断点续传信息文件: %s, 将尝试续传\n", id, savePath+downloadSuffix)
	}

	if remoteSize < 0 {
		fmt.Printf("[%d] 诊断: 远端未返回 Content-Length, 无法对比文件大小\n", id)
		return
	}

	info, err := os.Stat(savePath)
	if err != nil {
		fmt.Printf("[%d] 诊断: 本地文件不存在: %s (远端大小 %d)\n", id, savePath, remoteSize)
		return
	}

	verdict := "大小不一致"
	if !info.IsDir() && info.Size() == remoteSize {
		verdict = "大小一致"
	}
	fmt.Printf("[%d] 诊断: 本地大小 %d / 远端大小 %d, %s: %s\n", id, info.Size(), remoteSize, verdict, savePath)
}

func download(id int, downloadURL, savePath string, client *requester.HTTPClient, newCfg downloader.Config) error {
	var (
		file     *os.File
		writerAt io.WriterAt
		err      error
	)

	if !newCfg.IsTest {
		newCfg.InstanceStatePath = savePath + downloadSuffix

		// 创建下载的目录
		dir := filepath.Dir(savePath)
		fileInfo, err := os.Stat(dir)
		if err != nil {
			err = os.MkdirAll(dir, 0777)
			if err != nil {
				return err
			}
		} else if !fileInfo.IsDir() {
			return fmt.Errorf("%s, path %s: not a directory", StrDownloadInitError, dir)
		}

		file, err = os.OpenFile(savePath, os.O_CREATE|os.O_WRONLY, 0666)
		if file != nil {
			defer file.Close()
		}
		if err != nil {
			return fmt.Errorf("%s, %s", StrDownloadInitError, err)
		}

		// 空指针和空接口不等价
		if file != nil {
			writerAt = file
		}
	}

	download := downloader.NewDownloader(downloadURL, writerAt, &newCfg)
	download.SetClient(client)

	download.OnDownloadStatusEvent(func(status transfer.DownloadStatuser, workersCallback func(downloader.RangeWorkerFunc)) {
		if isPrintStatus {
			// 输出所有的worker状态
			var (
				builder = &strings.Builder{}
				tb      = tablewriter.NewWriter(builder)
			)
			tb.SetAutoWrapText(false)
			tb.SetBorder(false)
			tb.SetHeaderLine(false)
			tb.SetColumnSeparator("")
			tb.SetHeader([]string{"#", "status", "range", "left", "speeds", "error"})
			workersCallback(func(key int, worker *downloader.Worker) bool {
				wrange := worker.GetRange()
				tb.Append([]string{fmt.Sprint(worker.ID()), worker.GetStatus().StatusText(), wrange.ShowDetails(), strconv.FormatInt(wrange.Len(), 10), strconv.FormatInt(worker.GetSpeedsPerSecond(), 10), fmt.Sprint(worker.Err())})
				return true
			})
			tb.Render()
			fmt.Printf("\n\n" + builder.String())
		}

		var leftStr string
		left := status.TimeLeft()
		if left < 0 {
			leftStr = "-"
		} else {
			leftStr = left.String()
		}

		fmt.Printf("\r[%d] ↓ %s/%s %s/s in %s, left %s ............", id,
			converter.ConvertFileSize(status.Downloaded(), 2),
			converter.ConvertFileSize(status.TotalSize(), 2),
			converter.ConvertFileSize(status.SpeedsPerSecond(), 2),
			status.TimeElapsed()/1e7*1e7, leftStr,
		)
	})
	download.OnExecute(func() {
		if newCfg.IsTest {
			fmt.Printf("[%d] 测试下载开始\n\n", id)
		}
	})

	err = download.Execute()

	fmt.Printf("\n")
	if err != nil {
		// 下载失败, 删去空文件
		if info, infoErr := file.Stat(); infoErr == nil {
			if info.Size() == 0 {
				pcsverbose.Verbosef("[%d] remove empty file: %s\n", id, savePath)
				os.Remove(savePath)
			}
		}
		return err
	}

	if !newCfg.IsTest {
		fmt.Printf("[%d] 下载完成, 保存位置: %s\n", id, savePath)
	} else {
		fmt.Printf("[%d] 测试下载结束\n", id)
	}

	return nil
}

func main() {
	if showVersion {
		fmt.Println(Version)
		return
	}
	if flag.NArg() == 0 {
		flag.Usage()
		if runtime.GOOS == "windows" {
			bufio.NewReader(os.Stdin).ReadByte()
		}

		return
	}

	for k := range flag.Args() {
		var (
			savePath   string
			remoteSize int64
			err        error
		)
		if !test {
			savePath, remoteSize, err = getFileNameAndSize(flag.Arg(k))
			if err != nil {
				fmt.Println(err)
				return
			}
			diagnoseFile(k, savePath, remoteSize)
		}
		err = download(k, flag.Arg(k), savePath, nil, downloader.Config{
			MaxParallel:       parallel,
			CacheSize:         cacheSize,
			InstanceStatePath: savePath + downloadSuffix,
			IsTest:            test,
		})
		if err != nil {
			fmt.Printf("err: %s\n", err)
		}
	}
	fmt.Println()
}
