package main

import (
	"bufio"
	"flag"
	"fmt"
	"github.com/iikira/BaiduPCS-Go/pcsverbose"
	"github.com/iikira/BaiduPCS-Go/requester"
	"github.com/iikira/BaiduPCS-Go/requester/downloader"
	"os"
	"runtime"
)

var (
	parallel       int
	cacheSize      int
	test           bool
	downloadSuffix = ".downloader_downloading"
)

func init() {
	flag.IntVar(&parallel, "p", 5, "download max parallel")
	flag.IntVar(&cacheSize, "c", 30000, "download cache size")
	flag.BoolVar(&pcsverbose.IsVerbose, "verbose", false, "verbose")
	flag.BoolVar(&test, "test", false, "test download")
	flag.StringVar(&requester.UserAgent, "ua", "", "User-Agent")

	flag.Parse()
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
		downloader.DoDownload(flag.Arg(k), savePath, &downloader.Config{
			MaxParallel:       parallel,
			CacheSize:         cacheSize,
			InstanceStatePath: savePath + downloadSuffix,
			IsTest:            test,
		})
	}
	fmt.Println()
}
