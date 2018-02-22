package main

import (
	"flag"
	"fmt"
	"github.com/iikira/downloader"
)

var (
	parallel int
	testing  bool
)

func init() {
	flag.IntVar(&parallel, "p", 5, "download max parallel")
	flag.BoolVar(&testing, "t", false, "test mode")
	flag.Parse()
}

func main() {
	if flag.NArg() == 0 {
		flag.Usage()
		return
	}

	for k := range flag.Args() {
		downloader.DoDownload(flag.Arg(k), &downloader.Config{
			Parallel: parallel,
			Testing:  testing,
		})
	}
	fmt.Println()
}
