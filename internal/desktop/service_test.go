package desktop

import (
	"sync"
	"testing"
	"time"
)

func TestDesktopService_StartDownload_Concurrency(t *testing.T) {
	// 极简并发测试，确保同时请求不会创建两个活动任务
	s, err := NewDesktopService()
	if err != nil {
		t.Fatalf("初始化失败: %v", err)
	}

	// 模拟第一次启动任务，应当成功
	err1 := s.StartDownload(DownloadParams{URL: "invalid", Mode: "video", VideoID: 80, AudioID: -1})
	if err1 != nil {
		t.Fatalf("第一次启动任务失败: %v", err1)
	}

	// 立即启动第二次，应该因为有正在运行的任务而失败
	var err2 error
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		err2 = s.StartDownload(DownloadParams{URL: "invalid", Mode: "video", VideoID: 80, AudioID: -1})
	}()

	wg.Wait()
	if err2 == nil {
		t.Fatalf("第二次启动任务应该失败，但成功了，存在并发漏洞")
	}

	// 取消第一次任务
	s.CancelDownload()

	// 等待第一次任务彻底结束 (doDownload 返回后清理状态)
	time.Sleep(100 * time.Millisecond)

	// 第三次启动任务，应当成功
	err3 := s.StartDownload(DownloadParams{URL: "invalid", Mode: "video", VideoID: 80, AudioID: -1})
	if err3 != nil {
		t.Fatalf("第三次启动任务失败 (之前的任务取消后状态未正确清理): %v", err3)
	}
	s.CancelDownload()
}
