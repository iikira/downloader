package main

import (
	"bufio"
	"flag"
	"fmt"
	"github.com/iikira/iikira-go-utils/pcsverbose"
	"github.com/iikira/iikira-go-utils/requester"
	"github.com/iikira/iikira-go-utils/requester/downloader"
	"github.com/iikira/iikira-go-utils/requester/transfer"
	"github.com/iikira/iikira-go-utils/utils/converter"
	"github.com/olekukonko/tablewriter"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
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
)

func init() {
	flag.IntVar(&parallel, "p", 5, "download max parallel")
	flag.IntVar(&cacheSize, "c", 30000, "download cache size")
	flag.BoolVar(&pcsverbose.IsVerbose, "verbose", false, "verbose")
	flag.BoolVar(&test, "test", false, "test download")
	flag.BoolVar(&isPrintStatus, "status", false, "print status")
	flag.StringVar(&requester.UserAgent, "ua", "", "User-Agent")

	flag.Parse()
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
	if flag.NArg() == 0 {
		flag.Usage()
		if runtime.GOOS == "windows" {
			bufio.NewReader(os.Stdin).ReadByte()
		}

		return
	}

	for k := range flag.Args() {
		var (
			savePath string
			err      error
		)
		if !test {
			savePath, err = downloader.GetFileName(flag.Arg(k), nil)
			if err != nil {
				fmt.Println(err)
				return
			}
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
