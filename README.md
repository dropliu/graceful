# graceful
A library implemented by Go for graceful shutdown or restart of applications。使用Go实现的用于优雅关闭或重启应用程序的库


## 用法

1. 编译并执行example/main.go，下面输出8080端口和pid=27396

    go build ./example -o example && ./example

    serving [::]:8080 27396

2. 请求8080端口，同样会返回当前进程的pid 

    curl http://localhost:8080

    hello27396

3. 打开另外一个窗口，释放信号USR2，手动触发应用重启

    kill -USR2 27396

    serving [::]:8080 27896

4. 输出的27896是子进程PID，父进程已经自动结束，使用ps和curl验证子进程
    ps -aux | grep graceful

    root  27896  0.0  0.0 1224840 7096 pts/11   Sl   21:37   0:00 [graceful]

    curl localhost:8080

    hello27896

5. 使用term信号关闭子进程

    kill -term 27896

    process terminate 27896

## API

1. graceful.ListenTCP，先通过环境变量查找继承的Listener，没有则调用net.ListenTCP

2. graceful.Process.Start，调用系统调用syscall.StartProcess创建一个新进程

3. graceful.IsInherited 可以检查是否是子进程。父进程创建子进程之后，并不会自动关闭，业务开发者应当按照自己业务的特点进行定制