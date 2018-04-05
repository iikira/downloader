package main

import (
	"bufio"
	"flag"
	"fmt"
	"github.com/iikira/BaiduPCS-Go/downloader"
	"github.com/iikira/BaiduPCS-Go/requester"
	"os"
	"runtime"
)

var (
	parallel  int
	cacheSize int
	testing   bool
	verbose	  bool
	ua        string
)

func init() {
	flag.IntVar(&parallel, "p", 5, "download max parallel")
	flag.BoolVar(&testing, "t", false, "test mode")
	flag.BoolVar(&verbose, "verbose", false, "verbose")
	flag.StringVar(&ua, "ua", "", "user-agent")
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

	client := requester.NewHTTPClient()
	client.UserAgent = ua
	for k := range flag.Args() {
		downloader.DoDownload(flag.Arg(k), downloader.Config{
			Client:   client,
			Parallel: parallel,
			Testing:  testing,
		})
	}
	fmt.Println()
}
