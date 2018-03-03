package downloader

import (
	"os"
	"sync"
	"time"
)

var (
	mu sync.Mutex
)

// blockMonitor 延迟监控各线程状态,
// 重设长时间无响应, 和下载速度为 0 的线程
func (der *Downloader) blockMonitor() <-chan struct{} {
	c := make(chan struct{})
	go func() {
		for {
			// 下载完毕, 线程全部完成下载任务, 发送结束信号
			if der.status.BlockList.isAllDone() {
				c <- struct{}{}

				if !der.config.Testing {
					os.Remove(der.config.SavePath + DownloadingFileSuffix) // 删除断点信息
				}

				return
			}

			if !der.config.Testing {
				der.recordBreakPoint()
			}

			// 速度减慢, 开启监控
			if der.status.Speeds < der.status.MaxSpeeds/10 {
				der.status.MaxSpeeds = 0
				for k := range der.status.BlockList {
					go func(k int) {
						// 过滤已完成下载任务的线程
						if der.status.BlockList[k].isDone() {
							return
						}

						// 重设长时间无响应, 和下载速度为 0 的线程
						go func(k int) {

							if der.status.Speeds != 0 {
								// 设 old 速度监测点, 2 秒后检查速度有无变化
								old := der.status.BlockList[k].Begin
								time.Sleep(2 * time.Second)
								// 过滤 速度有变化, 或 2 秒内完成了下载任务 的线程, 不过滤正在等待写入磁盘的线程
								if der.status.BlockList[k].waitingToWrite || old != der.status.BlockList[k].Begin || der.status.BlockList[k].isDone() {
									return
								}
							}

							mu.Lock() // 加锁, 防止出现重复添加线程的状况 (实验阶段)

							// 重设连接
							if r := der.status.BlockList[k].resp; r != nil {
								r.Body.Close()
							}

							mu.Unlock() // 解锁
						}(k)

					}(k)
				}
			}
			time.Sleep(1 * time.Second) // 监测频率 1 秒
		}
	}()
	return c
}
