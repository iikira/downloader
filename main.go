package main

import (
	"bufio"
	"flag"
	"fmt"
	"github.com/iikira/BaiduPCS-Go/pcsverbose"
	"github.com/iikira/BaiduPCS-Go/requester/downloader"
	"os"
	"path/filepath"
	"runtime"
)

var (
	parallel       int
	cacheSize      int
	ua             string
	downloadSuffix = ".downloader_downloading"
)

func init() {
	flag.IntVar(&parallel, "p", 5, "download max parallel")
	flag.BoolVar(&pcsverbose.IsVerbose, "verbose", false, "verbose")
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
		downloader.DoDownload(flag.Arg(k), filepath.Base(flag.Arg(k)), &downloader.Config{
			MaxParallel:       parallel,
			CacheSize:         30000,
			InstanceStatePath: filepath.Base(flag.Arg(k)) + downloadSuffix,
		})
	}
	fmt.Println()
}
