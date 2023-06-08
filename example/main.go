package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"

	"github.com/graceful"
)

var (
	ppid = os.Getppid()
	pid  = os.Getpid()
)

const execName = "./graceful"

func main() {
	// 监听端口
	ln, err := graceful.ListenTCP("tcp", ":8080")
	if err != nil {
		fmt.Println("listenTCP", pid, err.Error())
		return
	}

	s := http.Server{
		Handler: http.HandlerFunc(hello),
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		fmt.Println("serving", ln.Addr().String(), pid)
		if err := s.Serve(ln); err != nil {
			fmt.Println("serve", err.Error())
		}
	}()

	proc := graceful.Process{
		Name: execName,
		// InheritEnv: true,
	}
	proc.Listener = append(proc.Listener, ln)
	Wait(proc, &s)

	wg.Wait()
	fmt.Println("process terminate", os.Getpid())
}

func hello(w http.ResponseWriter, rq *http.Request) {
	message := "hello" + strconv.Itoa(pid)
	w.Write([]byte(message))
}

func Wait(proc graceful.Process, s *http.Server) {
	if graceful.IsInherited() && ppid > 1 { // 子进程启动成功之后，发信号给父进程
		if err := syscall.Kill(ppid, syscall.SIGTERM); err != nil {
			fmt.Println("kill_parent", pid, ppid)
			return
		}
	}

	var ch = make(chan os.Signal, 10)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM, syscall.SIGUSR2)
	for sig := range ch {
		switch sig {
		case syscall.SIGINT, syscall.SIGTERM:
			if err := s.Shutdown(context.Background()); err != nil {
				fmt.Println("shutdown", pid, err.Error())
			}
			return
		case syscall.SIGUSR2:
			// NOTE: 启动成功子线程之后，不直接退出，而是让子进程发信号通知
			pid, err := proc.Start()
			if err != nil {
				fmt.Println("start_process", err.Error())
			} else {
				fmt.Println("child_process", pid)
			}
		}
	}
}
