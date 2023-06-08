package graceful

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
)

const (
	inheritedListenerKey  = "INHERITED_LISTENER"
	inheritdProcessEnvKey = "INHERITED_PROCESS"
)

var (
	inheritListenerOnce sync.Once
	inheritedFD         InheritedFD
)

// ListenTCP 监听tcp端口
func ListenTCP(network, addr string) (net.Listener, error) {
	return inheritedFD.ListenTCP(network, addr)
}

// IsInheritedProcess 是否继承的线程
func IsInherited() bool {
	return os.Getenv(inheritdProcessEnvKey) == "1"
}

// InhertedListener 从父进程继承的listener
type InheritedFD struct {
	lis []net.Listener
}

// inherit 获取从父进程继承的listener
func (l *InheritedFD) inheritListener() {
	// 从环境变量中获取fd列表
	env := os.Getenv(inheritedListenerKey)
	if len(env) == 0 {
		return
	}
	fdCount, err := strconv.Atoi(env)
	if err != nil {
		// fmt.Println("atoi", err.Error())
		return
	}
	// list := strings.Split(env, ",")
	// 从fd构建listener
	l.lis = make([]net.Listener, 0, fdCount)
	for i := 0; i < fdCount; i++ {
		// 解析文件描述符
		// fd, err := strconv.Atoi(list[i])
		// if err != nil {
		// 	fmt.Println("atoi", err.Error())
		// 	continue
		// }
		// 创建file对象
		file := os.NewFile(uintptr(i+3), "listener")
		if file == nil {
			fmt.Println("new_file", err.Error())
			continue
		}

		// 构建listener
		ln, err := net.FileListener(file)
		if err != nil {
			file.Close()
			fmt.Println("file_listener", err.Error())
			continue
		}
		fmt.Println(i+3, ln.Addr().String())
		l.lis = append(l.lis, ln)
	}
}

// ListenTCP 监听TCP端口
func (l *InheritedFD) ListenTCP(network string, addr string) (net.Listener, error) {
	// 解析地址
	laddr, err := net.ResolveTCPAddr(network, addr)
	if err != nil {
		return nil, err
	}
	// 检查继承的fd
	inheritListenerOnce.Do(l.inheritListener)
	// 从继承的fd里面查找，可用的连接
	var list = l.lis
	var ln net.Listener
	for i := range list {
		if list[i] != nil && isSameAddr(list[i].Addr(), laddr) {
			ln = list[i]
			list[i] = nil
			break
		}
	}
	// 如果没有可用的连接，即时创建
	if ln == nil {
		return net.ListenTCP(network, laddr)
	}

	return ln, nil
}

func isSameAddr(a1, a2 net.Addr) bool {
	if a1.Network() != a2.Network() {
		return false
	}
	a1s := a1.String()
	a2s := a2.String()
	if a1s == a2s {
		return true
	}

	// This allows for ipv6 vs ipv4 local addresses to compare as equal. This
	// scenario is common when listening on localhost.
	const ipv6prefix = "[::]"
	a1s = strings.TrimPrefix(a1s, ipv6prefix)
	a2s = strings.TrimPrefix(a2s, ipv6prefix)
	const ipv4prefix = "0.0.0.0"
	a1s = strings.TrimPrefix(a1s, ipv4prefix)
	a2s = strings.TrimPrefix(a2s, ipv4prefix)

	return a1s == a2s
}

// Close 关闭所有连接
func (l *InheritedFD) Close() {
	var lis = l.lis
	l.lis = nil
	for i := range lis {
		if lis[i] != nil {
			lis[i].Close()
			lis[i] = nil
		}
	}

}

// Process 进程参数
type Process struct {
	Name       string // 要加载的可执行文件
	Listener   []net.Listener
	InheritEnv bool     // 是否继承环境变量?
	Argv       []string // 命令行参数
}

// Filer 获取 os.File
type Filer interface {
	File() (f *os.File, err error)
}

// StartProcess 启动新的进程
// name  = 可执行文件的路径
// files = 新进程要继承的文件描述符(golang默认是close_on_exec，父进程的fd到了子进程会被关闭)
// args = 新进程启动时接收到的，命令行参数
func (p Process) Start() (int, error) {
	// 查找可执行文件是否存在
	file, err := exec.LookPath(p.Name)
	if err != nil {
		return -1, err
	}

	// 添加环境变量
	var envList []string
	// var listenerEnv = inheritedListenerKey + "="
	if p.InheritEnv {
		var envs = os.Environ()
		envList = make([]string, 0, len(envs)+2)
		for i := range envs {
			if !strings.HasPrefix(envs[i], "inherited") {
				envList = append(envList, envs[i])
			}
		}
	}
	envList = append(envList, inheritdProcessEnvKey+"=1")

	// 将lisnter 转为 os.File
	var files = make([]*os.File, 0, len(p.Listener)+4)
	for i := range p.Listener {
		filer, ok := p.Listener[i].(Filer)
		if !ok {
			continue
		}
		// 获取到文件描述符
		// FIXME: 这里异常是否需要抛出?
		file, err := filer.File()
		if err != nil {
			continue
		}
		files = append(files, file)
	}

	defer func() {
		for i := range files {
			files[i].Close()
		}
	}()

	// 收集文件描述符，并写入到环境变量
	if len(files) > 0 {
		// var n int
		// var list = make([]string, 0, len(files))
		// // 遍历所有文件，fd转string
		// for i := range files {
		// 	fd := strconv.Itoa(int(files[i].Fd()))
		// 	list = append(list, fd)
		// 	n += len(fd)
		// }

		// var sb strings.Builder
		// sb.Grow(n + len(listenerEnv))
		// sb.WriteString(listenerEnv)
		// sb.WriteString(list[0])
		// for _, s := range list[1:] {
		// 	sb.WriteString(",")
		// 	sb.WriteString(s)
		// }
		// 追加环境变量(inherited_listener=0,1,2)
		// envList = append(envList, sb.String())
		envList = append(envList, inheritedListenerKey+"="+strconv.Itoa(len(files)))
	}
	// 追加标准输入/输出
	files = append([]*os.File{os.Stdin, os.Stdout, os.Stderr}, files...)

	// 构建参数
	wd, _ := os.Getwd()
	attr := os.ProcAttr{
		Dir:   wd, // 当前执行路径
		Env:   envList,
		Files: files,
	}

	// 启动新进程
	proc, err := os.StartProcess(file, p.Argv, &attr)
	if err != nil {
		return -1, fmt.Errorf("start_process: %w", err)
	}

	return proc.Pid, nil
}
