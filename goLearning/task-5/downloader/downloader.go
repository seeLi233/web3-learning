package downloader

import (
	"fmt"
	"math/rand/v2"
	"sync"
	"time"
)

// ==================== 并发下载器 ====================

// DownloadTask 表示一个下载任务
type DownloadTask struct {
	URL      string
	FileName string
	Size     int64 // 文件大小（模拟用）
}

// DownloadResult 表示下载结果
type DownloadResult struct {
	Task     DownloadTask
	Duration time.Duration
	Err      error
}

// Downloader 并发下载器
type Downloader struct {
	concurrency int // 并发下载数
	tasks       chan DownloadTask
	results     chan DownloadResult
	wg          sync.WaitGroup
}

// NewDownloader 创建下载器
func NewDownloader(concurrency int) *Downloader {
	return &Downloader{
		concurrency: concurrency,
		tasks:       make(chan DownloadTask, concurrency*2),
		results:     make(chan DownloadResult, concurrency*2),
	}
}

// worker 下载工作者
func (d *Downloader) worker(id int) {
	defer d.wg.Done()

	for task := range d.tasks {
		start := time.Now()
		fmt.Printf("  [下载器 %d] 开始下载: %s (%d bytes)\n", id, task.FileName, task.Size)

		// 模拟下载（实际场景中这里是 HTTP GET）
		err := simulateDownload(task)
		duration := time.Since(start)

		result := DownloadResult{Task: task, Duration: duration, Err: err}
		d.results <- result

		if err != nil {
			fmt.Printf("  [下载器 %d] ❌ 下载失败: %s (%v)\n", id, task.FileName, err)
		} else {
			fmt.Printf("  [下载器 %d] ✅ 下载完成: %s (耗时 %v)\n", id, task.FileName, duration)
		}
	}
}

func simulateDownload(task DownloadTask) error {
	// 根据文件大小模拟下载时间
	delay := time.Duration(task.Size/1024) * time.Millisecond
	if delay > 2*time.Second {
		delay = 2 * time.Second
	}
	time.Sleep(delay)

	// 模拟 10% 概率下载失败
	if rand.IntN(10) == 0 {
		return fmt.Errorf("网络超时: %s", task.URL)
	}
	return nil
}

// Start 启动所有下载 worker
func (d *Downloader) Start() {
	for i := 0; i < d.concurrency; i++ {
		d.wg.Add(1)
		go d.worker(i)
	}
}

// Enqueue 入队一个下载任务
func (d *Downloader) Enqueue(task DownloadTask) {
	d.tasks <- task
}

// Close 关闭任务队列，等待所有 worker 完成后关闭结果队列
func (d *Downloader) Close() []DownloadResult {
	close(d.tasks) // 通知 worker 不再有新任务

	// 关键：并发 drain results，防止 worker 发送结果时阻塞
	var allResults []DownloadResult
	var mu sync.Mutex
	var drainWg sync.WaitGroup
	drainWg.Add(1)
	go func() {
		defer drainWg.Done()
		for r := range d.results {
			mu.Lock()
			allResults = append(allResults, r)
			mu.Unlock()
		}
	}()

	d.wg.Wait()      // 等待所有 worker 完成
	close(d.results) // 关闭 results → drain goroutine 退出
	drainWg.Wait()   // 等 drain 完成

	return allResults
}

// ==================== 生产者-消费者模式完整演示 ====================

func Demo() {
	fmt.Println("=== 并发下载器演示 ===")

	// 创建下载器（3 个并发 worker）
	downloader := NewDownloader(3)
	downloader.Start()

	// 任务列表（要下载的文件）
	tasks := []DownloadTask{
		{URL: "https://example.com/file1.zip", FileName: "file1.zip", Size: 500 * 1024},
		{URL: "https://example.com/file2.zip", FileName: "file2.zip", Size: 300 * 1024},
		{URL: "https://example.com/file3.zip", FileName: "file3.zip", Size: 1000 * 1024},
		{URL: "https://example.com/file4.zip", FileName: "file4.zip", Size: 200 * 1024},
		{URL: "https://example.com/file5.zip", FileName: "file5.zip", Size: 800 * 1024},
		{URL: "https://example.com/file6.zip", FileName: "file6.zip", Size: 150 * 1024},
		{URL: "https://example.com/file7.zip", FileName: "file7.zip", Size: 600 * 1024},
		{URL: "https://example.com/file8.zip", FileName: "file8.zip", Size: 100 * 1024},
	}

	// 生产者：逐个入队任务（直接从 main goroutine 发送，避免竞态）
	fmt.Println("开始入队下载任务...")
	for _, task := range tasks {
		downloader.Enqueue(task)
	}

	// 等待并收集结果
	results := downloader.Close()

	// 统计
	fmt.Println("\n=== 下载报告 ===")
	totalSize := int64(0)
	totalDuration := time.Duration(0)
	for _, r := range results {
		totalSize += r.Task.Size
		totalDuration += r.Duration
	}
	fmt.Printf("总文件数: %d\n", len(results))
	fmt.Printf("总大小: %.2f MB\n", float64(totalSize)/(1024*1024))
	fmt.Printf("并发数: 3\n")

	fmt.Printf("总耗时(并行): %.2f 秒\n", totalDuration.Seconds())
	fmt.Printf("平均每个文件: %.2f 秒\n", totalDuration.Seconds()/float64(len(results)))

}
