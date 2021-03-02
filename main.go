package main

import (
	"bufio"
	"context"
	"fmt"
	"github.com/posener/complete"
	"io/fs"
	"net"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

func anyOf(strs ...string) complete.Predictor {
	return complete.PredictFunc(func(a complete.Args) []string {
		return strs
	})
}

func withArgs(p complete.Predictor) complete.Command {
	return complete.Command{
		Args: p,
	}
}

func computeArgs(f func(a complete.Args) []string) complete.Command {
	return withArgs(complete.PredictFunc(f))
}

func main() {
	//fmt.Printf("%q", getDevices(complete.Args{}))
	//os.Exit(0)
	//fmt.Printf("%+v\n", getHost(complete.Args{}))
	//println("Done")
	//os.Exit(0)
	c := complete.New("adb", complete.Command{
		Sub: complete.Commands{
			"disconnect": computeArgs(getDevices),
			"uninstall":  computeArgs(getPackages),
			"install":    computeArgs(getApks),
			"shell": {
				Sub: map[string]complete.Command{
					"pm": {
						Sub: map[string]complete.Command{
							"clear": computeArgs(getPackages),
						},
					},
				},
				Args: anyOf("am broadcast -a", "pm clear"),
			},
			"connect": computeArgs(getHost),
			"tcpip":   withArgs(anyOf("5555")),
		},
		Args: anyOf("uninstall", "tcpip", "install", "devices", "shell"),
		Flags: map[string]complete.Predictor{
			"-s": complete.PredictFunc(getDevices),
		},
	})
	c.Complete()
}

func getDevices(complete.Args) (out []string) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "adb", "devices")
	b, err := cmd.StdoutPipe()
	if err != nil {
		panic(err)
	}
	if err := cmd.Start(); err != nil {
		panic(err)
	}
	scanner := bufio.NewScanner(b)
	scanner.Scan()
	for scanner.Scan() {
		elems := strings.Split(scanner.Text(), "\t")[0]
		if elems != "" {
			out = append(out, elems)
		}
	}
	return
}

func getApks(complete.Args) (out []string) {
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".apk") {
			out = append(out, path)
		}
		return nil
	})
	if err != nil {
		panic(err)
	}
	return
}

func getPackages(complete.Args) (out []string) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "adb", "shell", "pm", "list", "packages")
	b, err := cmd.StdoutPipe()
	if err != nil {
		panic(err)
	}
	if err = cmd.Start(); err != nil {
		panic(err)
	}
	scanner := bufio.NewScanner(b)
	for scanner.Scan() {
		text := scanner.Text()
		if strings.HasPrefix(text, "package:") {
			out = append(out, strings.TrimPrefix(text, "package:"))
		}
	}
	return
}

func getHost(complete.Args) (out []string) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var wg sync.WaitGroup
	wg.Add(255 - 2)
	ch := make(chan string)
	for i := 2; i < 255; i++ {
		go func(i int) {
			doActualPortScan(ctx, i, ch)
			wg.Done()
		}(i)
	}
	go func() {
		wg.Wait()
		close(ch)
	}()
	for s := range ch {
		out = append(out, s)
	}
	return out
}

func doActualPortScan(ctx context.Context, i int, ch chan<- string) {
	result := make(chan string, 1)
	go func() {
		ipAddress := fmt.Sprintf("192.168.1.%d:5555", i)
		complete.Log("Dialing %s", ipAddress)
		conn, err := net.Dial("tcp", ipAddress)
		if err != nil {
			complete.Log("%d sket sig", i)
			close(result)
		} else {
			complete.Log("%d gick bra", i)
			err = conn.Close()
			result <- ipAddress
			if err != nil {
				complete.Log("close failed %s\n", err)
			}
		}
	}()
	select {
	case <-ctx.Done():
	case value, ok := <-result:
		if ok {
			ch <- value
		}
	}
}
